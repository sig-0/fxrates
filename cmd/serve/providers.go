package serve

import (
	"time"

	"github.com/sig-0/fxrates/ingest"
	"github.com/sig-0/fxrates/provider/ves"
)

// defaultProviders returns the default ingestion providers
func defaultProviders() []ingest.Provider {
	bcvProvider := ves.NewBCVProvider(
		"https://www.bcv.org.ve/",
		time.Second*30,
	)
	bcvBanksProvider := ves.NewBCVBanksProvider(
		"https://www.bcv.org.ve/tasas-informativas-sistema-bancario",
		time.Second*30,
	)

	return []ingest.Provider{
		bcvProvider,
		bcvBanksProvider,
	}
}
