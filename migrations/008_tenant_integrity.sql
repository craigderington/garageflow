-- Enforce that relationships never cross shop boundaries. NOT VALID keeps this
-- migration deployable if historical data needs cleanup while still enforcing
-- the rule for every new or updated row.
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_id_shop ON users(id, shop_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_customers_id_shop ON customers(id, shop_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_vehicles_id_shop ON vehicles(id, shop_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_repair_orders_id_shop ON repair_orders(id, shop_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_bays_id_shop ON bays(id, shop_id);

ALTER TABLE vehicles
    ADD CONSTRAINT vehicles_customer_same_shop
    FOREIGN KEY (customer_id, shop_id) REFERENCES customers(id, shop_id)
    ON DELETE CASCADE NOT VALID;

ALTER TABLE repair_orders
    ADD CONSTRAINT repair_orders_customer_same_shop
    FOREIGN KEY (customer_id, shop_id) REFERENCES customers(id, shop_id)
    ON DELETE CASCADE NOT VALID,
    ADD CONSTRAINT repair_orders_vehicle_same_shop
    FOREIGN KEY (vehicle_id, shop_id) REFERENCES vehicles(id, shop_id)
    ON DELETE SET NULL (vehicle_id) NOT VALID,
    ADD CONSTRAINT repair_orders_creator_same_shop
    FOREIGN KEY (created_by, shop_id) REFERENCES users(id, shop_id)
    NOT VALID;

ALTER TABLE estimates
    ADD CONSTRAINT estimates_ro_same_shop
    FOREIGN KEY (repair_order_id, shop_id) REFERENCES repair_orders(id, shop_id)
    ON DELETE CASCADE NOT VALID;

ALTER TABLE labor_logs
    ADD CONSTRAINT labor_logs_mechanic_same_shop
    FOREIGN KEY (mechanic_id, shop_id) REFERENCES users(id, shop_id)
    NOT VALID,
    ADD CONSTRAINT labor_logs_ro_same_shop
    FOREIGN KEY (repair_order_id, shop_id) REFERENCES repair_orders(id, shop_id)
    ON DELETE CASCADE NOT VALID;

ALTER TABLE schedules
    ADD CONSTRAINT schedules_bay_same_shop
    FOREIGN KEY (bay_id, shop_id) REFERENCES bays(id, shop_id)
    ON DELETE CASCADE NOT VALID,
    ADD CONSTRAINT schedules_ro_same_shop
    FOREIGN KEY (repair_order_id, shop_id) REFERENCES repair_orders(id, shop_id)
    ON DELETE CASCADE NOT VALID,
    ADD CONSTRAINT schedules_technician_same_shop
    FOREIGN KEY (technician_id, shop_id) REFERENCES users(id, shop_id)
    NOT VALID;
