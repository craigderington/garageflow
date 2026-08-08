package inventory

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

type createPartRequest struct {
	Name        string  `json:"name"`
	SKU         string  `json:"sku"`
	Description string  `json:"description"`
	StockLevel  int     `json:"stock_level"`
	MinStock    int     `json:"min_stock"`
	UnitPrice   float64 `json:"unit_price"`
	VendorID    *string `json:"vendor_id"`
}

type restockRequest struct {
	PartID   string `json:"part_id"`
	Quantity int    `json:"quantity"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	includeArchived := r.URL.Query().Get("include_archived") == "true"

	rows, err := h.db.Query(r.Context(),
		`SELECT id, shop_id, name, sku, description, stock_level, min_stock, unit_price, vendor_id,
		 archived_at IS NOT NULL, created_at, updated_at
		 FROM inventory_parts WHERE shop_id = $1 AND ($2 OR archived_at IS NULL) ORDER BY name ASC`, shopID, includeArchived)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var parts []types.InventoryPart
	for rows.Next() {
		var p types.InventoryPart
		if err := rows.Scan(&p.ID, &p.ShopID, &p.Name, &p.SKU, &p.Description, &p.StockLevel, &p.MinStock, &p.UnitPrice, &p.VendorID, &p.Archived, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		parts = append(parts, p)
	}
	if parts == nil {
		parts = []types.InventoryPart{}
	}
	json.NewEncoder(w).Encode(parts)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req createPartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	p := types.InventoryPart{
		ID:          uuid.New().String(),
		ShopID:      shopID,
		Name:        req.Name,
		SKU:         req.SKU,
		Description: req.Description,
		StockLevel:  req.StockLevel,
		MinStock:    req.MinStock,
		UnitPrice:   req.UnitPrice,
		VendorID:    req.VendorID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO inventory_parts (id, shop_id, name, sku, description, stock_level, min_stock, unit_price, vendor_id, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.ID, p.ShopID, p.Name, p.SKU, p.Description, p.StockLevel, p.MinStock, p.UnitPrice, p.VendorID, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"create failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")

	var p types.InventoryPart
	err := h.db.QueryRow(r.Context(),
		`SELECT id, shop_id, name, sku, description, stock_level, min_stock, unit_price, vendor_id, archived_at IS NOT NULL, created_at, updated_at
		 FROM inventory_parts WHERE id = $1 AND shop_id = $2`, id, shopID,
	).Scan(&p.ID, &p.ShopID, &p.Name, &p.SKU, &p.Description, &p.StockLevel, &p.MinStock, &p.UnitPrice, &p.VendorID, &p.Archived, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Archived bool `json:"archived"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	tag, err := h.db.Exec(r.Context(),
		`UPDATE inventory_parts SET archived_at = CASE WHEN $1 THEN NOW() ELSE NULL END, updated_at = NOW()
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

	var req createPartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	ct, err := h.db.Exec(r.Context(),
		`UPDATE inventory_parts SET name = $1, sku = $2, description = $3, stock_level = $4, min_stock = $5, unit_price = $6, updated_at = NOW()
		 WHERE id = $7 AND shop_id = $8`,
		req.Name, req.SKU, req.Description, req.StockLevel, req.MinStock, req.UnitPrice, id, shopID)
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
		`DELETE FROM inventory_parts WHERE id = $1 AND shop_id = $2`, id, shopID)
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

func (h *Handler) Restock(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())

	var req restockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PartID == "" || req.Quantity <= 0 {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE inventory_parts SET stock_level = stock_level + $1, updated_at = NOW()
		 WHERE id = $2 AND shop_id = $3`, req.Quantity, req.PartID, shopID)
	if err != nil {
		http.Error(w, `{"error":"restock failed"}`, http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	var newStock int
	if err := h.db.QueryRow(r.Context(),
		`SELECT stock_level FROM inventory_parts WHERE id = $1 AND shop_id = $2`, req.PartID, shopID,
	).Scan(&newStock); err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}

	if newStock < 5 {
		h.bus.Publish(r.Context(), events.TypeInventoryLowStock, events.InventoryLowStockPayload{
			PartID: req.PartID,
			ShopID: shopID,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "restocked",
		"part_id":   req.PartID,
		"new_stock": newStock,
	})
}
