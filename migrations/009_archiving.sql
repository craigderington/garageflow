ALTER TABLE vehicles ADD COLUMN archived_at TIMESTAMPTZ;
ALTER TABLE inventory_parts ADD COLUMN archived_at TIMESTAMPTZ;

CREATE INDEX idx_vehicles_shop_active ON vehicles(shop_id) WHERE archived_at IS NULL;
CREATE INDEX idx_inventory_shop_active ON inventory_parts(shop_id) WHERE archived_at IS NULL;
