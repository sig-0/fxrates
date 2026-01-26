CREATE TABLE exchange_rates (
  id         BIGSERIAL PRIMARY KEY,
  base       VARCHAR(4)   NOT NULL,
  target     VARCHAR(4)   NOT NULL,
  rate       NUMERIC(20,4) NOT NULL,
  rate_type  VARCHAR(16) NOT NULL,
  source     VARCHAR(50) NOT NULL,
  as_of      TIMESTAMPTZ NOT NULL,
  fetched_at TIMESTAMPTZ NOT NULL,

  CONSTRAINT exchange_rates_base_target_diff CHECK (base <> target),
  CONSTRAINT exchange_rates_base_fmt CHECK (base ~ '^[A-Z]{3,4}$'),
  CONSTRAINT exchange_rates_target_fmt CHECK (target ~ '^[A-Z]{3,4}$'),

  CONSTRAINT exchange_rates_uniq
    UNIQUE (base, target, rate_type, source, as_of)
);

CREATE INDEX exchange_rates_asof_latest_idx
  ON exchange_rates (base, target, source, rate_type, as_of DESC);
