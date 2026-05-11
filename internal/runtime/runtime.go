package runtime

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"whale/internal/config"
	"whale/internal/llm"
	"whale/internal/memory"
	"whale/internal/session"
)

type AgentRuntime struct {
	RootDir   string
	Memory    *memory.Store
	Session   *session.Manager
	Policy    *memory.MemoryPolicy
	LLM       *llm.Client
	LLMConfig config.LLMConfig
	Trace     []TraceStep
}

type TraceStep struct {
	Time    time.Time `json:"time"`
	Stage   string    `json:"stage"`
	Message string    `json:"message"`
	Error   string    `json:"error,omitempty"`
}

type TurnResult struct {
	WindowID  string
	SessionID string
	TurnID    int
	AgentRole string
	Reply     string
	Trace     []TraceStep
}

type AutodreamResult struct {
	Mode               string
	WindowID           string
	ProcessedCount     int
	Summaries          []string
	Trace              []TraceStep
	ProcessedThreadIDs []string
	UpdatedMemoryFiles []string
	DecisionCount      int
}

type AutodreamInputPack struct {
	ThreadID          string                    `json:"thread_id"`
	WindowSummary     WindowSummary             `json:"window_summary"`
	RecentRawEvidence []TurnSummaryItem         `json:"recent_raw_evidence"`
	LongTermMemory    map[string]any            `json:"long_term_memory"`
	MemoryIndex       []memory.MemoryIndexEntry `json:"memory_index"`
	Prompt            string                    `json:"prompt"`
	GeneratedAt       time.Time                 `json:"generated_at"`
}

type AutodreamDecision struct {
	TargetFile        string   `json:"target_file"`
	Action            string   `json:"action"`
	MemoryType        string   `json:"memory_type"`
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	Keywords          []string `json:"keywords"`
	Importance        float64  `json:"importance"`
	Confidence        float64  `json:"confidence"`
	Reason            string   `json:"reason"`
	SkipReason        string   `json:"skip_reason,omitempty"`
	ReferenceThreadID string   `json:"reference_thread_id"`
	RelatedIndexIDs   []string `json:"related_index_ids,omitempty"`
}

type autodreamDecisionLoose struct {
	TargetFile        string      `json:"target_file"`
	Action            string      `json:"action"`
	MemoryType        string      `json:"memory_type"`
	Title             string      `json:"title"`
	Content           any         `json:"content"`
	Keywords          []string    `json:"keywords"`
	Importance        float64     `json:"importance"`
	Confidence        float64     `json:"confidence"`
	Reason            string      `json:"reason"`
	SkipReason        string      `json:"skip_reason,omitempty"`
	ReferenceThreadID string      `json:"reference_thread_id"`
	RelatedIndexIDs   []string    `json:"related_index_ids,omitempty"`
}

type AutodreamLLMResponse struct {
	Decisions []AutodreamDecision `json:"decisions"`
}

type autodreamLLMResponseLoose struct {
	Decisions []autodreamDecisionLoose `json:"decisions"`
}

type WindowSummary struct {
	WindowID          string            `json:"window_id"`
	Title             string            `json:"title,omitempty"`
	AgentRole         string            `json:"agent_role,omitempty"`
	TurnCount         int               `json:"turn_count"`
	CurrentSummary    string            `json:"current_summary"`
	SummarySegments   []SummarySegment  `json:"summary_segments"`
	RecentRawTurns    []TurnSummaryItem `json:"recent_raw_turns"`
	ImportantFacts    []string          `json:"important_facts,omitempty"`
	OpenQuestions     []string          `json:"open_questions,omitempty"`
	CandidateMemories []candidateMemory `json:"candidate_memories,omitempty"`
	EmotionSignal     string            `json:"emotion_signal"`
	Sensitive         bool              `json:"sensitive"`
	TargetFiles       []string          `json:"target_files"`
	SkipReasons       []string          `json:"skip_reasons,omitempty"`
	ExtractorPath     string            `json:"extractor_path"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type SummarySegment struct {
	SegmentID string    `json:"segment_id"`
	TurnRange [2]int    `json:"turn_range"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"`
}

type TurnSummaryItem struct {
	TurnID            int       `json:"turn_id"`
	AgentRole         string    `json:"agent_role,omitempty"`
	UserInput         string    `json:"user_input"`
	AssistantResponse string    `json:"assistant_response"`
	Timestamp         time.Time `json:"timestamp"`
}

type summaryRecord struct {
	SessionID         string            `json:"session_id"`
	WindowID          string            `json:"window_id"`
	TurnID            int               `json:"turn_id"`
	Timestamp         time.Time         `json:"timestamp"`
	ConversationBrief string            `json:"conversation_brief"`
	CandidateMemories []candidateMemory `json:"candidate_memories"`
	EmotionSignal     string            `json:"emotion_signal"`
	Sensitive         bool              `json:"sensitive"`
	TargetFiles       []string          `json:"target_files"`
	SkipReasons       []string          `json:"skip_reasons,omitempty"`
	ExtractorPath     string            `json:"extractor_path"`
}

type candidateMemory struct {
	TargetFile string   `json:"target_file"`
	Title      string   `json:"title"`
	Value      string   `json:"value"`
	Keywords   []string `json:"keywords"`
	Importance float64  `json:"importance"`
	Confidence float64  `json:"confidence"`
	SkipReason string   `json:"skip_reason,omitempty"`
	TargetID   string   `json:"target_id,omitempty"`
}

