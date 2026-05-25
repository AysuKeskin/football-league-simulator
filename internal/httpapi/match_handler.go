package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/AysuKeskin/football-league-simulator/internal/service"
)

// matchHandler serves the match-editing routes.
type matchHandler struct {
	svc *service.MatchService
}

func (h matchHandler) updateResult(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req updateResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		write(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	res, err := h.svc.UpdateResult(c.Request.Context(), id, service.UpdateResultInput{
		HomeGoals: *req.HomeGoals,
		AwayGoals: *req.AwayGoals,
		Reason:    req.Reason,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toEditResultResponse(res))
}

func (h matchHandler) audit(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	history, err := h.svc.GetAudit(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAuditResponses(history))
}
