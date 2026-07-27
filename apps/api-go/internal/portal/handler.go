package portal

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/types"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) GetEstimates(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "customer_id")

	rows, err := h.db.Query(r.Context(),
		`SELECT e.id, e.shop_id, e.repair_order_id, e.total, e.status, e.sent_at, e.approved_at, e.created_at, e.updated_at
		 FROM estimates e
		 JOIN repair_orders ro ON ro.id = e.repair_order_id
		 WHERE ro.customer_id = $1
		 ORDER BY e.created_at DESC`, customerID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var estimates []types.Estimate
	for rows.Next() {
		var e types.Estimate
		if err := rows.Scan(&e.ID, &e.ShopID, &e.RepairOrderID, &e.Total, &e.Status, &e.SentAt, &e.ApprovedAt, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		estimates = append(estimates, e)
	}
	if estimates == nil {
		estimates = []types.Estimate{}
	}
	json.NewEncoder(w).Encode(estimates)
}

func (h *Handler) GetServiceHistory(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "customer_id")

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, customer_id, vehicle_id, status, description, mileage, created_by, created_at, updated_at
		 FROM repair_orders WHERE customer_id = $1 ORDER BY created_at DESC`, customerID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []types.RepairOrder
	for rows.Next() {
		var o types.RepairOrder
		if err := rows.Scan(&o.ID, &o.ShopID, &o.CustomerID, &o.VehicleID, &o.Status, &o.Description, &o.Mileage, &o.CreatedBy, &o.CreatedAt, &o.UpdatedAt); err != nil {
			continue
		}
		orders = append(orders, o)
	}
	if orders == nil {
		orders = []types.RepairOrder{}
	}
	json.NewEncoder(w).Encode(orders)
}

func (h *Handler) ApproveEstimate(w http.ResponseWriter, r *http.Request) {
	estimateID := chi.URLParam(r, "id")

	_, err := h.db.Exec(r.Context(),
		`UPDATE estimates SET status = 'approved', approved_at = NOW(), updated_at = NOW() WHERE id = $1`,
		estimateID)
	if err != nil {
		http.Error(w, `{"error":"approve failed"}`, http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE repair_orders SET status = 'approved', updated_at = NOW()
		 WHERE id = (SELECT repair_order_id FROM estimates WHERE id = $1)`, estimateID)
	if err != nil {
		http.Error(w, `{"error":"update RO failed"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
}
