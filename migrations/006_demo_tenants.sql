-- Demo tenancy: each captured email gets its own throwaway shop.
ALTER TABLE shops
    ADD COLUMN is_demo BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN expires_at TIMESTAMPTZ;

CREATE TABLE demo_leads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    -- SET NULL, not CASCADE: a lead whose 14 days lapsed is still a lead.
    -- Cascading would delete the exact thing the email wall exists to collect.
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    return_token TEXT NOT NULL UNIQUE,
    ip TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ
);

-- The expiry sweep scans on this every tick.
CREATE INDEX idx_shops_expires_at ON shops (expires_at) WHERE expires_at IS NOT NULL;
