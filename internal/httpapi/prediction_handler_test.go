package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// playWeeks plays the given number of weeks on a league over HTTP.
func playWeeks(t *testing.T, r http.Handler, leagueID int64, weeks int) {
	t.Helper()
	for i := 0; i < weeks; i++ {
		if rec := doJSON(t, r, http.MethodPost, path(leagueID, "play-week"), ""); rec.Code != http.StatusOK {
			t.Fatalf("play-week %d status = %d", i+1, rec.Code)
		}
	}
}

// createLeague creates a league via HTTP and returns its id.
func createLeague(t *testing.T, r http.Handler, body string) int64 {
	t.Helper()
	rec := doJSON(t, r, http.MethodPost, "/api/v1/leagues", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

func TestPredictions_GatedBeforeWeek4(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Early"}`)
	playWeeks(t, r, id, 3)

	rec := doJSON(t, r, http.MethodGet, path(id, "predictions"), "")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 before week 4; body = %s", rec.Code, rec.Body.String())
	}
}

func TestPredictions_InProgressShape(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Mid","seed":3}`)
	playWeeks(t, r, id, 4)

	rec := doJSON(t, r, http.MethodGet, path(id, "predictions")+"?simulations=500", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Week        int  `json:"week"`
		Simulations int  `json:"simulations"`
		Finished    bool `json:"finished"`
		Predictions []struct {
			Team               string  `json:"team"`
			ChampionshipChance float64 `json:"championshipChance"`
		} `json:"predictions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Finished || body.Week != 4 || body.Simulations != 500 {
		t.Errorf("unexpected meta: %+v", body)
	}
	if len(body.Predictions) != 4 {
		t.Fatalf("predictions = %d, want 4", len(body.Predictions))
	}
}

func TestPredictions_FinishedShape(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Done"}`)
	if rec := doJSON(t, r, http.MethodPost, path(id, "play-all"), ""); rec.Code != http.StatusOK {
		t.Fatalf("play-all status = %d", rec.Code)
	}

	rec := doJSON(t, r, http.MethodGet, path(id, "predictions"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Finished       bool   `json:"finished"`
		Champion       string `json:"champion"`
		FinalStandings []struct {
			Rank int `json:"rank"`
		} `json:"finalStandings"`
		Predictions []json.RawMessage `json:"predictions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Finished || body.Champion == "" || len(body.FinalStandings) != 4 {
		t.Errorf("unexpected finished shape: %+v", body)
	}
	if len(body.Predictions) != 0 {
		t.Errorf("finished response should omit predictions, got %d", len(body.Predictions))
	}
}

func TestPredictions_NotFound(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := doJSON(t, r, http.MethodGet, "/api/v1/leagues/999999/predictions", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
