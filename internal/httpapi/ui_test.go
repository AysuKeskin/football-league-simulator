package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestUI_ServedAtRoot(t *testing.T) {
	r, _ := newTestRouter(t)

	rec := doJSON(t, r, http.MethodGet, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="app"`) || !strings.Contains(body, "vue@3") {
		t.Error("UI page missing Vue mount point or CDN reference")
	}
}
