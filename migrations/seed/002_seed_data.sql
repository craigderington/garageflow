INSERT INTO shops (id, name, slug) VALUES
    ('00000000-0000-0000-0000-000000000001', 'GarageFlow Demo', 'garageflow-demo');

-- password_hash below is bcrypt('password123'); matches the login screen demo hint.
INSERT INTO users (id, shop_id, email, name, role, password_hash) VALUES
    ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'owner@garageflow.app', 'Demo Owner', 'owner', '$2a$10$XGVOvJHFLnEBF1J2owhEdeXCwfHlv0B34vsrhsBsTQ1wiOonUs1Xq'),
    ('00000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'writer@garageflow.app', 'Demo Writer', 'service_writer', '$2a$10$XGVOvJHFLnEBF1J2owhEdeXCwfHlv0B34vsrhsBsTQ1wiOonUs1Xq'),
    ('00000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000001', 'tech@garageflow.app', 'Demo Tech', 'technician', '$2a$10$XGVOvJHFLnEBF1J2owhEdeXCwfHlv0B34vsrhsBsTQ1wiOonUs1Xq');

-- Default "Courtesy Check" inspection template for the demo shop. Moved here
-- from 005_inspections.sql so that migrations/*.sql contains schema only.
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
