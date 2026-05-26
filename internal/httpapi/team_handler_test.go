package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// createLeague is defined in prediction_handler_test.go (same test
// package) and reused here.

// firstTeamID returns the id of the first team in a league via the
// GET /leagues/:id/teams endpoint.
func firstTeamID(t *testing.T, r http.Handler, leagueID int64) int64 {
	t.Helper()
	rec := doJSON(t, r, http.MethodGet, path(leagueID, "teams"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list teams status = %d", rec.Code)
	}
	var teams []struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &teams)
	if len(teams) == 0 {
		t.Fatal("league has no teams")
	}
	return teams[0].ID
}

func teamRatingsPath(id int64) string {
	return "/api/v1/teams/" + strconv.FormatInt(id, 10) + "/ratings"
}

func TestListLeagueTeams_HTTP(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Teams"}`)

	rec := doJSON(t, r, http.MethodGet, path(id, "teams"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var teams []struct {
		Name    string `json:"name"`
		Attack  int    `json:"attack"`
		Defense int    `json:"defense"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &teams); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(teams) != 4 {
		t.Fatalf("teams = %d, want 4", len(teams))
	}
	if teams[0].Name == "" || teams[0].Attack == 0 {
		t.Errorf("team ratings not populated: %+v", teams[0])
	}
}

func TestUpdateRating_HTTP(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Rate"}`)
	teamID := firstTeamID(t, r, id)

	rec := doJSON(t, r, http.MethodPatch, teamRatingsPath(teamID), `{"attack":99,"midfield":88,"defense":77}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var team struct {
		Attack   int `json:"attack"`
		Midfield int `json:"midfield"`
		Defense  int `json:"defense"`
	}
	json.Unmarshal(rec.Body.Bytes(), &team)
	if team.Attack != 99 || team.Midfield != 88 || team.Defense != 77 {
		t.Errorf("updated ratings = %+v, want 99/88/77", team)
	}
}

func TestUpdateRating_MissingField(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Rate2"}`)
	teamID := firstTeamID(t, r, id)

	// midfield omitted → 400.
	rec := doJSON(t, r, http.MethodPatch, teamRatingsPath(teamID), `{"attack":80,"defense":80}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for missing field", rec.Code)
	}
}

func TestUpdateRating_OutOfRange(t *testing.T) {
	r, _ := newTestRouter(t)
	id := createLeague(t, r, `{"name":"Rate3"}`)
	teamID := firstTeamID(t, r, id)

	rec := doJSON(t, r, http.MethodPatch, teamRatingsPath(teamID), `{"attack":150,"midfield":80,"defense":80}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for out-of-range attack", rec.Code)
	}
}

func TestUpdateRating_NotFound(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := doJSON(t, r, http.MethodPatch, "/api/v1/teams/999999/ratings", `{"attack":80,"midfield":80,"defense":80}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestListCatalog_HTTP(t *testing.T) {
	r, _ := newTestRouter(t)
	// newTestRouter seeds 4 teams.
	rec := doJSON(t, r, http.MethodGet, "/api/v1/teams", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var teams []struct {
		Name string `json:"name"`
	}
	json.Unmarshal(rec.Body.Bytes(), &teams)
	if len(teams) != 4 {
		t.Errorf("catalog = %d teams, want 4", len(teams))
	}
}
