package ingest

import (
	"context"
	"time"

	"github.com/rs/xid"

	"github.com/sig-0/fxrates/storage/types"
)

// scheduledIngest is a single scheduled Provider ingest job
type scheduledIngest struct {
	at         time.Time
	provider   Provider
	providerID xid.ID
}

// Less is utilized to sort scheduled ingests by their due-time (latest == first)
func (a scheduledIngest) Less(b scheduledIngest) bool {
	return a.at.Before(b.at)
}

// workerInfo is the work context for the provider routine
type workerInfo struct {
	provider   Provider
	resCh      chan<- *workerResponse
	providerID xid.ID
}

// workerResponse is the provider routine response
type workerResponse struct {
	error      error                 // encountered error, if any
	rates      []*types.ExchangeRate // the fetched exchange rates
	providerID xid.ID                // the provider ID
}

// handleJob fetches using the provider
func handleJob(
	ctx context.Context,
	info *workerInfo,
) {
	rates, err := info.provider.Fetch(ctx)

	response := &workerResponse{
		error:      err,
		rates:      rates,
		providerID: info.providerID,
	}

	select {
	case <-ctx.Done():
	case info.resCh <- response:
	}
}
