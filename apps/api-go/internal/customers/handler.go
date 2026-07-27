package customers

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

type createCustomerRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
	Notes string `json:"notes"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, name, phone, email, notes, created_at, updated_at
		 FROM customers WHERE shop_id = $1 ORDER BY name ASC`, shopID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var customers []types.Customer
	for rows.Next() {
		var c types.Customer
		if err := rows.Scan(&c.ID, &c.ShopID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		customers = append(customers, c)
	}
	if customers == nil {
		customers = []types.Customer{}
	}
	json.NewEncoder(w).Encode(customers)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req createCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	c := types.Customer{
		ID:        uuid.New().String(),
		ShopID:    shopID,
		Name:      req.Name,
		Phone:     req.Phone,
		Email:     req.Email,
		Notes:     req.Notes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO customers (id, shop_id, name, phone, email, notes, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		c.ID, c.ShopID, c.Name, c.Phone, c.Email, c.Notes, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"create failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var c types.Customer
	err := h.db.QueryRow(r.Context(),
		`SELECT id, shop_id, name, phone, email, notes, created_at, updated_at
		 FROM customers WHERE id = $1 AND shop_id = $2`, id, shopID,
	).Scan(&c.ID, &c.ShopID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(c)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var req createCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	ct, err := h.db.Exec(r.Context(),
		`UPDATE customers SET name = $1, phone = $2, email = $3, notes = $4, updated_at = NOW()
		 WHERE id = $5 AND shop_id = $6`,
		req.Name, req.Phone, req.Email, req.Notes, id, shopID)
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
		`DELETE FROM customers WHERE id = $1 AND shop_id = $2`, id, shopID)
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
