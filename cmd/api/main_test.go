package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheckEndpoint(t *testing.T) {
	app := SetupApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test health endpoint: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if body["status"] != float64(http.StatusOK) {
		t.Errorf("Expected body status to be %d, got %v", http.StatusOK, body["status"])
	}

	if body["message"] != "healthy" {
		t.Errorf("Expected body message to be 'healthy', got %v", body["message"])
	}
}
