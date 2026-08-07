package scheduling

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/middleware"
)

type Bay struct {
	ID     string `json:"id"`
	ShopID string `json:"shop_id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type Schedule struct {
	ID            string    `json:"id"`
	ShopID        string    `json:"shop_id"`
	BayID         string    `json:"bay_id"`
	RepairOrderID string    `json:"repair_order_id"`
	TechnicianID  string    `json:"technician_id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	CreatedAt     time.Time `json:"created_at"`
}

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type createBayRequest struct {
	Name string `json:"name"`
}

func (h *Handler) ListBays(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, name, active FROM bays WHERE shop_id = $1 ORDER BY name`, shopID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var bays []Bay
	for rows.Next() {
		var b Bay
		if err := rows.Scan(&b.ID, &b.ShopID, &b.Name, &b.Active); err != nil {
			continue
		}
		bays = append(bays, b)
	}
	if bays == nil {
		bays = []Bay{}
	}
	json.NewEncoder(w).Encode(bays)
}

func (h *Handler) CreateBay(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req createBayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	b := Bay{
		ID:     uuid.New().String(),
		ShopID: shopID,
		Name:   req.Name,
		Active: true,
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO bays (id, shop_id, name, active) VALUES ($1,$2,$3,$4)`,
		b.ID, b.ShopID, b.Name, b.Active)
	if err != nil {
		http.Error(w, `{"error":"create failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(b)
}

func (h *Handler) DeleteBay(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	_, err := h.db.Exec(r.Context(),
		`DELETE FROM bays WHERE shop_id = $1 AND id = $2`, shopID, id)
	if err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

type createScheduleRequest struct {
	BayID         string `json:"bay_id"`
	RepairOrderID string `json:"repair_order_id"`
	TechnicianID  string `json:"technician_id"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

func (h *Handler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, bay_id, repair_order_id, COALESCE(technician_id::text, ''), start_time, end_time, created_at
		 FROM schedules WHERE shop_id = $1 ORDER BY start_time`, shopID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var schedules []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.ShopID, &s.BayID, &s.RepairOrderID, &s.TechnicianID, &s.StartTime, &s.EndTime, &s.CreatedAt); err != nil {
			continue
		}
		schedules = append(schedules, s)
	}
	if schedules == nil {
		schedules = []Schedule{}
	}
	json.NewEncoder(w).Encode(schedules)
}

func (h *Handler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req createScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)

	s := Schedule{
		ID:            uuid.New().String(),
		ShopID:        shopID,
		BayID:         req.BayID,
		RepairOrderID: req.RepairOrderID,
		TechnicianID:  req.TechnicianID,
		StartTime:     startTime,
		EndTime:       endTime,
		CreatedAt:     time.Now(),
	}

	var technician interface{}
	if req.TechnicianID != "" {
		technician = req.TechnicianID
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO schedules (id, shop_id, bay_id, repair_order_id, technician_id, start_time, end_time, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		s.ID, s.ShopID, s.BayID, s.RepairOrderID, technician, s.StartTime, s.EndTime, s.CreatedAt)
	if err != nil {
		http.Error(w, `{"error":"create failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(s)
}

type assignRequest struct {
	BayID         string `json:"bay_id"`
	RepairOrderID string `json:"repair_order_id"`
	TechnicianID  string `json:"technician_id,omitempty"`
}

func (h *Handler) AssignRO(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req assignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BayID == "" || req.RepairOrderID == "" {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Remove any existing schedule for this RO to prevent duplicate bay occupancy
	_, _ = h.db.Exec(r.Context(),
		`DELETE FROM schedules WHERE shop_id = $1 AND repair_order_id = $2`,
		shopID, req.RepairOrderID)

	var technician interface{}
	if req.TechnicianID != "" {
		technician = req.TechnicianID
	}

	now := time.Now()
	endTime := now.Add(2 * time.Hour)

	s := Schedule{
		ID:            uuid.New().String(),
		ShopID:        shopID,
		BayID:         req.BayID,
		RepairOrderID: req.RepairOrderID,
		TechnicianID:  req.TechnicianID,
		StartTime:     now,
		EndTime:       endTime,
		CreatedAt:     now,
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO schedules (id, shop_id, bay_id, repair_order_id, technician_id, start_time, end_time, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		s.ID, s.ShopID, s.BayID, s.RepairOrderID, technician, s.StartTime, s.EndTime, s.CreatedAt)
	if err != nil {
		http.Error(w, `{"error":"assign failed"}`, http.StatusInternalServerError)
		return
	}

	// Progress status to in_progress if currently created, diagnosed, or approved
	_, _ = h.db.Exec(r.Context(),
		`UPDATE repair_orders SET status = 'in_progress', updated_at = NOW()
		 WHERE shop_id = $1 AND id = $2 AND status IN ('created', 'diagnosed', 'approved')`,
		shopID, req.RepairOrderID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(s)
}

type unassignRequest struct {
	RepairOrderID string `json:"repair_order_id"`
}

func (h *Handler) UnassignRO(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req unassignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RepairOrderID == "" {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(r.Context(),
		`DELETE FROM schedules WHERE shop_id = $1 AND repair_order_id = $2`,
		shopID, req.RepairOrderID)
	if err != nil {
		http.Error(w, `{"error":"unassign failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "unassigned"})
}

func (h *Handler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	_, err := h.db.Exec(r.Context(),
		`DELETE FROM schedules WHERE shop_id = $1 AND id = $2`, shopID, id)
	if err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

