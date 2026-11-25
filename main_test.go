package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testSecret = "test-webhook-secret"

func TestReceiveEvent(t *testing.T) {
	router := NewRouter(testSecret)
	body := []byte(`{"event_id":"evt_123","event_type":"invoice.paid","timestamp":"2026-01-02T03:04:05Z","data":{"amount":42,"currency":"USD"}}`)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewReader(body))
	request.Header.Set("X-Signature", sign(body, testSecret))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, response.Code, response.Body)
	}

	var event EventPayload
	if err := json.Unmarshal(response.Body.Bytes(), &event); err != nil {
		t.Fatalf("decode accepted event: %v", err)
	}
	if event.EventID != "evt_123" || event.EventType != "invoice.paid" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if !event.Timestamp.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("unexpected timestamp: %s", event.Timestamp)
	}
	if string(event.Data) != `{"amount":42,"currency":"USD"}` {
		t.Fatalf("unexpected data: %s", event.Data)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/events/evt_123", nil)
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, getResponse.Code, getResponse.Body)
	}
}

func TestReceiveEventRejectsInvalidSignature(t *testing.T) {
	for _, signature := range []string{"", "sha256=invalid"} {
		t.Run(signature, func(t *testing.T) {
			router := NewRouter(testSecret)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewBufferString(`{}`))
			request.Header.Set("X-Signature", signature)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
			}
		})
	}
}

func TestReceiveEventRejectsMalformedJSONWithValidSignature(t *testing.T) {
	router := NewRouter(testSecret)
	body := []byte(`{"event_id":`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewReader(body))
	request.Header.Set("X-Signature", sign(body, testSecret))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestGetEventNotFound(t *testing.T) {
	router := NewRouter(testSecret)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events/missing", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
