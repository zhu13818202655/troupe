# Story：本地 Docker Compose 部署 Troupe（Apple Silicon Mac）+ 自建 OpenAI 兼容模型

> ## ⚠️ 已被取代 — 仅作历史分析保留
>
> 本文是**第一版**部署故事，写法早于 STORY-002 定稿，其中若干前提已不成立。请以下列文档为准：
>
> - **操作手册**：[`README.md`](./README.md)
> - **最终方案与决策**：[`../.github/story/002-mac-local-compose-deployment.md`](../.github/story/002-mac-local-compose-deployment.md)
>
> 具体被推翻的结论：
>
> | 本文旧说法 | 实际（已实现） |
> |---|---|
> | 后端用 **SQLite** 平台模式 | 后端用 **PostgreSQL + pgvector**（D1 决策） |
> | 项目名 **Astonish** | 全仓已改名 **Troupe**，配置目录 `~/.config/troupe` |
> | 根 `docker-compose.yml` 可用 | 该文件当时**完全不可用**（引用了不存在的 `docker/astonish/Dockerfile`、非法的 `studio` 子命令、rebrand 前的挂载路径）；**现已直接删除**，只保留 `deploy/compose.yaml` 一份 |
> | 配置目录 `~/.config/astonish` | `~/.config/troupe`，并保留对旧目录的**状态感知回退** |
>
> 下文第 1 节对**代码缺陷的定位**（max_completion_tokens 硬编码、DB 空值覆盖 config、
> `orgDBName` 命名分叉、PgBouncer/pgvector 陷阱等）经复核仍然成立，可作为背景参考。

---

> 目标：在你的 Mac 上用 Docker Compose 一键起服务（Nginx 反向代理 + 平台模式），并接入自建的 OpenAI 兼容推理服务（vLLM / SGLang / LMDeploy / Ollama / llama.cpp）。
>
> 本文是**实施故事**：第 1 节是仓库现状体检结论，第 2 节是需要改的代码（含具体 diff），第 3 节是已落地的部署文件，第 4–6 节是上线、接模型、验收步骤。所有结论都带 `文件:行号` 证据。

---

## 0. 结论摘要

**现状：仓库自带的部署资产当前是不可用的**，必须改动后才能部署成功。四个硬性断点：

| # | 断点 | 证据 |
|---|---|---|
| B1 | `docker-compose.yml:5` 引用的 `docker/astonish/Dockerfile` 不存在（实际是 `docker/troupe/Dockerfile`） | `ls docker/` → `sandbox-base`、`sandbox-openshell`、`troupe` |
| B2 | `docker-compose.yml:7` 的 `command: ["studio"]` 不是合法子命令，`command: ["daemon", "run"]` 才是 | `cmd/troupe/root.go:52-119` 无 `studio`；`cmd/troupe/daemon.go:36` 是 `run` |
| B3 | `Dockerfile:102` 要 `astonish-linux-${TARGETARCH}`，Makefile 产出的是 `troupe-linux-*` | `docker/troupe/Dockerfile:102` vs `Makefile:553,559` |
| B4 | `docker-compose.yml:16` 的 `TROUPE_HOST=0.0.0.0` 是无效变量，代码中从未读取 | 监听地址硬编码 `fmt.Sprintf(":%d", port)`，`pkg/launcher/studio.go:310` |

**另外两个"能起来但用不了"的功能性阻断**，这正是本 Story 的核心价值：

| # | 问题 | 影响 | 解法 |
|---|---|---|---|
| **R1** | `request_helpers.go:375-376`（以及 `317-318`）无条件用 DB 值覆盖 `default_provider` / `default_model`，即使 DB 为空 | platform（SQLite）模式下，**config.yaml 里配的默认模型永远不生效**，聊天报 `no provider configured` | 改动 **D1** |
| **R2** | `openai_compat` 硬编码 `max_completion_tokens = 64000` | 任何 `max_model_len < 64000` 的自建模型（绝大多数 7B–70B 都是 8k/32k/128k）**每次请求都返回 400** | 改动 **D2** |

> 若暂时不想改代码：R1 可通过「首次启动后在 Settings → Providers 里配一次」绕过；R2 **无法通过配置绕过**，只能换用 `max_model_len ≥ 64000` 的模型。