type rawTurnFile struct {
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

func NewAgentRuntime(rootDir string) *AgentRuntime {
	cfgPath := filepath.Join(rootDir, "data", "system", "llm_config.json")
	_ = config.EnsureDefaultLLMConfig(cfgPath)
	cfg, _ := config.LoadLLMConfig(cfgPath)
	if strings.TrimSpace(cfg.Provider) == "" {
		cfg = config.DefaultLLMConfig()
	}
	return &AgentRuntime{RootDir: rootDir, Memory: memory.NewStore(rootDir), Session: session.NewManager(rootDir), Policy: memory.DefaultMemoryPolicy(), LLMConfig: cfg, LLM: llm.NewClientFromConfig(cfg), Trace: make([]TraceStep, 0)}
}

func (r *AgentRuntime) Initialize() error {
	if err := r.Memory.Initialize(); err != nil { return err }
	if err := r.Session.Initialize(); err != nil { return err }
	if err := r.EnsureDefaultRoles(); err != nil { return err }
	r.Trace = append(r.Trace, TraceStep{Time: time.Now(), Stage: "init", Message: "runtime initialized"})
	return nil
}

func (r *AgentRuntime) HandleInput(windowID, userInput, requestedRole string) (TurnResult, error) {
	r.Trace = nil
	r.AddTrace("step1", "接收用户输入", nil)
	if strings.TrimSpace(windowID) == "" { return TurnResult{}, fmt.Errorf("window_id is required") }
	role := r.ResolveRole(requestedRole)
	if _, err := r.Session.EnsureThreadState(windowID); err != nil { r.AddTrace("step1b", "初始化线程状态失败", err) }
	r.AddTrace("step2", "读取窗口摘要与记忆索引", nil)
	windowSummaryText, err := r.loadWindowSummaryText(windowID)
	if err != nil { r.AddTrace("step3", "读取窗口摘要失败，使用空摘要", err) } else { r.AddTrace("step3", "装配窗口摘要", nil) }
	snippets, err := r.Memory.LoadContext(userInput)
	if err != nil { r.AddTrace("step4", "召回相关记忆失败，使用空上下文", err) } else { r.AddTrace("step4", fmt.Sprintf("召回 %d 条相关记忆", len(snippets)), nil) }
	recentTurnsText := r.loadRecentWindowTurnsText(windowID, 6)
	contextText := strings.TrimSpace(strings.Join(filterNonEmpty([]string{windowSummaryText, recentTurnsText, r.Memory.BuildContextText(snippets)}), "\n\n"))
	r.AddTrace("step5", "装配上下文", nil)
	allRoles, _ := r.LoadRoles()
	systemPrompt := buildSystemPromptForRole(role, allRoles)
	reply, llmErr := r.LLM.GenerateReply(llm.ReplyRequest{SystemPrompt: systemPrompt, Context: contextText, UserInput: userInput})
	if llmErr != nil { r.AddTrace("step6", "调用大模型失败，使用 fallback 回复", llmErr) } else { r.AddTrace("step6", "调用大模型完成", nil) }
	r.AddTrace("step7", "保存原始会话", nil)
	finalTrace := append([]TraceStep(nil), r.Trace...)
	turnID := r.nextTurnID(windowID)
	sessionID := fmt.Sprintf("session-%s", time.Now().Format("20060102"))
	turn := session.RawTurn{WindowID: windowID, SessionID: sessionID, TurnID: turnID, AgentRole: role.ID, UserInput: userInput, AssistantResponse: reply, Timestamp: time.Now(), Trace: finalTrace, ProcessedByAutodream: false}
	if err := r.Session.WriteRawTurn(turn); err != nil { return TurnResult{}, err }
	if err := r.updateWindowSummaryOnTurn(turn); err != nil { r.AddTrace("step8", "更新窗口摘要失败", err) } else { r.AddTrace("step8", "更新窗口摘要完成", nil) }
	return TurnResult{WindowID: windowID, SessionID: sessionID, TurnID: turnID, AgentRole: role.ID, Reply: reply, Trace: finalTrace}, nil
}

func (r *AgentRuntime) nextTurnID(windowID string) int {
	files, err := r.Session.ListRawTurnsByWindow(windowID)
	if err != nil || len(files) == 0 { return 1 }
	last := files[len(files)-1]
	base := filepath.Base(last)
	parts := strings.Split(strings.TrimSuffix(base, ".json"), "_")
	if len(parts) == 0 { return 1 }
	n, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil { return len(files) + 1 }
	return n + 1
}

func (r *AgentRuntime) loadWindowSummaryText(windowID string) (string, error) {
	path := filepath.Join(r.RootDir, "data", "sessions", "summaries", fmt.Sprintf("%s.summary.json", windowID))
	var summary WindowSummary
	if err := r.Memory.ReadJSON(path, &summary); err != nil { return "", err }
	return fmt.Sprintf("[window_summary/%s]\n%s", summary.WindowID, summary.CurrentSummary), nil
}

func (r *AgentRuntime) loadRecentWindowTurnsText(windowID string, recentCount int) string {
	files, err := r.Session.ListRawTurnsByWindow(windowID)
	if err != nil || len(files) == 0 { return "" }
	if recentCount <= 0 { recentCount = 3 }
	start := len(files) - recentCount
	if start < 0 { start = 0 }
	parts := make([]string, 0, len(files)-start)
	roles, _ := r.LoadRoles()
	for _, file := range files[start:] {
		var turn rawTurnFile
		if err := r.Memory.ReadJSON(file, &turn); err != nil { continue }
		parts = append(parts, fmt.Sprintf("[turn %d]\n用户：%s\n%s：%s", turn.TurnID, turn.UserInput, displayRoleNameFromProfiles(turn.AgentRole, roles), turn.AssistantResponse))
	}
	return strings.Join(parts, "\n\n")
}

func (r *AgentRuntime) updateWindowSummaryOnTurn(turn session.RawTurn) error {
	path := filepath.Join(r.RootDir, "data", "sessions", "summaries", fmt.Sprintf("%s.summary.json", turn.WindowID))
	var summary WindowSummary
	if err := r.Memory.ReadJSON(path, &summary); err != nil {
		summary = WindowSummary{WindowID: turn.WindowID, AgentRole: turn.AgentRole, SummarySegments: []SummarySegment{}, RecentRawTurns: []TurnSummaryItem{}, CandidateMemories: []candidateMemory{}, TargetFiles: []string{}, ExtractorPath: "window_rollup"}
	}
	if summary.WindowID == "" { summary.WindowID = turn.WindowID }
	summary.AgentRole = turn.AgentRole
	summary.TurnCount++
	summary.UpdatedAt = time.Now()
	summary.RecentRawTurns = append(summary.RecentRawTurns, TurnSummaryItem{TurnID: turn.TurnID, AgentRole: turn.AgentRole, UserInput: turn.UserInput, AssistantResponse: turn.AssistantResponse, Timestamp: turn.Timestamp})
	if len(summary.RecentRawTurns) > 6 { summary.RecentRawTurns = summary.RecentRawTurns[len(summary.RecentRawTurns)-6:] }
	if summary.CurrentSummary == "" { summary.CurrentSummary = buildWindowBrief(summary.RecentRawTurns) }
	if turn.TurnID%3 == 0 || shouldForceSegment(summary, turn.UserInput) {
		segmentSummary, segmentSource := r.buildSegmentSummaryWithLLM(turn.WindowID, summary)
		segment := SummarySegment{SegmentID: fmt.Sprintf("%s.segment.%03d", turn.WindowID, (turn.TurnID+2)/3), TurnRange: [2]int{max(1, turn.TurnID-2), turn.TurnID}, Summary: segmentSummary, CreatedAt: time.Now(), Source: segmentSource}
		summary.SummarySegments = append(summary.SummarySegments, segment)
		if len(summary.SummarySegments) > 8 { summary.SummarySegments = summary.SummarySegments[len(summary.SummarySegments)-8:] }
		summary.CurrentSummary = buildWindowCurrentSummary(summary)
	}
	summary.CandidateMemories = append(summary.CandidateMemories, candidateMemory{TargetFile: "events.json", Title: "窗口对话事件", Value: summary.CurrentSummary, Keywords: []string{"窗口", "对话"}, Importance: 0.6, Confidence: 0.7})
	summary.TargetFiles = uniqueStrings(append(summary.TargetFiles, "events.json", "recent_discussion_digest.json"))
	summary.ExtractorPath = "window_rollup"
	if _, err := r.Session.WriteWindowSummary(turn.WindowID, summary); err != nil { return err }
	if err := r.Session.MarkSummaryUpdated(turn.WindowID, turn.TurnID); err != nil { return err }
	if err := r.Session.TouchWindow(turn.WindowID, summary.TurnCount, filepath.Join("data", "sessions", "summaries", fmt.Sprintf("%s.summary.json", turn.WindowID))); err != nil { return err }
	return nil
}

func shouldForceSegment(summary WindowSummary, userInput string) bool { return len(strings.Fields(userInput)) > 40 || strings.Contains(userInput, "总结") || strings.Contains(userInput, "记住") || summary.CurrentSummary == "" }
func buildWindowBrief(turns []TurnSummaryItem) string { if len(turns) == 0 { return "暂无窗口摘要。" }; parts := make([]string, 0, len(turns)); for _, t := range turns { parts = append(parts, fmt.Sprintf("用户:%s / %s:%s", t.UserInput, displayRoleName(t.AgentRole), t.AssistantResponse)) }; return strings.Join(parts, " | ") }
func buildSegmentSummary(turns []TurnSummaryItem) string { return buildWindowBrief(turns) }
func (r *AgentRuntime) buildSegmentSummaryWithLLM(windowID string, summary WindowSummary) (string, string) {
	segmentInput := buildWindowBrief(summary.RecentRawTurns)
	if len(summary.SummarySegments) > 0 {
		last := summary.SummarySegments[len(summary.SummarySegments)-1]
		segmentInput = last.Summary + "\n" + segmentInput
	}
	prompt := buildWindowSummaryPrompt(windowID, summary, segmentInput)
	resp, err := r.LLM.GenerateReply(llm.ReplyRequest{SystemPrompt: windowSummarySystemPrompt(), Context: prompt, UserInput: "请压缩总结上述最近对话，并输出适合后续上下文使用的简洁摘要。"})
	if err != nil || strings.TrimSpace(resp) == "" { return buildSegmentSummary(summary.RecentRawTurns), "fallback_rules" }
	return strings.TrimSpace(resp), "llm"
}

func buildWindowSummaryPrompt(windowID string, summary WindowSummary, segmentInput string) string {
	parts := []string{
		fmt.Sprintf("窗口ID：%s", windowID),
		fmt.Sprintf("窗口标题：%s", summary.Title),
		fmt.Sprintf("历史摘要：\n%s", summary.CurrentSummary),
		fmt.Sprintf("最近原文：\n%s", segmentInput),
		"请输出简洁、可直接作为后续上下文使用的窗口分段摘要，包含：主线、发言角色关系、用户关注点、关键事实、待解决问题、可写入记忆信号。",
	}
	return strings.Join(filterNonEmpty(parts), "\n\n")
}

func windowSummarySystemPrompt() string {
	return `你是一个群聊窗口摘要器。你的任务不是回复用户，而是压缩总结当前窗口最近对话。
要求：
1. 输出中文。
2. 必须保留“是谁说的”这一层信息，尤其是不同角色的观点差异。
3. 保留主线、关键事实、用户偏好/观点/画像信号。
4. 尽量简洁，适合作为后续大模型上下文。
5. 不要编造不存在的事实。
6. 不要输出多余解释，只输出摘要正文。`
}

func buildWindowCurrentSummary(summary WindowSummary) string {
	segments := make([]string, 0, len(summary.SummarySegments))
	for _, seg := range summary.SummarySegments { segments = append(segments, seg.Summary) }
	if len(summary.RecentRawTurns) > 0 { segments = append(segments, buildWindowBrief(summary.RecentRawTurns)) }
	if len(segments) == 0 { return "暂无窗口摘要。" }
	return strings.Join(segments, "\n")
}

func displayRoleName(role string) string {
	switch normalizeLegacyRole(role) {
	case "yongzhen":
		return "永真"
	default:
		return "汉弗莱"
	}
}

func (r *AgentRuntime) RunAutodream() (AutodreamResult, error) {
	trace := []TraceStep{}
	add := func(stage, msg string, err error) { step := TraceStep{Time: time.Now(), Stage: stage, Message: msg}; if err != nil { step.Error = err.Error() }; trace = append(trace, step) }
	add("dream-start", "开始扫描窗口摘要与 raw 会话", nil)
	windowStore, err := r.Session.LoadWindows()
	if err != nil { add("dream-window-scan", "读取窗口列表失败", err); return AutodreamResult{Mode: "global", Trace: trace}, err }
	processed := 0
	summaries := make([]string, 0)
	processedThreadIDs := make([]string, 0, len(windowStore.Windows))
	updatedMemoryFiles := make([]string, 0)
	decisionCount := 0
	for _, win := range windowStore.Windows {
		result, err := r.runAutodreamForWindow(win, &trace)
		if err != nil {
			add("dream-window", fmt.Sprintf("线程 %s 处理失败", win.ID), err)
			continue
		}
		if result.WindowTouched {
			processedThreadIDs = append(processedThreadIDs, win.ID)
		}
		processed += result.ProcessedCount
		summaries = append(summaries, result.Summaries...)
		updatedMemoryFiles = append(updatedMemoryFiles, result.UpdatedMemoryFiles...)
		decisionCount += result.DecisionCount
	}
	add("dream-end", fmt.Sprintf("Autodream 完成，处理 %d 条会话", processed), nil)
	return AutodreamResult{Mode: "global", ProcessedCount: processed, Summaries: uniqueStrings(summaries), Trace: trace, ProcessedThreadIDs: uniqueStrings(processedThreadIDs), UpdatedMemoryFiles: uniqueStrings(updatedMemoryFiles), DecisionCount: decisionCount}, nil
}

func (r *AgentRuntime) RunAutodreamForThread(windowID string) (AutodreamResult, error) {
	trace := []TraceStep{}
	add := func(stage, msg string, err error) { step := TraceStep{Time: time.Now(), Stage: stage, Message: msg}; if err != nil { step.Error = err.Error() }; trace = append(trace, step) }
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		err := fmt.Errorf("window_id is required for thread mode")
		add("dream-start", "单线程子梦缺少 window_id", err)
		return AutodreamResult{Mode: "thread", Trace: trace}, err
	}
	win, err := r.Session.GetWindow(windowID)
	if err != nil {
		add("dream-window", fmt.Sprintf("未找到线程 %s", windowID), err)
		return AutodreamResult{Mode: "thread", WindowID: windowID, Trace: trace}, err
	}
	add("dream-start", fmt.Sprintf("开始处理线程 %s", windowID), nil)
	result, err := r.runAutodreamForWindow(*win, &trace)
	if err != nil {
		add("dream-end", fmt.Sprintf("线程 %s 处理失败", windowID), err)
		return AutodreamResult{Mode: "thread", WindowID: windowID, Trace: trace}, err
	}
	add("dream-end", fmt.Sprintf("线程 %s 处理完成，处理 %d 条会话", windowID, result.ProcessedCount), nil)
	processedThreadIDs := []string{}
	if result.WindowTouched {
		processedThreadIDs = append(processedThreadIDs, windowID)
	}
	return AutodreamResult{Mode: "thread", WindowID: windowID, ProcessedCount: result.ProcessedCount, Summaries: uniqueStrings(result.Summaries), Trace: trace, ProcessedThreadIDs: processedThreadIDs, UpdatedMemoryFiles: uniqueStrings(result.UpdatedMemoryFiles), DecisionCount: result.DecisionCount}, nil
}

