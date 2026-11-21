package api

import (
	"context"
	"encoding/json"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/ai"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/database"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	store *database.Store
	ai    *ai.Client
	token string
}

func New(store *database.Store, aiClient *ai.Client, token string) *Server {
	return &Server{store, aiClient, token}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.Handle("/api/", s.auth(http.HandlerFunc(s.api)))
	return mux
}
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.Header.Get("Authorization") != "Bearer "+s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func write(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	err := s.ai.Healthy(ctx)
	status, _ := s.store.Status(r.Context())
	write(w, map[string]any{"ok": err == nil, "ollama": err == nil, "automation": status})
}
func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/status":
		status, err := s.store.Status(r.Context())
		if err != nil {
			http.Error(w, "database unavailable", 500)
			return
		}
		write(w, map[string]string{"automation": status})
	case r.Method == "POST" && r.URL.Path == "/api/automation/pause":
		s.setStatus(w, r, "paused")
	case r.Method == "POST" && r.URL.Path == "/api/automation/resume":
		s.setStatus(w, r, "running")
	case r.Method == "POST" && r.URL.Path == "/api/automation/stop":
		s.setStatus(w, r, "stopped")
	case r.Method == "POST" && r.URL.Path == "/api/chat/test":
		s.testChat(w, r)
	case r.Method == "GET" && r.URL.Path == "/api/conversations":
		write(w, []any{})
	case strings.HasPrefix(r.URL.Path, "/api/conversations/") || strings.HasPrefix(r.URL.Path, "/api/users/"):
		http.Error(w, "not found", 404)
	default:
		http.NotFound(w, r)
	}
}
func (s *Server) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	if err := s.store.SetStatus(r.Context(), status); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	write(w, map[string]string{"automation": status})
}
func (s *Server) testChat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil || strings.TrimSpace(in.Message) == "" {
		http.Error(w, "message is required", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Second)
	defer cancel()
	reply, err := s.ai.Generate(ctx, in.Message)
	if err != nil {
		http.Error(w, "local inference failed: "+err.Error(), 502)
		return
	}
	write(w, map[string]string{"reply": reply})
}