**产出文件（已在 `deploy/` 下）**：

```
deploy/
├── Dockerfile              # 3 阶段自包含构建（Node 编 UI → Go 编二进制 → Debian 运行时）
├── compose.yaml            # 主编排：troupe + nginx（+ 可选 postgres profile）
├── env.example             # 环境变量模板（含密钥生成说明）
├── config.example.yaml     # config.yaml 模板（含 openai_compat 实例 + 详尽坑位注释）
├── nginx/nginx.conf        # SSE 正确的反向代理配置
├── .gitignore              # 忽略 .env / state/
└── DEPLOY_STORY.md         # 本文档
```

---

## 1. 现状体检

### 1.1 运行时架构（已核实）

- **单一二进制 + 内嵌 SPA**：`//go:embed all:dist`（`web/embed.go:9`），Studio 服务在 `pkg/launcher/studio.go`。
- **`daemon run` 就是服务入口**：`daemon.Run`（`pkg/daemon/run.go:74-1789`）一路走到 `launcher.NewStudioServer(port)`（`run.go:1596`），端口优先级 flag > `config.yaml daemon.port` > 9393（`run.go:109-116`）。
- **已无 personal/file 模式**：`storage.backend` 为空或 `file` 时会被强制改写为 `sqlite`（`run.go:136-141`），非 `postgres`/`sqlite` 直接启动失败（`run.go:208-211`）。**本地部署即 platform 模式**，第一个注册的用户自动成为 org owner + platform superadmin（`pkg/api/auth_platform.go:199-209`、`:711-722`）。
- **认证是 JWT platform auth**，旧文档里的 device auth / `daemon.auth.disabled` 已是死代码，不可再用。
- **默认 SQLite 的表现**：`file:<dataDir>/platform.db`，每个 org 一个 `.db`（`run.go:205-207`，`entstore/tenant_router.go`）。

### 1.2 与容器相关的运行时事实

| 事实 | 位置 | 部署含义 |
|---|---|---|
| 配置目录 = `XDG_CONFIG_HOME/astonish` 或 `~/.config/astonish` | `app_config.go:1672-1691` | 容器内 `/root/.config/astonish`，**必须持久化**（内含 `.store_key`、`credentials.enc`、`models/`、`logs/`） |
| SQLite 数据目录 = `config.dao_dir` → `TROUPE_DATA_DIR` → `~/.local/share/astonish` | `app_config.go:632-646` | **目录必须预先存在**，`openSQLite` 不做 MkdirAll，缺目录 → `PingContext` 失败 → fatal |
| embedding 初始化无条件且失败即 fatal | `run.go:246-263` | 默认会下载 ~23MB `all-MiniLM-L6-v2`；首次启动需外网，之后缓存 |
| `TROUPE_MASTER_KEY` 必须是 64 位 hex | `pkg/credentials/crypto.go:118-124` | 格式错只会 warn 并降级为明文存储，不会崩 |
| `TROUPE_JWT_SECRET` 缺失时每次启动随机生成 | `run.go:317-320` | 不设的话**每次 restart 都掉登录态** |
| `default` 模式日志写文件 | `pkg/daemon/logger.go:89-103` | 必须设 `TROUPE_LOG_DEST=stdout`，否则 `docker compose logs` 是空的 |
| sandbox 未显式配置时默认 **enabled** | `pkg/sandbox/config.go:31-36` | 本镜像以 root 运行不会触发 sudo 重 exec，但显式 `enabled: false` 更可控 |
| 端口固定绑 `:port`（全网卡） | `studio.go:310-314` | 容器内天然可被同网段访问；对外出口交给 Compose/Nginx 控制 |

---

## 2. 代码改动清单

优先级：**P0 = 不改动就跑不通 / 跑不对**，P1 = 强烈建议，P2 = 清理优化。

### D1（P0）让 config.yaml 的默认 provider/model 在 platform 模式生效

**问题**：两处受害者，`pkg/api/request_helpers.go:375-379`（HTTP 请求路径）与 `:317-321`（上下文路径，后台任务用）：

```go
appCfg.General.DefaultProvider = resolved.General.DefaultProvider   // ← DB 为空时也被赋值为 ""
appCfg.General.DefaultModel    = resolved.General.DefaultModel
```

