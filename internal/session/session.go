package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SessionState struct {
	SessionID   string    `json:"session_id"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	RecentInput string    `json:"recent_input,omitempty"`
}

type ConversationWindow struct {
	ID              string    `json:"id"`
	Title           string    `json:"title,omitempty"`
	AgentRole       string    `json:"agent_role,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	TurnCount       int       `json:"turn_count"`
	SummaryPath     string    `json:"summary_path,omitempty"`
	SummarySegments []string  `json:"summary_segment_paths,omitempty"`
	LastActiveAt    time.Time `json:"last_active_at"`
}

type WindowStore struct {
	Windows []ConversationWindow `json:"windows"`
}

type RoleRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Persona     string    `json:"persona"`
	Avatar      string    `json:"avatar,omitempty"`
	Theme       string    `json:"theme,omitempty"`
	Builtin     bool      `json:"builtin"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SystemNotes string    `json:"system_notes,omitempty"`
}

type RoleStore struct {
	Roles []RoleRecord `json:"roles"`
}

type ThreadAutodreamStatus string

const (
	ThreadStatusPending       ThreadAutodreamStatus = "pending"
	ThreadStatusProcessing    ThreadAutodreamStatus = "processing"
	ThreadStatusCompleted     ThreadAutodreamStatus = "completed"
	ThreadStatusPartialFailed ThreadAutodreamStatus = "partial_failed"
	ThreadStatusFailed        ThreadAutodreamStatus = "failed"
)

type ThreadState struct {
	ThreadID             string                `json:"thread_id"`
	AutodreamStatus      ThreadAutodreamStatus `json:"autodream_status"`
	LastRawTurnID        int                   `json:"last_raw_turn_id"`
	LastSummaryUpdatedAt time.Time             `json:"last_summary_updated_at,omitempty"`
	LastAutodreamAt      time.Time             `json:"last_autodream_at,omitempty"`
	LastAutodreamResult  string                `json:"last_autodream_result,omitempty"`
	LastError            string                `json:"last_error,omitempty"`
}

type ThreadStateStore struct {
	Threads []ThreadState `json:"threads"`
}

type RawTurn struct {
	WindowID             string    `json:"window_id"`
	SessionID            string    `json:"session_id"`
	TurnID               int       `json:"turn_id"`
	AgentRole            string    `json:"agent_role,omitempty"`
	UserInput            string    `json:"user_input"`
	AssistantResponse    string    `json:"assistant_response"`
	Timestamp            time.Time `json:"timestamp"`
	Trace                any       `json:"trace"`
	ProcessedByAutodream bool      `json:"processed_by_autodream"`
}

type Manager struct{ RootDir string }

func NewManager(rootDir string) *Manager { return &Manager{RootDir: rootDir} }

func (m *Manager) Initialize() error {
	if err := os.MkdirAll(filepath.Join(m.RootDir, "data", "sessions", "raw"), 0o755); err != nil { return err }
	if err := os.MkdirAll(filepath.Join(m.RootDir, "data", "sessions", "summaries"), 0o755); err != nil { return err }
	if err := m.ensureRoleStore(); err != nil { return err }
	return m.ensureThreadStateFile()
}

func (m *Manager) LoadState() (*SessionState, error) { path := filepath.Join(m.RootDir, "data", "system", "runtime_state.json"); b, err := os.ReadFile(path); if err != nil { return nil, err }; var state SessionState; if len(b) == 0 { return nil, fmt.Errorf("empty session state") }; if err := json.Unmarshal(b, &state); err != nil { return nil, err }; return &state, nil }

func (m *Manager) windowsPath() string     { return filepath.Join(m.RootDir, "data", "sessions", "windows.json") }
func (m *Manager) rawDir() string          { return filepath.Join(m.RootDir, "data", "sessions", "raw") }
func (m *Manager) summaryDir() string      { return filepath.Join(m.RootDir, "data", "sessions", "summaries") }
func (m *Manager) threadStatePath() string { return filepath.Join(m.RootDir, "data", "sessions", "thread_states.json") }
func (m *Manager) roleStorePath() string   { return filepath.Join(m.RootDir, "data", "sessions", "roles.json") }

func (m *Manager) ensureThreadStateFile() error {
	if err := os.MkdirAll(filepath.Dir(m.threadStatePath()), 0o755); err != nil { return err }
	if _, err := os.Stat(m.threadStatePath()); err == nil { return nil }
	store := ThreadStateStore{Threads: []ThreadState{}}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil { return err }
	return os.WriteFile(m.threadStatePath(), b, 0o644)
}

func (m *Manager) ensureRoleStore() error {
	if err := os.MkdirAll(filepath.Dir(m.roleStorePath()), 0o755); err != nil { return err }
	if _, err := os.Stat(m.roleStorePath()); err == nil { return nil }
	store := RoleStore{Roles: []RoleRecord{}}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil { return err }
	return os.WriteFile(m.roleStorePath(), b, 0o644)
}

func (m *Manager) LoadRoleStore() (RoleStore, error) {
	if err := m.ensureRoleStore(); err != nil { return RoleStore{}, err }
	b, err := os.ReadFile(m.roleStorePath())
	if err != nil { return RoleStore{}, err }
	if len(strings.TrimSpace(string(b))) == 0 { return RoleStore{Roles: []RoleRecord{}}, nil }
	var store RoleStore
	if err := json.Unmarshal(b, &store); err != nil { return RoleStore{}, err }
	sort.SliceStable(store.Roles, func(i, j int) bool { return store.Roles[i].CreatedAt.Before(store.Roles[j].CreatedAt) })
	return store, nil
}

func (m *Manager) SaveRoleStore(store RoleStore) error {
	if err := os.MkdirAll(filepath.Dir(m.roleStorePath()), 0o755); err != nil { return err }
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil { return err }
	return os.WriteFile(m.roleStorePath(), b, 0o644)
}

func (m *Manager) LoadThreadStates() (ThreadStateStore, error) {
	if err := m.ensureThreadStateFile(); err != nil { return ThreadStateStore{}, err }
	var store ThreadStateStore
	b, err := os.ReadFile(m.threadStatePath())
	if err != nil { return store, err }
	if len(strings.TrimSpace(string(b))) == 0 { return ThreadStateStore{Threads: []ThreadState{}}, nil }
	if err := json.Unmarshal(b, &store); err != nil { return store, err }
	sort.SliceStable(store.Threads, func(i, j int) bool { return store.Threads[i].ThreadID < store.Threads[j].ThreadID })
	return store, nil
}

func (m *Manager) SaveThreadStates(store ThreadStateStore) error {
	if err := os.MkdirAll(filepath.Dir(m.threadStatePath()), 0o755); err != nil { return err }
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil { return err }
	return os.WriteFile(m.threadStatePath(), b, 0o644)
}

func (m *Manager) UpsertThreadState(state ThreadState) error {
	store, err := m.LoadThreadStates()
	if err != nil { return err }
	updated := false
	for i := range store.Threads {
		if store.Threads[i].ThreadID == state.ThreadID {
			store.Threads[i] = state
			updated = true
			break
		}
	}
	if !updated { store.Threads = append(store.Threads, state) }
	return m.SaveThreadStates(store)
}

func (m *Manager) GetThreadState(threadID string) (ThreadState, bool, error) {
	store, err := m.LoadThreadStates()
	if err != nil { return ThreadState{}, false, err }
	for _, state := range store.Threads { if state.ThreadID == threadID { return state, true, nil } }
	return ThreadState{}, false, nil
}

func (m *Manager) EnsureThreadState(threadID string) (ThreadState, error) {
	state, ok, err := m.GetThreadState(threadID)
	if err != nil { return ThreadState{}, err }
	if ok { return state, nil }
	state = ThreadState{ThreadID: threadID, AutodreamStatus: ThreadStatusPending}
	if err := m.UpsertThreadState(state); err != nil { return ThreadState{}, err }
	return state, nil
}

func (m *Manager) SetThreadStatus(threadID string, status ThreadAutodreamStatus, lastTurnID int, result, lastErr string) error {
	state, _ := m.EnsureThreadState(threadID)
	state.AutodreamStatus = status
	if lastTurnID > state.LastRawTurnID { state.LastRawTurnID = lastTurnID }
	if strings.TrimSpace(result) != "" { state.LastAutodreamResult = result }
	if strings.TrimSpace(lastErr) != "" { state.LastError = lastErr }
	if status == ThreadStatusCompleted || status == ThreadStatusPartialFailed || status == ThreadStatusFailed { state.LastAutodreamAt = time.Now() }
	return m.UpsertThreadState(state)
}

func (m *Manager) MarkSummaryUpdated(threadID string, lastTurnID int) error {
	state, _ := m.EnsureThreadState(threadID)
	if lastTurnID > state.LastRawTurnID { state.LastRawTurnID = lastTurnID }
	state.LastSummaryUpdatedAt = time.Now()
	if state.AutodreamStatus == "" { state.AutodreamStatus = ThreadStatusPending }
	return m.UpsertThreadState(state)
}

func (m *Manager) ListRawTurnsByWindow(windowID string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(m.rawDir(), fmt.Sprintf("%s_turn_*.json", windowID)))
	if err != nil { return nil, err }
	sort.Strings(files)
	return files, nil
}

func (m *Manager) RawTurnMeta(path string) (threadID string, turnID int, timestamp time.Time, err error) {
	b, err := os.ReadFile(path)
	if err != nil { return "", 0, time.Time{}, err }
	var turn RawTurn
	if err := json.Unmarshal(b, &turn); err != nil { return "", 0, time.Time{}, err }
	return turn.WindowID, turn.TurnID, turn.Timestamp, nil
}

func (m *Manager) DescribeThreads() ([]ThreadState, error) {
	store, err := m.LoadThreadStates()
	if err != nil { return nil, err }
	return store.Threads, nil
}

func (m *Manager) WriteRawTurn(turn RawTurn) error {
	if strings.TrimSpace(turn.WindowID) == "" { turn.WindowID = "window-default" }
	turn.AgentRole = normalizeAgentRole(turn.AgentRole)
	path := filepath.Join(m.rawDir(), fmt.Sprintf("%s_turn_%03d.json", turn.WindowID, turn.TurnID))
	b, err := json.MarshalIndent(turn, "", "  ")
	if err != nil { return err }
	if err := os.WriteFile(path, b, 0o644); err != nil { return err }
	_, _ = m.EnsureThreadState(turn.WindowID)
	return m.SetThreadStatus(turn.WindowID, ThreadStatusPending, turn.TurnID, "", "")
}

func (m *Manager) WriteWindowSummary(windowID string, v any) (string, error) {
	if err := os.MkdirAll(m.summaryDir(), 0o755); err != nil { return "", err }
	path := filepath.Join(m.summaryDir(), fmt.Sprintf("%s.summary.json", windowID))
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil { return "", err }
	if err := os.WriteFile(path, b, 0o644); err != nil { return "", err }
	if meta, ok := v.(map[string]any); ok {
		if turnID, ok := meta["turn_id"].(int); ok { _ = m.MarkSummaryUpdated(windowID, turnID) }
	}
	return path, nil
}

func (m *Manager) WriteWindowSegment(windowID, segmentID string, v any) (string, error) {
	if err := os.MkdirAll(m.summaryDir(), 0o755); err != nil { return "", err }
	path := filepath.Join(m.summaryDir(), fmt.Sprintf("%s.%s.json", windowID, segmentID))
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil { return "", err }
	if err := os.WriteFile(path, b, 0o644); err != nil { return "", err }
	return path, nil
}

func (m *Manager) LoadWindows() (WindowStore, error) {
	var store WindowStore
	b, err := os.ReadFile(m.windowsPath())
	if err != nil {
		if os.IsNotExist(err) { return WindowStore{Windows: []ConversationWindow{}}, nil }
		return store, err
	}
	if len(strings.TrimSpace(string(b))) == 0 { return WindowStore{Windows: []ConversationWindow{}}, nil }
	if err := json.Unmarshal(b, &store); err != nil { return store, err }
	changed := false
	for i := range store.Windows {
		role := normalizeAgentRole(store.Windows[i].AgentRole)
		if store.Windows[i].AgentRole != role {
			store.Windows[i].AgentRole = role
			changed = true
		}
	}
	sort.SliceStable(store.Windows, func(i, j int) bool { return store.Windows[i].LastActiveAt.After(store.Windows[j].LastActiveAt) })
	if changed {
		_ = m.SaveWindows(store)
	}
	return store, nil
}

func (m *Manager) SaveWindows(store WindowStore) error {
	if err := os.MkdirAll(filepath.Dir(m.windowsPath()), 0o755); err != nil { return err }
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil { return err }
	return os.WriteFile(m.windowsPath(), b, 0o644)
}

func (m *Manager) CreateWindow(title string) (ConversationWindow, error) {
	return m.CreateWindowWithRole(title, "humphrey")
}

func (m *Manager) CreateWindowWithRole(title, agentRole string) (ConversationWindow, error) {
	store, err := m.LoadWindows()
	if err != nil { return ConversationWindow{}, err }
	now := time.Now()
	maxNum := 0
	for _, w := range store.Windows {
		var n int
		if _, err := fmt.Sscanf(w.ID, "window-%03d", &n); err == nil && n > maxNum { maxNum = n }
	}
	id := fmt.Sprintf("window-%03d", maxNum+1)
	title = strings.TrimSpace(title)
	if title == "" { title = fmt.Sprintf("窗口 %03d", maxNum+1) }
	agentRole = normalizeAgentRole(agentRole)
	win := ConversationWindow{ID: id, Title: title, AgentRole: agentRole, CreatedAt: now, UpdatedAt: now, LastActiveAt: now}
	store.Windows = append(store.Windows, win)
	if err := m.SaveWindows(store); err != nil { return ConversationWindow{}, err }
	_, _ = m.EnsureThreadState(id)
	return win, nil
}

func (m *Manager) GetWindow(id string) (*ConversationWindow, error) {
	store, err := m.LoadWindows()
	if err != nil { return nil, err }
	for i := range store.Windows { if store.Windows[i].ID == id { return &store.Windows[i], nil } }
	return nil, fmt.Errorf("window not found: %s", id)
}

func (m *Manager) TouchWindow(id string, turnCount int, summaryPath string) error {
	store, err := m.LoadWindows()
	if err != nil { return err }
	now := time.Now()
	updated := false
	for i := range store.Windows {
		if store.Windows[i].ID != id { continue }
		store.Windows[i].UpdatedAt = now
		store.Windows[i].LastActiveAt = now
		store.Windows[i].TurnCount = turnCount
		if strings.TrimSpace(summaryPath) != "" { store.Windows[i].SummaryPath = summaryPath }
		if strings.TrimSpace(store.Windows[i].AgentRole) == "" { store.Windows[i].AgentRole = "humphrey" }
		updated = true
		break
	}
	if !updated { return fmt.Errorf("window not found: %s", id) }
	return m.SaveWindows(store)
}

func (m *Manager) RenameWindow(id, title string) error {
	store, err := m.LoadWindows()
	if err != nil { return err }
	title = strings.TrimSpace(title)
	if title == "" { return fmt.Errorf("title is empty") }
	updated := false
	for i := range store.Windows {
		if store.Windows[i].ID != id { continue }
		store.Windows[i].Title = title
		store.Windows[i].UpdatedAt = time.Now()
		updated = true
		break
	}
	if !updated { return fmt.Errorf("window not found: %s", id) }
	return m.SaveWindows(store)
}

func (m *Manager) DeleteWindow(id string) error {
	store, err := m.LoadWindows()
	if err != nil { return err }
	filtered := make([]ConversationWindow, 0, len(store.Windows))
	removed := false
	for _, w := range store.Windows {
		if w.ID == id { removed = true; continue }
		filtered = append(filtered, w)
	}
	if !removed { return fmt.Errorf("window not found: %s", id) }
	store.Windows = filtered
	if err := m.SaveWindows(store); err != nil { return err }
	files, _ := m.ListRawTurnsByWindow(id)
	for _, f := range files { _ = os.Remove(f) }
	_ = os.Remove(filepath.Join(m.summaryDir(), fmt.Sprintf("%s.summary.json", id)))
	return nil
}

func (m *Manager) ReadWindowRawTurns(id string) ([]RawTurn, error) {
	files, err := m.ListRawTurnsByWindow(id)
	if err != nil { return nil, err }
	turns := make([]RawTurn, 0, len(files))
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil { continue }
		var turn RawTurn
		if err := json.Unmarshal(b, &turn); err != nil { continue }
		turn.AgentRole = normalizeAgentRole(turn.AgentRole)
		turns = append(turns, turn)
	}
	sort.SliceStable(turns, func(i, j int) bool { return turns[i].TurnID < turns[j].TurnID })
	return turns, nil
}

func normalizeAgentRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "yongzhen", "yongzhenn", "yong-zheng", "永真", "doubao":
		return "yongzhen"
	case "humphrey", "hanfulai", "hanfulei", "sir_humphrey", "sir-humphrey", "汉弗莱":
		return "humphrey"
	case "":
		return "humphrey"
	default:
		return role
	}
}

func NormalizeAgentRoleForRuntime(role string) string {
	return normalizeAgentRole(role)
}
