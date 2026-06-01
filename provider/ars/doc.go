// Package ars provides Argentine Peso (ARS) exchange rate providers that talk
// directly to Argentine sources.
//
// Argentina has a fragmented FX market in which several legal and informal
// USD/ARS rates coexist due to capital controls. This package only covers the
// providers anchored on Argentine institutions:
//
//   - BCRA Mayorista (api.bcra.gob.ar) — reference rate, 3-hour cadence.
//   - BNA Oficial (www.bna.com.ar/Personas) — retail banknote rate, 1-hour
//     cadence, scraped from the id="billetes" section.
//
// The parallel rates (Blue, MEP, CCL, Tarjeta) live in provider/dolarapi, and
// the USDT/ARS Binance P2P feed is instantiated from provider/crypto.
package ars
