// Package inspections implements Digital Vehicle Inspections (DVI): a tech walks
// a vehicle against a template, marks each item good/attention/urgent, attaches
// photos, and sends a public report the customer approves from their phone.
package inspections

import (
	"context"
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

	"github.com/garageflow/api-go/internal/email"
	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/sms"
	"github.com/garageflow/api-go/internal/storage"
)

const maxUpload = 32 << 20

type Handler struct {
	db     *pgxpool.Pool
	store  storage.Store
	sms    sms.Sender
	mailer email.Sender
	appURL string
}

func NewHandler(db *pgxpool.Pool, store storage.Store, smsSender sms.Sender, mailer email.Sender, appURL string) *Handler {
	return &Handler{db: db, store: store, sms: smsSender, mailer: mailer, appURL: appURL}
}

type Photo struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

type Item struct {
	ID        string  `json:"id"`
	Section   string  `json:"section"`
	Label     string  `json:"label"`
	Condition string  `json:"condition"`
	Note      string  `json:"note"`
	Price     float64 `json:"price"`
	Position  int     `json:"position"`
	Approved  bool    `json:"approved"`
	Photos    []Photo `json:"photos"`
}

type Inspection struct {
	ID            string     `json:"id"`
	RepairOrderID string     `json:"repair_order_id"`
	Status        string     `json:"status"`
	PublicToken   string     `json:"public_token"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        *time.Time `json:"sent_at"`
	Items         []Item     `json:"items"`
}

func (h *Handler) roInShop(ctx context.Context, roID, shopID string) bool {
	var x string
	return h.db.QueryRow(ctx, `SELECT id FROM repair_orders WHERE id=$1 AND shop_id=$2`, roID, shopID).Scan(&x) == nil
}

// load builds the full inspection (items + photos). shopID may be "" to skip the
// tenant filter (used by the public report, which is keyed by token instead).
func (h *Handler) load(ctx context.Context, inspectionID, shopID string) (*Inspection, error) {
	var insp Inspection
	q := `SELECT id, repair_order_id, status, COALESCE(public_token,''), created_at, sent_at FROM inspections WHERE id=$1`
	args := []any{inspectionID}
	if shopID != "" {
		q += " AND shop_id=$2"
		args = append(args, shopID)
	}
	if err := h.db.QueryRow(ctx, q, args...).Scan(
		&insp.ID, &insp.RepairOrderID, &insp.Status, &insp.PublicToken, &insp.CreatedAt, &insp.SentAt,
	); err != nil {
		return nil, err
	}

	rows, err := h.db.Query(ctx,
		`SELECT id, section, label, condition, note, price, position, approved
		 FROM inspection_items WHERE inspection_id=$1 ORDER BY position, section`, inspectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]*Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Section, &it.Label, &it.Condition, &it.Note, &it.Price, &it.Position, &it.Approved); err != nil {
			continue
		}
		it.Photos = []Photo{}
		insp.Items = append(insp.Items, it)
	}
	for i := range insp.Items {
		byID[insp.Items[i].ID] = &insp.Items[i]
	}

	pr, err := h.db.Query(ctx,
		`SELECT id, item_id, filename, content_type FROM inspection_item_photos WHERE inspection_id=$1 ORDER BY created_at`, inspectionID)
	if err == nil {
		defer pr.Close()
		for pr.Next() {
			var id, itemID, fn, ct string
			if err := pr.Scan(&id, &itemID, &fn, &ct); err != nil {
				continue
			}
			if it := byID[itemID]; it != nil {
				it.Photos = append(it.Photos, Photo{ID: id, Filename: fn, ContentType: ct})
			}
		}
	}
	if insp.Items == nil {
		insp.Items = []Item{}
	}
	return &insp, nil
}

type templateItem struct {
	Section string `json:"section"`
	Label   string `json:"label"`
}

// CreateForRO starts a new inspection on a repair order, copying items from the
// shop's default template.
func (h *Handler) CreateForRO(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	userID := middleware.GetUserID(r.Context())
	roID := chi.URLParam(r, "id")

	if !h.roInShop(r.Context(), roID, shopID) {
		http.Error(w, `{"error":"repair order not found"}`, http.StatusNotFound)
		return
	}

	var templateID string
	var raw []byte
	err := h.db.QueryRow(r.Context(),
		`SELECT id, items FROM inspection_templates WHERE shop_id=$1 AND is_default ORDER BY created_at LIMIT 1`, shopID,
	).Scan(&templateID, &raw)
	var items []templateItem
	if err == nil {
		_ = json.Unmarshal(raw, &items)
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		http.Error(w, `{"error":"tx failed"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	inspID := uuid.New().String()
	var tmpl any
	if templateID != "" {
		tmpl = templateID
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO inspections (id, shop_id, repair_order_id, template_id, created_by) VALUES ($1,$2,$3,$4,$5)`,
		inspID, shopID, roID, tmpl, userID); err != nil {
		http.Error(w, `{"error":"create failed"}`, http.StatusInternalServerError)
		return
	}
	for i, it := range items {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO inspection_items (shop_id, inspection_id, section, label, position) VALUES ($1,$2,$3,$4,$5)`,
			shopID, inspID, it.Section, it.Label, i); err != nil {
			http.Error(w, `{"error":"seed items failed"}`, http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, `{"error":"commit failed"}`, http.StatusInternalServerError)
		return
	}

	insp, err := h.load(r.Context(), inspID, shopID)
	if err != nil {
		http.Error(w, `{"error":"load failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(insp)
}

// GetByRO returns the most recent inspection for a repair order (404 if none).
func (h *Handler) GetByRO(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	roID := chi.URLParam(r, "id")

	var inspID string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM inspections WHERE repair_order_id=$1 AND shop_id=$2 ORDER BY created_at DESC LIMIT 1`, roID, shopID).Scan(&inspID)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	insp, err := h.load(r.Context(), inspID, shopID)
	if err != nil {
		http.Error(w, `{"error":"load failed"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(insp)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	insp, err := h.load(r.Context(), chi.URLParam(r, "id"), shopID)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(insp)
}

type updateItemRequest struct {
	Condition *string  `json:"condition"`
	Note      *string  `json:"note"`
	Price     *float64 `json:"price"`
}

func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	inspID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "item_id")

	var req updateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Condition != nil {
		switch *req.Condition {
		case "pending", "good", "attention", "urgent":
		default:
			http.Error(w, `{"error":"invalid condition"}`, http.StatusBadRequest)
			return
		}
	}
	ct, err := h.db.Exec(r.Context(),
		`UPDATE inspection_items SET
		   condition = COALESCE($1, condition),
		   note      = COALESCE($2, note),
		   price     = COALESCE($3, price)
		 WHERE id=$4 AND inspection_id=$5 AND shop_id=$6`,
		req.Condition, req.Note, req.Price, itemID, inspID, shopID)
	if err != nil {
		http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	h.db.Exec(r.Context(), `UPDATE inspections SET updated_at=NOW() WHERE id=$1`, inspID)
	w.WriteHeader(http.StatusNoContent)
}

func allowedType(ct string) bool {
	return strings.HasPrefix(ct, "image/") || ct == "application/pdf"
}

func (h *Handler) UploadItemPhoto(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	inspID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "item_id")

	var x string
	if h.db.QueryRow(r.Context(),
		`SELECT id FROM inspection_items WHERE id=$1 AND inspection_id=$2 AND shop_id=$3`, itemID, inspID, shopID).Scan(&x) != nil {
		http.Error(w, `{"error":"item not found"}`, http.StatusNotFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1024)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		http.Error(w, `{"error":"file too large"}`, http.StatusBadRequest)
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
		http.Error(w, `{"error":"unsupported file type"}`, http.StatusUnsupportedMediaType)
		return
	}

	id := uuid.New().String()
	safe := path.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if safe == "" || safe == "." || safe == "/" {
		safe = "photo"
	}
	key := fmt.Sprintf("inspections/%s/%s/%s/%s-%s", shopID, inspID, itemID, id, safe)
	if err := h.store.Put(r.Context(), key, file, header.Size, contentType); err != nil {
		http.Error(w, `{"error":"storage write failed"}`, http.StatusBadGateway)
		return
	}
	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO inspection_item_photos (id, shop_id, inspection_id, item_id, object_key, filename, content_type, size_bytes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		id, shopID, inspID, itemID, key, safe, contentType, header.Size); err != nil {
		_ = h.store.Delete(r.Context(), key)
		http.Error(w, `{"error":"save failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Photo{ID: id, Filename: safe, ContentType: contentType})
}

func (h *Handler) PhotoContent(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	h.streamPhoto(w, r, chi.URLParam(r, "id"), chi.URLParam(r, "photo_id"), shopID)
}

// streamPhoto writes the object bytes. shopID may be "" for token-authorized
// public access (the caller is responsible for authorizing the inspection).
func (h *Handler) streamPhoto(w http.ResponseWriter, r *http.Request, inspID, photoID, shopID string) {
	q := `SELECT object_key, content_type, filename FROM inspection_item_photos WHERE id=$1 AND inspection_id=$2`
	args := []any{photoID, inspID}
	if shopID != "" {
		q += " AND shop_id=$3"
		args = append(args, shopID)
	}
	var key, ct, fn string
	if err := h.db.QueryRow(r.Context(), q, args...).Scan(&key, &ct, &fn); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	obj, err := h.store.Get(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	defer obj.Close()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", fn))
	w.Header().Set("Cache-Control", "private, max-age=300")
	io.Copy(w, obj)
}

func (h *Handler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	inspID := chi.URLParam(r, "id")
	photoID := chi.URLParam(r, "photo_id")
	var key string
	if h.db.QueryRow(r.Context(),
		`SELECT object_key FROM inspection_item_photos WHERE id=$1 AND inspection_id=$2 AND shop_id=$3`, photoID, inspID, shopID).Scan(&key) != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	_ = h.store.Delete(r.Context(), key)
	h.db.Exec(r.Context(), `DELETE FROM inspection_item_photos WHERE id=$1 AND shop_id=$2`, photoID, shopID)
	w.WriteHeader(http.StatusNoContent)
}

// Send marks the inspection sent, mints a public token, and delivers the report
// link to the customer by SMS and email (best-effort).
func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	shopID := middleware.GetShopID(r.Context())
	inspID := chi.URLParam(r, "id")

	var roID, token string
	var existing *string
	if err := h.db.QueryRow(r.Context(),
		`SELECT repair_order_id, public_token FROM inspections WHERE id=$1 AND shop_id=$2`, inspID, shopID).Scan(&roID, &existing); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if existing != nil && *existing != "" {
		token = *existing
	} else {
		token = strings.ReplaceAll(uuid.New().String(), "-", "")
	}

	if _, err := h.db.Exec(r.Context(),
		`UPDATE inspections SET status='sent', public_token=$1, sent_at=NOW(), updated_at=NOW() WHERE id=$2 AND shop_id=$3`,
		token, inspID, shopID); err != nil {
		http.Error(w, `{"error":"send failed"}`, http.StatusInternalServerError)
		return
	}

	// Look up the customer to notify (best-effort).
	var phone, custEmail, name string
	h.db.QueryRow(r.Context(),
		`SELECT COALESCE(c.phone,''), COALESCE(c.email,''), COALESCE(c.name,'')
		 FROM repair_orders ro JOIN customers c ON c.id = ro.customer_id WHERE ro.id=$1`, roID).Scan(&phone, &custEmail, &name)

	link := strings.TrimRight(h.appURL, "/") + "/inspect/" + token
	go h.deliver(shopID, phone, custEmail, name, link)

	json.NewEncoder(w).Encode(map[string]string{"public_token": token, "url": link})
}

// deliver runs on its own detached goroutine (see Send), so it cannot reuse
// the request context — that context is cancelled as soon as Send returns,
// which would abort delivery mid-flight. shopID is captured from the request
// context before that happens and re-injected here so the demo guard, which
// keys off middleware.GetShopID, still sees the right tenant instead of
// failing closed and suppressing delivery for every shop, demo or not.
func (h *Handler) deliver(shopID, phone, custEmail, name, link string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx = middleware.WithShopID(ctx, shopID)
	if phone != "" {
		_ = h.sms.Send(ctx, phone, "Your vehicle inspection is ready: "+link)
	}
	if custEmail != "" && h.mailer != nil {
		html := fmt.Sprintf(`<p>Hi %s,</p><p>Your vehicle inspection is ready to review.</p><p><a href="%s">View your inspection</a></p>`, name, link)
		text := "Your vehicle inspection is ready: " + link
		_ = h.mailer.Send(ctx, custEmail, "Your vehicle inspection", html, text)
	}
}