type autodreamWindowResult struct {
	WindowTouched      bool
	ProcessedCount     int
	Summaries          []string
	UpdatedMemoryFiles []string
	DecisionCount      int
}

func (r *AgentRuntime) runAutodreamForWindow(win session.ConversationWindow, trace *[]TraceStep) (autodreamWindowResult, error) {
	add := func(stage, msg string, err error) {
		step := TraceStep{Time: time.Now(), Stage: stage, Message: msg}
		if err != nil { step.Error = err.Error() }
		*trace = append(*trace, step)
	}
	result := autodreamWindowResult{Summaries: []string{}, UpdatedMemoryFiles: []string{}}
	windowSummaryPath := filepath.Join(r.RootDir, "data", "sessions", "summaries", fmt.Sprintf("%s.summary.json", win.ID))
	windowSummary, hasSummary := r.loadWindowSummary(windowSummaryPath)
	threadState, _ := r.Session.EnsureThreadState(win.ID)
	if hasSummary && r.isSummarySufficient(windowSummary) {
		pack, err := r.buildAutodreamInputPack(win.ID, windowSummary)
		if err != nil { add("dream-pack", fmt.Sprintf("构造线程 %s 输入包失败", win.ID), err); return result, nil }
		add("dream-pack", fmt.Sprintf("线程 %s 输入包已装配", win.ID), nil)
		if err := r.Session.SetThreadStatus(win.ID, session.ThreadStatusProcessing, threadState.LastRawTurnID, "window-summary processing", ""); err != nil { add("dream-status", "更新线程状态为 processing 失败", err) }
		llmResult, err := r.runAutodreamDecision(pack)
		if err != nil { add("dream-llm", "子梦判断失败", err); _ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusFailed, threadState.LastRawTurnID, "llm failed", err.Error()); return result, nil }
		writtenFiles, err := r.applyAutodreamDecisions(llmResult.Decisions, win.ID, windowSummary)
		if err != nil {
			add("dream-window-memory", "基于窗口摘要写入记忆失败", err)
			_ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusPartialFailed, threadState.LastRawTurnID, "window-summary partial_failed", err.Error())
		} else {
			add("dream-index", fmt.Sprintf("线程 %s 索引已更新", win.ID), nil)
			result.WindowTouched = true
			result.Summaries = append(result.Summaries, windowSummaryPath)
			result.UpdatedMemoryFiles = append(result.UpdatedMemoryFiles, writtenFiles...)
			result.DecisionCount += len(llmResult.Decisions)
			_ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusCompleted, threadState.LastRawTurnID, fmt.Sprintf("window-summary decisions=%d", len(llmResult.Decisions)), "")
		}
	}
	rawFiles, err := r.Session.ListRawTurnsByWindow(win.ID)
	if err != nil { add("dream-raw-scan", fmt.Sprintf("扫描窗口 raw 失败 %s", win.ID), err); return result, nil }
	for _, file := range rawFiles {
		var turn rawTurnFile
		if err := r.Memory.ReadJSON(file, &turn); err != nil { add("dream-read", "读取 raw 会话失败", err); continue }
		if turn.ProcessedByAutodream { continue }
		_ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusProcessing, turn.TurnID, "raw processing", "")
		if hasSummary && r.isSummarySufficient(windowSummary) {
			add("dream-skip-raw", fmt.Sprintf("窗口 %s 摘要充足，跳过重复 raw 提取", win.ID), nil)
			turn.ProcessedByAutodream = true
			if err := r.Memory.WriteJSON(file, turn); err != nil { add("dream-mark", "标记 raw 会话已整理失败", err); _ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusPartialFailed, turn.TurnID, "raw marked partial_failed", err.Error()); continue }
			add("dream-mark", "标记 raw 会话已整理", nil)
			result.WindowTouched = true
			result.ProcessedCount++
			_ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusCompleted, turn.TurnID, "raw completed", "")
			continue
		}
		if hasSummary && len(windowSummary.SummarySegments) > 0 {
			add("dream-raw-selective", fmt.Sprintf("窗口 %s 摘要不足，仅补处理最近 raw", win.ID), nil)
			if !r.isRecentRawTurn(file, rawFiles, 3) {
				continue
			}
		}
		add("dream-turn", fmt.Sprintf("整理会话 %s#%d", turn.WindowID, turn.TurnID), nil)
		summary := buildWindowSummaryFromTurn(turn)
		summary.ExtractorPath = "raw_fallback"
		if _, err := r.Session.WriteWindowSummary(win.ID, summary); err != nil { add("dream-summary", "写入摘要失败", err); _ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusPartialFailed, turn.TurnID, "summary failed", err.Error()); continue }
		windowSummary = summary
		hasSummary = true
		add("dream-summary", fmt.Sprintf("生成摘要 %s", filepath.Base(windowSummaryPath)), nil)
		pack, err := r.buildAutodreamInputPack(win.ID, windowSummary)
		if err != nil { add("dream-pack", fmt.Sprintf("构造线程 %s 输入包失败", win.ID), err); _ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusPartialFailed, turn.TurnID, "pack failed", err.Error()); continue }
		add("dream-pack", fmt.Sprintf("线程 %s 输入包已装配", win.ID), nil)
		llmResult, err := r.runAutodreamDecision(pack)
		if err != nil { add("dream-llm", "子梦判断失败", err); _ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusFailed, turn.TurnID, "llm failed", err.Error()); continue }
		writtenFiles, err := r.applyAutodreamDecisions(llmResult.Decisions, win.ID, windowSummary)
		if err != nil { add("dream-memory", "写入记忆失败", err); _ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusPartialFailed, turn.TurnID, "memory failed", err.Error()); continue }
		add("dream-index", fmt.Sprintf("线程 %s 索引已更新", win.ID), nil)
		turn.ProcessedByAutodream = true
		if err := r.Memory.WriteJSON(file, turn); err != nil { add("dream-mark", "标记 raw 会话已整理失败", err); _ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusPartialFailed, turn.TurnID, "mark failed", err.Error()); continue }
		add("dream-mark", "标记 raw 会话已整理", nil)
		result.WindowTouched = true
		result.ProcessedCount++
		result.Summaries = append(result.Summaries, windowSummaryPath)
		result.UpdatedMemoryFiles = append(result.UpdatedMemoryFiles, writtenFiles...)
		result.DecisionCount += len(llmResult.Decisions)
		_ = r.Session.SetThreadStatus(win.ID, session.ThreadStatusCompleted, turn.TurnID, fmt.Sprintf("decisions=%d", len(llmResult.Decisions)), "")
	}
	result.Summaries = uniqueStrings(result.Summaries)
	result.UpdatedMemoryFiles = uniqueStrings(result.UpdatedMemoryFiles)
	return result, nil
}

