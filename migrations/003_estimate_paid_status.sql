-- Allow estimates to be marked 'paid' once payment is collected (Stripe / dev settlement).
ALTER TABLE estimates DROP CONSTRAINT IF EXISTS estimates_status_check;
ALTER TABLE estimates ADD CONSTRAINT estimates_status_check
    CHECK (status IN ('draft','sent','approved','rejected','paid'));
