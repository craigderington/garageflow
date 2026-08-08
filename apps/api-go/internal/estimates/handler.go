package estimates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/events"
	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/payments"
	"github.com/garageflow/api-go/internal/types"
)

type Handler struct {
	db     *pgxpool.Pool
	bus    *events.Bus
	pay    *payments.Service
	appURL string
	isDemo func(context.Context) bool
}

// NewHandler builds the estimates handler. isDemo reports whether the request's
// tenant is a demo shop; Pay uses it to keep a prospect away from the
// merchant's real Stripe account. It is injected rather than imported so this
// package does not depend on internal/demo (which depends on middleware, sms
// and email) just for one predicate — main.go passes demo.IsDemoShop.
//
// A nil isDemo fails CLOSED — every shop is treated as a demo and no real
// Checkout Session is ever created — matching demo.GuardSMS/GuardEmail. A
// wiring mistake should cost a merchant a card payment, not charge a
// prospect's.
func NewHandler(db *pgxpool.Pool, bus *events.Bus, pay *payments.Service, appURL string, isDemo func(context.Context) bool) *Handler {
	if isDemo == nil {
		isDemo = func(context.Context) bool { return true }
	}
	return &Handler{db: db, bus: bus, pay: pay, appURL: appURL, isDemo: isDemo}
}

type createEstimateRequest struct {
	RepairOrderID string              `json:"repair_order_id"`
	Items         []estimateItemInput `json:"items"`
}

type estimateItemInput struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req createEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RepairOrderID == "" || len(req.Items) == 0 {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	var total float64
	for _, item := range req.Items {
		if item.Description == "" || item.Quantity <= 0 || item.UnitPrice < 0 {
			http.Error(w, `{"error":"invalid estimate item"}`, http.StatusBadRequest)
			return
		}
		total += item.Quantity * item.UnitPrice
	}

	est := types.Estimate{
		ID:            uuid.New().String(),
		ShopID:        shopID,
		RepairOrderID: req.RepairOrderID,
		Total:         total,
		Status:        "draft",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		http.Error(w, `{"error":"transaction failed"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(),
		`INSERT INTO estimates (id, shop_id, repair_order_id, total, status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		est.ID, est.ShopID, est.RepairOrderID, est.Total, est.Status, est.CreatedAt, est.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"create estimate failed"}`, http.StatusInternalServerError)
		return
	}

	for _, item := range req.Items {
		itemID := uuid.New().String()
		itemTotal := item.Quantity * item.UnitPrice
		_, err = tx.Exec(r.Context(),
			`INSERT INTO estimate_items (id, estimate_id, type, description, quantity, unit_price, total)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			itemID, est.ID, item.Type, item.Description, item.Quantity, item.UnitPrice, itemTotal)
		if err != nil {
			http.Error(w, `{"error":"create item failed"}`, http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, `{"error":"commit failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(est)
}

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.transition(r.Context(), id, shopID, "approved", "approved"); err != nil {
		http.Error(w, `{"error":"approve failed"}`, http.StatusInternalServerError)
		return
	}

	h.bus.Publish(r.Context(), events.TypeEstimateApproved, events.EstimateApprovedPayload{
		EstimateID: id,
		ShopID:     shopID,
	})

	json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
}

func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.transition(r.Context(), id, shopID, "sent", "estimate_sent"); err != nil {
		http.Error(w, `{"error":"send failed"}`, http.StatusInternalServerError)
		return
	}

	h.bus.Publish(r.Context(), events.TypeEstimateSent, events.EstimateSentPayload{
		EstimateID: id,
		ShopID:     shopID,
	})

	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

func (h *Handler) transition(ctx context.Context, id, shopID, estimateStatus, roStatus string) error {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var roID string
	stampColumn := "sent_at"
	if estimateStatus == "approved" {
		stampColumn = "approved_at"
	}
	query := `UPDATE estimates SET status = $1, ` + stampColumn + ` = NOW(), updated_at = NOW()
		WHERE id = $2 AND shop_id = $3 RETURNING repair_order_id`
	if err := tx.QueryRow(ctx, query, estimateStatus, id, shopID).Scan(&roID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx,
		`UPDATE repair_orders SET status = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3`,
		roStatus, roID, shopID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("repair order not found")
	}
	return tx.Commit(ctx)
}

func (h *Handler) GetByRO(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	roID := chi.URLParam(r, "ro_id")

	var est types.Estimate
	err := h.db.QueryRow(r.Context(),
		`SELECT id, shop_id, repair_order_id, total, status, sent_at, approved_at, created_at, updated_at
		 FROM estimates WHERE repair_order_id = $1 AND shop_id = $2`, roID, shopID,
	).Scan(&est.ID, &est.ShopID, &est.RepairOrderID, &est.Total, &est.Status, &est.SentAt, &est.ApprovedAt, &est.CreatedAt, &est.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT id, estimate_id, type, description, quantity, unit_price, total
		 FROM estimate_items WHERE estimate_id = $1`, est.ID)
	if err != nil {
		http.Error(w, `{"error":"query items failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []types.EstimateItem
	for rows.Next() {
		var item types.EstimateItem
		if err := rows.Scan(&item.ID, &item.EstimateID, &item.Type, &item.Description, &item.Quantity, &item.UnitPrice, &item.Total); err != nil {
			continue
		}
		items = append(items, item)
	}
	if items == nil {
		items = []types.EstimateItem{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"estimate": est,
		"items":    items,
	})
}

// Pay creates a Stripe Checkout Session to collect payment on an estimate.
// With Stripe configured it returns the hosted checkout URL (the estimate is
// marked paid by the webhook). In dev mode (no Stripe key) it marks the
// estimate paid immediately and returns a local success URL.
func (h *Handler) Pay(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var total float64
	var status string
	err := h.db.QueryRow(r.Context(),
		`SELECT total, status FROM estimates WHERE id = $1 AND shop_id = $2`, id, shopID,
	).Scan(&total, &status)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if status == "paid" {
		http.Error(w, `{"error":"already paid"}`, http.StatusConflict)
		return
	}

	amountCents := int64(total*100 + 0.5)
	success := h.appURL + "/estimates?paid=" + id
	cancel := h.appURL + "/estimates"

	// A demo tenant must never touch the merchant's live Stripe account.
	// Without this, a prospect clicking "collect payment" in the demo would
	// create a real Checkout Session on the real account in production. Settle
	// locally exactly as the no-key dev path does, so the prospect still sees
	// the estimate go paid and the whole flow demos correctly.
	if h.isDemo(r.Context()) {
		if err := h.settle(r, id, shopID); err != nil {
			http.Error(w, `{"error":"settle failed"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"url": success, "mode": "dev"})
		return
	}

	session, err := h.pay.CreateCheckoutSession(
		r.Context(), amountCents, "usd", "GarageFlow estimate "+id[:8], success, cancel,
		map[string]string{"estimate_id": id, "shop_id": shopID},
	)
	if err != nil {
		http.Error(w, `{"error":"payment init failed"}`, http.StatusBadGateway)
		return
	}

	// In dev mode there is no webhook, so settle immediately.
	if session.Mode == "dev" {
		if err := h.settle(r, id, shopID); err != nil {
			http.Error(w, `{"error":"settle failed"}`, http.StatusInternalServerError)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]string{"url": session.URL, "mode": session.Mode})
}

// settle marks an estimate paid without a Stripe round trip — the path taken
// when no Stripe key is configured and when the tenant is a demo.
func (h *Handler) settle(r *http.Request, id, shopID string) error {
	_, err := h.db.Exec(r.Context(),
		`UPDATE estimates SET status = 'paid', updated_at = NOW() WHERE id = $1 AND shop_id = $2`, id, shopID)
	return err
}

// Webhook receives Stripe events (no auth/tenant middleware). On a completed
// checkout it marks the corresponding estimate paid.
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, `{"error":"read failed"}`, http.StatusBadRequest)
		return
	}
	eventType, metadata, err := h.pay.VerifyWebhook(body, r.Header.Get("Stripe-Signature"))
	if err != nil {
		http.Error(w, `{"error":"invalid signature"}`, http.StatusBadRequest)
		return
	}
	if eventType == "checkout.session.completed" {
		if estimateID := metadata["estimate_id"]; estimateID != "" {
			h.db.Exec(r.Context(),
				`UPDATE estimates SET status = 'paid', updated_at = NOW() WHERE id = $1`, estimateID)
		}
	}
	json.NewEncoder(w).Encode(map[string]bool{"received": true})
}

