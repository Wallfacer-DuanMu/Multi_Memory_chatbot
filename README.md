一个支持多角色扮演、群聊互动与长期记忆管理的智能聊天系统，可基于会话摘要自动提炼并沉淀用户关键信息。

项目正在持续更新中

它不是一个普通的 chatbot，而是一个结合了“多角色群聊”“长期记忆”“线程摘要”“子梦系统”的实验性 AI 对话项目。

<img width="1627" height="912" alt="image" src="https://github.com/user-attachments/assets/3932139e-f7c5-448c-b6a8-f4ee65e7427e" />

## 核心功能

### 1. Agent Runtime 运行机制

这个项目不是把 prompt 直接拼接后丢给模型，而是实现了一个可重复运行的 Runtime 流程。

一次输入进入系统后，当前会经过以下阶段：

1. **Recent Context Loading**
   - 读取最近若干轮 raw 对话
   - 读取记忆索引与长期 / 中期记忆召回结果

2. **Context Assembly**
   - 将窗口摘要、最近原文、召回记忆拼装成上下文
   - 角色系统 Prompt 与群聊规则在这一层注入

3. **Reply Generation**
   - 调用大模型生成角色回复
   - 如果模型失败，则进入 fallback 路径，保证系统不中断

4. **Session Persistence**
   - 把本轮 `raw turn` 持久化到 `data/sessions/raw/`
   - 同步更新窗口级摘要 `data/sessions/summaries/`

5. **Autodream / Memory Consolidation**
   - 后续通过 `/dream` 或 HTTP API 触发“子梦”流程
   - 由摘要驱动结构化记忆判断，而不是只依赖单轮原文
   - 把有价值的信息写入 `profile / preferences / events / emotions / relationship / digest / memory_index`

这个系统当前已经具备一个最小可用的 Agent Runtime 雏形：

- 输入处理
- 状态管理
- 记忆召回
- 回复生成
- 会话持久化
- 摘要压缩
- 记忆沉淀
- 执行轨迹记录
- 失败回退

### 2. 多角色沟通系统

用户可以自定义加入自己喜欢的角色，例如来自：

- 影视剧
- 游戏
- 小说

在聊天时，用户可以：

- 选择某个角色作为当前对话角色
- 在发言时直接点名某个角色

系统内部为角色设置了统一的群聊式 Prompt 规则，使角色能够理解：

- 自己并不是在进行一对一对话
- 当前处在一个多人对话环境中
- 历史消息中不同的话分别是谁说的
- 可以对其他角色的观点做出回应、补充或点评

目前项目默认保留两个内置角色：

- 英剧人物汉弗莱（`humphrey`）
- 《只狼》NPC 永真（`yongzhen`）

### 3. 线程摘要与记忆沉淀

当用户选中某个线程并执行导入时，系统会把该线程的摘要信息交给大模型，通过内置 Prompt 判断：

- 哪些信息有价值
- 哪些内容适合存入长期记忆系统
- 应该写入哪类记忆文件

随后系统将自动执行记忆沉淀。

---

## Runtime 具体实现架构概览

当前 Runtime 可以理解为由几个核心层组成：

### 1. Runtime Orchestrator

- 位置：`internal/runtime/`
- 核心对象：`AgentRuntime`
- 责任：串联一次对话完整生命周期，包括输入处理、上下文组装、调用 LLM、写入 raw turn、更新窗口摘要、触发 trace

### 2. Session Layer

- 位置：`internal/session/`
- 核心对象：`ConversationWindow`、`ThreadState`、`RawTurn`
- 责任：维护窗口、线程状态、原始会话、摘要文件路径与 turn 计数

### 3. Memory Layer

- 位置：`internal/memory/`
- 核心对象：`MemoryIndex`、`MemoryIndexEntry`、`MemoryPolicy`
- 责任：维护长期 / 中期记忆、记忆索引、召回排序、记忆状态（`active / merged / archived / rejected`）

### 4. Tool-like Processing Layer

- 位置：`internal/runtime/` + `internal/memory/`
- 当前已实际体现出的两类核心工具能力：
  - **记忆读写工具**：读取索引、召回上下文、写回 `profile/preferences/events` 等文件
  - **摘要 / 提取工具**：把 raw turn 压缩成 window summary，并进一步交给 autodream 做结构化判断

虽然目前 `internal/tools/` 只保留了通用接口，但运行时内部已经把“工具能力”拆成独立阶段执行，而不是交给单一 prompt 一次性完成。

---

## 当前实现细节

### 1. 会话与线程状态管理

系统目前维护以下状态文件：

- **会话窗口状态**：`windows.json`
- **线程运行状态**：`thread_states.json`
- **运行时配置状态**：`runtime_state.json`

其中线程状态会记录：

- 当前线程是否为 `pending / processing / completed / failed`
- 最近处理到哪个 raw turn
- 最近一次摘要更新时间
- 最近一次 autodream 结果与错误

### 2. 结构化记忆读写

系统当前把记忆拆成多类 JSON 文件：

