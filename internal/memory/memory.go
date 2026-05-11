package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	RootDir string
}

type MemoryPolicy struct {
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	AllowedTypes []string `json:"allowed_types"`
}

type MemoryIndex struct {
	GeneratedAt string             `json:"generated_at"`
	Entries     []MemoryIndexEntry `json:"entries"`
}

type MemoryIndexEntry struct {
	ID             string   `json:"id"`
	TargetType     string   `json:"target_type"`
	File           string   `json:"file"`
	TargetPath     string   `json:"target_path,omitempty"`
	TargetID       string   `json:"target_id,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
	Importance     float64  `json:"importance"`
	Confidence     float64  `json:"confidence"`
	Sensitivity    bool     `json:"sensitivity"`
	UpdatedAt      string   `json:"updated_at"`
	DefaultInclude  bool     `json:"default_include"`
	Title          string   `json:"title,omitempty"`
	Snippet        string   `json:"snippet,omitempty"`
	Status         string   `json:"status,omitempty"`
	MergedInto     string   `json:"merged_into,omitempty"`
	RetryCount     int      `json:"retry_count,omitempty"`
	LastError      string   `json:"last_error,omitempty"`
	LastAttemptAt  string   `json:"last_attempt_at,omitempty"`
	ThreadID       string   `json:"thread_id,omitempty"`
	ExpiresAt      string   `json:"expires_at,omitempty"`
	Scope          string   `json:"scope,omitempty"`
}

type MemorySnippet struct {
	File    string
	Kind    string
	Title   string
	Snippet string
}

type scoredSnippet struct {
	MemorySnippet
	Score float64
}

func NewStore(rootDir string) *Store { return &Store{RootDir: rootDir} }

func DefaultMemoryPolicy() *MemoryPolicy {
	return &MemoryPolicy{Version: "1.0", Description: "Simple stable memory policy for whale demo.", AllowedTypes: []string{"profile", "relationship", "preference", "opinion", "event", "emotion", "digest", "index"}}
}

func (s *Store) Initialize() error {
	paths := []string{filepath.Join(s.RootDir, "data", "sessions", "raw"), filepath.Join(s.RootDir, "data", "sessions", "summaries"), filepath.Join(s.RootDir, "data", "memory", "long_term"), filepath.Join(s.RootDir, "data", "memory", "mid_term"), filepath.Join(s.RootDir, "data", "memory", "digest"), filepath.Join(s.RootDir, "data", "memory", "index"), filepath.Join(s.RootDir, "data", "system")}
	for _, p := range paths { if err := os.MkdirAll(p, 0o755); err != nil { return err } }
	defaults := map[string]any{filepath.Join(s.RootDir, "data", "memory", "long_term", "profile.json"): map[string]any{"name": "鲸鱼", "summary": "桌面悬浮精灵，正在学习认识用户。", "tags": []string{}}, filepath.Join(s.RootDir, "data", "memory", "long_term", "relationship.json"): map[string]any{"summary": "与用户持续互动中，关系仍在建立。"}, filepath.Join(s.RootDir, "data", "memory", "long_term", "preferences.json"): []map[string]any{}, filepath.Join(s.RootDir, "data", "memory", "long_term", "opinions.json"): []map[string]any{}, filepath.Join(s.RootDir, "data", "memory", "mid_term", "events.json"): []map[string]any{}, filepath.Join(s.RootDir, "data", "memory", "mid_term", "emotions.json"): map[string]any{"current": "neutral", "history": []any{}}, filepath.Join(s.RootDir, "data", "memory", "digest", "recent_discussion_digest.json"): map[string]any{"summary": "暂无摘要。"}, filepath.Join(s.RootDir, "data", "memory", "index", "memory_index.json"): MemoryIndex{GeneratedAt: time.Now().Format(time.RFC3339), Entries: []MemoryIndexEntry{}}, filepath.Join(s.RootDir, "data", "system", "runtime_state.json"): map[string]any{"status": "ready", "last_error": nil}, filepath.Join(s.RootDir, "data", "system", "memory_policy.json"): DefaultMemoryPolicy()}
	for path, value := range defaults { if err := ensureJSONFile(path, value); err != nil { return err } }
	return nil
}

func (s *Store) ReadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil { return err }
	if len(b) == 0 { return errors.New("json file is empty") }
	if err := json.Unmarshal(b, v); err != nil { return fmt.Errorf("decode json %s: %w", path, err) }
	return nil
}

func (s *Store) WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return err }
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil { return err }
	return os.WriteFile(path, b, 0o644)
}

func (s *Store) LoadMemoryIndex() (MemoryIndex, error) {
	var idx MemoryIndex
	err := s.ReadJSON(filepath.Join(s.RootDir, "data", "memory", "index", "memory_index.json"), &idx)
	return idx, err
}

func (s *Store) AppendIndexEntry(entry MemoryIndexEntry) error {
	idx, err := s.LoadMemoryIndex()
	if err != nil { idx = MemoryIndex{GeneratedAt: time.Now().Format(time.RFC3339), Entries: []MemoryIndexEntry{}} }
	idx.GeneratedAt = time.Now().Format(time.RFC3339)
	entry = normalizeIndexEntry(entry)
	for i, existing := range idx.Entries {
		if existing.ID == entry.ID {
			existing = mergeIndexEntries(existing, entry)
			idx.Entries[i] = existing
			return s.WriteJSON(filepath.Join(s.RootDir, "data", "memory", "index", "memory_index.json"), idx)
		}
		if fuzzyIndexMatch(existing, entry) && entry.Status != "rejected" {
			next := mergeIndexEntries(existing, entry)
			next.Status = "merged"
			next.MergedInto = existing.ID
			next.UpdatedAt = time.Now().Format(time.RFC3339)
			idx.Entries[i] = next
			return s.WriteJSON(filepath.Join(s.RootDir, "data", "memory", "index", "memory_index.json"), idx)
		}
	}
	idx.Entries = append(idx.Entries, entry)
	return s.WriteJSON(filepath.Join(s.RootDir, "data", "memory", "index", "memory_index.json"), idx)
}

func (s *Store) LoadContext(query string) ([]MemorySnippet, error) {
	idx, err := s.LoadMemoryIndex()
	if err != nil { return nil, err }
	keywords := extractKeywords(query)
	matches := make([]scoredSnippet, 0)
	push := func(file, kind, title, snippet string, score float64) { if snippet == "" { return }; matches = append(matches, scoredSnippet{MemorySnippet: MemorySnippet{File: file, Kind: kind, Title: title, Snippet: snippet}, Score: score}) }
	for _, entry := range idx.Entries {
		score := scoreIndexEntry(entry, keywords)
		if score <= 0 { continue }
		push(entry.File, entry.TargetType, entry.Title, entry.Snippet, score)
	}
	load := func(rel string, out any) error { return s.ReadJSON(filepath.Join(s.RootDir, rel), out) }
	var profile map[string]any
	if err := load(filepath.Join("data", "memory", "long_term", "profile.json"), &profile); err == nil { push("profile.json", "profile", "用户画像/自我介绍", stringifyJSON(profile), 0.7) }
	var relationship map[string]any
	if err := load(filepath.Join("data", "memory", "long_term", "relationship.json"), &relationship); err == nil { push("relationship.json", "relationship", "关系状态", stringifyJSON(relationship), 0.65) }
	var digest map[string]any
	if err := load(filepath.Join("data", "memory", "digest", "recent_discussion_digest.json"), &digest); err == nil { push("recent_discussion_digest.json", "digest", "近期摘要", stringifyJSON(digest), 0.6) }
	sort.SliceStable(matches, func(i, j int) bool { if matches[i].Score == matches[j].Score { return matches[i].File < matches[j].File }; return matches[i].Score > matches[j].Score })
	out := make([]MemorySnippet, 0, len(matches))
	seen := map[string]bool{}
	for _, m := range matches {
		key := m.File + "|" + m.Title + "|" + m.Snippet
		if seen[key] { continue }
		seen[key] = true
		out = append(out, m.MemorySnippet)
		if len(out) >= 6 { break }
	}
	return out, nil
}

func (s *Store) BuildContextText(snippets []MemorySnippet) string {
	if len(snippets) == 0 { return "暂无可召回记忆。" }
	parts := make([]string, 0, len(snippets))
	for _, sn := range snippets { parts = append(parts, fmt.Sprintf("[%s/%s] %s: %s", sn.Kind, sn.File, sn.Title, sn.Snippet)) }
	return strings.Join(parts, "\n")
}

func ensureJSONFile(path string, value any) error {
	if _, err := os.Stat(path); err == nil { return nil }
	store := NewStore(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	return store.WriteJSON(path, value)
}

func stringifyJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func extractKeywords(query string) []string { fields := strings.Fields(strings.ToLower(query)); if len(fields) > 6 { fields = fields[:6] }; return fields }
func matchesKeywords(text string, keywords []string) bool { text = strings.ToLower(text); for _, kw := range keywords { if kw != "" && strings.Contains(text, kw) { return true } }; return false }
func entryHasDefaultRecall(entry MemoryIndexEntry) bool { return strings.Contains(strings.ToLower(entry.Title), "用户") || strings.Contains(strings.ToLower(entry.TargetType), "profile") }
func shouldSkipRecall(entry MemoryIndexEntry) bool { status := strings.ToLower(strings.TrimSpace(entry.Status)); if status == "archived" || status == "deprecated" || status == "rejected" { return true }; if entry.Sensitivity && !entry.DefaultInclude { return true }; if entry.ExpiresAt != "" && time.Now().After(parseTimeSafe(entry.ExpiresAt)) { return true }; return false }
func scoreIndexEntry(entry MemoryIndexEntry, keywords []string) float64 { if shouldSkipRecall(entry) { return 0 }; score := entry.Importance*0.5 + entry.Confidence*0.3; if entry.DefaultInclude { score += 0.2 }; if entry.Sensitivity { score -= 0.2 }; if matchesKeywords(entry.Title+" "+entry.Snippet, keywords) { score += 0.8 }; if strings.EqualFold(entry.Status, "merged") { score -= 0.4 }; if strings.EqualFold(entry.Scope, "thread") { score += 0.1 }; return score }
func parseTimeSafe(v string) time.Time { t, err := time.Parse(time.RFC3339, v); if err != nil { return time.Time{} }; return t }

func normalizeIndexEntry(entry MemoryIndexEntry) MemoryIndexEntry {
	if entry.Status == "" { entry.Status = "active" }
	if entry.UpdatedAt == "" { entry.UpdatedAt = time.Now().Format(time.RFC3339) }
	return entry
}

func mergeIndexEntries(existing, incoming MemoryIndexEntry) MemoryIndexEntry {
	if incoming.Status != "" { existing.Status = incoming.Status }
	if incoming.MergedInto != "" { existing.MergedInto = incoming.MergedInto }
	if incoming.RetryCount != 0 { existing.RetryCount = incoming.RetryCount }
	if incoming.LastError != "" { existing.LastError = incoming.LastError }
	if incoming.LastAttemptAt != "" { existing.LastAttemptAt = incoming.LastAttemptAt }
	if incoming.ThreadID != "" { existing.ThreadID = incoming.ThreadID }
	if len(incoming.Keywords) > 0 { existing.Keywords = uniqueStrings(append(existing.Keywords, incoming.Keywords...)) }
	if incoming.Importance > existing.Importance { existing.Importance = incoming.Importance }
	if incoming.Confidence > existing.Confidence { existing.Confidence = incoming.Confidence }
	if incoming.Snippet != "" { existing.Snippet = incoming.Snippet }
	if incoming.Title != "" { existing.Title = incoming.Title }
	if incoming.ExpiresAt != "" { existing.ExpiresAt = incoming.ExpiresAt }
	if incoming.Scope != "" { existing.Scope = incoming.Scope }
	existing.UpdatedAt = time.Now().Format(time.RFC3339)
	return existing
}

func fuzzyIndexMatch(a, b MemoryIndexEntry) bool {
	if a.TargetType != b.TargetType { return false }
	if normalizeIDPart(a.Title) == normalizeIDPart(b.Title) { return true }
	if len(a.Keywords) > 0 && len(b.Keywords) > 0 && keywordOverlap(a.Keywords, b.Keywords) >= 2 { return true }
	if normalizeIDPart(a.Snippet) == normalizeIDPart(b.Snippet) && a.Snippet != "" { return true }
	return false
}

func keywordOverlap(a, b []string) int {
	set := map[string]bool{}
	for _, v := range a { set[strings.ToLower(strings.TrimSpace(v))] = true }
	count := 0
	for _, v := range b { if set[strings.ToLower(strings.TrimSpace(v))] { count++ } }
	return count
}

func normalizeIDPart(s string) string { s = strings.ToLower(strings.TrimSpace(s)); s = strings.ReplaceAll(s, " ", "-"); s = strings.ReplaceAll(s, "/", "-"); return s }
func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) == "" || seen[v] { continue }
		seen[v] = true
		out = append(out, v)
	}
	return out
}
