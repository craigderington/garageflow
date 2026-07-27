# GarageFlow Engineering Spec v1

## Overview
GarageFlow is a cloud-native auto repair shop management platform built in Go + Postgres + Redis with a modular monolith architecture.

## Core Problem
Independent repair shops rely on legacy systems (Mitchell1, Shop-Ware) with outdated UX, high cost, and poor cloud-native workflows.

## Architecture
- Backend: Go (chi, sqlc)
- DB: PostgreSQL (multi-tenant via shop_id)
- Cache/Queue: Redis + Asynq
- Realtime: WebSockets/SSE
- Frontend: Next.js + TS
- Infra: Docker + Caddy + MinIO

## Domain Modules
### 1. Identity & Auth
- Magic link login
- Optional password fallback
- Session cookies (httpOnly)
- RBAC: Owner, Service Writer, Technician, Admin

### 2. Customers
- Customer profiles
- Vehicle ownership mapping
- Service history timeline

### 3. Vehicles
- VIN tracking
- Mileage history
- Maintenance records

### 4. Repair Orders (Core)
Lifecycle:
- Created → Diagnosed → Estimate Sent → Approved → In Progress → Completed → Invoiced → Closed

### 5. Estimates
- Line items
- Labor + parts
- Approval workflow via SMS/email portal

### 6. Inventory
- Parts catalog
- Stock levels
- Vendor tracking
- Purchase orders

### 7. Labor Tracking
- Mechanic clock-in/out
- Job assignment
- Time per RO

### 8. Scheduling
- Bay-based scheduling
- Technician assignment
- Optional dispatch board (premium)

### 9. Customer Portal
- View estimates
- Approve work
- View invoices
- Service history
- Photo inspection gallery

## Multi-Tenancy
All tables include:
- shop_id (required)
- created_at / updated_at
- soft delete optional

## API Style
REST + WebSockets:
- /auth
- /customers
- /vehicles
- /repair-orders
- /estimates
- /inventory

## Events
- repair_order.created
- estimate.sent
- estimate.approved
- mechanic.clocked_in
- inventory.low_stock

## Realtime
- RO status updates
- mechanic activity feed
- dashboard updates

## Security
- JWT internal tokens
- session cookies external
- RBAC middleware
- rate limiting per shop

## Performance Targets
- <200ms API latency
- <1s dashboard load
- realtime update lag <250ms

