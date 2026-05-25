package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestStandingsHistory_HTTP(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Hist"}`)
	if rec := doJSON(t, r, http.MethodPost, path(id, "play-all"), ""); rec.Code != http.StatusOK {
		t.Fatalf("play-all status = %d", rec.Code)
	}

	rec := doJSON(t, r, http.MethodGet, path(id, "standings/history"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var history []struct {
		Week       int    `json:"week"`
		CapturedAt string `json:"capturedAt"`
		Standings  []struct {
			Rank int `json:"rank"`
		} `json:"standings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(history) != 6 {
		t.Fatalf("history weeks = %d, want 6", len(history))
	}
	for i, w := range history {
		if w.Week != i+1 {
			t.Errorf("history[%d].week = %d, want %d (ascending)", i, w.Week, i+1)
		}
		if w.CapturedAt == "" {
			t.Errorf("week %d missing capturedAt", w.Week)
		}
		if len(w.Standings) != 4 {
			t.Errorf("week %d has %d rows, want 4", w.Week, len(w.Standings))
		}
	}
}

func TestStandingsHistory_EmptyBeforePlay(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Fresh"}`)

	rec := doJSON(t, r, http.MethodGet, path(id, "standings/history"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := rec.Body.String(); body != "[]" {
		t.Errorf("body = %s, want [] for unplayed league", body)
	}
}

func TestStandingsHistory_NotFound(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := doJSON(t, r, http.MethodGet, "/api/v1/leagues/999999/standings/history", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
