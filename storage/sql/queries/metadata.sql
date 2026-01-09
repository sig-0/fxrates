-- name: ListSources :many
SELECT DISTINCT source
FROM exchange_rates
ORDER BY source;

-- name: ListCurrencies :many
SELECT code
FROM (
  SELECT DISTINCT base AS code FROM exchange_rates
  UNION
  SELECT DISTINCT target AS code FROM exchange_rates
) c
ORDER BY code;