func (r *AgentRuntime) applyAutodreamMemory(summary summaryRecord) error {
	for _, mem := range summary.CandidateMemories {
		if strings.TrimSpace(mem.TargetFile) == "" { continue }
		switch mem.TargetFile {
		case "profile.json": if err := updateProfile(r, mem); err != nil { return err }
		case "relationship.json": if err := updateRelationship(r, mem); err != nil { return err }
		case "preferences.json": if err := appendMemoryEntry(r, filepath.Join("data", "memory", "long_term", "preferences.json"), mem); err != nil { return err }
		case "opinions.json": if err := appendMemoryEntry(r, filepath.Join("data", "memory", "long_term", "opinions.json"), mem); err != nil { return err }
		case "events.json": if err := appendMemoryEntry(r, filepath.Join("data", "memory", "mid_term", "events.json"), mem); err != nil { return err }
		case "emotions.json": if err := updateEmotions(r, mem); err != nil { return err }
		case "recent_discussion_digest.json": if err := updateDigest(r, summary); err != nil { return err }
		}
		if err := r.Memory.AppendIndexEntry(memory.MemoryIndexEntry{ID: fmt.Sprintf("%s-%d-%s", summary.WindowID, summary.TurnID, mem.TargetFile), TargetType: mem.TargetFile, File: mem.TargetFile, TargetPath: mem.TargetID, Keywords: mem.Keywords, Importance: mem.Importance, Confidence: mem.Confidence, Sensitivity: summary.Sensitive, UpdatedAt: time.Now().Format(time.RFC3339), DefaultInclude: mem.TargetFile == "profile.json" || mem.TargetFile == "relationship.json", Title: mem.Title, Snippet: mem.Value}); err != nil { return err }
	}
	return nil
}

