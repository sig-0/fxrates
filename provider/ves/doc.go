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
// ## Binance P2P (USDT)
//
// Source: "BinanceP2P"
// API: https://p2p.binance.com/bapi/c2c/v2/friendly/c2c/adv/search
// Interval: 10 minutes
//
// Fetches peer-to-peer USDT/VES rates from Binance.
// Returns BUY and SELL rates calculated as the median of filtered offers.
//
// Offer collection:
//   - Fetches up to 30 offers (3 pages of 10)
//   - Parses price, limits, availability, and advertiser metrics
//
// Offer filtering (strict, then relaxed if needed):
//   - Minimum 50 monthly orders (relaxed: 20)
//   - Minimum 95% completion rate (relaxed: 90%)
//   - Minimum 50 USDT available
//   - Typical transaction amount of 100 USDT must be within limits
//
// Quality scoring uses Wilson lower bound on completion rate to favor
// advertisers with both high completion rates and sufficient order volume.
//
// Final rate is the median of the top 12 offers sorted by price
// (ascending for BUY, descending for SELL), with quality as tiebreaker.
package ves
