package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/AysuKeskin/football-league-simulator/internal/service"
)

// predictionHandler serves the championship-prediction route. It is its
// own handler (not folded into leagueHandler) because it is backed by a
// distinct service — one handler per service, as elsewhere.
type predictionHandler struct {
	svc *service.PredictionService
}

func (h predictionHandler) predictions(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	// Absent or non-numeric simulations → 0, which the service treats as
	// "use the default". A negative is likewise normalized by the service.
	sims, _ := strconv.Atoi(c.Query("simulations"))

	res, err := h.svc.Predict(c.Request.Context(), id, sims)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPredictionResponse(res))
}
