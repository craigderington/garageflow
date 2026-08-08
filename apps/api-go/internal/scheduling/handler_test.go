package scheduling

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garageflow/api-go/internal/middleware"
)

func TestCreateScheduleRejectsInvalidTimesBeforeDatabaseWrite(t *testing.T) {
	h := NewHandler(nil)
	cases := []string{
		`{"bay_id":"b","repair_order_id":"r","start_time":"bad","end_time":"2026-08-07T12:00:00Z"}`,
		`{"bay_id":"b","repair_order_id":"r","start_time":"2026-08-07T13:00:00Z","end_time":"2026-08-07T12:00:00Z"}`,
	}
	for _, body := range cases {
		req := httptest.NewRequest(http.MethodPost, "/schedule", strings.NewReader(body))
		req = req.WithContext(middleware.WithShopID(req.Context(), "shop"))
		res := httptest.NewRecorder()
		h.CreateSchedule(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("body=%s: code=%d, want 400", body, res.Code)
		}
	}
}
