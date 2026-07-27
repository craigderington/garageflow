# API Spec

Auth:
POST /auth/magic-link
POST /auth/verify

Repair Orders:
GET /repair-orders
POST /repair-orders
PATCH /repair-orders/{id}

Estimates:
POST /estimates
POST /estimates/{id}/approve

Inventory:
GET /inventory
POST /inventory/restock

Realtime:
WS /ws
