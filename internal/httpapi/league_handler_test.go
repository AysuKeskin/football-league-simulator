package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AysuKeskin/football-league-simulator/internal/fixture"
	"github.com/AysuKeskin/football-league-simulator/internal/httpapi"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
	"github.com/AysuKeskin/football-league-simulator/internal/service"
	"github.com/AysuKeskin/football-league-simulator/internal/simulation"
	"github.com/AysuKeskin/football-league-simulator/internal/standings"
)

// fakePinger lets the router build without a real DB ping for /ready.
type fakePinger struct{}

func (fakePinger) Ping(context.Context) error { return nil }

// newTestRouter wires the real service over a dbtest pool and seeds four
// teams so CreateLeague's default path works.
func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.New(t)

	for _, name := range []string{"Alpha", "Bravo", "Charlie", "Delta"} {
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO teams (name, attack, midfield, defense) VALUES ($1, 80, 80, 80)`, name,
		); err != nil {
			t.Fatalf("seed team %s: %v", name, err)
		}
	}

	repos := postgres.NewRepositories(pool)
	tx := postgres.NewTransactor(pool)
	leagueSvc := service.NewLeagueService(repos, tx, fixture.New(), simulation.New(), standings.New())
	matchSvc := service.NewMatchService(repos, tx, standings.New())
	teamSvc := service.NewTeamService(repos)
	return httpapi.NewRouter(fakePinger{}, leagueSvc, matchSvc, teamSvc), pool
}

func doJSON(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestLeagueLifecycle drives the PDF's core flow end-to-end over HTTP:
// create → play-all → standings → confirm FINISHED.
func TestLeagueLifecycle(t *testing.T) {
	r, _ := newTestRouter(t)

	// Create with default teams.
	rec := doJSON(t, r, http.MethodPost, "/api/v1/leagues", `{"name":"E2E"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID         int64  `json:"id"`
		TotalWeeks int    `json:"totalWeeks"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.TotalWeeks != 6 || created.Status != "NOT_STARTED" {
		t.Fatalf("unexpected created league: %+v", created)
	}

	// Play all.
	id := created.ID
	rec = doJSON(t, r, http.MethodPost, path(id, "play-all"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("play-all status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var play struct {
		Status      string `json:"status"`
		CurrentWeek int    `json:"currentWeek"`
		PlayedWeeks []struct {
			Week int `json:"week"`
		} `json:"playedWeeks"`
		Standings []struct {
			Rank   int `json:"rank"`
			Points int `json:"points"`
		} `json:"standings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &play); err != nil {
		t.Fatalf("decode play-all: %v", err)
	}
	if play.Status != "FINISHED" || play.CurrentWeek != 6 || len(play.PlayedWeeks) != 6 {
		t.Errorf("unexpected play-all result: %+v", play)
	}
	if len(play.Standings) != 4 || play.Standings[0].Rank != 1 {
		t.Errorf("standings malformed: %+v", play.Standings)
	}

	// League now reports FINISHED via GET.
	rec = doJSON(t, r, http.MethodGet, path(id, ""), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}
}

func TestCreateLeague_ValidationError(t *testing.T) {
	r, _ := newTestRouter(t)

	// Missing required "name" → 400 with INVALID_INPUT.
	rec := doJSON(t, r, http.MethodPost, "/api/v1/leagues", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "INVALID_INPUT" {
		t.Errorf("error code = %q, want INVALID_INPUT", body.Error.Code)
	}
}

func TestGetLeague_NotFound(t *testing.T) {
	r, _ := newTestRouter(t)

	rec := doJSON(t, r, http.MethodGet, "/api/v1/leagues/999999", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

func TestPlayAll_OnFinishedReturnsConflict(t *testing.T) {
	r, _ := newTestRouter(t)

	rec := doJSON(t, r, http.MethodPost, "/api/v1/leagues", `{"name":"Once"}`)
	var created struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)

	if rec := doJSON(t, r, http.MethodPost, path(created.ID, "play-all"), ""); rec.Code != http.StatusOK {
		t.Fatalf("first play-all status = %d", rec.Code)
	}
	rec = doJSON(t, r, http.MethodPost, path(created.ID, "play-all"), "")
	if rec.Code != http.StatusConflict {
		t.Errorf("second play-all status = %d, want 409", rec.Code)
	}
}

// path builds /api/v1/leagues/{id}[/suffix].
func path(id int64, suffix string) string {
	base := "/api/v1/leagues/" + strconv.FormatInt(id, 10)
	if suffix == "" {
		return base
	}
	return base + "/" + suffix
}
