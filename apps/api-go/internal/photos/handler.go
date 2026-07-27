// Package photos handles repair-order photo/attachment upload, listing,
// streaming, and deletion. Bytes go to the object store; metadata to Postgres.
package photos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/storage"
)

const maxUpload = 32 << 20 // 32 MiB

type Handler struct {
	db    *pgxpool.Pool
	store storage.Store
}

func NewHandler(db *pgxpool.Pool, store storage.Store) *Handler {
	return &Handler{db: db, store: store}
}

type Photo struct {
	ID            string    `json:"id"`
	RepairOrderID string    `json:"repair_order_id"`
	Filename      string    `json:"filename"`
	ContentType   string    `json:"content_type"`
	SizeBytes     int64     `json:"size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
}

func allowedType(ct string) bool {
	return strings.HasPrefix(ct, "image/") || ct == "application/pdf"
}

// roInShop confirms the repair order exists in the caller's shop.
func (h *Handler) roInShop(r *http.Request, roID, shopID string) bool {
	var x string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM repair_orders WHERE id = $1 AND shop_id = $2`, roID, shopID).Scan(&x)
	return err == nil
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	roID := chi.URLParam(r, "id")

	rows, err := h.db.Query(r.Context(),
		`SELECT id, repair_order_id, filename, content_type, size_bytes, created_at
		 FROM repair_order_photos WHERE repair_order_id = $1 AND shop_id = $2 ORDER BY created_at ASC`,
		roID, shopID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	photos := []Photo{}
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.RepairOrderID, &p.Filename, &p.ContentType, &p.SizeBytes, &p.CreatedAt); err != nil {
			continue
		}
		photos = append(photos, p)
	}
	json.NewEncoder(w).Encode(photos)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	userID := middleware.GetUserID(r.Context())
	roID := chi.URLParam(r, "id")

	if !h.roInShop(r, roID, shopID) {
		http.Error(w, `{"error":"repair order not found"}`, http.StatusNotFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1024)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !allowedType(contentType) {
		http.Error(w, `{"error":"unsupported file type (images and PDF only)"}`, http.StatusUnsupportedMediaType)
		return
	}

	id := uuid.New().String()
	safeName := path.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if safeName == "" || safeName == "." || safeName == "/" {
		safeName = "upload"
	}
	key := fmt.Sprintf("%s/%s/%s-%s", shopID, roID, id, safeName)

	if err := h.store.Put(r.Context(), key, file, header.Size, contentType); err != nil {
		http.Error(w, `{"error":"storage write failed"}`, http.StatusBadGateway)
		return
	}

	var p Photo
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO repair_order_photos (id, shop_id, repair_order_id, object_key, filename, content_type, size_bytes, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, repair_order_id, filename, content_type, size_bytes, created_at`,
		id, shopID, roID, key, safeName, contentType, header.Size, userID,
	).Scan(&p.ID, &p.RepairOrderID, &p.Filename, &p.ContentType, &p.SizeBytes, &p.CreatedAt)
	if err != nil {
		_ = h.store.Delete(r.Context(), key) // don't leak an orphaned object
		http.Error(w, `{"error":"save failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// Content streams the raw bytes (authenticated). The web app fetches this with
// credentials and turns it into a blob URL, so MinIO is never exposed publicly.
func (h *Handler) Content(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	roID := chi.URLParam(r, "id")
	photoID := chi.URLParam(r, "photo_id")

	var key, contentType, filename string
	err := h.db.QueryRow(r.Context(),
		`SELECT object_key, content_type, filename FROM repair_order_photos
		 WHERE id = $1 AND repair_order_id = $2 AND shop_id = $3`, photoID, roID, shopID,
	).Scan(&key, &contentType, &filename)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	obj, err := h.store.Get(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	defer obj.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	w.Header().Set("Cache-Control", "private, max-age=300")
	io.Copy(w, obj)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	roID := chi.URLParam(r, "id")
	photoID := chi.URLParam(r, "photo_id")

	var key string
	err := h.db.QueryRow(r.Context(),
		`SELECT object_key FROM repair_order_photos
		 WHERE id = $1 AND repair_order_id = $2 AND shop_id = $3`, photoID, roID, shopID).Scan(&key)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	_ = h.store.Delete(r.Context(), key)
	if _, err := h.db.Exec(r.Context(),
		`DELETE FROM repair_order_photos WHERE id = $1 AND shop_id = $2`, photoID, shopID); err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
