package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// errorBody is the single error envelope every failing endpoint returns.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// respondError maps a domain sentinel error to an HTTP status and a
// stable error code. Unknown errors become 500 with their message
// suppressed — internal failure detail must not leak to clients.
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		write(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		write(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, domain.ErrConflict):
		write(c, http.StatusConflict, "CONFLICT", err.Error())
	default:
		write(c, http.StatusInternalServerError, "INTERNAL", "internal server error")
	}
}

func write(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
