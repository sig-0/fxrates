-- name: SaveExchangeRate :exec
INSERT INTO exchange_rates (
  base, target, rate, rate_type, source, as_of, fetched_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
);

-- name: RateAsOf :many
WITH latest AS (
  SELECT DISTINCT ON (target, source, rate_type)
    *
  FROM exchange_rates
  WHERE base = sqlc.arg('base')
    AND (sqlc.narg('target')::text IS NULL OR target = sqlc.narg('target')::text)
    AND (sqlc.narg('source')::text IS NULL OR source = sqlc.narg('source')::text)
    AND (sqlc.narg('rate_type')::text IS NULL OR rate_type = sqlc.narg('rate_type')::text)
    AND as_of <= sqlc.arg('as_of')
  ORDER BY target, source, rate_type, as_of DESC
)
SELECT
  latest.*,
  COUNT(*) OVER()::bigint AS total
FROM latest
ORDER BY target, source, rate_type
LIMIT LEAST(sqlc.arg('limit')::int, 500)
OFFSET sqlc.arg('offset')::bigint;
