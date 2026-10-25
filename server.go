package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// NewServer builds the HTTP contract:
//
//	GET  /deployments/{namespace}/{name}  → 200 Deployment JSON
//	POST /scale {"namespace","name","replicas"} → 200 updated Deployment JSON
//
// Errors are JSON {"error": "..."} with 400 (invalid), 404 (not found),
// or 500. Each request runs under a 5-second context so a hung backend
// cannot pin a handler forever.
func NewServer(s Scaler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /deployments/{namespace}/{name}", handleGet(s))
	mux.HandleFunc("POST /scale", handleScale(s))
	return mux
}

type scaleRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int32  `json:"replicas"`
}

func handleGet(s Scaler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := contextWithTimeout(r)
		defer cancel()
		d, err := s.GetReplicas(ctx, r.PathValue("namespace"), r.PathValue("name"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}

func handleScale(s Scaler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req scaleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON with namespace, name, replicas"})
			return
		}
		ctx, cancel := contextWithTimeout(r)
		defer cancel()
		d, err := s.Scale(ctx, req.Namespace, req.Name, req.Replicas)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}

func contextWithTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Second)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
