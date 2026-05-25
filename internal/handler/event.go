package handler

import (
	"context"
	"encoding/json"
	"net/http"
)

type EventService interface {
	Create(ctx context.Context, eventType, payload string) error
}

type EventHandler struct {
	service EventService
}

func NewEventHandler(service EventService) *EventHandler {
	return &EventHandler{service: service}
}

type CreateEventRequest struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

func (req CreateEventRequest) validate() []string {
	var errs []string
	if req.Type == "" {
		errs = append(errs, "type is required")
	}
	if req.Payload == "" {
		errs = append(errs, "payload is required")
	}
	return errs
}

func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := req.validate(); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string][]string{
			"errors": errs,
		})
		return
	}

	if err := h.service.Create(r.Context(), req.Type, req.Payload); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create event")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
	})
}
