package handler

import (
	"encoding/json"
	"net/http"
)

type EventHandler struct{}

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

func (h EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
	})
}
