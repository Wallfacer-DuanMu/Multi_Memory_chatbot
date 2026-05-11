const API_BASE = "http://127.0.0.1:8080";
const LONG_TERM_FILES = [
  { key: "profile", label: "profile.json", path: "../data/memory/long_term/profile.json" },
  { key: "relationship", label: "relationship.json", path: "../data/memory/long_term/relationship.json" },
  { key: "preferences", label: "preferences.json", path: "../data/memory/long_term/preferences.json" },
  { key: "opinions", label: "opinions.json", path: "../data/memory/long_term/opinions.json" },
  { key: "digest", label: "recent_discussion_digest.json", path: "../data/memory/digest/recent_discussion_digest.json" },
  { key: "index", label: "memory_index.json", path: "../data/memory/index/memory_index.json" },
];
const MID_TERM_FILES = [
  { key: "events", label: "events.json", path: "../data/memory/mid_term/events.json" },
  { key: "emotions", label: "emotions.json", path: "../data/memory/mid_term/emotions.json" },
  { key: "runtime", label: "runtime_state.json", path: "../data/system/runtime_state.json" },
  { key: "policy", label: "memory_policy.json", path: "../data/system/memory_policy.json" },
];

const DEFAULT_ROLE_THEME = {
  title: "角色",
  apiPrompt: "default",
};

function normalizeRole(role) {
  const value = String(role || "").trim().toLowerCase();
  if (!value) return "humphrey";
  if (value === "doubao" || value === "yongzhen") return "yongzhen";
  return value;
}

function getThemeForRole() {
  return DEFAULT_ROLE_THEME;
}

const state = {
  role: "humphrey",
  roles: [],
  windows: [],
  activeWindowId: "",
  messagesByWindow: new Map(),
  summaries: new Map(),
  threadStates: new Map(),
  recallByWindow: new Map(),
  memoryFiles: new Map(),
  latestChatTrace: [],
  latestDreamTrace: [],
  latestDreamResult: null,
  selectedMemoryKey: "",
  selectedDreamWindowId: "",
  roleComposerOpen: false,
  roleSettingsOpen: false,
  dreamRun: {
    inProgress: false,
    mode: "",
    windowId: "",
    startedAt: 0,
  },
};

const els = {
  navTabs: [...document.querySelectorAll(".nav-tab")],
  views: [...document.querySelectorAll(".view-panel")],
  sessionList: document.getElementById("sessionList"),
  messageList: document.getElementById("messageList"),
  traceStatus: document.getElementById("traceStatus"),
  composerForm: document.getElementById("composerForm"),
  composerInput: document.getElementById("composerInput"),
  newChatBtn: document.getElementById("newChatBtn"),
  renameChatBtn: document.getElementById("renameChatBtn"),
  deleteChatBtn: document.getElementById("deleteChatBtn"),
  sendBtn: document.getElementById("sendBtn"),
  chatTitle: document.getElementById("chatTitle"),
  activeWindowName: document.getElementById("activeWindowName"),
  windowCount: document.getElementById("windowCount"),
  memoryCount: document.getElementById("memoryCount"),
  countdownLabel: document.getElementById("countdownLabel"),
  dreamCountdownInline: document.getElementById("dreamCountdownInline"),
  healthBadge: document.getElementById("healthBadge"),
  summaryList: document.getElementById("summaryList"),
  longTermList: document.getElementById("longTermList"),
  midTermList: document.getElementById("midTermList"),
  memoryDetail: document.getElementById("memoryDetail"),
  dreamSummaryList: document.getElementById("dreamSummaryList"),
  dreamTitle: document.getElementById("dreamTitle"),
  dreamSummaryDetail: document.getElementById("dreamSummaryDetail"),
  dreamImportResult: document.getElementById("dreamImportResult"),
  dreamMemoryLinks: document.getElementById("dreamMemoryLinks"),
  dreamTraceList: document.getElementById("dreamTraceList"),
  threadDreamBtn: document.getElementById("threadDreamBtn"),
  globalDreamBtn: document.getElementById("globalDreamBtn"),
  refreshAllBtn: document.getElementById("refreshAllBtn"),
  roleAvatarRail: document.getElementById("roleAvatarRail"),
  roleSpotlight: document.getElementById("roleSpotlight"),
  roleSpotlightAvatar: document.getElementById("roleSpotlightAvatar"),
  roleSpotlightName: document.getElementById("roleSpotlightName"),
  roleSpotlightTag: document.getElementById("roleSpotlightTag"),
  roleSpotlightPersona: document.getElementById("roleSpotlightPersona"),
  toggleRoleSettingsBtn: document.getElementById("toggleRoleSettingsBtn"),
  toggleRoleComposerBtn: document.getElementById("toggleRoleComposerBtn"),
  roleSettingsPanel: document.getElementById("roleSettingsPanel"),
  roleForm: document.getElementById("roleCreateForm"),
  roleNameInput: document.getElementById("roleNameInput"),
  rolePersonaInput: document.getElementById("rolePersonaInput"),
  roleAvatarFileInput: document.getElementById("roleAvatarFileInput"),
  roleManagerList: document.getElementById("roleManagerList"),
  sessionItemTemplate: document.getElementById("sessionItemTemplate"),
};

document.addEventListener("DOMContentLoaded", () => {
  bindEvents();
  startCountdown();
  initializeApp();
});

