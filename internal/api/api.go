package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/ai"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/browser"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/conversation"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/database"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	store     *database.Store
	ai        *ai.Client
	browser   *browser.Manager
	processor *conversation.Processor
	token     string
}

func New(store *database.Store, aiClient *ai.Client, manager *browser.Manager, processor *conversation.Processor, token string) *Server {
	return &Server{store: store, ai: aiClient, browser: manager, processor: processor, token: token}
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
		connected, loggedIn, reason := s.browser.Status()
		write(w, map[string]any{"automation": status, "browser_connected": connected, "instagram_logged_in": loggedIn, "browser_reason": reason})
	case r.Method == "POST" && r.URL.Path == "/api/browser/state":
		s.browserState(w, r)
	case r.Method == "POST" && r.URL.Path == "/api/browser/incoming":
		s.incoming(w, r)
	case r.Method == "POST" && r.URL.Path == "/api/automation/pause":
		s.setStatus(w, r, "paused")
	case r.Method == "POST" && r.URL.Path == "/api/automation/resume":
		s.setStatus(w, r, "running")
	case r.Method == "POST" && r.URL.Path == "/api/automation/stop":
		s.setStatus(w, r, "stopped")
	case r.Method == "POST" && r.URL.Path == "/api/chat/test":
		s.testChat(w, r)
	case r.Method == "GET" && r.URL.Path == "/api/conversations":
		items, err := s.store.Conversations(r.Context())
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		write(w, items)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/conversations/"):
		s.conversation(w, r)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/users/") && strings.HasSuffix(r.URL.Path, "/memories"):
		s.memories(w, r)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/users/") && strings.Contains(r.URL.Path, "/memories/"):
		s.deleteMemory(w, r)
	default:
		http.NotFound(w, r)
	}
}
func pathID(r *http.Request, index int) (int64, bool) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) <= index {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[index], 10, 64)
	return id, err == nil && id > 0
}
func (s *Server) conversation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, 2)
	if !ok {
		http.Error(w, "invalid conversation id", 400)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/messages") || strings.Count(strings.Trim(r.URL.Path, "/"), "/") == 2 {
		items, err := s.store.Messages(r.Context(), id)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		write(w, items)
		return
	}
	http.NotFound(w, r)
}
func (s *Server) memories(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, 2)
	if !ok {
		http.Error(w, "invalid user id", 400)
		return
	}
	items, err := s.store.UserMemories(r.Context(), id)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	write(w, items)
}
func (s *Server) deleteMemory(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(r, 2)
	if !ok {
		http.Error(w, "invalid user id", 400)
		return
	}
	memoryID, ok := pathID(r, 4)
	if !ok {
		http.Error(w, "invalid memory id", 400)
		return
	}
	if err := s.store.DeleteMemory(r.Context(), userID, memoryID); err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) incoming(w http.ResponseWriter, r *http.Request) {
	var in browser.IncomingMessage
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in) != nil {
		http.Error(w, "invalid message", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()
	reply, err := s.processor.Handle(ctx, in)
	if err != nil {
		http.Error(w, "message processing failed: "+err.Error(), 502)
		return
	}
	write(w, map[string]string{"reply": reply})
}
func (s *Server) browserState(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil {
		http.Error(w, "invalid state", 400)
		return
	}
	switch in.Kind {
	case "ready":
		s.browser.Report(true, true, "")
	case "paused":
		s.browser.Report(true, false, in.Reason)
	default:
		http.Error(w, "invalid state", 400)
		return
	}
	write(w, map[string]string{"status": "recorded"})
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