`resolved` 来自纯 DB 解析（`provider.ResolveEffectiveConfig`，`resolve.go:30` 从空白 AppConfig 开始，从不读 config.yaml）。于是空 DB → `DefaultProvider = ""` → `Pool.Get` 直接返回 `no provider configured`（`pkg/provider/pool.go:45-47`）。

对比 daemon 自己的 cascade 是**有条件的**（`pkg/daemon/run.go:2243-2249`：DB 非空才用 DB，否则保留 config.yaml），以及 `applyTeamNonProviderSettings`（`request_helpers.go:413-423`）也是条件覆盖 —— **只有这两行是不一致的**。

**改法**（两处同样处理，保持与 daemon cascade 一致）：

```go
	mergeResolvedProviders(appCfg, resolved)

	// DB values only win when explicitly set. Mirrors the daemon-level cascade
	// in cascadePlatformProviders (pkg/daemon/run.go:2243-2249): without this,
	// an empty platform DB clears the default_provider/default_model baked into
	// config.yaml, leaving the chat path with "no provider configured".
	if resolved.General.DefaultProvider != "" {
		appCfg.General.DefaultProvider = resolved.General.DefaultProvider
	}
	if resolved.General.DefaultModel != "" {
		appCfg.General.DefaultModel = resolved.General.DefaultModel
	}
	if resolved.General.WebSearchTool != "" {
		appCfg.General.WebSearchTool = resolved.General.WebSearchTool
	}
	if resolved.General.WebExtractTool != "" {
		appCfg.General.WebExtractTool = resolved.General.WebExtractTool
	}
	if resolved.PerplexityWebSearch.Provider != "" {
		appCfg.PerplexityWebSearch = resolved.PerplexityWebSearch
	}
```

- 文件：`pkg/api/request_helpers.go`，第 **317-321** 行与第 **375-379** 行。
- 风险：低。DB 已配置时行为完全不变；仅在 DB 为空时改为回退到 config.yaml。
- 验收：清空 DB 后，仅靠 config.yaml 的 `general.default_provider` 能直接开始对话。

### D2（P0）openai_compat 的 `max_completion_tokens` 可配置

**问题**：

```go
// pkg/provider/openai_compat/openai_compat.go:48
Provider: openai_provider.NewProviderWithMaxTokens(client, modelName, true, 64000)
```

```go
// pkg/provider/openai/openai.go:103-105
if p.maxCompletionTokens > 0 {
    openAIReq.MaxCompletionTokens = p.maxCompletionTokens
}
```

无条件下发 `max_completion_tokens: 64000`。vLLM / LMDeploy / llama.cpp 遇到超过自家 `max_model_len` 的值会返回 **400 Bad Request**（`This model's maximum context length is N tokens`）。这是自建模型接入的头号失败原因，且 `pkg/provider/llmerror/max_output.go` 只针对 SAP/Vertex 做降配解析，不会自动恢复。

**改法**（3 个文件，唯一调用点是 `factory.go:373`）：

1. `pkg/provider/openai_compat/openai_compat.go` — 增加入参：

```go
// NewProvider creates a new OpenAI Compatible provider.
// maxTokens caps max_completion_tokens per request; <= 0 disables sending the
// field entirely, which is what self-hosted servers with a small max_model_len
// require (64000 is rejected with HTTP 400 by most local stacks).
func NewProvider(apiKey, baseURL, modelName string, maxTokens int, debug bool) model.LLM {
	config := openai.DefaultConfig(apiKey)
	...
	return &Provider{
		Provider: openai_provider.NewProviderWithMaxTokens(client, modelName, true, maxTokens),
	}
}
```

2. `pkg/provider/factory.go:361-373` — 从实例配置读取，并提供合理默认：

```go
	case "openai_compat":
		apiKey := instance["api_key"]
		if apiKey == "" {
			return nil, fmt.Errorf("API key not set for OpenAI Compatible provider")
		}
		baseURL := instance["base_url"]
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if modelName == "" {
			modelName = "gpt-4o"
		}
		// Self-hosted stacks (vLLM, LMDeploy, llama.cpp) reject the historic
		// 64000 default when their max_model_len is smaller. Allow an explicit
		// cap; fall back to the configured context length when available.
		maxTokens := 64000
		if v := strings.TrimSpace(instance["max_output_tokens"]); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				maxTokens = n
			}
		} else if configured := cfg.General.ContextLength; configured > 0 {
			maxTokens = configured
		}
		return openai_compat.NewProvider(apiKey, baseURL, modelName, maxTokens, debugMode), nil
```

