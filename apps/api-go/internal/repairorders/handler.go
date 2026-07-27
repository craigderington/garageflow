package repairorders

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/events"
	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/types"
)

type Handler struct {
	db  *pgxpool.Pool
	bus *events.Bus
}

func NewHandler(db *pgxpool.Pool, bus *events.Bus) *Handler {
	return &Handler{db: db, bus: bus}
}

type createRORequest struct {
	CustomerID  string `json:"customer_id"`
	VehicleID   string `json:"vehicle_id"`
	Description string `json:"description"`
	Mileage     int    `json:"mileage"`
}

type updateRORequest struct {
	Status      *string `json:"status,omitempty"`
	Description *string `json:"description,omitempty"`
	Mileage     *int    `json:"mileage,omitempty"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, customer_id, COALESCE(vehicle_id::text, ''), status, description, mileage, COALESCE(created_by::text, ''), created_at, updated_at
		 FROM repair_orders WHERE shop_id = $1 ORDER BY created_at DESC`, shopID)
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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	userID := middleware.GetUserID(r.Context())

	var req createRORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	o := types.RepairOrder{
		ID:          uuid.New().String(),
		ShopID:      shopID,
		CustomerID:  req.CustomerID,
		VehicleID:   req.VehicleID,
		Status:      types.ROStatusCreated,
		Description: req.Description,
		Mileage:     req.Mileage,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// vehicle_id is an optional UUID FK (ON DELETE SET NULL); insert NULL rather
	// than an empty string when a repair order is opened without a vehicle.
	var vehicle interface{}
	if o.VehicleID != "" {
		vehicle = o.VehicleID
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO repair_orders (id, shop_id, customer_id, vehicle_id, status, description, mileage, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		o.ID, o.ShopID, o.CustomerID, vehicle, o.Status, o.Description, o.Mileage, o.CreatedBy, o.CreatedAt, o.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"create failed"}`, http.StatusInternalServerError)
		return
	}

	h.bus.Publish(r.Context(), events.TypeRepairOrderCreated, events.RepairOrderCreatedPayload{
		RepairOrderID: o.ID,
		ShopID:        shopID,
		CustomerID:    o.CustomerID,
	})

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(o)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var o types.RepairOrder
	err := h.db.QueryRow(r.Context(),
		`SELECT id, shop_id, customer_id, COALESCE(vehicle_id::text, ''), status, description, mileage, COALESCE(created_by::text, ''), created_at, updated_at
		 FROM repair_orders WHERE id = $1 AND shop_id = $2`, id, shopID,
	).Scan(&o.ID, &o.ShopID, &o.CustomerID, &o.VehicleID, &o.Status, &o.Description, &o.Mileage, &o.CreatedBy, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(o)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var req updateRORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Status != nil {
		_, err := h.db.Exec(r.Context(),
			`UPDATE repair_orders SET status = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3`,
			*req.Status, id, shopID)
		if err != nil {
			http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
			return
		}
	}

	if req.Description != nil {
		_, err := h.db.Exec(r.Context(),
			`UPDATE repair_orders SET description = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3`,
			*req.Description, id, shopID)
		if err != nil {
			http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
			return
		}
	}

	if req.Mileage != nil {
		_, err := h.db.Exec(r.Context(),
			`UPDATE repair_orders SET mileage = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3`,
			*req.Mileage, id, shopID)
		if err != nil {
			http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
