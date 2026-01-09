-- Clears all entries in the exchange_rates table

BEGIN;

-- Delete all rates, resetting auto-increment ID
TRUNCATE TABLE exchange_rates RESTART IDENTITY CASCADE;

COMMIT;