function bindEvents() {
  els.navTabs.forEach((tab) => tab.addEventListener("click", () => switchView(tab.dataset.view)));
  els.newChatBtn?.addEventListener("click", createWindow);
  els.renameChatBtn?.addEventListener("click", renameWindow);
  els.deleteChatBtn?.addEventListener("click", deleteWindow);
  els.composerForm?.addEventListener("submit", sendMessage);
  els.globalDreamBtn?.addEventListener("click", () => runDream("global"));
  els.threadDreamBtn?.addEventListener("click", () => runDream("thread"));
  els.refreshAllBtn?.addEventListener("click", initializeApp);
  els.roleForm?.addEventListener("submit", createRole);
  els.toggleRoleSettingsBtn?.addEventListener("click", toggleRoleSettings);
  els.toggleRoleComposerBtn?.addEventListener("click", toggleRoleComposer);
  applyRoleTheme();
}

async function initializeApp() {
  await Promise.all([loadRoles(), checkHealth(), loadWindows(), loadMemorySummary(), loadMemoryFiles()]);
  renderMemoryColumns();
  renderDreamPanel();
  applyRoleTheme();
}

async function loadRoles() {
  const data = await fetchJSON(`${API_BASE}/api/roles`, { roles: [] });
  state.roles = data.roles || [];
  renderRoleOrbit();
}

function renderRoleOrbit() {
  renderRoleAvatarRail();
  renderRoleSpotlight();
  renderRoleManager();
}

function renderRoleAvatarRail() {
  if (!els.roleAvatarRail) return;
  const roles = state.roles.length
    ? state.roles
    : [
        { id: "humphrey", name: "汉弗莱", builtin: true },
        { id: "yongzhen", name: "永真", builtin: true },
      ];
  els.roleAvatarRail.innerHTML = roles
    .map((role) => {
      const active = normalizeRole(role.id) === normalizeRole(state.role);
      return `
        <button class="role-avatar-chip ${active ? "active" : ""}" data-role-id="${escapeHTML(role.id)}" type="button">
          <div class="role-avatar-shell">${renderRoleAvatarVisual(role)}</div>
          <span class="role-avatar-label">${escapeHTML(role.name)}</span>
        </button>
      `;
    })
    .join("");

  [...els.roleAvatarRail.querySelectorAll(".role-avatar-chip")].forEach((button) => {
    button.addEventListener("click", () => {
      state.role = normalizeRole(button.dataset.roleId);
      applyRoleTheme();
      renderRoleOrbit();
      updateActiveWindowHeader();
      renderMessages();
    });
  });
}

function renderRoleSpotlight() {
  const role = getRoleDisplay(state.role) || state.roles[0] || { id: "humphrey", name: "汉弗莱", persona: "默认角色", builtin: true };
  if (els.roleSpotlightAvatar) {
    els.roleSpotlightAvatar.innerHTML = renderRoleAvatarVisual(role, true);
  }
  if (els.roleSpotlightName) {
    els.roleSpotlightName.textContent = role.name || "角色";
  }
  if (els.roleSpotlightPersona) {
    els.roleSpotlightPersona.textContent = role.persona || "请选择上方角色头像，查看其基础介绍，并立即切换下方聊天角色。";
  }
  if (els.roleSpotlightTag) {
    els.roleSpotlightTag.textContent = role.builtin ? "内置角色 · 当前发言" : "自定义角色 · 当前发言";
  }
}

function toggleRoleSettings() {
  state.roleSettingsOpen = !state.roleSettingsOpen;
  if (els.roleSettingsPanel) {
    els.roleSettingsPanel.classList.toggle("collapsed", !state.roleSettingsOpen);
  }
  if (els.toggleRoleSettingsBtn) {
    els.toggleRoleSettingsBtn.innerHTML = state.roleSettingsOpen
      ? '<i class="fa-solid fa-xmark"></i><span>收起设置</span>'
      : '<i class="fa-solid fa-gear"></i><span>设置</span>';
  }
}

function toggleRoleComposer() {
  state.roleComposerOpen = !state.roleComposerOpen;
  if (!els.roleForm) return;
  els.roleForm.classList.toggle("collapsed", !state.roleComposerOpen);
  if (els.toggleRoleComposerBtn) {
    els.toggleRoleComposerBtn.innerHTML = state.roleComposerOpen
      ? '<i class="fa-solid fa-xmark"></i><span>收起创建器</span>'
      : '<i class="fa-solid fa-user-plus"></i><span>新增角色</span>';
  }
}

function switchView(view) {
  els.navTabs.forEach((tab) => tab.classList.toggle("active", tab.dataset.view === view));
  els.views.forEach((panel) => panel.classList.toggle("active", panel.id === `view-${view}`));
}

async function checkHealth() {
  try {
    const res = await fetch(`${API_BASE}/api/health`);
    if (!res.ok) throw new Error("health check failed");
    els.healthBadge.textContent = "在线";
    els.healthBadge.className = "status-pill online";
  } catch {
    els.healthBadge.textContent = "离线";
    els.healthBadge.className = "status-pill offline";
  }
}

