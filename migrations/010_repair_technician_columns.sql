-- Some early installations were initialized before migration 007 was added and
-- the old migration runner did not track/apply later files. Make the technician
-- fields self-healing for those databases.
ALTER TABLE users ADD COLUMN IF NOT EXISTS specialities TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS hourly_rate NUMERIC(10,2) NOT NULL DEFAULT 100.00;
