package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// EventPayload is the event envelope accepted by the webhook endpoint.
type EventPayload struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// ErrorResponse is returned when a request cannot be processed.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

type eventStore struct {
	mu     sync.RWMutex
	events map[string]EventPayload
}

type webhookHandler struct {
	secret string
	store  eventStore
}

func NewRouter(secret string) *gin.Engine {
	handler := &webhookHandler{
		secret: secret,
		store: eventStore{
			events: make(map[string]EventPayload),
		},
	}

	router := gin.New()
	router.Use(gin.Recovery())
	api := router.Group("/api/v1")
	api.POST("/webhooks", handler.receiveEvent)
	api.GET("/events/:event_id", handler.getEvent)
	return router
}

func (h *webhookHandler) receiveEvent(c *gin.Context) {
	body, err := readRequestBody(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "malformed_json", "Request body could not be read")
		return
	}

	if !validSignature(h.secret, body, c.GetHeader("X-Signature")) {
		writeError(c, http.StatusUnauthorized, "invalid_signature", "A valid X-Signature header is required")
		return
	}

	var event EventPayload
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(c, http.StatusBadRequest, "malformed_json", "Request body must be valid JSON with an RFC3339 timestamp")
		return
	}
	if strings.TrimSpace(event.EventID) == "" ||
		strings.TrimSpace(event.EventType) == "" ||
		event.Timestamp.IsZero() ||
		len(event.Data) == 0 {
		writeError(c, http.StatusBadRequest, "invalid_event", "event_id, event_type, timestamp, and data are required")
		return
	}

	h.store.mu.Lock()
	h.store.events[event.EventID] = event
	h.store.mu.Unlock()

	c.JSON(http.StatusAccepted, event)
}

func (h *webhookHandler) getEvent(c *gin.Context) {
	eventID := c.Param("event_id")
	h.store.mu.RLock()
	event, ok := h.store.events[eventID]
	h.store.mu.RUnlock()
	if !ok {
		writeError(c, http.StatusNotFound, "event_not_found", "No event was found for the requested event_id")
		return
	}

	c.JSON(http.StatusOK, event)
}

func readRequestBody(c *gin.Context) ([]byte, error) {
	body, err := c.GetRawData()
	if err != nil {
		return nil, err
	}
	return body, nil
}

func validSignature(secret string, body []byte, signature string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}

	provided, err := hex.DecodeString(strings.TrimPrefix(signature, prefix))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		return false
	}
	expected := mac.Sum(nil)
	return len(provided) == len(expected) && subtle.ConstantTimeCompare(provided, expected) == 1
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: message})
}

func main() {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		gin.DefaultErrorWriter.Write([]byte("WEBHOOK_SECRET must be set\n"))
		os.Exit(1)
	}

	if err := runServer(secret, os.Getenv("PORT")); err != nil && !errors.Is(err, http.ErrServerClosed) {
		gin.DefaultErrorWriter.Write([]byte(err.Error() + "\n"))
		os.Exit(1)
	}
}

func runServer(secret, port string) error {
	if port == "" {
		port = "8080"
	}
	return http.ListenAndServe(":"+port, NewRouter(secret))
}