async function loadWindows() {
  try {
    const res = await fetch(`${API_BASE}/api/windows`);
    if (!res.ok) throw new Error("load windows failed");
    const data = await res.json();
    state.windows = data.windows || [];
    els.windowCount.textContent = String(state.windows.length);

    if (!state.activeWindowId && state.windows.length) {
      state.activeWindowId = state.windows[0].id;
    } else if (state.activeWindowId && !state.windows.some((item) => item.id === state.activeWindowId)) {
      state.activeWindowId = state.windows[0]?.id || "";
    }

    await Promise.all(state.windows.map((item) => hydrateWindow(item.id)));
    renderSessionList();
    renderDreamPanel();
    updateActiveWindowHeader();
    renderMessages();
    renderChatTrace();
  } catch (error) {
    console.error(error);
    state.windows = [];
    renderSessionList();
  }
}

async function hydrateWindow(windowId) {
  const [messages, statePayload, recallItems] = await Promise.all([
    fetchJSON(`${API_BASE}/api/windows/${windowId}/messages`, []),
    fetchJSON(`${API_BASE}/api/windows/${windowId}/state`, null),
    fetchJSON(`${API_BASE}/api/windows/${windowId}/recall-preview`, { items: [] }),
  ]);

  state.messagesByWindow.set(windowId, normalizeMessages(messages));
  if (statePayload?.summary) state.summaries.set(windowId, statePayload.summary);
  if (statePayload?.thread_state) state.threadStates.set(windowId, statePayload.thread_state);
  state.recallByWindow.set(windowId, recallItems.items || []);
}

function getRoleDisplay(roleId) {
  const raw = String(roleId || "").trim();
  const normalized = normalizeRole(raw);
  return state.roles.find((role) => normalizeRole(role.id) === normalized || String(role.id || "").trim() === raw) || null;
}

function getRoleLabel(roleId) {
  const role = getRoleDisplay(roleId);
  if (role?.name) return role.name;
  const raw = String(roleId || "").trim();
  if (raw && normalizeRole(raw) !== raw) {
    return raw;
  }
  if (raw) {
    return `已删除角色 · ${raw}`;
  }
  return "对方";
}

function getRoleAvatarMarkup(roleId) {
  const role = getRoleDisplay(roleId);
  const avatarSrc = resolveAvatarPath(role);
  if (avatarSrc) {
    return `<img src="${escapeHTML(avatarSrc)}" alt="${escapeHTML(role?.name || roleId)}头像" class="message-avatar-image" loading="eager" decoding="sync" />`;
  }
  return getAvatarFallback(roleId);
}

function renderRoleAvatarVisual(role, large = false) {
  const avatarSrc = resolveAvatarPath(role);
  if (avatarSrc) {
    return `<img src="${escapeHTML(avatarSrc)}" alt="${escapeHTML(role?.name || role?.id || '角色')}头像" class="${large ? "role-spotlight-image" : "role-manager-avatar-image"}" loading="eager" decoding="sync" />`;
  }
  return escapeHTML((role?.name || role?.id || "角").slice(0, 1));
}

function resolveAvatarPath(role) {
  const builtinPath = getBuiltinAvatarPath(role);
  if (builtinPath) {
    return encodeURI(`${builtinPath}?__ts=${Date.now()}`);
  }

  const value = String(role?.avatar || "").trim();
  if (!value) return "";
  if (/^https?:\/\//i.test(value) || value.startsWith("data:")) {
    return value;
  }

  const normalized = value.replaceAll('\\', '/');
  if (normalized.startsWith('/uploads/')) {
    return encodeURI(`/frontend${normalized}?__ts=${Date.now()}`);
  }
  if (normalized.startsWith('/')) {
    return encodeURI(`${normalized}?__ts=${Date.now()}`);
  }
  return encodeURI(`${new URL(normalized, window.location.href).pathname}?__ts=${Date.now()}`);
}

function getBuiltinAvatarPath(role) {
  const roleId = normalizeRole(role?.id || role);
  const roleName = String(role?.name || "").trim();
  if (roleId === "humphrey" || roleName === "汉弗莱") {
    return "/头像/汉弗莱.png";
  }
  if (roleId === "yongzhen" || roleName === "永真") {
    return "/头像/永真.png";
  }
  return "";
}

function normalizeMessages(rawTurns) {
  return (rawTurns || []).flatMap((turn) => {
    const userText = turn.user_input || turn.userInput || "";
    const assistantText =
      turn.assistant_response || turn.assistant_reply || turn.reply || turn.assistantReply || turn.assistantResponse || "";
    const turnId = turn.turn_id || turn.turnId || "-";
    const rawRole = turn.agent_role || turn.agentRole || "humphrey";
    const agentRole = normalizeRole(rawRole);
    const roleLabel = getRoleLabel(rawRole);
    return [
      {
        role: "user",
        text: userText,
        meta: `Turn ${turnId}`,
        label: "用户",
      },
      {
        role: "assistant",
        agentRole,
        text: assistantText,
        meta: `Turn ${turnId}`,
        label: roleLabel,
      },
    ].filter((item) => item.text.trim());
  });
}

