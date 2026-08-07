package users

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

type createUserRequest struct {
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Role         types.Role `json:"role"`
	Specialities string     `json:"specialities"`
	HourlyRate   float64    `json:"hourly_rate"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, email, name, role, COALESCE(specialities, ''), COALESCE(hourly_rate, 100.00), created_at, updated_at
		 FROM users WHERE shop_id = $1 ORDER BY name ASC, email ASC`, shopID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var userList []types.User
	for rows.Next() {
		var u types.User
		if err := rows.Scan(&u.ID, &u.ShopID, &u.Email, &u.Name, &u.Role, &u.Specialities, &u.HourlyRate, &u.CreatedAt, &u.UpdatedAt); err != nil {
			continue
		}
		userList = append(userList, u)
	}
	if userList == nil {
		userList = []types.User{}
	}
	json.NewEncoder(w).Encode(userList)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		req.Role = types.RoleTechnician
	}
	if req.HourlyRate <= 0 {
		req.HourlyRate = 100.00
	}

	u := types.User{
		ID:           uuid.New().String(),
		ShopID:       shopID,
		Email:        req.Email,
		Name:         req.Name,
		Role:         req.Role,
		Specialities: req.Specialities,
		HourlyRate:   req.HourlyRate,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO users (id, shop_id, email, name, role, specialities, hourly_rate, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		u.ID, u.ShopID, u.Email, u.Name, u.Role, u.Specialities, u.HourlyRate, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"create failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(r.Context(),
		`UPDATE users SET name = COALESCE(NULLIF($1, ''), name), email = COALESCE(NULLIF($2, ''), email),
		 role = COALESCE(NULLIF($3, ''), role), specialities = $4, hourly_rate = $5, updated_at = NOW()
		 WHERE id = $6 AND shop_id = $7`,
		req.Name, req.Email, string(req.Role), req.Specialities, req.HourlyRate, id, shopID)
	if err != nil {
		http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	_, err := h.db.Exec(r.Context(),
		`DELETE FROM users WHERE id = $1 AND shop_id = $2`, id, shopID)
	if err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
