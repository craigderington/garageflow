package inspections

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// resolveToken returns (inspectionID, shopID, repairOrderID) for a public token.
func (h *Handler) resolveToken(r *http.Request, token string) (string, string, string, bool) {
	var inspID, shopID, roID string
	err := h.db.QueryRow(r.Context(),
		`SELECT id, shop_id, repair_order_id FROM inspections WHERE public_token=$1 AND status IN ('sent','reviewed')`, token,
	).Scan(&inspID, &shopID, &roID)
	if err != nil {
		return "", "", "", false
	}
	return inspID, shopID, roID, true
}

type publicReport struct {
	Inspection *Inspection `json:"inspection"`
	ShopName   string      `json:"shop_name"`
	Vehicle    string      `json:"vehicle"`
	Customer   string      `json:"customer"`
}

// Report is the unauthenticated customer-facing inspection view.
func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	inspID, shopID, roID, ok := h.resolveToken(r, chi.URLParam(r, "token"))
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	insp, err := h.load(r.Context(), inspID, "")
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	rep := publicReport{Inspection: insp}
	h.db.QueryRow(r.Context(), `SELECT COALESCE(name,'') FROM shops WHERE id=$1`, shopID).Scan(&rep.ShopName)
	var year int
	var make, model string
	h.db.QueryRow(r.Context(),
		`SELECT COALESCE(v.year,0), COALESCE(v.make,''), COALESCE(v.model,'')
		 FROM repair_orders ro LEFT JOIN vehicles v ON v.id = ro.vehicle_id WHERE ro.id=$1`, roID,
	).Scan(&year, &make, &model)
	if year > 0 {
		rep.Vehicle = itoa(year) + " "
	}
	rep.Vehicle += make + " " + model
	h.db.QueryRow(r.Context(),
		`SELECT COALESCE(c.name,'') FROM repair_orders ro JOIN customers c ON c.id=ro.customer_id WHERE ro.id=$1`, roID).Scan(&rep.Customer)

	json.NewEncoder(w).Encode(rep)
}

func (h *Handler) ReportPhoto(w http.ResponseWriter, r *http.Request) {
	inspID, _, _, ok := h.resolveToken(r, chi.URLParam(r, "token"))
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	h.streamPhoto(w, r, inspID, chi.URLParam(r, "photo_id"), "")
}

type approveRequest struct {
	ItemIDs []string `json:"item_ids"`
}

// Approve marks the customer-selected items approved and, if the repair order
// has no estimate yet, generates one from the approved priced items and flips
// the RO to 'approved'.
func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	inspID, shopID, roID, ok := h.resolveToken(r, chi.URLParam(r, "token"))
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	var req approveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		http.Error(w, `{"error":"tx failed"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	// Mark selected recommended items approved; reset any previously-approved not in the set.
	if _, err := tx.Exec(r.Context(),
		`UPDATE inspection_items SET approved = (id = ANY($1)), approved_at = CASE WHEN id = ANY($1) THEN NOW() ELSE NULL END
		 WHERE inspection_id=$2 AND condition IN ('attention','urgent')`, req.ItemIDs, inspID); err != nil {
		http.Error(w, `{"error":"approve failed"}`, http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE inspections SET status='reviewed', reviewed_at=NOW(), updated_at=NOW() WHERE id=$1`, inspID); err != nil {
		http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
		return
	}

	// Gather approved priced items.
	rows, err := tx.Query(r.Context(),
		`SELECT label, price FROM inspection_items WHERE inspection_id=$1 AND approved AND price > 0`, inspID)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	type line struct {
		label string
		price float64
	}
	var lines []line
	var total float64
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.label, &l.price); err == nil {
			lines = append(lines, l)
			total += l.price
		}
	}
	rows.Close()

	estimateCreated := false
	if len(lines) > 0 {
		var existing string
		err := tx.QueryRow(r.Context(), `SELECT id FROM estimates WHERE repair_order_id=$1 AND shop_id=$2 LIMIT 1`, roID, shopID).Scan(&existing)
		if err != nil { // no estimate yet -> create one from approved work
			estID := uuid.New().String()
			now := time.Now()
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO estimates (id, shop_id, repair_order_id, total, status, approved_at, created_at, updated_at)
				 VALUES ($1,$2,$3,$4,'approved',$5,$5,$5)`, estID, shopID, roID, total, now); err != nil {
				http.Error(w, `{"error":"estimate failed"}`, http.StatusInternalServerError)
				return
			}
			for _, l := range lines {
				if _, err := tx.Exec(r.Context(),
					`INSERT INTO estimate_items (id, estimate_id, type, description, quantity, unit_price, total)
					 VALUES ($1,$2,'labor',$3,1,$4,$4)`, uuid.New().String(), estID, l.label, l.price); err != nil {
					http.Error(w, `{"error":"estimate items failed"}`, http.StatusInternalServerError)
					return
				}
			}
			tx.Exec(r.Context(), `UPDATE repair_orders SET status='approved', updated_at=NOW() WHERE id=$1 AND shop_id=$2`, roID, shopID)
			estimateCreated = true
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, `{"error":"commit failed"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"approved":         len(req.ItemIDs),
		"estimate_created": estimateCreated,
		"total":            total,
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