function renderSessionList() {
  if (!state.windows.length) {
    els.sessionList.innerHTML = `<div class="empty-mini">暂无会话，请先新建一个线程。</div>`;
    return;
  }

  const fragment = document.createDocumentFragment();
  state.windows.forEach((windowItem) => {
    const button = els.sessionItemTemplate.content.firstElementChild.cloneNode(true);
    button.classList.toggle("active", windowItem.id === state.activeWindowId);
    const roleName = getRoleDisplay(windowItem.agent_role || windowItem.agentRole)?.name || "群聊";
    button.querySelector(".session-title").textContent = windowItem.title || windowItem.id;
    button.querySelector(".session-meta").textContent = `${windowItem.id} · ${windowItem.turn_count || 0} 轮 · 当前主角色 ${roleName}`;
    button.addEventListener("click", async () => {
      state.activeWindowId = windowItem.id;
      state.selectedDreamWindowId = windowItem.id;
      if (!state.messagesByWindow.has(windowItem.id)) {
        await hydrateWindow(windowItem.id);
      }
      renderSessionList();
      updateActiveWindowHeader();
      renderMessages();
      renderDreamPanel();
    });
    fragment.appendChild(button);
  });
  els.sessionList.innerHTML = "";
  els.sessionList.appendChild(fragment);
}

function updateActiveWindowHeader() {
  const activeWindow = state.windows.find((item) => item.id === state.activeWindowId);
  const currentRole = getRoleDisplay(state.role)?.name || getRoleLabel(state.role);
  els.chatTitle.textContent = activeWindow ? `${activeWindow.title} · 当前发言角色 ${currentRole}` : "请选择一个会话";
  els.activeWindowName.textContent = activeWindow ? activeWindow.id : "未选择";
}

function renderMessages() {
  const messages = state.messagesByWindow.get(state.activeWindowId) || [];
  if (!state.activeWindowId || !messages.length) {
    els.messageList.className = "message-list empty-state";
    els.messageList.innerHTML = `
      <div class="empty-illustration"><i class="fa-regular fa-message"></i></div>
      <h4>这个线程还没有聊天记录</h4>
      <p>它现在是一个动态多人群聊。点击上方角色头像，就能立刻切换成对应角色继续发言。</p>
    `;
    return;
  }

  els.messageList.className = "message-list";
  els.messageList.innerHTML = messages.map((message) => renderMessageItem(message)).join("");
  els.messageList.scrollTop = els.messageList.scrollHeight;
}

function renderMessageItem(message) {
  const role = message.role === "assistant" ? "assistant" : "user";
  const messageRole = normalizeRole(message.agentRole || state.role);
  const label = role === "assistant" ? message.label || getRoleLabel(message.agentRole || messageRole) : message.label || "用户";
  const avatar = role === "assistant"
    ? `<div class="message-avatar avatar-assistant" aria-hidden="true">${getRoleAvatarMarkup(message.agentRole || messageRole)}</div>`
    : `<div class="message-avatar avatar-user" aria-hidden="true">我</div>`;
  const metaText = message.pending ? message.meta || "对方正在输入..." : message.meta || "";
  const bubbleContent = message.pending
    ? '<div class="typing-indicator" aria-label="对方正在输入"><span></span><span></span><span></span></div>'
    : escapeHTML(message.text);

  return `
    <article class="message-row ${role}${message.pending ? " pending" : ""}">
      ${avatar}
      <div class="message-stack">
        <div class="message-speaker ${role}">${escapeHTML(label)}</div>
        <div class="message-meta">${escapeHTML(metaText)}</div>
        <div class="message-bubble${message.pending ? " message-bubble-pending" : ""}">${bubbleContent}</div>
      </div>
    </article>
  `;
}

function renderChatTrace() {
  if (!els.traceStatus) return;
  if (!state.latestChatTrace.length) {
    els.traceStatus.textContent = "等待发送消息。";
    return;
  }

  const latest = state.latestChatTrace[state.latestChatTrace.length - 1] || {};
  const parts = [latest.stage || latest.name || "处理中", latest.message || latest.detail || ""]
    .map((item) => String(item || "").trim())
    .filter(Boolean);
  els.traceStatus.textContent = parts.join(" · ") || "处理中...";
}

async function createWindow() {
  const title = window.prompt("请输入新会话标题", `新会话 ${state.windows.length + 1}`);
  if (!title) return;
  const res = await fetch(`${API_BASE}/api/windows`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title }),
  });
  if (!res.ok) {
    window.alert("新建会话失败。");
    return;
  }
  const created = await res.json();
  state.activeWindowId = created.id;
  state.selectedDreamWindowId = created.id;
  await loadWindows();
}

