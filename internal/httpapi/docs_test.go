package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestOpenAPISpec_Served(t *testing.T) {
	r, _ := newTestRouter(t)

	rec := doJSON(t, r, http.MethodGet, "/openapi.yaml", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "openapi: 3") {
		t.Error("spec missing OpenAPI 3 version marker")
	}
	if !strings.Contains(body, "/api/v1/leagues") {
		t.Error("spec does not document the leagues paths")
	}
	if !strings.Contains(body, "PredictionResponse") {
		t.Error("spec missing component schemas")
	}
}

func TestSwaggerUI_Served(t *testing.T) {
	r, _ := newTestRouter(t)

	rec := doJSON(t, r, http.MethodGet, "/swagger", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "swagger-ui") || !strings.Contains(body, "/openapi.yaml") {
		t.Errorf("swagger page missing UI assets or spec reference")
	}
}
