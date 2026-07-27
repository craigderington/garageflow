INSERT INTO shops (id, name, slug) VALUES
    ('00000000-0000-0000-0000-000000000001', 'GarageFlow Demo', 'garageflow-demo');

-- password_hash below is bcrypt('password123'); matches the login screen demo hint.
INSERT INTO users (id, shop_id, email, name, role, password_hash) VALUES
    ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'owner@garageflow.app', 'Demo Owner', 'owner', '$2a$10$XGVOvJHFLnEBF1J2owhEdeXCwfHlv0B34vsrhsBsTQ1wiOonUs1Xq'),
    ('00000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'writer@garageflow.app', 'Demo Writer', 'service_writer', '$2a$10$XGVOvJHFLnEBF1J2owhEdeXCwfHlv0B34vsrhsBsTQ1wiOonUs1Xq'),
    ('00000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000001', 'tech@garageflow.app', 'Demo Tech', 'technician', '$2a$10$XGVOvJHFLnEBF1J2owhEdeXCwfHlv0B34vsrhsBsTQ1wiOonUs1Xq');
