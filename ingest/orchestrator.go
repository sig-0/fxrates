package ingest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/rs/xid"
	"github.com/sig-0/iq"

	"github.com/sig-0/fxrates/storage"
)

var (
	errInvalidProvider = errors.New("invalid provider")
	errInvalidInterval = errors.New("invalid interval")
)

// Orchestrator is the main job scheduler for registered providers
type Orchestrator struct {
	storage storage.Storage
	logger  *slog.Logger

	registeredProviders sync.Map

	q             iq.Queue[scheduledIngest]
	queryInterval time.Duration
	qMux          sync.Mutex
}

// New creates a new Orchestrator instance
func New(storage storage.Storage, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		storage:       storage,
		q:             iq.NewQueue[scheduledIngest](),
		queryInterval: time.Second, // every second
	}

	// Apply the options
	for _, opt := range opts {
		opt(o)
	}

	return o
}

// Register registers a new provider with the orchestrator.
// The provider is immediately queued up for execution
func (o *Orchestrator) Register(p Provider) error {
	if p == nil || p.Name() == "" {
		return errInvalidProvider
	}

	if p.Interval() <= 0 {
		return errInvalidInterval
	}

	// Register the provider
	id := xid.New()
	o.registeredProviders.Store(id, p)

	o.logger.Info(
		"registered new provider",
		"name", p.Name(),
	)

	// Schedule the job
	o.scheduleIngest(
		time.Now().UTC(),
		id,
		p,
	)

	return nil
}

// Start starts the provider orchestration service loop [BLOCKING]
func (o *Orchestrator) Start(ctx context.Context) error {
	collectorCh := make(chan *workerResponse, 100) // TODO make the size configurable

	// Start a listener for monitoring jobs
	ticker := time.NewTicker(o.queryInterval)
	defer ticker.Stop()

	// handleIngest initializes all jobs that are executable (due)
	handleIngest := func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				nextSI := o.nextIngest()
				if nextSI == nil {
					return // nothing to schedule anymore
				}

				o.logger.Info(
					"scheduling ingest",
					"name", nextSI.provider.Name(),
				)

				// Spawn worker
				info := &workerInfo{
					provider:   nextSI.provider,
					providerID: nextSI.providerID,
					resCh:      collectorCh,
				}

				go handleJob(ctx, info)
			}
		}
	}

	// Initialize the first set of due jobs (on boot)
	handleIngest()

	for {
		select {
		case <-ctx.Done():
			o.logger.Info("orchestrator service shut down")
			close(collectorCh)

			return nil
		case <-ticker.C:
			handleIngest()
		case response := <-collectorCh:
			now := time.Now().UTC()

			rpRaw, ok := o.registeredProviders.Load(response.providerID)

			if !ok {
				o.logger.Error(
					"unable to load registered provider",
					"id", response.providerID.String(),
				)

				continue
			}

			rp, _ := rpRaw.(Provider)

			// Save the observed exchange rate
			if response.error != nil {
				o.logger.Error(
					"error encountered during rate fetch",
					"id", response.providerID.String(),
					"err", response.error.Error(),
				)

				// Retry ingest job soon
				o.scheduleIngest(
					now.Add(time.Second*10), // TODO retry exponentially?
					response.providerID,
					rp,
				)

				continue
			}

			// Save the provider-fetched rates
			for _, rate := range response.rates {
				// TODO overkill?
				saveCtx, cancelFn := context.WithTimeout(ctx, time.Second*10)

				if err := o.storage.SaveExchangeRate(saveCtx, rate); err != nil {
					o.logger.Error(
						"unable to save exchange rate",
						"base", rate.Base,
						"target", rate.Target,
						"source", rate.Source,
						"err", err,
					)
				}

				o.logger.Info(
					"saved exchange rate",
					"base", rate.Base,
					"target", rate.Target,
					"source", rate.Source,
					"rate", rate.Rate,
					"rate_type", rate.RateType,
					"effective_date", rate.AsOf.String(),
				)

				cancelFn()
			}

			// Schedule a new ingest for this provider
			o.scheduleIngest(
				now.Add(rp.Interval()),
				response.providerID,
				rp,
			)
		}
	}
}

// scheduleIngest schedules a new provider ingest
func (o *Orchestrator) scheduleIngest(
	at time.Time,
	providerID xid.ID,
	provider Provider,
) {
	o.qMux.Lock()
	defer o.qMux.Unlock()

	futureSI := scheduledIngest{
		at:         at,
		providerID: providerID,
		provider:   provider,
	}

	o.q.Push(futureSI)
}

// nextIngest fetches the next due ingest job, as of the moment of calling
func (o *Orchestrator) nextIngest() *scheduledIngest {
	o.qMux.Lock()
	defer o.qMux.Unlock()

	now := time.Now().UTC()

	// Check if anything needs to be scheduled
	if o.q.Len() == 0 {
		return nil // nothing to schedule, all jobs are running
	}

	// Check if the top element is due
	if o.q.Index(0).at.After(now) {
		return nil // nothing to schedule, latest job is in the future
	}

	// Grab the next job
	nextSI := o.q.PopFront()

	return nextSI
}
