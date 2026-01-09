-- name: SaveExchangeRate :exec
INSERT INTO exchange_rates (
  base, target, rate, rate_type, source, as_of, fetched_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
);

-- name: RateAsOf :one
SELECT *
FROM exchange_rates
WHERE base = $1
  AND target = $2
  AND source = $3
  AND rate_type = $4
  AND as_of <= $5
ORDER BY as_of DESC
LIMIT 1;

-- name: RatesInRange :many
SELECT
  exchange_rates.*,
  COUNT(*) OVER()::bigint AS total
FROM exchange_rates
WHERE base = $1
  AND target = $2
  AND source = $3
  AND rate_type = $4
  AND as_of >= $5
  AND as_of <= $6
ORDER BY as_of ASC
LIMIT $7
OFFSET $8::bigint;