> `GetProvider` 已有 `cfg *config.AppConfig` 入参，`cfg.General.ContextLength` 可直接取用（`factory.go:118` 附近）。需确认 `strconv` 已在 import 列表。

3. `pkg/config/app_config.go` 注释补充新键；`GetRequiredFields()`（`openai_compat.go:91-94`）保持 `api_key`/`base_url` 不变（新键可选）。

- 风险：低。`max_output_tokens` 未配置时仍走 64000（`type: openai` 之外的既有用户不受影响）。唯一行为差：若 `general.context_length` 有值且未显式配 `max_output_tokens`，则按 context_length 下发——这比硬编码 64000 更贴近语义。
- 验收：对 `max_model_len=32768` 的 vLLM 发起对话不再返回 400。

### D3（P0）修复构建链

三选一，推荐 A：

**A. 采用 `deploy/Dockerfile`（三阶段自包含）** —— 本文已提供。彻底消除 `astonish-linux-*` 与 `troupe-linux-*` 的命名耦合，宿主机只需 Docker。

**B. 最小补丁（保留现有 `docker/troupe/Dockerfile` 的"主机交叉编译"模式）**：

```diff
 # docker/troupe/Dockerfile:102
-COPY astonish-linux-${TARGETARCH} /usr/local/bin/astonish
+COPY troupe-linux-${TARGETARCH} /usr/local/bin/troupe
 # docker/troupe/Dockerfile:107-108
-ENTRYPOINT ["astonish"]
-CMD ["daemon", "run"]
+ENTRYPOINT ["troupe"]
+CMD ["daemon", "run"]
```

```diff
 # docker-compose.yml
-      dockerfile: docker/astonish/Dockerfile
+      dockerfile: docker/troupe/Dockerfile
-    command: ["studio"]
+    command: ["daemon", "run"]
-      - TROUPE_HOST=0.0.0.0
```

并同步 `.gitignore:23`（`astonish-linux-*` → 加 `troupe-linux-*`），否则 `git status` 会被 100MB 产物污染。

**C. 两者并存**：日常开发用 B（快），交付/给别人用 A（可复现）。

> 注意 `ENTRYPOINT` 名字本身不影响行为：`cmd/troupe/root.go:35` 只读 `os.Args[1]` 做子命令分派，全仓库唯一读 `os.Args[0]` 的是 sudo 提权分支（`pkg/sandbox/escalate.go:43`）。

### D4（P1）`base_url` 规范化：避免 `/v1/v1`

`openai_compat.go:27-37`（`ListModels` 的 `:66-75` 是同一份逻辑的复制）只判断了 `/v1` 后缀，没处理尾斜杠：

| 输入 | 结果 |
|---|---|
| `http://host:8000` | ✅ `…/v1/chat/completions` |
| `http://host:8000/v1` | ✅ 正常 |
| **`http://host:8000/v1/`** | ❌ `…/v1/v1/chat/completions` → 404 |
| **`http://host:8000/v1/chat/completions`** | ❌ `…/v1/chat/completions/v1/chat/completions` → 404 |

建议抽出共享 helper 并规范化：

```go
func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.TrimRight(raw, "/")
	for _, suffix := range []string{"/v1/chat/completions", "/v1"} {
		if strings.HasSuffix(strings.ToLower(raw), suffix) {
			raw = raw[:len(raw)-len(suffix)]
			break
		}
	}
	return raw + "/v1"
}
```

同时**修掉 `factory.go:366-369` 的静默回落**：`base_url` 为空时打到 `api.openai.com/v1`，用一个假 key 得到 401，报错信息完全指不到根因。建议改为直接返回明确错误（与 UI 的 "Test Connection" 行为对齐，`factory.go:744-747`）。

### D5（P1）config.yaml 支持环境变量插值（容器友好）

`pkg/config/app_config.go:1774-1803` 是裸 `yaml.Unmarshal`，**不支持 `${VAR}`**（注意 `docs/website/docs/configuration/config-reference.md:260-268` 声称支持 — 文档与实现不符）。