async function renameWindow() {
  if (!state.activeWindowId) {
    window.alert("请先选择会话。");
    return;
  }
  const current = state.windows.find((item) => item.id === state.activeWindowId);
  const title = window.prompt("请输入新的会话标题", current?.title || "");
  if (!title) return;
  const res = await fetch(`${API_BASE}/api/windows/${state.activeWindowId}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title }),
  });
  if (!res.ok) {
    window.alert("重命名失败。");
    return;
  }
  await loadWindows();
}

async function deleteWindow() {
  if (!state.activeWindowId) {
    window.alert("请先选择会话。");
    return;
  }
  const ok = window.confirm("确认删除当前会话及其摘要记录吗？");
  if (!ok) return;
  const res = await fetch(`${API_BASE}/api/windows/${state.activeWindowId}`, { method: "DELETE" });
  if (!res.ok) {
    window.alert("删除失败。");
    return;
  }
  state.messagesByWindow.delete(state.activeWindowId);
  state.summaries.delete(state.activeWindowId);
  state.threadStates.delete(state.activeWindowId);
  state.recallByWindow.delete(state.activeWindowId);
  state.activeWindowId = "";
  await loadWindows();
}

async function createRole(event) {
  event.preventDefault();
  const name = els.roleNameInput?.value.trim();
  const persona = els.rolePersonaInput?.value.trim();
  const file = els.roleAvatarFileInput?.files?.[0];
  if (!name || !persona) {
    window.alert("新增角色时必须填写名字和人设。");
    return;
  }
  try {
    const formData = new FormData();
    formData.append("name", name);
    formData.append("persona", persona);
    if (file) formData.append("avatar", file);
    const res = await fetch(`${API_BASE}/api/roles`, {
      method: "POST",
      body: formData,
    });
    if (!res.ok) throw new Error("create role failed");
    const role = await res.json();
    await loadRoles();
    state.role = normalizeRole(role.id);
    applyRoleTheme();
    if (els.roleForm) els.roleForm.reset();
    state.roleComposerOpen = false;
    toggleRoleComposer();
    updateActiveWindowHeader();
  } catch (error) {
    console.error(error);
    window.alert("新增角色失败，请检查是否重名或后端未启动。");
  }
}

function renderRoleManager() {
  if (!els.roleManagerList) return;
  els.roleManagerList.innerHTML = state.roles
    .map((role) => {
      const builtin = role.id === "humphrey" || role.id === "yongzhen" || role.builtin;
      return `
        <article class="role-manager-row" data-role-id="${escapeHTML(role.id)}">
          <div class="role-manager-identity">
            <div class="role-manager-avatar">${renderRoleAvatarVisual(role)}</div>
            <div class="role-manager-heading">
              <h4>${escapeHTML(role.name)}</h4>
              <p class="role-manager-meta">${builtin ? "内置角色" : "自定义角色"}</p>
            </div>
          </div>
          <div class="role-editor inline">
            <textarea data-field="persona" rows="2" placeholder="填写角色设定">${escapeHTML(role.persona || "")}</textarea>
          </div>
          <div class="role-manager-actions inline">
            <button class="secondary-btn small" data-action="save">保存</button>
            <button class="danger-btn small" data-action="delete" ${builtin ? "disabled" : ""}>删除</button>
          </div>
        </article>
      `;
    })
    .join("");

  [...els.roleManagerList.querySelectorAll("[data-action='save']")].forEach((button) => {
    button.addEventListener("click", async () => {
      const card = button.closest("[data-role-id]");
      const roleId = card?.dataset.roleId;
      const persona = card?.querySelector("textarea[data-field='persona']")?.value.trim();
      if (!roleId || !persona) return;
      try {
        const res = await fetch(`${API_BASE}/api/roles/${roleId}`, {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ persona }),
        });
        if (!res.ok) throw new Error("save role failed");
        await loadRoles();
      } catch (error) {
        console.error(error);
        window.alert("保存角色人设失败。");
      }
    });
  });

  [...els.roleManagerList.querySelectorAll("[data-action='delete']")].forEach((button) => {
    button.addEventListener("click", async () => {
      const card = button.closest("[data-role-id]");
      const roleId = card?.dataset.roleId;
      if (!roleId) return;
      if (!window.confirm(`确认删除角色 ${roleId} 吗？`)) return;
      try {
        const res = await fetch(`${API_BASE}/api/roles/${roleId}`, { method: "DELETE" });
        if (!res.ok) throw new Error("delete role failed");
        if (normalizeRole(state.role) === normalizeRole(roleId)) {
          state.role = "humphrey";
        }
        await loadRoles();
        applyRoleTheme();
      } catch (error) {
        console.error(error);
        window.alert("删除角色失败。");
      }
    });
  });
}

async function sendMessage(event) {
  event.preventDefault();
  const text = els.composerInput.value.trim();
  if (!state.activeWindowId) {
    window.alert("请先创建或选择会话。");
    return;
  }
  if (!text) return;

  const windowId = state.activeWindowId;
  const activeRole = normalizeRole(state.role);
  const existing = [...(state.messagesByWindow.get(windowId) || [])];
  const optimisticMessages = [
    ...existing,
    { role: "user", text, meta: "已发送", label: "用户" },
    {
      role: "assistant",
      agentRole: activeRole,
      label: getRoleLabel(activeRole),
      text: "",
      meta: "对方正在输入...",
      pending: true,
    },
  ];

  state.messagesByWindow.set(windowId, optimisticMessages);
  els.composerInput.value = "";
  els.sendBtn.disabled = true;
  if (els.traceStatus) {
    els.traceStatus.textContent = "step1 · 正在发送消息...";
  }
  renderMessages();
  renderChatTrace();

  try {
    const res = await fetch(`${API_BASE}/api/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        window_id: windowId,
        user_input: text,
        role: activeRole,
        prompt_mode: getThemeForRole(activeRole).apiPrompt,
      }),
    });
    if (!res.ok) throw new Error("chat failed");
    const data = await res.json();
    const replyRole = normalizeRole(data.agent_role || activeRole);
    const roleLabel = getRoleLabel(data.agent_role || replyRole);
    const settledMessages = [
      ...existing,
      { role: "user", text, meta: `Turn ${data.turn_id}`, label: "用户" },
      {
        role: "assistant",
        agentRole: replyRole,
        label: roleLabel,
        text: data.reply || data.assistant_response || "",
        meta: `Turn ${data.turn_id}`,
      },
    ];
    state.messagesByWindow.set(windowId, settledMessages);
    state.latestChatTrace = data.trace || [];
    state.recallByWindow.set(windowId, data.recalled_memories || []);
    if (data.thread_summary) state.summaries.set(windowId, data.thread_summary);

    renderMessages();
    renderChatTrace();
    updateActiveWindowHeader();

    Promise.all([loadWindows(), loadMemorySummary()])
      .then(() => {
        renderDreamPanel();
      })
      .catch((refreshError) => {
        console.error("post-send refresh failed:", refreshError);
      });
  } catch (error) {
    state.messagesByWindow.set(windowId, existing);
    renderMessages();
    console.error(error);
    window.alert("发送失败，请检查 Go Runtime 是否已启动。");
  } finally {
    els.sendBtn.disabled = false;
  }
}

