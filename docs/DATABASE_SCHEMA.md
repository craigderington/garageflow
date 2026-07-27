# Database Schema

Core tables:

users(id, shop_id, email, role)
customers(id, shop_id, name, phone)
vehicles(id, shop_id, vin, customer_id)
repair_orders(id, shop_id, status, customer_id, vehicle_id)
estimates(id, shop_id, repair_order_id)
estimate_items(id, estimate_id, type, cost)
inventory_parts(id, shop_id, name, stock_level)
labor_logs(id, shop_id, mechanic_id, repair_order_id, minutes)
invoices(id, shop_id, repair_order_id, total)
audit_logs(id, shop_id, action, entity, timestamp)