容器化场景下这很痛：想把 `base_url` 或密钥从 `.env` 注入就必须手改 YAML。建议**显式 opt-in**，避免破坏现有含 `$` 的值：

```go
data, err := os.ReadFile(path)
...
if os.Getenv("TROUPE_CONFIG_ENV_EXPAND") == "true" {
    data = []byte(os.ExpandEnv(string(data)))
}
```

> 过渡方案（不改代码）：用 `deploy/config.example.yaml` 做模板 + 一段 sed/envsubst 写入 `state/config/config.yaml`。本 Story 的目录结构已按此设计。

### D6（P2）允许 `openai_compat` 的空 api_key

`factory.go:362-365` 要求 `api_key` 非空，且 `openai_compat` 在 `ProviderEnvMapping`（`provider_env.go:10-57`）里没有条目，因此**没有环境变量回退**。本地推理服务通常不鉴权，只能填 `EMPTY` 占位符（可行但反直觉）。若放宽为空，注意不要同时引入 env 回退，以免意外读取 `OPENAI_API_KEY`。

### D7（P2）清理过期注释与文档

避免后来者被误导：

- `app_config.go:599-622`：仍在描述 "file 是默认 / personal mode / device auth"，与 `run.go:136-141` 的实现相反。
- `run.go:71-73` 注释提到写 PID 文件，实际全仓库非测试代码无 `WritePID` 调用。
- `docs/website/docs/configuration/config-reference.md:260-268`：`${VAR}` 插值不成立。
- `docker-compose.yml`（根目录）已失效；**现已删除**，只保留 `deploy/compose.yaml`（所有 compose 命令在 `deploy/` 下执行）。

### D8（P2）`.dockerignore` 增加 `web/dist`

现有注释写着 "web/dist IS required"（针对旧的单阶段 COPY 二进制方案）。多阶段构建里 dist 由 Stage 1 产出并 `COPY --from` 注入，宿主机的 `web/dist` 不但无用，还可能把空目录混进 `COPY . .`。加一行 `web/dist` 可减少约 12MB 构建上下文。

配合本文 Dockerfile 已用「先 `COPY . .`、再从 Stage 1 `COPY dist`」的顺序规避该冲突。

### D9（P2）为 `openai_compat` 补 context window 探测

`context_window.go:86-133` 的 `resolveFromProviderAPI` switch 没有 `openai_compat` 分支 → 恒返回 0 → 走静态表 → 未命中 → **回退 200,000**。

后果不是报错，而是危险：平台认为上下文极长，压缩（默认阈值 0.8，`app_config.go:1188-1194`）永远不触发，直到服务端溢出。可为 `openai_compat` 增加一条探测（用 `GET /v1/models` 的元数据，vLLM 会返回 `max_model_len`）。在此之前，**必须在 config.yaml 显式设 `general.context_length`**（已在模板中标注）。

---

## 3. 部署文件说明

### 3.1 `deploy/Dockerfile` — 三阶段自包含构建

```
Stage 1  node:22-bookworm-slim   npm ci → npm run build      → web/dist/<version>/
Stage 2  golang:1.27-bookworm    go mod download → go build   → /out/troupe
Stage 3  node / uv               拉取预编译运行时
Stage 4  debian:bookworm-slim    apt(chromium 运行时 + curl) + node + uv + pptx worker + 二进制
```

关键设计决策：

- **必须 glibc**：`pkg/codeintel` 用 `purego.Dlopen`，即使 `CGO_ENABLED=0` 也会产出动态链接 ELF（依赖 `libdl.so.2`）。Alpine/musl/distroless 会让二进制跑不起来。这一约束继承自 `docker/troupe/Dockerfile:8-9`。
- **`CGO_ENABLED=0`即可**：不需要 gcc。同理 treesitter `.so` 是可选的（缺失时会退化为 `grep_search`/`find_files`，`bindings.go:130-145`），本镜像不预编译。
- **`npm ci` 必须含 devDependencies**：`npm run build` 先跑 `tsc --noEmit`（`web/package.json:9`），typescript 在 devDeps。
- **empty-dist 断言**：`//go:embed all:dist` 遇到空目录会直接编译失败（"contains no embeddable files"），因此加了一步显式检查，让失败点清晰。
- **`TROUPE_LOG_DEST=stdout`**：保留 `TROUPE_MODE=default`（否则丢掉 scheduler/channels/fleet），但把日志导向 stdout。
- **`HEALTHCHECK`**：`/api/healthz` 已存在且免鉴权（`pkg/api/handlers.go:1018`，白名单 `auth_platform_middleware.go:102`）。注意它在 embedding 初始化与 Studio pre-warm **之后**才可用，所以 `start-period` 给了 180s。

