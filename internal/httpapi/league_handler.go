package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/AysuKeskin/football-league-simulator/internal/service"
)

// leagueHandler holds the service the league routes delegate to. Handlers
// stay thin: parse input, call the service, map the result or error.
type leagueHandler struct {
	svc *service.LeagueService
}

func (h leagueHandler) create(c *gin.Context) {
	var req createLeagueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		write(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	league, err := h.svc.CreateLeague(c.Request.Context(), service.CreateLeagueInput{
		Name:    req.Name,
		TeamIDs: req.TeamIDs,
		Seed:    req.Seed,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toLeagueResponse(league))
}

func (h leagueHandler) list(c *gin.Context) {
	leagues, err := h.svc.ListLeagues(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]leagueResponse, len(leagues))
	for i := range leagues {
		out[i] = toLeagueResponse(&leagues[i])
	}
	c.JSON(http.StatusOK, out)
}

func (h leagueHandler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	league, err := h.svc.GetLeague(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toLeagueResponse(league))
}

func (h leagueHandler) playWeek(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := h.svc.PlayWeek(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPlayResponse(res))
}

func (h leagueHandler) playAll(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := h.svc.PlayAll(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPlayResponse(res))
}

func (h leagueHandler) reset(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Reset(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h leagueHandler) recalculate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := h.svc.Recalculate(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toStandingResponses(rows))
}

func (h leagueHandler) standings(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := h.svc.Standings(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toStandingResponses(rows))
}

func (h leagueHandler) fixtures(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	matches, err := h.svc.Fixtures(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, fixturesResponse{Weeks: groupByWeek(matches)})
}

func (h leagueHandler) weekDetail(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	week, err := strconv.Atoi(c.Param("week"))
	if err != nil || week < 1 {
		write(c, http.StatusBadRequest, "INVALID_INPUT", "week must be a positive integer")
		return
	}

	detail, err := h.svc.WeekDetail(c.Request.Context(), id, week)
	if err != nil {
		respondError(c, err)
		return
	}
	ms := make([]matchResponse, len(detail.Matches))
	for i, m := range detail.Matches {
		ms[i] = toMatchResponse(m)
	}
	c.JSON(http.StatusOK, weekDetailResponse{
		Week:      detail.Week,
		Matches:   ms,
		Standings: toStandingResponses(detail.Standings),
	})
}

// parseID extracts the :id path param as int64, writing a 400 and
// returning ok=false when it is missing or malformed.
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		write(c, http.StatusBadRequest, "INVALID_INPUT", "id must be a positive integer")
		return 0, false
	}
	return id, true
}