func (r *AgentRuntime) AddTrace(stage, message string, err error) { step := TraceStep{Time: time.Now(), Stage: stage, Message: message}; if err != nil { step.Error = err.Error() }; r.Trace = append(r.Trace, step) }
func (r *AgentRuntime) String() string { return fmt.Sprintf("AgentRuntime(root=%s)", r.RootDir) }
func FormatTrace(trace []TraceStep) string { lines := make([]string, 0, len(trace)); for _, step := range trace { msg := fmt.Sprintf("%s | %s | %s", step.Time.Format(time.RFC3339), step.Stage, step.Message); if strings.TrimSpace(step.Error) != "" { msg += " | error=" + step.Error }; lines = append(lines, msg) }; return strings.Join(lines, "\n") }

func buildSummary(turn rawTurnFile) (summaryRecord, string) {
	lower := strings.ToLower(turn.UserInput)
	mems := []candidateMemory{{TargetFile: "events.json", Title: "会话事件", Value: turn.UserInput, Keywords: []string{"对话", "事件"}, Importance: 0.7, Confidence: 0.8}}
	targets := []string{"events.json"}
	extractor := "fallback_rules"
	skip := []string{}
	if strings.Contains(lower, "喜欢") || strings.Contains(lower, "偏好") { mems = append(mems, candidateMemory{TargetFile: "preferences.json", Title: "用户偏好", Value: turn.UserInput, Keywords: []string{"偏好"}, Importance: 0.8, Confidence: 0.75}); targets = append(targets, "preferences.json") }
	if strings.Contains(lower, "我叫") || strings.Contains(lower, "我是") { mems = append(mems, candidateMemory{TargetFile: "profile.json", Title: "用户画像", Value: turn.UserInput, Keywords: []string{"画像"}, Importance: 0.9, Confidence: 0.7, TargetID: "profile"}); targets = append(targets, "profile.json") }
	if strings.Contains(lower, "开心") || strings.Contains(lower, "难过") { mems = append(mems, candidateMemory{TargetFile: "emotions.json", Title: "情绪变化", Value: turn.UserInput, Keywords: []string{"情绪"}, Importance: 0.6, Confidence: 0.7}); targets = append(targets, "emotions.json") }
	if strings.Contains(lower, "观点") || strings.Contains(lower, "认为") { mems = append(mems, candidateMemory{TargetFile: "opinions.json", Title: "用户观点", Value: turn.UserInput, Keywords: []string{"观点"}, Importance: 0.65, Confidence: 0.7}); targets = append(targets, "opinions.json") }
	if strings.Contains(lower, "关系") || strings.Contains(lower, "管家") { mems = append(mems, candidateMemory{TargetFile: "relationship.json", Title: "关系描述", Value: turn.UserInput, Keywords: []string{"关系"}, Importance: 0.75, Confidence: 0.72}); targets = append(targets, "relationship.json") }
	if len(mems) == 1 { skip = append(skip, "无明显可写入记忆，保留为事件摘要") }
	return summaryRecord{SessionID: turn.SessionID, WindowID: turn.WindowID, TurnID: turn.TurnID, Timestamp: turn.Timestamp, ConversationBrief: turn.UserInput, CandidateMemories: mems, EmotionSignal: detectEmotion(lower), Sensitive: false, TargetFiles: uniqueStrings(targets), SkipReasons: skip, ExtractorPath: extractor}, extractor
}

func buildWindowSummaryFromTurn(turn rawTurnFile) WindowSummary {
	record, _ := buildSummary(turn)
	currentSummary := strings.TrimSpace(turn.UserInput)
	if strings.TrimSpace(turn.AssistantResponse) != "" {
		currentSummary = fmt.Sprintf("用户：%s\n助手：%s", turn.UserInput, turn.AssistantResponse)
	}
	return WindowSummary{
		WindowID:          turn.WindowID,
		AgentRole:         turn.AgentRole,
		TurnCount:         max(1, turn.TurnID),
		CurrentSummary:    currentSummary,
		SummarySegments:   []SummarySegment{{SegmentID: fmt.Sprintf("%s.segment.raw.%03d", turn.WindowID, turn.TurnID), TurnRange: [2]int{turn.TurnID, turn.TurnID}, Summary: currentSummary, CreatedAt: turn.Timestamp, Source: "raw_fallback"}},
		RecentRawTurns:    []TurnSummaryItem{{TurnID: turn.TurnID, AgentRole: turn.AgentRole, UserInput: turn.UserInput, AssistantResponse: turn.AssistantResponse, Timestamp: turn.Timestamp}},
		CandidateMemories: record.CandidateMemories,
		EmotionSignal:     record.EmotionSignal,
		Sensitive:         record.Sensitive,
		TargetFiles:       record.TargetFiles,
		SkipReasons:       record.SkipReasons,
		ExtractorPath:     record.ExtractorPath,
		UpdatedAt:         time.Now(),
	}
}