### 3.2 `deploy/compose.yaml`

- 服务 `troupe` 只 `expose: 9393`（不发布到宿主），Nginx 是唯一入口；需要绕过时在 `ports` 里放开即可。
- 两个 bind mount：`./state/config:/root/.config/astonish`、`./state/data:/data`。均为**必须持久化**的前缀树；改用 named volume 也可以，但 bind mount 便于在 Mac 上直接查看 SQLite 文件。
- `depends_on: condition: service_healthy` 确保 Nginx 不在应用冷启动期接流量。
- PostgreSQL 作为 `--profile postgres` 可选服务保留（SQLite → Postgres 官方定位为改一行配置 + 迁库）。

### 3.3 `deploy/env.example`

只放**代码真正会读**的变量。故意不含 `TROUPE_HOST` / `TROUPE_PORT`（不存在）。两个密钥 `TROUPE_MASTER_KEY` / `TROUPE_JWT_SECRET` 是必填项，缺了不会崩但会静默降级（前者降级为明文存储、后者导致每次重启掉登录态）。

### 3.4 `deploy/config.example.yaml`

详见文件内注释。要点：

- `storage.backend: sqlite` + `data_dir: /data` —— 目录必须已存在。
- `storage.auth.require_email_verification: false` —— 否则**第二个用户**起需要 SMTP。
- `sandbox.enabled: false` —— 首次部署可控；启用需挂载 `/var/run/docker.sock`。
- `memory.embedding` 指向自建推理服务的 `/v1/embeddings` —— 免去首次联网下载 23MB 模型。**注意这需要你的服务真的提供 embedding 模型**，否则启动 fatal；那种情况就删掉该 block 走默认本地 embedder。
- `general.context_length` 必须显式写（见 D9）。

### 3.5 `deploy/nginx/nginx.conf`

为 SSE 而生。三个不能改错的点：

1. `proxy_buffering off` + `proxy_read_timeout 3600s`：默认 60s 读超时会**静默截断**长对话流。app 侧已在 6 个 handler 上设置了 `X-Accel-Buffering: no`（如 `pkg/api/chat_handlers.go:694`、`pkg/api/run_handler.go:744`），Nginx 会逐响应地尊重它，这里的配置是双保险。
2. **用 `/api/` 前缀而不是按 SSE 路径写正则**。实际路由并不规整：主聊天流是 `/api/chat`（**没有尾斜杠**），fleet 流在 `/api/fleet-plans/...`（`pkg/api/handlers.go:1262`、`:1359`）。形如 `^/api/(chat|fleet)/` 的正则会把这两者都漏掉。统一在 `/api/` 上关缓冲、关 gzip，静态资源（SPA）再单独开 gzip。
3. **不要加 `listen [::]:80`**。Docker 网络多数未启用 IPv6，加了会导致 Nginx 启动时 `socket() [::]:80 failed (97: Address family not supported)` 而直接退出。

此外，`X-Forwarded-Proto` **必须与实际一致**。app 据此决定 cookie 的 `Secure` 标志（`pkg/api/auth_platform.go:1019-1024`）——在纯 HTTP 环境下写 `https` 会导致浏览器丢弃 cookie、登录态循环失效。配置文件里已预留 HTTPS server 块（注释态）。

---

## 4. 本地部署步骤（Apple Silicon Mac）

```bash
cd /Users/louis/codes/troupe/deploy

# 1) 密钥
cp env.example .env
openssl rand -hex 32 | tr -d '\n' | pbcopy   # 粘贴到 .env 的 TROUPE_MASTER_KEY=
openssl rand -hex 32 | tr -d '\n' | pbcopy   # 粘贴到 .env 的 TROUPE_JWT_SECRET=

# 2) 应用配置
mkdir -p state/config state/data
cp config.example.yaml state/config/config.yaml
#    编辑 state/config/config.yaml：填 base_url / default_model / context_length
#    （建议先用真实的占位 model id，稍后在 UI 里用「拉取模型列表」确认）

# 3) 构建并启动（首次约 5-10 分钟：npm ci + go build）
docker compose up -d --build

# 4) 观察冷启动（第一次会下载 embedding 模型，或直接使用你配的远程 embedding）
docker compose logs -f troupe

# 5) 打开
open http://localhost:8080
#    第一个注册的用户自动成为 superadmin，且免邮箱验证
```