async function loadMemorySummary() {
  try {
    const data = await fetchJSON(`${API_BASE}/api/memory/summary`, { count: 0, entries: [] });
    els.memoryCount.textContent = `${data.count || 0} 项`;
    state.memoryFiles.set("memory_index_summary", data);
  } catch (error) {
    console.error(error);
    els.memoryCount.textContent = "0 项";
  }
}

async function loadMemoryFiles() {
  const allFiles = [...LONG_TERM_FILES, ...MID_TERM_FILES];
  await Promise.all(
    allFiles.map(async (file) => {
      const content = await fetchJSON(file.path, null);
      state.memoryFiles.set(file.key, { ...file, content });
    }),
  );
}

function renderMemoryColumns() {
  renderSummaryList();
  renderMemoryFileList(els.longTermList, LONG_TERM_FILES);
  renderMemoryFileList(els.midTermList, MID_TERM_FILES);
  if (!state.selectedMemoryKey) {
    const firstSummary = [...state.summaries.keys()][0];
    state.selectedMemoryKey = firstSummary ? `summary:${firstSummary}` : LONG_TERM_FILES[0].key;
  }
  renderMemoryDetail();
}

function renderSummaryList() {
  const summaryEntries = state.windows.map((windowItem) => {
    const summary = state.summaries.get(windowItem.id);
    const threadState = state.threadStates.get(windowItem.id);
    const imported = Boolean(threadState?.last_dream_at || threadState?.dream_imported || summary?.imported_at);
    const summaryText = summary?.summary || summary?.latest_summary || summary?.digest || "暂未生成摘要内容";
    return { windowItem, imported, summaryText };
  });

  if (!summaryEntries.length) {
    els.summaryList.innerHTML = `<div class="empty-mini">暂无摘要线程。先在聊天页创建会话并产生消息。</div>`;
    return;
  }

  els.summaryList.innerHTML = summaryEntries
    .map(
      ({ windowItem, imported, summaryText }) => `
        <button class="memory-item ${state.selectedMemoryKey === `summary:${windowItem.id}` ? "active" : ""}" data-memory-key="summary:${windowItem.id}">
          <div class="panel-header">
            <h4>${escapeHTML(windowItem.title)}</h4>
            <span class="summary-status ${imported ? "imported" : "pending"}">${imported ? "已导入" : "待导入"}</span>
          </div>
          <p>${escapeHTML(summaryText).slice(0, 120)}</p>
          <span class="memory-file-meta">${escapeHTML(windowItem.id)} · ${windowItem.turn_count || 0} 轮</span>
        </button>
      `,
    )
    .join("");

  [...els.summaryList.querySelectorAll(".memory-item")].forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedMemoryKey = button.dataset.memoryKey;
      renderMemoryColumns();
    });
  });
}

function renderMemoryFileList(container, files) {
  container.innerHTML = files
    .map((file) => {
      const payload = state.memoryFiles.get(file.key)?.content;
      const preview = JSON.stringify(payload, null, 2).slice(0, 120) || "暂无内容";
      return `
        <button class="memory-item ${state.selectedMemoryKey === file.key ? "active" : ""}" data-memory-key="${file.key}">
          <h4>${file.label}</h4>
          <p>${escapeHTML(preview)}</p>
          <span class="memory-file-meta">${file.path.replace("../", "")}</span>
        </button>
      `;
    })
    .join("");

  [...container.querySelectorAll(".memory-item")].forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedMemoryKey = button.dataset.memoryKey;
      renderMemoryColumns();
    });
  });
}

function renderMemoryDetail() {
  if (!state.selectedMemoryKey) return;

  if (state.selectedMemoryKey.startsWith("summary:")) {
    const windowId = state.selectedMemoryKey.replace("summary:", "");
    const win = state.windows.find((item) => item.id === windowId);
    const summary = state.summaries.get(windowId);
    const threadState = state.threadStates.get(windowId);
    els.memoryDetail.className = "memory-detail";
    els.memoryDetail.innerHTML = `
      <div class="memory-detail-card">
        <strong class="memory-badge">短期记忆摘要</strong>
        <h4>${escapeHTML(win?.title || windowId)}</h4>
        <p>${escapeHTML(windowId)} · ${threadState?.status || "状态未知"}</p>
      </div>
      <div class="memory-detail-card">
        <h4>摘要 JSON</h4>
        <pre>${escapeHTML(JSON.stringify(summary || { message: "暂无摘要内容" }, null, 2))}</pre>
      </div>
      <div class="memory-detail-card">
        <h4>线程状态</h4>
        <pre>${escapeHTML(JSON.stringify(threadState || { message: "暂无线程状态" }, null, 2))}</pre>
      </div>
    `;
    return;
  }

  const filePayload = state.memoryFiles.get(state.selectedMemoryKey);
  els.memoryDetail.className = "memory-detail";
  els.memoryDetail.innerHTML = `
    <div class="memory-detail-card">
      <strong class="memory-badge">记忆文件</strong>
      <h4>${escapeHTML(filePayload?.label || state.selectedMemoryKey)}</h4>
      <p>${escapeHTML(filePayload?.path?.replace("../", "") || "")}</p>
    </div>
    <div class="memory-detail-card">
      <h4>JSON 内容</h4>
      <pre>${escapeHTML(JSON.stringify(filePayload?.content || { message: "文件读取失败或暂无内容" }, null, 2))}</pre>
    </div>
  `;
}