func detectEmotion(lower string) string { switch { case strings.Contains(lower, "开心") || strings.Contains(lower, "高兴"): return "positive"; case strings.Contains(lower, "难过") || strings.Contains(lower, "失落") || strings.Contains(lower, "生气"): return "negative"; default: return "neutral" } }
func uniqueStrings(values []string) []string { seen := map[string]bool{}; out := make([]string, 0, len(values)); for _, v := range values { if !seen[v] { seen[v] = true; out = append(out, v) } }; return out }
func updateProfile(r *AgentRuntime, mem candidateMemory) error { path := filepath.Join(r.RootDir, "data", "memory", "long_term", "profile.json"); var current map[string]any; if err := r.Memory.ReadJSON(path, &current); err != nil { current = map[string]any{} }; current["summary"] = mem.Value; current["updated_at"] = time.Now().Format(time.RFC3339); if current["name"] == nil { current["name"] = "鲸鱼" }; tags := map[string]bool{}; if arr, ok := current["tags"].([]any); ok { for _, v := range arr { if s, ok := v.(string); ok { tags[s] = true } } }; if arr, ok := current["tags"].([]string); ok { for _, v := range arr { tags[v] = true } }; tags[mem.Title] = true; current["tags"] = mapKeys(tags); return r.Memory.WriteJSON(path, current) }
func updateRelationship(r *AgentRuntime, mem candidateMemory) error { path := filepath.Join(r.RootDir, "data", "memory", "long_term", "relationship.json"); var current map[string]any; if err := r.Memory.ReadJSON(path, &current); err != nil { current = map[string]any{} }; current["summary"] = mem.Value; current["updated_at"] = time.Now().Format(time.RFC3339); current["interaction_style"] = "稳重、克制、体面"; return r.Memory.WriteJSON(path, current) }
func updateEmotions(r *AgentRuntime, mem candidateMemory) error { path := filepath.Join(r.RootDir, "data", "memory", "mid_term", "emotions.json"); var current map[string]any; if err := r.Memory.ReadJSON(path, &current); err != nil { current = map[string]any{} }; current["current"] = mem.Title; history, _ := current["history"].([]any); history = append(history, map[string]any{"value": mem.Title, "note": mem.Value, "updated_at": time.Now().Format(time.RFC3339)}); current["history"] = history; return r.Memory.WriteJSON(path, current) }
func (r *AgentRuntime) loadWindowSummary(path string) (WindowSummary, bool) {
	var summary WindowSummary
	if err := r.Memory.ReadJSON(path, &summary); err != nil { return WindowSummary{}, false }
	return summary, true
}

func (r *AgentRuntime) isSummarySufficient(summary WindowSummary) bool {
	if len(summary.SummarySegments) >= 1 && len(strings.TrimSpace(summary.CurrentSummary)) >= 20 { return true }
	return false
}

func (r *AgentRuntime) isRecentRawTurn(file string, rawFiles []string, recentCount int) bool {
	if recentCount <= 0 { recentCount = 3 }
	if len(rawFiles) <= recentCount { return true }
	start := len(rawFiles) - recentCount
	for _, rf := range rawFiles[start:] {
		if rf == file { return true }
	}
	return false
}

func (r *AgentRuntime) buildAutodreamInputPack(threadID string, summary WindowSummary) (AutodreamInputPack, error) {
	memoryIndex, err := r.Memory.LoadMemoryIndex()
	if err != nil { return AutodreamInputPack{}, err }
	longTerm := map[string]any{}
	for _, rel := range []string{"data/memory/long_term/profile.json", "data/memory/long_term/relationship.json", "data/memory/long_term/preferences.json", "data/memory/long_term/opinions.json", "data/memory/mid_term/events.json", "data/memory/mid_term/emotions.json", "data/memory/digest/recent_discussion_digest.json"} {
		var v any
		if err := r.Memory.ReadJSON(filepath.Join(r.RootDir, rel), &v); err == nil { longTerm[rel] = v }
	}
	filteredIndex := filterIndexByThread(memoryIndex.Entries, threadID)
	pack := AutodreamInputPack{ThreadID: threadID, WindowSummary: summary, RecentRawEvidence: summary.RecentRawTurns, LongTermMemory: longTerm, MemoryIndex: filteredIndex, Prompt: buildAutodreamSystemPrompt(), GeneratedAt: time.Now()}
	return pack, nil
}

func buildAutodreamSystemPrompt() string {
	return `你是子梦记忆整理判断器，不是普通聊天助手。
你的主输入是线程摘要，不是单条 raw。
你需要对照现有记忆，判断本线程内容是否值得写入长期记忆。
你必须优先保护长期稳定记忆，避免单轮内容误改画像。
你可以更积极保留事件类、近期主线类信息。
你必须输出结构化 JSON，不能输出散文式说明。
对不确定内容，宁可跳过，不要强行写入。
对相近内容，优先建议更新或合并，而不是无脑新增。`
}

func (r *AgentRuntime) runAutodreamDecision(pack AutodreamInputPack) (AutodreamLLMResponse, error) {
	prompt, err := json.MarshalIndent(pack, "", "  ")
	if err != nil { return AutodreamLLMResponse{}, err }
	resp, err := r.LLM.GenerateReply(llm.ReplyRequest{SystemPrompt: pack.Prompt, Context: string(prompt), UserInput: "请输出 decisions 数组的 JSON 结果，严格遵守字段要求。"})
	if err != nil { return AutodreamLLMResponse{}, err }
	parsed, err := parseAutodreamResponse(resp, pack.ThreadID)
	if err != nil { return AutodreamLLMResponse{}, err }
	if err := validateAutodreamResponse(parsed); err != nil { return AutodreamLLMResponse{}, err }
	return parsed, nil
}

func parseAutodreamResponse(resp string, threadID string) (AutodreamLLMResponse, error) {
	cleaned := extractJSONObjectOrArray(resp)
	if strings.TrimSpace(cleaned) == "" {
		return AutodreamLLMResponse{}, fmt.Errorf("empty response after cleanup")
	}
	var loose autodreamLLMResponseLoose
	if strings.HasPrefix(strings.TrimSpace(cleaned), "[") {
		var arr []autodreamDecisionLoose
		if err := json.Unmarshal([]byte(cleaned), &arr); err != nil {
			return AutodreamLLMResponse{}, err
		}
		loose.Decisions = arr
	} else {
		if err := json.Unmarshal([]byte(cleaned), &loose); err != nil {
			return AutodreamLLMResponse{}, err
		}
	}
	result := AutodreamLLMResponse{Decisions: make([]AutodreamDecision, 0, len(loose.Decisions))}
	for _, dec := range loose.Decisions {
		result.Decisions = append(result.Decisions, normalizeAutodreamDecision(dec, threadID))
	}
	return result, nil
}