- `profile.json`
- `relationship.json`
- `preferences.json`
- `opinions.json`
- `events.json`
- `emotions.json`
- `recent_discussion_digest.json`
- `memory_index.json`

这意味着用户信息并不只是“存在 prompt 里”，而是会被整理成明确的数据结构。

### 3. 记忆更新与冲突处理

当前版本虽然还不是最终形态，但已经有基础冲突 / 合并机制：

- `memory_index` 会对相近条目做 fuzzy merge
- 新写入的记忆会带有 `status / merged_into / confidence / importance`
- 低置信度内容可以被标成 `rejected` 或 `archived`
- `preferences / opinions / events` 通过去重与合并逻辑避免无脑追加

也就是说，系统已经开始体现“更新旧记忆”而不是“永远堆新文本”的方向。

---

## 项目目录结构

```text
Multi_Memory_chatbot/
├─ data/
│  ├─ memory/
│  │  ├─ long_term/          # 长期记忆（画像、关系、偏好、观点）
│  │  └─ mid_term/           # 中期记忆（事件、情绪等）
│  ├─ sessions/
│  │  ├─ roles.json          # 角色列表
│  │  ├─ thread_states.json  # 线程状态
│  │  └─ windows.json        # 会话窗口列表
│  └─ system/
│     ├─ llm_config.json     # 模型配置
│     ├─ memory_policy.json  # 记忆策略
│     └─ runtime_state.json  # 运行时状态
├─ frontend/                 # 前端页面与脚本
├─ internal/
│  ├─ api/                   # HTTP API
│  ├─ config/                # 配置加载
│  ├─ llm/                   # 大模型调用封装
│  ├─ memory/                # 记忆处理逻辑
│  ├─ runtime/               # 核心运行时与角色系统
│  ├─ session/               # 会话、窗口、raw/summary 管理
│  └─ tools/                 # 工具层接口
├─ index.html
├─ go.mod
└─ README.md
```

---

## 技术栈

当前项目以 Go 为主，配合原生前端页面构成一个轻量原型：

- Go 1.22
- 原生 HTML / CSS / JavaScript
- 本地 JSON 文件作为会话与记忆存储

---

## CLI 基础命令

CLI 中目前支持的基础命令包括：

- `/new [标题]`：新建窗口
- `/window window-001`：切换到已有窗口
- `/list`：查看窗口列表
- `/dream`：手动触发子梦系统

---

## HTTP API 最小调用说明

如果你用 HTTP 模式来展示这个作业，建议至少演示下面这几个接口。

### 1. 创建窗口

```bash
curl -X POST http://127.0.0.1:8080/api/windows \
  -H "Content-Type: application/json" \
  -d '{"title":"面试演示窗口"}'
```

### 2. 发送对话

```bash
curl -X POST http://127.0.0.1:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"window_id":"window-001","user_input":"我叫小青，在深圳做产品，这两周准备面试，有点焦虑。"}'
```

返回结果中通常会包含本轮回复、窗口信息以及相关状态更新。

---

## 配置说明

模型配置文件位于：

- `data/system/llm_config.json`

你需要根据自己的模型服务填写：

- `provider`
- `base_url`
- `api_key`
- `model`

请注意：

- 不要把真实 `api_key` 上传到 GitHub
- 建议后续改造为 `.env` 或本地私有配置方式

---

## 数据与隐私说明

本项目包含会话、摘要、记忆等数据结构。

在公开仓库前，建议不要提交以下内容中的真实数据：

- 原始会话记录
- 自动摘要结果
- 用户画像与长期记忆
- API Key
- 本地测试数据

当前仓库更适合作为：

- 原型展示
- 架构说明
- 功能思路演示

而不是直接承载真实用户数据。

---

## 对照初期目标要求的完成情况

目前已经完成的部分包括：

- 中长期记忆文件结构
- 子梦系统的基础流程
- 简单前端与 HTTP 服务
- 基本可解释的 Agent Runtime 执行链路

仍待继续完善的方向包括：

- 更稳定的记忆提取与写入策略
- 更清晰的前后端交互体验
- 更完善的角色编辑与管理能力
- 更完整的运行文档与部署说明
- 更严格的数据隔离与隐私清理机制
- 更健壮的错误处理与日志系统

---

## 后续计划

接下来准备逐步打磨：

- 完善 README 与项目文档
- 优化目录结构与配置方式
- 增强多角色互动体验
- 改进子梦系统的记忆筛选质量
- 增加上传 GitHub 所需的 `.gitignore` 与公开发布规范
- 持续清理测试数据与无关文件

---

## 适合谁关注这个项目

如果你对以下方向感兴趣，这个项目可能值得参考：

- 多角色 AI 对话系统
- 带长期记忆的聊天产品
- 本地文件式记忆存储原型
- 基于摘要的记忆沉淀机制
- 群聊式 AI 角色交互

---

## 声明

这是一个仍在持续打磨中的个人项目原型。

当前公开的重点在于展示设计思路、角色机制与记忆系统结构，后续会继续逐步上传更完善的版本。
