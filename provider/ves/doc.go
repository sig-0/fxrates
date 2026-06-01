// Package ves provides exchange rate providers for Venezuelan Bolivar (VES).
//
// # Providers
//
// ## BCV (Official Central Bank)
//
// Source: "BCV"
// URL: https://www.bcv.org.ve/
// Interval: 24 hours
//
// Scrapes official exchange rates from Banco Central de Venezuela.
// Returns MID rates for multiple currency pairs:
//
//	USD/VES, EUR/VES, CNY/VES, TRY/VES, RUB/VES
//
// The effective date (AsOf) is parsed from the "Fecha Valor" field on the page.
//
// ## BCV Banks (Bank Rates)
//
// Source: Bank name (e.g., "Banesco", "Mercantil")
// URL: https://www.bcv.org.ve/tasas-informativas-sistema-bancario
// Interval: 24 hours
//
// Scrapes USD/VES rates reported by individual Venezuelan banks.
// Returns BUY and SELL rates for each bank. Only the most recent
// date's rates are returned (must be within 7 days).
//
// ## Binance P2P (USDT/VES)
//
// Implemented by the shared provider/crypto package (NewBinanceP2P) and
// instantiated for USDT/VES in cmd/serve/providers.go.
package ves