**首次访问时，在 Settings → Providers 再做一件事**（除非已应用 D1）：显式选择 Default Provider / Default Model 并保存——因为未改动的代码会用空的 DB 值覆盖 config.yaml 的默认值。

---

## 5. 接入自建 OpenAI 兼容模型

### 5.1 `base_url` 怎么写（三种场景）

| 模型服务位置 | 容器内应填写的 base_url |
|---|---|
| **Mac 宿主**（Ollama / LM Studio / llama.cpp） | `http://host.docker.internal:11434/v1` |
| **同一 compose 网络的其它容器** | `http://<service-name>:<port>/v1` |
| **局域网另一台机器** | `http://<lan-ip>:<port>/v1` |

> Docker Desktop 已内置 `host.docker.internal` DNS 解析（无需 `extra_hosts`）。Linux 宿主需手动加 `--add-host=host.docker.internal:host-gateway`。

### 5.2 填写规则（务必照做）

1. **不要带尾斜杠**。`http://host:8000/v1/` 会被拼成 `/v1/v1/chat/completions` → 404（D4）。
2. **不要带 `/chat/completions`**。库会自动拼装。
3. **`api_key` 必须非空**，无鉴权就填 `EMPTY`（`factory.go:362-365` 拒绝空串）。
4. **显式设置 `general.context_length`**，否则回退 200k，压缩永不触发。
5. 应用 D2 前，确认服务端 `max_model_len ≥ 64000`，否则每次请求 400。

### 5.3 推荐路径：UI 配置（无需改代码）

Settings → Providers（Platform 层，需 superadmin）→ Add Provider Instance：
- Type：`OpenAI Compatible`（UI 选项已存在，`web/src/components/settings/ProvidersSettings.tsx:146`）
- `base_url` / `api_key` 按 5.2 填写
- Test Connection：`POST /api/settings/providers/test`（`provider_settings_handlers.go:512-548`）
- 拉取模型列表：`GET /api/providers/{id}/models`（`settings_handlers.go:603-635`）
- 最后在 Default Configuration 里选 provider + model 并保存

保存后会立即失效 LLM 缓存池并热切换（`provider_settings_handlers.go:119-138`），无需重启。

> 说明：通过 config.yaml 配置的实例在 UI 里显示为 `read_only`（`provider_settings_handlers.go:352-357`），两种来源可共存，同名冲突时 DB 优先（`mergeResolvedProviders`，`request_helpers.go:397-408`）。

---

## 6. 验收清单

```bash
# 1) 容器健康
docker compose ps                       # troupe = healthy, troupe-nginx = Up

# 2) 免鉴权健康端点（已在容器内做过 Healthcheck）
docker compose exec troupe curl -fsS http://127.0.0.1:9393/api/healthz   # {"status":"ok"}
docker compose exec troupe curl -fsS http://127.0.0.1:9393/api/readyz    # platform 模式会 ping DB

# 3) 经 Nginx 的 SPA 入口
curl -fsS -o /dev/null -w '%{http_code}\n' http://localhost:8080/        # 200

# 4) UI 确实已内嵌（出现 "serving web assets from filesystem" 说明 dist 没编进去）
docker compose logs troupe 2>&1 | grep -i "web assets"

# 5) SSE 不被 Nginx 缓冲：逐步输出的流式响应应当逐 chunk 抵达，而非全部走完才一次性返回
time curl -N -sS http://localhost:8080/api/healthz >/dev/null            # 探针应瞬间返回

# 6) 二进制版本（验证 ldflags）
docker compose exec troupe troupe version

# 7) 可选：pre-commit 级检查
make lint && go test ./pkg/config ./pkg/provider ./pkg/api
```

**数据持久化验证**：`docker compose down && docker compose up -d` 后账户应仍存在（验证 `TROUPE_JWT_SECRET` 生效）；`ls deploy/state/data` 应能看到 `platform.db` 及 `orgs/`。

