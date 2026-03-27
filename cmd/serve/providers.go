package serve

import (
	"time"

	"github.com/sig-0/fxrates/ingest"
	"github.com/sig-0/fxrates/provider/brl"
	"github.com/sig-0/fxrates/provider/clp"
	"github.com/sig-0/fxrates/provider/cop"
	"github.com/sig-0/fxrates/provider/crypto"
	"github.com/sig-0/fxrates/provider/eur"
	"github.com/sig-0/fxrates/provider/pen"
	"github.com/sig-0/fxrates/provider/ves"
)

// defaultProviders returns the default ingestion providers
func defaultProviders() []ingest.Provider {
	var (
		// Official BCV rates
		bcvProvider = ves.NewBCVProvider(
			"https://www.bcv.org.ve/",
			time.Second*30,
		)

		// Official BCV bank rates
		bcvBanksProvider = ves.NewBCVBanksProvider(
			"https://www.bcv.org.ve/tasas-informativas-sistema-bancario",
			time.Second*30,
		)

		// Median Binance P2P USDT rate
		binanceP2PProvider = ves.NewBinanceP2PProvider(time.Second * 30)

		// Colombia TRM (Superintendencia Financiera)
		sfcProvider = cop.NewSFCProvider(time.Second * 30)

		// Peru SBS rates (BCRP)
		bcrpProvider = pen.NewBCRPProvider(time.Second * 30)

		// Chile Dólar Observado (findic.cl)
		findicProvider = clp.NewFindicProvider(time.Second * 30)

		// Brazil PTAX rates (BCB)
		bcbProvider = brl.NewBCBProvider(time.Second * 30)

		// EUR/USD ECB reference rate
		ecbProvider = eur.NewECBProvider(time.Second * 30)

		// USDC/USD Coinbase spot price
		coinbaseUSDCProvider = crypto.NewCoinbaseUSDCProvider(time.Second * 30)
	)

	return []ingest.Provider{
		bcvProvider,
		bcvBanksProvider,
		binanceP2PProvider,
		sfcProvider,
		bcrpProvider,
		findicProvider,
		bcbProvider,
		ecbProvider,
		coinbaseUSDCProvider,
	}
}
