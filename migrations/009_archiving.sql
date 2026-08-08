ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
ALTER TABLE inventory_parts ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_vehicles_shop_active ON vehicles(shop_id) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inventory_shop_active ON inventory_parts(shop_id) WHERE archived_at IS NULL;
