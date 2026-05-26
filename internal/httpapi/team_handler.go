package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/AysuKeskin/football-league-simulator/internal/service"
)

// teamHandler serves the team-catalog routes (rating updates and listing
// a league's teams), backed by its own service.
type teamHandler struct {
	svc *service.TeamService
}

func (h teamHandler) updateRating(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req updateRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		write(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	team, err := h.svc.UpdateRating(c.Request.Context(), id, service.UpdateRatingInput{
		Attack:   req.Attack,
		Midfield: req.Midfield,
		Defense:  req.Defense,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTeamResponse(*team))
}

func (h teamHandler) listByLeague(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	teams, err := h.svc.ListByLeague(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTeamResponses(teams))
}

func (h teamHandler) listCatalog(c *gin.Context) {
	teams, err := h.svc.ListCatalog(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTeamResponses(teams))
}
