package labor

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

type clockInRequest struct {
	RepairOrderID string `json:"repair_order_id"`
	Description   string `json:"description"`
}

type clockOutRequest struct {
	Minutes     int    `json:"minutes"`
	Description string `json:"description"`
}

func (h *Handler) ClockIn(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	userID := middleware.GetUserID(r.Context())

	var req clockInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	log := types.LaborLog{
		ID:            uuid.New().String(),
		ShopID:        shopID,
		MechanicID:    userID,
		RepairOrderID: req.RepairOrderID,
		Description:   req.Description,
		ClockIn:       time.Now(),
		CreatedAt:     time.Now(),
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO labor_logs (id, shop_id, mechanic_id, repair_order_id, description, clock_in, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		log.ID, log.ShopID, log.MechanicID, log.RepairOrderID, log.Description, log.ClockIn, log.CreatedAt)
	if err != nil {
		http.Error(w, `{"error":"clock in failed"}`, http.StatusInternalServerError)
		return
	}

	h.bus.Publish(r.Context(), events.TypeMechanicClockedIn, events.MechanicClockedInPayload{
		MechanicID:    userID,
		RepairOrderID: req.RepairOrderID,
		ShopID:        shopID,
	})

	_, err = h.db.Exec(r.Context(),
		`UPDATE repair_orders SET status = 'in_progress', updated_at = NOW() WHERE id = $1 AND shop_id = $2`,
		req.RepairOrderID, shopID)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(log)
}

func (h *Handler) ClockOut(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	userID := middleware.GetUserID(r.Context())
	logID := chi.URLParam(r, "id")

	var req clockOutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	now := time.Now()
	_, err := h.db.Exec(r.Context(),
		`UPDATE labor_logs SET clock_out = $1, minutes = $2, description = COALESCE(NULLIF($3, ''), description)
		 WHERE id = $4 AND shop_id = $5 AND mechanic_id = $6`,
		now, req.Minutes, req.Description, logID, shopID, userID)
	if err != nil {
		http.Error(w, `{"error":"clock out failed"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "clocked_out"})
}

func (h *Handler) ListByRO(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	roID := chi.URLParam(r, "ro_id")

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, mechanic_id, repair_order_id, minutes, description, clock_in, clock_out, created_at
		 FROM labor_logs WHERE shop_id = $1 AND repair_order_id = $2 ORDER BY clock_in DESC`,
		shopID, roID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []types.LaborLog
	for rows.Next() {
		var l types.LaborLog
		if err := rows.Scan(&l.ID, &l.ShopID, &l.MechanicID, &l.RepairOrderID, &l.Minutes, &l.Description, &l.ClockIn, &l.ClockOut, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []types.LaborLog{}
	}
	json.NewEncoder(w).Encode(logs)
}
