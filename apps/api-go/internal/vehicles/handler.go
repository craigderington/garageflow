package vehicles

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/types"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type createVehicleRequest struct {
	CustomerID   string `json:"customer_id"`
	VIN          string `json:"vin"`
	Make         string `json:"make"`
	Model        string `json:"model"`
	Year         int    `json:"year"`
	Color        string `json:"color"`
	LicensePlate string `json:"license_plate"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	includeArchived := r.URL.Query().Get("include_archived") == "true"

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, customer_id, vin, make, model, year, color, license_plate,
		 archived_at IS NOT NULL, created_at, updated_at
		 FROM vehicles WHERE shop_id = $1 AND ($2 OR archived_at IS NULL) ORDER BY created_at DESC`, shopID, includeArchived)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var vehicles []types.Vehicle
	for rows.Next() {
		var v types.Vehicle
		if err := rows.Scan(&v.ID, &v.ShopID, &v.CustomerID, &v.VIN, &v.Make, &v.Model, &v.Year, &v.Color, &v.LicensePlate, &v.Archived, &v.CreatedAt, &v.UpdatedAt); err != nil {
			continue
		}
		vehicles = append(vehicles, v)
	}
	if vehicles == nil {
		vehicles = []types.Vehicle{}
	}
	json.NewEncoder(w).Encode(vehicles)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req createVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	v := types.Vehicle{
		ID:           uuid.New().String(),
		ShopID:       shopID,
		CustomerID:   req.CustomerID,
		VIN:          req.VIN,
		Make:         req.Make,
		Model:        req.Model,
		Year:         req.Year,
		Color:        req.Color,
		LicensePlate: req.LicensePlate,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO vehicles (id, shop_id, customer_id, vin, make, model, year, color, license_plate, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		v.ID, v.ShopID, v.CustomerID, v.VIN, v.Make, v.Model, v.Year, v.Color, v.LicensePlate, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"create failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var v types.Vehicle
	err := h.db.QueryRow(r.Context(),
		`SELECT id, shop_id, customer_id, vin, make, model, year, color, license_plate, archived_at IS NOT NULL, created_at, updated_at
		 FROM vehicles WHERE id = $1 AND shop_id = $2`, id, shopID,
	).Scan(&v.ID, &v.ShopID, &v.CustomerID, &v.VIN, &v.Make, &v.Model, &v.Year, &v.Color, &v.LicensePlate, &v.Archived, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(v)
}

type archiveRequest struct {
	Archived bool `json:"archived"`
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")
	var req archiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	tag, err := h.db.Exec(r.Context(),
		`UPDATE vehicles SET archived_at = CASE WHEN $1 THEN NOW() ELSE NULL END, updated_at = NOW()
		 WHERE id = $2 AND shop_id = $3`, req.Archived, id, shopID)
	if err != nil {
		http.Error(w, `{"error":"archive failed"}`, http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	h.Get(w, r)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var req createVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	ct, err := h.db.Exec(r.Context(),
		`UPDATE vehicles SET vin = $1, make = $2, model = $3, year = $4, color = $5, license_plate = $6,
		 customer_id = COALESCE(NULLIF($7, '')::uuid, customer_id), updated_at = NOW()
		 WHERE id = $8 AND shop_id = $9`,
		req.VIN, req.Make, req.Model, req.Year, req.Color, req.LicensePlate, req.CustomerID, id, shopID)
	if err != nil {
		http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	h.Get(w, r)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	ct, err := h.db.Exec(r.Context(),
		`DELETE FROM vehicles WHERE id = $1 AND shop_id = $2`, id, shopID)
	if err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