func validateAutodreamResponse(resp AutodreamLLMResponse) error {
	if len(resp.Decisions) == 0 { return fmt.Errorf("no decisions returned") }
	allowed := map[string]bool{"add": true, "update": true, "merge": true, "skip": true}
	allowedFiles := map[string]bool{"profile.json": true, "relationship.json": true, "preferences.json": true, "opinions.json": true, "events.json": true, "emotions.json": true, "recent_discussion_digest.json": true}
	for i, dec := range resp.Decisions {
		action := strings.ToLower(strings.TrimSpace(dec.Action))
		if !allowed[action] { return fmt.Errorf("invalid action at decision %d", i) }
		if !allowedFiles[strings.TrimSpace(dec.TargetFile)] { return fmt.Errorf("invalid target_file at decision %d", i) }
		if strings.TrimSpace(dec.ReferenceThreadID) == "" { return fmt.Errorf("missing reference_thread_id at decision %d", i) }
		if strings.TrimSpace(dec.Title) == "" && action != "skip" { return fmt.Errorf("missing title at decision %d", i) }
		if strings.TrimSpace(dec.Content) == "" && action != "skip" { return fmt.Errorf("missing content at decision %d", i) }
		if dec.Confidence < 0 || dec.Confidence > 1 || dec.Importance < 0 || dec.Importance > 1 { return fmt.Errorf("out of range score at decision %d", i) }
		if action == "skip" && strings.TrimSpace(dec.SkipReason) == "" { return fmt.Errorf("skip needs skip_reason at decision %d", i) }
		if action == "merge" && len(dec.RelatedIndexIDs) == 0 { return fmt.Errorf("merge needs related_index_ids at decision %d", i) }
	}
	return nil
}

func (r *AgentRuntime) applyAutodreamDecisions(decisions []AutodreamDecision, threadID string, summary WindowSummary) ([]string, error) {
	if len(decisions) == 0 { return nil, fmt.Errorf("no decisions to apply") }
	updatedFiles := make([]string, 0)
	for _, dec := range decisions {
		action := strings.ToLower(strings.TrimSpace(dec.Action))
		if action == "skip" { continue }
		mem := candidateMemory{TargetFile: dec.TargetFile, Title: dec.Title, Value: dec.Content, Keywords: dec.Keywords, Importance: dec.Importance, Confidence: dec.Confidence, TargetID: dec.ReferenceThreadID}
		indexStatus := "active"
		switch action {
		case "add", "update":
			indexStatus = "active"
		case "merge":
			indexStatus = "merged"
		default:
			return nil, fmt.Errorf("unsupported action: %s", dec.Action)
		}
		if dec.Confidence < 0.55 { indexStatus = "rejected" }
		switch dec.TargetFile {
		case "profile.json": if err := updateProfile(r, mem); err != nil { return nil, err }
		case "relationship.json": if err := updateRelationship(r, mem); err != nil { return nil, err }
		case "preferences.json": if err := appendMemoryEntry(r, filepath.Join("data", "memory", "long_term", "preferences.json"), mem); err != nil { return nil, err }
		case "opinions.json": if err := appendMemoryEntry(r, filepath.Join("data", "memory", "long_term", "opinions.json"), mem); err != nil { return nil, err }
		case "events.json": if err := appendMemoryEntry(r, filepath.Join("data", "memory", "mid_term", "events.json"), mem); err != nil { return nil, err }
		case "emotions.json": if err := updateEmotions(r, mem); err != nil { return nil, err }
		case "recent_discussion_digest.json": if err := updateDigest(r, summaryRecord{WindowID: threadID, ConversationBrief: mem.Value, TargetFiles: []string{"recent_discussion_digest.json"}, Sensitive: summary.Sensitive}); err != nil { return nil, err }
		}
		updatedFiles = append(updatedFiles, dec.TargetFile)
		entryID := fmt.Sprintf("%s-%s-%s", threadID, dec.TargetFile, normalizeIDPart(dec.Title))
		if action == "merge" && len(dec.RelatedIndexIDs) > 0 { entryID = fmt.Sprintf("%s-merge-%s", threadID, normalizeIDPart(dec.Title)) }
		if err := r.Memory.AppendIndexEntry(memory.MemoryIndexEntry{ID: entryID, TargetType: dec.TargetFile, File: dec.TargetFile, TargetPath: mem.TargetID, Keywords: dec.Keywords, Importance: dec.Importance, Confidence: dec.Confidence, Sensitivity: summary.Sensitive, UpdatedAt: time.Now().Format(time.RFC3339), DefaultInclude: dec.TargetFile == "profile.json" || dec.TargetFile == "relationship.json", Title: dec.Title, Snippet: dec.Content, Status: indexStatus, MergedInto: firstNonEmpty(dec.RelatedIndexIDs), ThreadID: threadID, LastAttemptAt: time.Now().Format(time.RFC3339)}); err != nil { return nil, err }
	}
	return uniqueStrings(updatedFiles), nil
}

func (r *AgentRuntime) applyAutodreamFromWindowSummary(summary *WindowSummary) error {
	if summary == nil { return nil }
	pack, err := r.buildAutodreamInputPack(summary.WindowID, *summary)
	if err != nil { return err }
	result, err := r.runAutodreamDecision(pack)
	if err != nil { return err }
	_, err = r.applyAutodreamDecisions(result.Decisions, summary.WindowID, *summary)
	return err
}

