BEGIN;

ALTER TABLE exchange_rates
  ALTER COLUMN base TYPE VARCHAR(4),
  ALTER COLUMN target TYPE VARCHAR(4);

ALTER TABLE exchange_rates
  DROP CONSTRAINT IF EXISTS exchange_rates_base_fmt,
  DROP CONSTRAINT IF EXISTS exchange_rates_target_fmt;

ALTER TABLE exchange_rates
  ADD CONSTRAINT exchange_rates_base_fmt CHECK (base ~ '^[A-Z]{3,4}$'),
  ADD CONSTRAINT exchange_rates_target_fmt CHECK (target ~ '^[A-Z]{3,4}$');

COMMIT;