type updateEstimateRequest struct {
	Status *string             `json:"status,omitempty"`
	Items  []estimateItemInput `json:"items,omitempty"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var req updateEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		http.Error(w, `{"error":"transaction failed"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	if req.Status != nil {
		_, err := tx.Exec(r.Context(),
			`UPDATE estimates SET status = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3`,
			*req.Status, id, shopID)
		if err != nil {
			http.Error(w, `{"error":"update status failed"}`, http.StatusInternalServerError)
			return
		}
	}

	if req.Items != nil {
		if len(req.Items) == 0 {
			http.Error(w, `{"error":"estimate requires at least one item"}`, http.StatusBadRequest)
			return
		}
		var total float64
		for _, item := range req.Items {
			if item.Description == "" || item.Quantity <= 0 || item.UnitPrice < 0 {
				http.Error(w, `{"error":"invalid estimate item"}`, http.StatusBadRequest)
				return
			}
			total += item.Quantity * item.UnitPrice
		}

		tag, err := tx.Exec(r.Context(),
			`UPDATE estimates SET total = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3`,
			total, id, shopID)
		if err != nil {
			http.Error(w, `{"error":"update total failed"}`, http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		_, err = tx.Exec(r.Context(), `DELETE FROM estimate_items WHERE estimate_id = $1`, id)
		if err != nil {
			http.Error(w, `{"error":"clear items failed"}`, http.StatusInternalServerError)
			return
		}

		for _, item := range req.Items {
			itemID := uuid.New().String()
			itemTotal := item.Quantity * item.UnitPrice
			_, err = tx.Exec(r.Context(),
				`INSERT INTO estimate_items (id, estimate_id, type, description, quantity, unit_price, total)
				 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				itemID, id, item.Type, item.Description, item.Quantity, item.UnitPrice, itemTotal)
			if err != nil {
				http.Error(w, `{"error":"insert item failed"}`, http.StatusInternalServerError)
				return
			}
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, `{"error":"commit failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