func filterIndexByThread(entries []memory.MemoryIndexEntry, threadID string) []memory.MemoryIndexEntry {
	filtered := make([]memory.MemoryIndexEntry, 0)
	for _, entry := range entries {
		if entry.ThreadID == threadID || strings.Contains(entry.ID, threadID) || strings.Contains(entry.TargetPath, threadID) {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 { return entries }
	return filtered
}

func extractJSONObjectOrArray(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "```") {
		raw = stripCodeFences(raw)
	}
	startObj := strings.Index(raw, "{")
	startArr := strings.Index(raw, "[")
	start := -1
	if startObj >= 0 && startArr >= 0 {
		if startObj < startArr { start = startObj } else { start = startArr }
	} else if startObj >= 0 {
		start = startObj
	} else if startArr >= 0 {
		start = startArr
	}
	if start < 0 {
		return strings.TrimSpace(raw)
	}
	candidate := strings.TrimSpace(raw[start:])
	if strings.HasPrefix(candidate, "{") {
		if end := findBalancedJSONEnd(candidate, '{', '}'); end >= 0 {
			return strings.TrimSpace(candidate[:end+1])
		}
	}
	if strings.HasPrefix(candidate, "[") {
		if end := findBalancedJSONEnd(candidate, '[', ']'); end >= 0 {
			return strings.TrimSpace(candidate[:end+1])
		}
	}
	return strings.TrimSpace(candidate)
}

func stripCodeFences(raw string) string {
	lines := strings.Split(raw, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func findBalancedJSONEnd(s string, open, close rune) int {
	depth := 0
	inString := false
	escaped := false
	for i, r := range s {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		if r == '"' {
			inString = true
			continue
		}
		if r == open {
			depth++
		}
		if r == close {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func normalizeAutodreamDecision(dec autodreamDecisionLoose, threadID string) AutodreamDecision {
	action := normalizeAutodreamAction(dec.Action)
	referenceThreadID := strings.TrimSpace(dec.ReferenceThreadID)
	if referenceThreadID == "" {
		referenceThreadID = threadID
	}
	return AutodreamDecision{
		TargetFile:        strings.TrimSpace(dec.TargetFile),
		Action:            action,
		MemoryType:        strings.TrimSpace(dec.MemoryType),
		Title:             strings.TrimSpace(dec.Title),
		Content:           stringifyAutodreamContent(dec.Content),
		Keywords:          uniqueStrings(dec.Keywords),
		Importance:        clamp01(dec.Importance),
		Confidence:        clamp01(dec.Confidence),
		Reason:            strings.TrimSpace(dec.Reason),
		SkipReason:        strings.TrimSpace(dec.SkipReason),
		ReferenceThreadID: referenceThreadID,
		RelatedIndexIDs:   uniqueStrings(dec.RelatedIndexIDs),
	}
}

func normalizeAutodreamAction(action string) string {
	key := strings.ToLower(strings.TrimSpace(action))
	if key == "" {
		return key
	}
	mapping := map[string]string{
		"add": "add",
		"create": "add",
		"append": "add",
		"new": "add",
		"update": "update",
		"modify": "update",
		"edit": "update",
		"revise": "update",
		"merge": "merge",
		"combine": "merge",
		"consolidate": "merge",
		"skip": "skip",
		"ignore": "skip",
		"discard": "skip",
		"retain": "skip",
		"none": "skip",
		"pass": "skip",
	}
	if mapped, ok := mapping[key]; ok {
		return mapped
	}
	return key
}

func stringifyAutodreamContent(content any) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return strings.TrimSpace(fmt.Sprint(v))
		}
		return strings.TrimSpace(string(b))
	}
}

func clamp01(v float64) float64 {
	if v < 0 { return 0 }
	if v > 1 { return 1 }
	return v
}

func (r *AgentRuntime) combineWindowSummarySegments(summary WindowSummary) string {
	segments := make([]string, 0, len(summary.SummarySegments))
	if len(summary.SummarySegments) > 0 {
		sorted := append([]SummarySegment(nil), summary.SummarySegments...)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt.Before(sorted[j].CreatedAt) })
		for _, seg := range sorted { if strings.TrimSpace(seg.Summary) != "" { segments = append(segments, seg.Summary) } }
	}
	if len(segments) == 0 && strings.TrimSpace(summary.CurrentSummary) != "" { segments = append(segments, summary.CurrentSummary) }
	if len(segments) == 0 { return "暂无窗口摘要。" }
	return strings.Join(segments, "\n")
}

func updateDigest(r *AgentRuntime, summary summaryRecord) error { path := filepath.Join(r.RootDir, "data", "memory", "digest", "recent_discussion_digest.json"); var current map[string]any; if err := r.Memory.ReadJSON(path, &current); err != nil { current = map[string]any{} }; current["summary"] = summary.ConversationBrief; current["updated_at"] = time.Now().Format(time.RFC3339); current["target_files"] = summary.TargetFiles; return r.Memory.WriteJSON(path, current) }
func appendMemoryEntry(r *AgentRuntime, relPath string, mem candidateMemory) error { path := filepath.Join(r.RootDir, relPath); var current []map[string]any; if err := r.Memory.ReadJSON(path, &current); err != nil { current = []map[string]any{} }; current = dedupeMemoryEntries(current, mem); return r.Memory.WriteJSON(path, current) }
func dedupeMemoryEntries(current []map[string]any, mem candidateMemory) []map[string]any { key := normalizeIDPart(mem.Title + "-" + strings.Join(mem.Keywords, "-")); merged := false; for i, item := range current { if normalizeIDPart(fmt.Sprint(item["title"])) == normalizeIDPart(mem.Title) || normalizeIDPart(fmt.Sprint(item["id"])) == key || overlapKeywordsAny(item["keywords"], mem.Keywords) >= 2 { current[i]["value"] = mem.Value; current[i]["updated_at"] = time.Now().Format(time.RFC3339); current[i]["importance"] = maxExistingFloat(item["importance"], mem.Importance); current[i]["confidence"] = maxExistingFloat(item["confidence"], mem.Confidence); current[i]["status"] = chooseMemoryStatus(item["status"], mem.Importance, mem.Confidence); if kw, ok := item["keywords"].([]any); ok { current[i]["keywords"] = mergeKeywordsAny(kw, mem.Keywords) } else { current[i]["keywords"] = uniqueStrings(mem.Keywords) }; if current[i]["status"] == "archived" { current[i]["archived_at"] = time.Now().Format(time.RFC3339) }; merged = true; break } }; if !merged { status := chooseMemoryStatus(nil, mem.Importance, mem.Confidence); item := map[string]any{"id": key, "title": mem.Title, "value": mem.Value, "keywords": uniqueStrings(mem.Keywords), "importance": mem.Importance, "confidence": mem.Confidence, "updated_at": time.Now().Format(time.RFC3339), "status": status}; if status == "archived" { item["archived_at"] = time.Now().Format(time.RFC3339) }; current = append(current, item) }; return current }
func chooseMemoryStatus(existing any, importance, confidence float64) string { if s, ok := existing.(string); ok && s == "resolved" { return "resolved" }; if importance < 0.35 || confidence < 0.45 { return "archived" }; return "active" }
func overlapKeywordsAny(values any, more []string) int { kws := []string{}; switch v := values.(type) { case []any: for _, x := range v { if s, ok := x.(string); ok { kws = append(kws, s) } }; case []string: kws = append(kws, v...) }; set := map[string]bool{}; for _, k := range kws { set[strings.ToLower(strings.TrimSpace(k))] = true }; count := 0; for _, k := range more { if set[strings.ToLower(strings.TrimSpace(k))] { count++ } }; return count }
func normalizeIDPart(s string) string { s = strings.ToLower(strings.TrimSpace(s)); s = strings.ReplaceAll(s, " ", "-"); s = strings.ReplaceAll(s, "/", "-"); return s }
func firstNonEmpty(values []string) string { for _, v := range values { if strings.TrimSpace(v) != "" { return v } }; return "" }
func maxf(a, b float64) float64 { if a > b { return a }; return b }
func maxExistingFloat(v any, fallback float64) float64 { switch t := v.(type) { case float64: return maxf(t, fallback); case int: return maxf(float64(t), fallback); default: return fallback } }
func mergeKeywordsAny(values []any, more []string) []string { kws := make([]string, 0, len(values)+len(more)); for _, v := range values { if s, ok := v.(string); ok { kws = append(kws, s) } }; kws = append(kws, more...); return uniqueStrings(kws) }
func mapKeys(m map[string]bool) []string { out := make([]string, 0, len(m)); for k := range m { out = append(out, k) }; sort.Strings(out); return out }
func filterNonEmpty(values []string) []string { out := make([]string, 0, len(values)); for _, v := range values { if strings.TrimSpace(v) != "" { out = append(out, v) } }; return out }
func max(a, b int) int { if a > b { return a }; return b }