---

## 7. 已知坑与排障矩阵

| 现象 | 根因 | 处置 |
|---|---|---|
| 启动即退出，日志 `no matching files found` / embed 报错 | `web/dist` 为空或未生成 | 检查 Stage 1 是否成功；本机编译需 `make build-ui` |
| `Failed to open storage ... no such file or directory` | SQLite data_dir 不存在 | 确保 `state/data` 已挂载且存在；或改配一个确实存在的路径 |
| `sqlite` 打开失败 / `PingContext` 报错 | 数据目录不存在或不可写 | 确保 `state/data` 已创建并被挂载；容器内以 root 运行应可写 |
| 启动卡很久后失败（embedding） | 首次下载 23MB 失败（无外网/被墙） | 改用自建 embedding（模板已配），或预先把 `models/sentence-transformers_all-MiniLM-L6-v2/` 放进 `state/config/models/` |
| 登录成功但刷新就登出 | `TROUPE_JWT_SECRET` 未设，重启换了签名 key | 设置该变量后重新注册/登录一次 |
| 第二个用户注册卡住要验证码 | 默认 `require_email_verification: true` 且无 SMTP | 模板已设 `false`；生产环境请配 SMTP |
| 聊天瞬间失败 `no provider configured` | D1 未应用且 DB 未设默认 | UI 里设 Default Configuration，或应用 D1 |
| 模型返回 `400 ... maximum context length` | D2 的 64000 硬编码 | 应用 D2，或改用 `max_model_len ≥ 64000` |
| 请求 401 且看起来打到了 OpenAI 官方 | `base_url` 配空或拼写错误被静默回落 | 应用 D4；检查 UI 里实际保存的值 |
| 页面 404/cookie 丢失循环登出 | Nginx 误设 `X-Forwarded-Proto https` 而实际是 HTTP | 改为 `http`（或真正配 TLS 后再改） |
| 长对话流到一半断开 | Nginx `proxy_read_timeout` 默认 60s | 配置已设为 3600s |
| `docker compose logs` 空的 | `default` 模式默认写文件 | 配置已设 `TROUPE_LOG_DEST=stdout` |
| 构建时 `sudo: not found` 退出 | sandbox 默认 enabled 且非 root 运行 Linux | 模板已 `sandbox.enabled: false` |
| `git status` 出现上百 MB 未跟踪文件 | `troupe-linux-*` 未被 `.gitignore` 覆盖 | 见 D3 的 `.gitignore` 同步项 |

---

## 8. 改动文件总览

**新增（已落地）**

```
deploy/Dockerfile
deploy/compose.yaml
deploy/env.example
deploy/config.example.yaml
deploy/nginx/nginx.conf
deploy/.gitignore
deploy/DEPLOY_STORY.md
```

**建议修改**

| 优先级 | 文件 | 说明 |
|---|---|---|
| P0 | `pkg/api/request_helpers.go` (:317-321, :375-379) | D1 条件覆盖默认值 |
| P0 | `pkg/provider/openai_compat/openai_compat.go`, `pkg/provider/factory.go` (:361-373) | D2 max_tokens 可配置 |
| P0 | `docker-compose.yml`（根）或改用 `deploy/compose.yaml`；`docker/troupe/Dockerfile:102,107` | D3 构建链 |
| P1 | `pkg/provider/openai_compat/openai_compat.go:27-37,66-75`；`factory.go:366-369` | D4 URL 规范化 + 去掉静默回落 |
| P1 | `pkg/config/app_config.go:1774-1803` | D5 可选的环境变量插值 |
| P2 | `pkg/provider/factory.go:362-365` | D6 允许空 api_key |
| P2 | `app_config.go:599-622`、`daemon/run.go:71-73`、网站文档 | D7 过期注释清理 |
| P2 | `.dockerignore` | D8 忽略 `web/dist` |
| P2 | `pkg/provider/context_window.go:86-133` | D9 openai_compat 上下文探测 |

**回归测试建议**：`go test ./pkg/config ./pkg/provider ./pkg/api` + `cd web && npm test`。刻意避开 `make test-e2e`（需要 PostgreSQL、真实 LLM key 与 Kubernetes）。