function renderDreamPanel() {
  const list = state.windows
    .map((windowItem) => {
      const summary = state.summaries.get(windowItem.id);
      return {
        ...windowItem,
        summaryText: summary?.summary || summary?.latest_summary || summary?.digest || summary?.current_summary || "暂无摘要",
      };
    })
    .sort((a, b) => String(b.updated_at || "").localeCompare(String(a.updated_at || "")));

  if (!state.selectedDreamWindowId) {
    state.selectedDreamWindowId = state.activeWindowId || list[0]?.id || "";
  }

  if (!list.length) {
    els.dreamSummaryList.innerHTML = `<div class="empty-mini">暂无可导入的摘要线程。</div>`;
    els.dreamTitle.textContent = "选择一个摘要线程";
    syncDreamActionState();
    return;
  }

  els.dreamSummaryList.innerHTML = list
    .map((item) => {
      const isSelected = state.selectedDreamWindowId === item.id;
      const isRunning = state.dreamRun.inProgress && state.dreamRun.windowId === item.id;
      return `
        <button class="session-item ${isSelected ? "active" : ""} ${isRunning ? "is-running" : ""}" data-window-id="${item.id}" ${state.dreamRun.inProgress ? "disabled" : ""}>
          <span class="session-title">${escapeHTML(item.title)}</span>
          <span class="session-meta">${escapeHTML(item.summaryText).slice(0, 80)}</span>
          ${isRunning ? '<span class="session-running-badge">导入中...</span>' : ""}
        </button>
      `;
    })
    .join("");

  [...els.dreamSummaryList.querySelectorAll(".session-item")].forEach((button) => {
    button.addEventListener("click", () => {
      if (state.dreamRun.inProgress) return;
      state.selectedDreamWindowId = button.dataset.windowId;
      renderDreamPanel();
    });
  });

  const selected = list.find((item) => item.id === state.selectedDreamWindowId) || list[0];
  state.selectedDreamWindowId = selected.id;
  const runningTitle = state.dreamRun.inProgress && state.dreamRun.windowId === selected.id ? `${selected.title} · 子梦导入进行中` : selected.title;
  els.dreamTitle.textContent = runningTitle;
  const summary = state.summaries.get(selected.id) || { message: "暂无摘要" };
  const threadState = state.threadStates.get(selected.id) || {};

  renderJSONBox(els.dreamSummaryDetail, summary);
  renderDreamMemoryLinks(selected.id, threadState);
  renderDreamTrace();
  syncDreamActionState();
  if (state.dreamRun.inProgress) {
    renderDreamProgressResult();
  } else if (!state.latestDreamResult) {
    els.dreamImportResult.className = "json-box empty-mini";
    els.dreamImportResult.textContent = "可对当前线程触发单线程导入，或从顶部执行全局导入。";
  }
}

function renderDreamMemoryLinks(windowId, threadState) {
  const latestFiles = state.latestDreamResult?.updated_memory_files || [];
  const items = [
    `摘要文件：data/sessions/summaries/${windowId}.summary.json`,
    "长期记忆：profile / relationship / preferences / opinions / digest / index",
    "中期记忆：events / emotions",
    latestFiles.length ? `本次更新文件：${latestFiles.join(" / ")}` : "本次更新文件：等待执行后显示",
    `线程状态：${JSON.stringify(threadState)}`,
  ];
  els.dreamMemoryLinks.className = "detail-list";
  els.dreamMemoryLinks.innerHTML = items.map((item) => `<article class="recall-item"><p>${escapeHTML(item)}</p></article>`).join("");
}

function renderDreamTrace() {
  if (state.dreamRun.inProgress && !state.latestDreamTrace.length) {
    els.dreamTraceList.className = "trace-list empty-mini";
    els.dreamTraceList.textContent = "子梦系统正在处理，请稍候，完成后这里会展示导入轨迹。";
    return;
  }

  if (!state.latestDreamTrace.length) {
    els.dreamTraceList.className = "trace-list empty-mini";
    els.dreamTraceList.textContent = "运行子梦后，这里会显示导入轨迹与更新文件。";
    return;
  }

  els.dreamTraceList.className = "trace-list";
  els.dreamTraceList.innerHTML = state.latestDreamTrace
    .map(
      (step, index) => `
        <article class="trace-item">
          <strong>Dream ${index + 1} · ${escapeHTML(step.name || step.stage || "runtime")}</strong>
          <p>${escapeHTML(step.detail || step.message || JSON.stringify(step))}</p>
        </article>
      `,
    )
    .join("");
}

