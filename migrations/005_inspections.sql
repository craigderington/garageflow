-- Digital Vehicle Inspections (DVI): the tech walks the car against a template,
-- marks each item, attaches photos, and sends a public report to the customer
-- who approves recommended work (which can become an estimate).

CREATE TABLE inspection_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    -- items: [{ "section": "Brakes", "label": "Front pads" }, ...]
    items JSONB NOT NULL DEFAULT '[]',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE inspections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    repair_order_id UUID NOT NULL REFERENCES repair_orders(id) ON DELETE CASCADE,
    template_id UUID REFERENCES inspection_templates(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'in_progress'
        CHECK (status IN ('in_progress','sent','reviewed')),
    public_token TEXT UNIQUE,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    reviewed_at TIMESTAMPTZ
);
CREATE INDEX idx_inspections_ro ON inspections(repair_order_id);

CREATE TABLE inspection_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inspection_id UUID NOT NULL REFERENCES inspections(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    section TEXT NOT NULL DEFAULT '',
    label TEXT NOT NULL,
    -- pending = not yet checked; good/attention/urgent = the tech's call
    condition TEXT NOT NULL DEFAULT 'pending'
        CHECK (condition IN ('pending','good','attention','urgent')),
    note TEXT NOT NULL DEFAULT '',
    price NUMERIC(10,2) NOT NULL DEFAULT 0,
    position INT NOT NULL DEFAULT 0,
    approved BOOLEAN NOT NULL DEFAULT false,
    approved_at TIMESTAMPTZ
);
CREATE INDEX idx_inspection_items_inspection ON inspection_items(inspection_id);

CREATE TABLE inspection_item_photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    inspection_id UUID NOT NULL REFERENCES inspections(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES inspection_items(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_inspection_item_photos_item ON inspection_item_photos(item_id);

-- Seed a default "Courtesy Check" template for the demo shop.
INSERT INTO inspection_templates (shop_id, name, is_default, items) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Courtesy Check',
    true,
    '[
        {"section":"Brakes","label":"Front brake pads"},
        {"section":"Brakes","label":"Rear brake pads"},
        {"section":"Brakes","label":"Brake fluid"},
        {"section":"Tires","label":"Front tire tread"},
        {"section":"Tires","label":"Rear tire tread"},
        {"section":"Tires","label":"Tire pressure"},
        {"section":"Fluids","label":"Engine oil level"},
        {"section":"Fluids","label":"Coolant level"},
        {"section":"Fluids","label":"Washer fluid"},
        {"section":"Battery & Electrical","label":"Battery health"},
        {"section":"Battery & Electrical","label":"Headlights / taillights"},
        {"section":"Under Hood","label":"Serpentine belt"},
        {"section":"Under Hood","label":"Air filter"},
        {"section":"Wipers","label":"Wiper blades"}
    ]'
);
