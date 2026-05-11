package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"whale/internal/memory"
	"whale/internal/runtime"
	"whale/internal/session"
)

type Server struct {
	runtime *runtime.AgentRuntime
	mux     *http.ServeMux
}

type ChatRequest struct {
	WindowID   string `json:"window_id"`
	UserInput  string `json:"user_input"`
	Role       string `json:"role,omitempty"`
	PromptMode string `json:"prompt_mode,omitempty"`
}

type ChatResponse struct {
	WindowID         string                 `json:"window_id"`
	SessionID        string                 `json:"session_id"`
	TurnID           int                    `json:"turn_id"`
	AgentRole        string                 `json:"agent_role,omitempty"`
	Reply            string                 `json:"reply"`
	Trace            []runtime.TraceStep    `json:"trace"`
	RecalledMemories []memory.MemorySnippet `json:"recalled_memories,omitempty"`
	ThreadSummary    any                    `json:"thread_summary,omitempty"`
}

type DreamRequest struct {
	WindowID string `json:"window_id,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

type DreamResponse struct {
	Mode               string                `json:"mode,omitempty"`
	WindowID           string                `json:"window_id,omitempty"`
	ProcessedCount     int                   `json:"processed_count"`
	Summaries          []string              `json:"summaries"`
	Trace              []runtime.TraceStep   `json:"trace"`
	ThreadStates       []session.ThreadState `json:"thread_states,omitempty"`
	ProcessedThreadIDs []string              `json:"processed_thread_ids,omitempty"`
	UpdatedMemoryFiles []string              `json:"updated_memory_files,omitempty"`
	DecisionCount      int                   `json:"decision_count,omitempty"`
}

type WindowCreateRequest struct {
	Title string `json:"title"`
}

type WindowUpdateRequest struct {
	Title string `json:"title"`
}

type RoleUpdateRequest struct {
	Name    string `json:"name,omitempty"`
	Persona string `json:"persona,omitempty"`
	Avatar  string `json:"avatar,omitempty"`
}

func NewServer(rt *runtime.AgentRuntime) *Server {
	s := &Server{runtime: rt, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/windows", s.handleWindows)
	s.mux.HandleFunc("/api/windows/", s.handleWindowByID)
	s.mux.HandleFunc("/api/chat", s.handleChat)
	s.mux.HandleFunc("/api/dream", s.handleDream)
	s.mux.HandleFunc("/api/memory/summary", s.handleMemorySummary)
	s.mux.HandleFunc("/api/roles", s.handleRoles)
	s.mux.HandleFunc("/api/roles/", s.handleRoleByID)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	respondJSON(w, map[string]any{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

func (s *Server) handleWindows(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		store, err := s.runtime.Session.LoadWindows()
		if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
		respondJSON(w, store)
	case http.MethodPost:
		var req WindowCreateRequest
		if err := decodeJSON(r, &req); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
		win, err := s.runtime.Session.CreateWindow(req.Title)
		if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
		respondJSON(w, win)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWindowByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/windows/")
	if id == "" { http.NotFound(w, r); return }
	parts := strings.Split(strings.Trim(id, "/"), "/")
	id = parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPatch:
			var req WindowUpdateRequest
			if err := decodeJSON(r, &req); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
			if err := s.runtime.Session.RenameWindow(id, req.Title); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
			win, err := s.runtime.Session.GetWindow(id)
			if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
			respondJSON(w, win)
		case http.MethodDelete:
			if err := s.runtime.Session.DeleteWindow(id); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
			respondJSON(w, map[string]any{"deleted": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) >= 2 {
		switch parts[1] {
		case "state":
			if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
			state, _, _ := s.runtime.Session.GetThreadState(id)
			win, _ := s.runtime.Session.GetWindow(id)
			summary := s.readWindowSummary(id)
			respondJSON(w, map[string]any{"window": win, "thread_state": state, "summary": summary})
		case "messages":
			if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
			turns, err := s.runtime.Session.ReadWindowRawTurns(id)
			if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
			respondJSON(w, turns)
		case "recall-preview":
			if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
			respondJSON(w, map[string]any{"window_id": id, "items": s.previewRecall(id)})
		default:
			http.NotFound(w, r)
		}
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var req ChatRequest
	if err := decodeJSON(r, &req); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	if strings.TrimSpace(req.WindowID) == "" { http.Error(w, "window_id is required", http.StatusBadRequest); return }
	snippets, _ := s.runtime.Memory.LoadContext(req.UserInput)
	result, err := s.runtime.HandleInput(req.WindowID, req.UserInput, req.Role)
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	threadSummary := s.readWindowSummary(req.WindowID)
	respondJSON(w, ChatResponse{WindowID: result.WindowID, SessionID: result.SessionID, TurnID: result.TurnID, AgentRole: result.AgentRole, Reply: result.Reply, Trace: result.Trace, RecalledMemories: snippets, ThreadSummary: threadSummary})
}

func (s *Server) handleDream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var req DreamRequest
	if err := decodeJSON(r, &req); err != nil && err.Error() != "EOF" { http.Error(w, err.Error(), http.StatusBadRequest); return }
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" { mode = "global" }
	var (
		result runtime.AutodreamResult
		err    error
	)
	switch mode {
	case "thread":
		if strings.TrimSpace(req.WindowID) == "" { http.Error(w, "window_id is required for thread mode", http.StatusBadRequest); return }
		result, err = s.runtime.RunAutodreamForThread(req.WindowID)
	case "global":
		result, err = s.runtime.RunAutodream()
	default:
		http.Error(w, "invalid dream mode", http.StatusBadRequest)
		return
	}
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	states, _ := s.runtime.Session.DescribeThreads()
	respondJSON(w, DreamResponse{Mode: result.Mode, WindowID: result.WindowID, ProcessedCount: result.ProcessedCount, Summaries: result.Summaries, Trace: result.Trace, ThreadStates: states, ProcessedThreadIDs: result.ProcessedThreadIDs, UpdatedMemoryFiles: result.UpdatedMemoryFiles, DecisionCount: result.DecisionCount})
}

func (s *Server) handleMemorySummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	idx, err := s.runtime.Memory.LoadMemoryIndex()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	respondJSON(w, map[string]any{"generated_at": idx.GeneratedAt, "count": len(idx.Entries), "entries": idx.Entries})
}

func (s *Server) handleRoles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		roles, err := s.runtime.LoadRoles()
		if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
		respondJSON(w, map[string]any{"roles": roles})
	case http.MethodPost:
		if err := r.ParseMultipartForm(8 << 20); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
		name := strings.TrimSpace(r.FormValue("name"))
		persona := strings.TrimSpace(r.FormValue("persona"))
		var avatarPath string
		file, header, err := r.FormFile("avatar")
		if err == nil {
			avatarPath, err = s.runtime.SaveAvatarUpload(name, file, header)
			if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
		}
		role, err := s.runtime.CreateRole(name, persona, avatarPath)
		if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
		respondJSON(w, role)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRoleByID(w http.ResponseWriter, r *http.Request) {
	roleID := strings.TrimPrefix(r.URL.Path, "/api/roles/")
	roleID = strings.TrimSpace(strings.Trim(roleID, "/"))
	if roleID == "" { http.NotFound(w, r); return }
	switch r.Method {
	case http.MethodPatch:
		var req RoleUpdateRequest
		if err := decodeJSON(r, &req); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
		role, err := s.runtime.UpdateRole(roleID, req.Name, req.Persona, req.Avatar)
		if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
		respondJSON(w, role)
	case http.MethodDelete:
		if err := s.runtime.DeleteRole(roleID); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
		respondJSON(w, map[string]any{"deleted": true, "role_id": roleID})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) readWindowSummary(windowID string) any {
	path := filepath.Join(s.runtime.RootDir, "data", "sessions", "summaries", fmt.Sprintf("%s.summary.json", windowID))
	var summary any
	if err := s.runtime.Memory.ReadJSON(path, &summary); err != nil { return nil }
	return summary
}

func (s *Server) previewRecall(windowID string) []memory.MemorySnippet {
	turns, err := s.runtime.Session.ReadWindowRawTurns(windowID)
	if err != nil || len(turns) == 0 { return nil }
	input := turns[len(turns)-1].UserInput
	snippets, _ := s.runtime.Memory.LoadContext(input)
	return snippets
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
