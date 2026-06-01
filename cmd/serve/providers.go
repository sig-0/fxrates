package serve

import (
	"time"

	"github.com/sig-0/fxrates/ingest"
	"github.com/sig-0/fxrates/provider/ars"
	"github.com/sig-0/fxrates/provider/brl"
	"github.com/sig-0/fxrates/provider/clp"
	"github.com/sig-0/fxrates/provider/cop"
	"github.com/sig-0/fxrates/provider/crypto"
	"github.com/sig-0/fxrates/provider/dolarapi"
	"github.com/sig-0/fxrates/provider/eur"
	"github.com/sig-0/fxrates/provider/pen"
	"github.com/sig-0/fxrates/provider/ves"
	"github.com/sig-0/fxrates/storage/types"
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

		// Median Binance P2P USDT/VES rate
		binanceP2PProvider = crypto.NewBinanceP2P(crypto.BinanceP2PConfig{
			Asset:    types.CurrencyUSDT,
			Fiat:     types.CurrencyVES,
			Source:   types.SourceBinanceP2P,
			Name:     "Binance P2P (USDT/VES)",
			Interval: time.Minute * 10,
			Timeout:  time.Second * 30,
		})

		// Argentina BCRA Mayorista (reference rate)
		bcraProvider = ars.NewBCRAProvider(time.Second * 30)

		// Argentina BNA Oficial (retail banknote rate)
		bnaProvider = ars.NewBNAProvider(time.Second * 30)

		// Argentina DolarAPI parallel rates (Blue, MEP, CCL, Tarjeta)
		dolarAPIProvider = dolarapi.New(time.Second * 30)

		// Argentina Binance P2P USDT/ARS
		binanceP2PARSProvider = crypto.NewBinanceP2P(crypto.BinanceP2PConfig{
			Asset:    types.CurrencyUSDT,
			Fiat:     types.CurrencyARS,
			Source:   types.SourceBinanceP2P,
			Name:     "Binance P2P (USDT/ARS)",
			Interval: time.Minute * 10,
			Timeout:  time.Second * 30,
		})

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
		bcraProvider,
		bnaProvider,
		dolarAPIProvider,
		binanceP2PARSProvider,
		sfcProvider,
		bcrpProvider,
		findicProvider,
		bcbProvider,
		ecbProvider,
		coinbaseUSDCProvider,
	}
}
