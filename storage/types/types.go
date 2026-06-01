package types

import "time"

type Currency string

const (
	CurrencyUSD  Currency = "USD"
	CurrencyEUR  Currency = "EUR"
	CurrencyCNY  Currency = "CNY"
	CurrencyTRY  Currency = "TRY"
	CurrencyRUB  Currency = "RUB"
	CurrencyVES  Currency = "VES"
	CurrencyUSDT Currency = "USDT"
	CurrencyCOP  Currency = "COP"
	CurrencyPEN  Currency = "PEN"
	CurrencyCLP  Currency = "CLP"
	CurrencyBRL  Currency = "BRL"
	CurrencyUSDC Currency = "USDC"
	CurrencyARS  Currency = "ARS"
)

func (c Currency) String() string {
	return string(c)
}

type RateType string

const (
	RateTypeMID  RateType = "MID"
	RateTypeBUY  RateType = "BUY"
	RateTypeSELL RateType = "SELL"
)

func (r RateType) String() string {
	return string(r)
}

type Source string

const (
	SourceBCV      Source = "BCV"      // Banco Central de Venezuela
	SourceSFC      Source = "SFC"      // Superintendencia Financiera de Colombia
	SourceBCRP     Source = "BCRP"     // Banco Central de Reserva del Perú
	SourceBCCH     Source = "BCCH"     // Banco Central de Chile
	SourceBCB      Source = "BCB"      // Banco Central do Brasil
	SourceECB      Source = "ECB"      // European Central Bank
	SourceCoinbase Source = "Coinbase" // Coinbase

	SourceBCRA            Source = "BCRA"            // Banco Central de la República Argentina
	SourceBNA             Source = "BNA"             // Banco de la Nación Argentina
	SourceBinanceP2P      Source = "BinanceP2P"      // Binance P2P marketplace
	SourceDolarAPIBlue    Source = "DolarAPIBlue"    // DolarAPI Blue (informal) rate
	SourceDolarAPIMEP     Source = "DolarAPIMEP"     // DolarAPI MEP (bolsa) rate
	SourceDolarAPICCL     Source = "DolarAPICCL"     // DolarAPI Contado con Liquidación rate
	SourceDolarAPITarjeta Source = "DolarAPITarjeta" // DolarAPI Tarjeta (turista) rate
)

func (s Source) String() string {
	return string(s)
}

type ExchangeRate struct {
	AsOf      time.Time `json:"as_of"`
	FetchedAt time.Time `json:"fetched_at"`
	Base      Currency  `json:"base"`
	Target    Currency  `json:"target"`
	RateType  RateType  `json:"rate_type"`
	Source    Source    `json:"source"`
	Rate      float64   `json:"rate"`
}

type Pair struct {
	Base   Currency `json:"base"`
	Target Currency `json:"target"`
}

type RateQuery struct {
	Target   *Currency `json:"target"`
	RateType *RateType `json:"rate_type"`
	Source   *Source   `json:"source"`
	Base     Currency  `json:"base"`
	Offset   int64     `json:"offset"`
	Limit    int32     `json:"limit"`
}

// Page wraps the results for pagination
type Page[T any] struct {
	Results []T   `json:"results"`
	Total   int64 `json:"total"`
}
