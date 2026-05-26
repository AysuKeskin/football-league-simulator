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
	if !strings.Contains(body, `id="app"`) || !strings.Contains(body, "/assets/vue.global.prod.js") {
		t.Error("UI page missing Vue mount point or local runtime reference")
	}
}

func TestUI_ServesLocalVueRuntime(t *testing.T) {
	r, _ := newTestRouter(t)

	rec := doJSON(t, r, http.MethodGet, "/assets/vue.global.prod.js", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
	if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable asset caching", cache)
	}
	if !strings.Contains(rec.Body.String(), "vue v") {
		t.Error("Vue runtime response missing version banner")
	}
}
