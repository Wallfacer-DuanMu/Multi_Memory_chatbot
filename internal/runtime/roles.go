package runtime

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"whale/internal/session"
)

type RoleProfile struct {
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

func (r *AgentRuntime) EnsureDefaultRoles() error {
	store, err := r.Session.LoadRoleStore()
	if err != nil {
		return err
	}
	changed := false
	if _, ok := findRoleByID(recordsToProfiles(store.Roles), "humphrey"); !ok {
		store.Roles = append(store.Roles, session.RoleRecord{
			ID:        "humphrey",
			Name:      "汉弗莱",
			Persona:   "英剧《Yes, Minister》中的 Sir Humphrey Appleby。老练、克制、机智、略带官僚幽默，像一位高段位的管家型政治顾问。",
			Avatar:    "/头像/汉弗莱.png",
			Theme:     "humphrey",
			Builtin:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
		changed = true
	}
	if _, ok := findRoleByID(recordsToProfiles(store.Roles), "yongzhen"); !ok {
		store.Roles = append(store.Roles, session.RoleRecord{
			ID:        "yongzhen",
			Name:      "永真",
			Persona:   "《只狼》中的永真。温柔、沉静、疗愈、善于观察，面对用户时能给出安抚、整理与细腻的判断。",
			Avatar:    "/头像/永真.png",
			Theme:     "yongzhen",
			Builtin:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
		changed = true
	}
	if changed {
		return r.Session.SaveRoleStore(store)
	}
	return nil
}

func (r *AgentRuntime) LoadRoles() ([]RoleProfile, error) {
	if err := r.EnsureDefaultRoles(); err != nil {
		return nil, err
	}
	store, err := r.Session.LoadRoleStore()
	if err != nil {
		return nil, err
	}
	changed := false
	for i := range store.Roles {
		if normalizeRoleID(store.Roles[i].ID) == "humphrey" && strings.TrimSpace(store.Roles[i].Avatar) != "/frontend/assets/avatars/humphrey.png" {
			store.Roles[i].Avatar = "/frontend/assets/avatars/humphrey.png"
			changed = true
		}
		if normalizeRoleID(store.Roles[i].ID) == "yongzhen" && strings.TrimSpace(store.Roles[i].Avatar) != "/frontend/assets/avatars/yongzhen.png" {
			store.Roles[i].Avatar = "/frontend/assets/avatars/yongzhen.png"
			changed = true
		}
	}
	if changed {
		if err := r.Session.SaveRoleStore(store); err != nil {
			return nil, err
		}
	}
	return recordsToProfiles(store.Roles), nil
}

func (r *AgentRuntime) CreateRole(name, persona, avatar string) (RoleProfile, error) {
	store, err := r.Session.LoadRoleStore()
	if err != nil {
		return RoleProfile{}, err
	}
	name = strings.TrimSpace(name)
	persona = strings.TrimSpace(persona)
	if name == "" {
		return RoleProfile{}, fmt.Errorf("name is required")
	}
	if persona == "" {
		return RoleProfile{}, fmt.Errorf("persona is required")
	}
	id := normalizeRoleID(name)
	if _, ok := findRoleByID(recordsToProfiles(store.Roles), id); ok {
		return RoleProfile{}, fmt.Errorf("role already exists: %s", id)
	}
	role := RoleProfile{
		ID:        id,
		Name:      name,
		Persona:   persona,
		Avatar:    strings.TrimSpace(avatar),
		Theme:     chooseThemeByID(id),
		Builtin:   false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.Roles = append(store.Roles, profileToRecord(role))
	if err := r.Session.SaveRoleStore(store); err != nil {
		return RoleProfile{}, err
	}
	return role, nil
}

func (r *AgentRuntime) UpdateRole(roleID, name, persona, avatar string) (RoleProfile, error) {
	store, err := r.Session.LoadRoleStore()
	if err != nil {
		return RoleProfile{}, err
	}
	roleID = normalizeRoleID(roleID)
	for i := range store.Roles {
		if normalizeRoleID(store.Roles[i].ID) != roleID {
			continue
		}
		if strings.TrimSpace(name) != "" {
			store.Roles[i].Name = strings.TrimSpace(name)
		}
		if strings.TrimSpace(persona) != "" {
			store.Roles[i].Persona = strings.TrimSpace(persona)
		}
		if strings.TrimSpace(avatar) != "" {
			if old := strings.TrimSpace(store.Roles[i].Avatar); old != "" && old != strings.TrimSpace(avatar) {
				_ = r.deleteAvatarFile(old)
			}
			store.Roles[i].Avatar = strings.TrimSpace(avatar)
		}
		store.Roles[i].UpdatedAt = time.Now()
		if err := r.Session.SaveRoleStore(store); err != nil {
			return RoleProfile{}, err
		}
		return recordToProfile(store.Roles[i]), nil
	}
	return RoleProfile{}, fmt.Errorf("role not found: %s", roleID)
}

func (r *AgentRuntime) DeleteRole(roleID string) error {
	store, err := r.Session.LoadRoleStore()
	if err != nil {
		return err
	}
	roleID = normalizeRoleID(roleID)
	filtered := make([]session.RoleRecord, 0, len(store.Roles))
	removed := false
	for _, role := range store.Roles {
		if normalizeRoleID(role.ID) != roleID {
			filtered = append(filtered, role)
			continue
		}
		if role.Builtin || roleID == "humphrey" || roleID == "yongzhen" {
			return fmt.Errorf("builtin role cannot be deleted")
		}
		if strings.TrimSpace(role.Avatar) != "" {
			_ = r.deleteAvatarFile(role.Avatar)
		}
		removed = true
	}
	if !removed {
		return fmt.Errorf("role not found: %s", roleID)
	}
	store.Roles = filtered
	return r.Session.SaveRoleStore(store)
}

func (r *AgentRuntime) SaveAvatarUpload(roleID string, file multipart.File, header *multipart.FileHeader) (string, error) {
	if file == nil || header == nil {
		return "", nil
	}
	defer file.Close()
	if err := os.MkdirAll(filepath.Join(r.RootDir, "frontend", "uploads", "avatars"), 0o755); err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".png"
	}
	filename := fmt.Sprintf("%s-%d%s", normalizeRoleID(roleID), time.Now().UnixNano(), ext)
	fullPath := filepath.Join(r.RootDir, "frontend", "uploads", "avatars", filename)
	out, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := out.ReadFrom(file); err != nil {
		return "", err
	}
	return "/frontend/uploads/avatars/" + filename, nil
}

func (r *AgentRuntime) deleteAvatarFile(rel string) error {
	rel = strings.TrimSpace(rel)
	if rel == "" || strings.HasPrefix(rel, "http://") || strings.HasPrefix(rel, "https://") {
		return nil
	}
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.TrimPrefix(rel, "/frontend/")
	fullPath := filepath.Join(r.RootDir, "frontend", rel)
	return os.Remove(fullPath)
}

func (r *AgentRuntime) ResolveRole(requestedRole string) RoleProfile {
	roles, err := r.LoadRoles()
	if err == nil {
		if role, ok := findRoleByID(roles, normalizeRoleID(requestedRole)); ok {
			return role
		}
	}
	return RoleProfile{ID: "humphrey", Name: "汉弗莱", Persona: "英剧《Yes, Minister》中的 Sir Humphrey Appleby。", Theme: "humphrey", Builtin: true}
}

func buildSystemPromptForRole(role RoleProfile, roles []RoleProfile) string {
	memberLines := make([]string, 0, len(roles))
	for _, item := range roles {
		memberLines = append(memberLines, fmt.Sprintf("- %s（role_id=%s）：%s", item.Name, item.ID, strings.TrimSpace(item.Persona)))
	}
	return strings.TrimSpace(fmt.Sprintf(`回复的语言，取决于用户的提问语言。

你当前扮演的角色：%s（role_id=%s）。
你的核心人设：%s

当前对话不是一对一，而是一个动态多人群组。群组中永远至少有用户，以及若干 AI 角色。
你在回复前，必须先理解历史消息里每一条发言是谁说的。历史记录会明确标注“用户”或具体角色名。
你必须遵守以下规则：
1. 不要把别的角色说过的话误认成你自己说的。
2. 你可以针对用户发言直接回复，也可以适度评价、补充、呼应其他角色先前的观点。
3. 如果历史里已经有用户与其他角色的讨论，你应把它当成你此刻看到的群聊记录，再在此基础上继续发言。
4. 允许适度互动性，但不要为了互动而打断用户主线。
5. 保持你自己的人设稳定，不要突然切换成别的角色口吻。
6. 如果用户点名某位角色，你只需以你当前这个角色身份回应用户，不要假装替别的角色发言。

当前群成员设定：
%s`, role.Name, role.ID, role.Persona, strings.Join(memberLines, "\n")))
}

func displayRoleNameFromProfiles(roleID string, roles []RoleProfile) string {
	if role, ok := findRoleByID(roles, roleID); ok {
		return role.Name
	}
	return displayRoleName(roleID)
}

func findRoleByID(roles []RoleProfile, id string) (RoleProfile, bool) {
	id = normalizeRoleID(id)
	for _, role := range roles {
		if normalizeRoleID(role.ID) == id {
			return role, true
		}
	}
	return RoleProfile{}, false
}

func normalizeRoleID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	switch value {
	case "doubao", "yongzhen", "永真":
		return "yongzhen"
	case "humphrey", "汉弗莱":
		return "humphrey"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return fmt.Sprintf("role-%d", time.Now().Unix())
	}
	return out
}

func chooseThemeByID(id string) string {
	if normalizeRoleID(id) == "humphrey" {
		return "humphrey"
	}
	return "yongzhen"
}

func normalizeLegacyRole(role string) string {
	return session.NormalizeAgentRoleForRuntime(role)
}

func recordToProfile(record session.RoleRecord) RoleProfile {
	return RoleProfile{
		ID:          record.ID,
		Name:        record.Name,
		Persona:     record.Persona,
		Avatar:      record.Avatar,
		Theme:       record.Theme,
		Builtin:     record.Builtin,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
		SystemNotes: record.SystemNotes,
	}
}

func profileToRecord(profile RoleProfile) session.RoleRecord {
	return session.RoleRecord{
		ID:          profile.ID,
		Name:        profile.Name,
		Persona:     profile.Persona,
		Avatar:      profile.Avatar,
		Theme:       profile.Theme,
		Builtin:     profile.Builtin,
		CreatedAt:   profile.CreatedAt,
		UpdatedAt:   profile.UpdatedAt,
		SystemNotes: profile.SystemNotes,
	}
}

func recordsToProfiles(records []session.RoleRecord) []RoleProfile {
	profiles := make([]RoleProfile, 0, len(records))
	for _, record := range records {
		profiles = append(profiles, recordToProfile(record))
	}
	return profiles
}
