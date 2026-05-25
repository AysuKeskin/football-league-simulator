package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// playedLeague creates a league via HTTP, plays it all, and returns the
// league id so editing tests have played matches to mutate.
func playedLeague(t *testing.T, r http.Handler) int64 {
	t.Helper()
	rec := doJSON(t, r, http.MethodPost, "/api/v1/leagues", `{"name":"Edit","seed":7}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if rec := doJSON(t, r, http.MethodPost, path(created.ID, "play-all"), ""); rec.Code != http.StatusOK {
		t.Fatalf("play-all status = %d", rec.Code)
	}
	return created.ID
}

// firstMatchID reads the league's fixtures over HTTP and returns the id
// of the first match in week 1.
func firstMatchID(t *testing.T, r http.Handler, leagueID int64) int64 {
	t.Helper()
	rec := doJSON(t, r, http.MethodGet, path(leagueID, "fixtures"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("fixtures status = %d", rec.Code)
	}
	var body struct {
		Weeks []struct {
			Week    int `json:"week"`
			Matches []struct {
				ID int64 `json:"id"`
			} `json:"matches"`
		} `json:"weeks"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	for _, w := range body.Weeks {
		if w.Week == 1 {
			return w.Matches[0].ID
		}
	}
	t.Fatal("no week-1 match found")
	return 0
}

func TestUpdateMatchResult_HTTP(t *testing.T) {
	r, _ := newTestRouter(t)
	leagueID := playedLeague(t, r)
	matchID := firstMatchID(t, r, leagueID)

	body := `{"homeGoals":5,"awayGoals":0,"reason":"manual correction after review"}`
	rec := doJSON(t, r, http.MethodPut, matchPath(matchID, ""), body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var edit struct {
		Match struct {
			HomeGoals *int   `json:"homeGoals"`
			AwayGoals *int   `json:"awayGoals"`
			Status    string `json:"status"`
		} `json:"match"`
		Standings []struct {
			Rank int `json:"rank"`
		} `json:"standings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &edit); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if edit.Match.HomeGoals == nil || *edit.Match.HomeGoals != 5 || *edit.Match.AwayGoals != 0 {
		t.Errorf("match not updated: %+v", edit.Match)
	}
	if len(edit.Standings) != 4 {
		t.Errorf("standings len = %d, want 4", len(edit.Standings))
	}
}

func TestUpdateMatchResult_MissingGoalsRejected(t *testing.T) {
	r, _ := newTestRouter(t)
	leagueID := playedLeague(t, r)
	matchID := firstMatchID(t, r, leagueID)

	// homeGoals omitted → 400 (pointer + binding:"required").
	rec := doJSON(t, r, http.MethodPut, matchPath(matchID, ""), `{"awayGoals":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for missing homeGoals", rec.Code)
	}
}

func TestUpdateMatchResult_ZeroGoalsAccepted(t *testing.T) {
	r, _ := newTestRouter(t)
	leagueID := playedLeague(t, r)
	matchID := firstMatchID(t, r, leagueID)

	// Explicit 0-0 must be accepted (pointer distinguishes 0 from absent).
	rec := doJSON(t, r, http.MethodPut, matchPath(matchID, ""), `{"homeGoals":0,"awayGoals":0}`)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for explicit 0-0; body=%s", rec.Code, rec.Body.String())
	}
}

func TestMatchAudit_HTTP(t *testing.T) {
	r, _ := newTestRouter(t)
	leagueID := playedLeague(t, r)
	matchID := firstMatchID(t, r, leagueID)

	doJSON(t, r, http.MethodPut, matchPath(matchID, ""), `{"homeGoals":3,"awayGoals":1,"reason":"first"}`)
	doJSON(t, r, http.MethodPut, matchPath(matchID, ""), `{"homeGoals":2,"awayGoals":2,"reason":"second"}`)

	rec := doJSON(t, r, http.MethodGet, matchPath(matchID, "audit"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit status = %d", rec.Code)
	}
	var history []struct {
		NewHomeGoals int    `json:"newHomeGoals"`
		Reason       string `json:"reason"`
		ChangedAt    string `json:"changedAt"`
	}
	json.Unmarshal(rec.Body.Bytes(), &history)
	if len(history) != 2 {
		t.Fatalf("audit entries = %d, want 2", len(history))
	}
	// Newest first.
	if history[0].Reason != "second" || history[0].ChangedAt == "" {
		t.Errorf("unexpected newest audit entry: %+v", history[0])
	}
}

func TestRecalculate_HTTP(t *testing.T) {
	r, _ := newTestRouter(t)
	leagueID := playedLeague(t, r)

	rec := doJSON(t, r, http.MethodPost, path(leagueID, "recalculate"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("recalculate status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var standings []struct {
		Rank int `json:"rank"`
	}
	json.Unmarshal(rec.Body.Bytes(), &standings)
	if len(standings) != 4 {
		t.Errorf("recalculated standings len = %d, want 4", len(standings))
	}
}

func TestUpdateMatchResult_NotFound(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := doJSON(t, r, http.MethodPut, "/api/v1/matches/999999", `{"homeGoals":1,"awayGoals":0}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// matchPath builds /api/v1/matches/{id}[/suffix].
func matchPath(id int64, suffix string) string {
	base := "/api/v1/matches/" + strconv.FormatInt(id, 10)
	if suffix == "" {
		return base
	}
	return base + "/" + suffix
}