function syncDreamActionState() {
  const inProgress = state.dreamRun.inProgress;
  if (els.threadDreamBtn) {
    const disabled = inProgress || !state.selectedDreamWindowId;
    els.threadDreamBtn.disabled = disabled;
    els.threadDreamBtn.classList.toggle("is-loading", inProgress && state.dreamRun.mode === "thread");
    els.threadDreamBtn.innerHTML = inProgress && state.dreamRun.mode === "thread"
      ? '<i class="fa-solid fa-spinner fa-spin"></i><span>正在导入当前线程...</span>'
      : '<i class="fa-regular fa-star"></i><span>子梦导入当前线程</span>';
  }
  if (els.globalDreamBtn) {
    els.globalDreamBtn.disabled = inProgress;
    els.globalDreamBtn.classList.toggle("is-loading", inProgress && state.dreamRun.mode === "global");
    els.globalDreamBtn.innerHTML = inProgress && state.dreamRun.mode === "global"
      ? '<i class="fa-solid fa-spinner fa-spin"></i><span>正在执行全局导入...</span>'
      : '<i class="fa-solid fa-wand-magic-sparkles"></i><span>执行全局子梦</span>';
  }
}

function renderDreamProgressResult() {
  const elapsedSeconds = Math.max(0, Math.floor((Date.now() - (state.dreamRun.startedAt || Date.now())) / 1000));
  const scopeText = state.dreamRun.mode === "thread"
    ? `当前线程 ${state.dreamRun.windowId || state.selectedDreamWindowId}`
    : "全部线程";
  els.dreamImportResult.className = "json-box dream-progress-box";
  els.dreamImportResult.innerHTML = `
    <div class="dream-progress-state">
      <div class="dream-progress-spinner"><i class="fa-solid fa-spinner fa-spin"></i></div>
      <div>
        <strong>子梦系统正在导入中</strong>
        <p>正在处理：${escapeHTML(scopeText)}</p>
        <p>已运行 ${elapsedSeconds} 秒。请勿重复点击，完成后会自动刷新结果。</p>
      </div>
    </div>
  `;
}

async function runDream(mode) {
  if (state.dreamRun.inProgress) {
    renderDreamProgressResult();
    return;
  }

  if (mode === "thread" && !state.selectedDreamWindowId) {
    window.alert("请先选择一个摘要线程。");
    return;
  }

  state.dreamRun = {
    inProgress: true,
    mode,
    windowId: mode === "thread" ? state.selectedDreamWindowId : "",
    startedAt: Date.now(),
  };
  state.latestDreamTrace = [];
  syncDreamActionState();
  renderDreamPanel();

  try {
    const payload = mode === "thread" ? { window_id: state.selectedDreamWindowId, mode: "thread" } : { mode: "global" };
    const res = await fetch(`${API_BASE}/api/dream`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error("dream failed");
    const data = await res.json();
    state.latestDreamResult = data;
    state.latestDreamTrace = data.trace || [];
    renderJSONBox(els.dreamImportResult, data);
    renderDreamTrace();
    await Promise.all([loadWindows(), loadMemorySummary(), loadMemoryFiles()]);
    renderMemoryColumns();
    renderDreamPanel();
  } catch (error) {
    console.error(error);
    window.alert("子梦执行失败，请检查后端状态。");
  } finally {
    state.dreamRun = {
      inProgress: false,
      mode: "",
      windowId: "",
      startedAt: 0,
    };
    syncDreamActionState();
    renderDreamPanel();
  }
}

function renderJSONBox(element, payload) {
  element.className = "json-box";
  element.innerHTML = `<pre>${escapeHTML(JSON.stringify(payload, null, 2))}</pre>`;
}

function applyRoleTheme() {
  document.documentElement.setAttribute("data-role", "default");
  renderRoleSpotlight();
  if (state.activeWindowId) {
    updateActiveWindowHeader();
    renderMessages();
  }
}

function getAvatarFallback(role) {
  const normalized = normalizeRole(role);
  const roleDisplay = getRoleDisplay(normalized);
  if (roleDisplay?.name) return roleDisplay.name.slice(0, 1);
  return normalized === "humphrey" ? "汉" : "角";
}

function startCountdown() {
  const tick = () => {
    const now = new Date();
    const target = new Date(now);
    target.setHours(24, 0, 0, 0);
    const diff = target.getTime() - now.getTime();
    const hours = String(Math.floor(diff / 3600000)).padStart(2, "0");
    const minutes = String(Math.floor((diff % 3600000) / 60000)).padStart(2, "0");
    const seconds = String(Math.floor((diff % 60000) / 1000)).padStart(2, "0");
    const text = `${hours}:${minutes}:${seconds}`;
    els.countdownLabel.textContent = text;
    els.dreamCountdownInline.textContent = text;
  };
  tick();
  window.setInterval(tick, 1000);
}

async function fetchJSON(url, fallback) {
  try {
    const cacheBreaker = url.includes("?") ? `&__ts=${Date.now()}` : `?__ts=${Date.now()}`;
    const res = await fetch(`${url}${cacheBreaker}`, {
      cache: "no-store",
      headers: {
        "Cache-Control": "no-cache, no-store, must-revalidate",
        Pragma: "no-cache",
      },
    });
    if (!res.ok) throw new Error(url);
    return await res.json();
  } catch (error) {
    console.warn("fetch failed:", url, error);
    return fallback;
  }
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
