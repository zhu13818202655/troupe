# STORY-002 — Mac 本地 Docker Compose 部署（PostgreSQL + OpenAI 兼容模型 + Web 端可调用）

> **状态**：**已实现**（Track 1–4 代码与部署文件已落地并通过全量测试；剩余端到端起栈受限于本机无法访问 Docker Hub，见文末「实施结果」）
> **创建**：2026-10-10
> **相关**：[STORY-001 完成 troupe 品牌改名](001-complete-troupe-rebrand.md)（本 story 会触及其 Track B 的部分条目）
> **目标环境**：Apple Silicon Mac + Docker Desktop，部署产物全部位于 `deploy/`
> **前置**：`deploy/` 已有初版 `Dockerfile` / `compose.yaml` / `env.example` / `config.example.yaml` / `nginx/nginx.conf` / `DEPLOY_STORY.md`
> **操作手册**：[`../../deploy/README.md`](../../deploy/README.md)

---

## 0. 先请你确认的 6 个决策

这 6 项决定后面所有改动范围，**建议先勾完再开工**。

| # | 决策 | 选项 | 我的建议 |
|---|---|---|---|
| **D1** | 默认数据库后端 | ① PostgreSQL ② SQLite | **① PostgreSQL**。理由：sandbox GC / OpenShell 协调器只在 `Backend=="postgres"` 时启用（`pkg/daemon/run.go:1370/1421/1496`），SQLite 会永久失去这部分能力；且与你后续上生产一致 |
| **D2** | Redis 怎么处理 | ① 不引入 ② 引入容器但不接线 ③ 引入并实现真实用途 | **① 不引入**（见 1.3，代码里 0 依赖 0 使用）。若你确实要，我建议走 ③，用途锁定为"共享限流存储"，那是一个独立可交付的小改动 |
| **D3** | 配置目录 `~/.config/astonish` 是否改名 | ① 改 ② 不改 | **① 改**，但要带**兼容回落**：新目录不存在而旧目录存在时，沿用旧目录并打 warning。这样本机老安装不会失联 |
| **D4** | PG 库名/角色前缀 `astonish_*` 是否改名 | ① 改 ② 不改 | **① 改**。本次是全新部署，无历史库；但这是 STORY-001 Track B 的活，需要你确认一起做还是拆开 |
| **D5** | 品牌文案（大写 `Astonish`，约 1072 行）是否在本 story 一并改 | ① 只改部署/运行时相关 ② 全量改 | **① 只改部署路径会碰到的**（配置目录、PG 前缀、镜像名、前端 Logo/登录页），文案全量归 STORY-001 Track A，避免两个 story 互相冲突 |
| **D6** | sandbox（容器隔离）是否启用 | ① 先关 ② 开 | **① 先关**。启用需把 `/var/run/docker.sock` 挂进容器（DooD），且 Mac 上无 GPU 直通；关掉不影响 Web 端对话能力 |

---

## 1. 目前已有的能力（盘点结论）

### 1.1 已具备、可直接用的

| 能力 | 证据 | 说明 |
|---|---|---|
| 单二进制 + 内嵌 SPA | `web/embed.go:9` `//go:embed all:dist` | `daemon run` 同时提供 UI 和 API |
| Studio 服务 | `pkg/launcher/studio.go:115-325` | 监听 `:<port>`，默认 9393 |
| Platform 多租户 | `pkg/store/entstore/` | platform / org / team / personal 四层 scope |
| JWT 内置认证 | `pkg/api/auth_platform.go:127-243` | **首个注册用户自动成为 org owner + platform superadmin，且免邮箱验证** |
| `openai_compat` provider | `pkg/provider/openai_compat/openai_compat.go` | 已有实现，非从零开发 |
| `/v1` 自动补全 | `openai_compat.go:27-37` | 填 `http://host:8000` 会自动补成 `/v1` |
| 拉取模型列表 | `openai_compat.go:63-89`；`pkg/api/settings_handlers.go:603-635` | UI 可调 `GET /api/providers/{id}/models` |
| UI 配置 provider | `web/src/components/settings/ProvidersSettings.tsx:146` | 下拉里已有 `OpenAI Compatible`；含 Test Connection |
| 三层 provider 设置 | Platform / Org / Team 各自 API（`pkg/api/handlers.go:1114-1121`） | DB 值优先于 config.yaml，同名覆盖 |
| PostgreSQL 后端 | `pkg/store/entstore/`（唯一实现） | 含 `BootstrapPlatform` 自动建库、建角色、装扩展、全量建表 |
| 免鉴权健康端点 | `pkg/api/handlers.go:1018-1019`，白名单 `auth_platform_middleware.go:102` | `/api/healthz`、`/api/readyz` 可直接 curl |
| SSE 流式 | `pkg/api/chat_handlers.go:694` 等 6 处 `X-Accel-Buffering: no` | 应用侧已为反代做好准备 |

### 1.2 已有但有缺陷的（详见第 2 节）

- `openai_compat` 的 `max_completion_tokens` 硬编码 64000
- config.yaml 的 `default_provider` / `default_model` 在 platform 模式下被空 DB 值覆盖
- `base_url` 尾斜杠会产生 `/v1/v1`；空值静默打到官方
- `openai_compat` 无 context window 探测，一律回退 200000
- PG 后端：预建库会导致只建 10/24 张表
- org 库命名在两处实现不一致

### 1.3 Redis：**目前完全没有**

| 检查项 | 结果 |
|---|---|
| `go.mod` 中的 redis 依赖 | **0**（`grep -i redis go.mod` 无命中） |
| `pkg/` `cmd/` 中的 redis 使用 | **0**（仅 `pkg/tui/plan_render_test.go:322-323` 的**测试夹具文本**里出现过 "Redis caching" 字样） |

即：**加一个 Redis 容器，今天没有任何代码会连它**，它只会占内存、增加启动依赖。
若你出于"后面要用"想先放着，我建议放在 `profiles: ["redis"]` 里按需启动，而不是默认拉起。
若你希望它真正发挥作用，最小且完整的一个用途是**共享限流**：现在的限流是每实例进程内的（`pkg/api/ratelimit.go`，auth 端点 10 次/分钟/IP），多副本时形同虚设。我可以把它改成 Redis 后端——这是一次独立的、约 1 个文件的改动。

---

## 2. 缺什么（差距清单）

### 2.1 阻塞级（不改就跑不通 / 跑不对）

| ID | 问题 | 证据 | 后果 |
|---|---|---|---|
| **B1** | PG 镜像不带 pgvector | `entstore/bootstrap.go:127-129` `CREATE EXTENSION vector` **失败即返回 err**；`tenant_router.go:300-302` 同样 | 用 `postgres:17-alpine` 会在 bootstrap 时**硬失败** |
| **B2** | 预建 platform 库 → 只建 10/24 张表 | `platform_extras.go:78-94` PG 分支只 `Create` 白名单表（`:114-131` 共 10 张）；`MigrateAllSchemas` 是 no-op（`:246-253`） | `users` / `organizations` / `platform_settings` / `platform_secrets` 等 14 张表**不存在**，且**静默**——daemon 打印 "postgres (platform mode)" 正常起来，注册时才炸 |
| **B3** | `instance_suffix` 不设时 org 库名不一致 | `config.OrgDBName`→`astonish_org_{slug}`（`app_config.go:758-764`）vs `tenant_router.orgDBName`→`astonish_{slug}`（`tenant_router.go:115-121`）；写入用前者（`auth_platform.go:690`），连接用后者（`tenant_router.go:186`） | 注册后连不上 org 库 |
| **B4** | `max_completion_tokens` 硬编码 64000 | `openai_compat.go:48` → `openai.go:103-105` 无条件下发 | 任何 `max_model_len < 64000` 的自建模型**每次请求 400**。这是自建模型接入的**头号失败原因**，且 `pkg/provider/llmerror/max_output.go` 不处理 vLLM 类错误 |
| **B5** | config.yaml 的默认模型不生效 | `pkg/api/request_helpers.go:375-379` 与 `:317-321` 无条件用 DB 值覆盖（DB 为空也赋 `""`） | `Pool.Get` 返回 `no provider configured`（`pool.go:45-47`）。注意 daemon 自己的 cascade 是**有条件**的（`run.go:2243-2249`），只有这两处不一致 |

### 2.2 重要但不致命

| ID | 问题 | 证据 | 说明 |
|---|---|---|---|
| **B6** | `base_url` 尾斜杠 → `/v1/v1` | `openai_compat.go:27-37` 只判 `/v1` 后缀 | 填 `http://h:8000/v1/` 会 404，且错误信息完全指不到根因 |
| **B7** | `base_url` 为空静默打官方 | `factory.go:366-369` 回落 `https://api.openai.com/v1` | 与 UI 的 "Test Connection" 行为不一致（后者硬报错，`factory.go:744-747`） |
| **B8** | openai_compat 无 context window 探测 | `context_window.go:86-133` 的 switch 无 `openai_compat` 分支 → 恒返回 0 → 回退 **200000**（`:66`） | 压缩永不触发，长对话直到服务端溢出才失败 |
| **B9** | `api_key` 必须非空 | `factory.go:362-365` | 本地无鉴权服务只能填 `EMPTY` 占位符；openai_compat 还没有 env 回退（`provider_env.go:10-57` 无此条目） |
| **B10** | `troupe platform status` 不读 `TROUPE_PLATFORM_DSN` | `cmd/troupe/platform.go:52-55`、`:366` 用裸字段 | 只用 env 配 DSN 时，该命令查不到库（不影响 daemon） |
| **B11** | DSN 为空静默降级 SQLite | `entstore.go:283-289` `detectDialect` 按前缀嗅探，非 postgres 一律判 SQLite | `backend: postgres` 但 DSN 没配会静默起成内存库 |
| **B12** | Bootstrap 带 `WithDropColumn/WithDropIndex(true)` | `bootstrap.go:131-136` | 语义是"对齐 Ent 定义并删多余列"，**对已有数据的库是破坏性的**。全新部署无害，但要写在文档里 |

### 2.3 品牌残留（与 STORY-001 重叠，本 story 只做部署相关）

本次实测基线（排除 `web/dist`、node_modules；扫描 `pkg cmd docker ent web/src docs tests extension scripts tools Makefile .gitignore .dockerignore docker-compose.yml README.md`）：

| 标识符 | 剩余行数 | 部署路径上是否会遇到 |
|---|---|---|
| 含 `astonish`/`Astonish` 的行 | **3678** | — |
| 大写 `Astonish`（文案） | **1072** | 前端登录页/Logo（归 STORY-001 Track A） |
| `astonish.io/`（k8s 标签） | 79 | 否（本地部署不涉及 k8s） |
| `astonish-sandbox`（镜像名） | 107 | 否（sandbox 先关） |
| `astonish-session`（容器名） | 45 | 否 |
| `libastonish`（原生库） | 45 | 可选（treesitter 可选特性） |
| `ghcr.io/sap`（registry） | 54 | 否（我们自己构建镜像） |
| **`astonish_platform` / `astonish_app`（PG 库名与角色）** | **14 + 14** | **是** ← D4 |
| **`~/.config/astonish`（配置目录）** | `app_config.go:1675/1684/1689` 共 3 处 | **是** ← D3 |
| **`docker/astonish`（失效引用）** | 5（含根 `docker-compose.yml:5`） | **是** ← 必须修 |

前端用户可见：`web/src/components/AstonishLogo.tsx`(7)、`TopBar.tsx`(5)、`SetupWizard.tsx`(5)、`settings/ChatSettings.tsx`(4)、`LoginPage.tsx`(3) 等。

---

## 3. 要改什么（改动清单）

> 每行都标了**所属 Story**：`S2`=本 story，`S1`=STORY-001（本 story 顺带做或需要它配合）。

### Track 1 — OpenAI 兼容模型可用性（`S2`，全部 P0/P1）

| # | 改动 | 文件 | 优先级 |
|---|---|---|---|
| T1.1 | `max_completion_tokens` 改为可配置：新增实例键 `max_output_tokens`；未配置时回退到 `general.context_length`，再回退 64000 | `pkg/provider/factory.go:361-373`、`pkg/provider/openai_compat/openai_compat.go:24-50`（唯一调用点 `factory.go:373`） | **P0** |
| T1.2 | 默认 provider/model 改为条件覆盖（两处：`request_helpers.go:317-321` 与 `:375-379`） | `pkg/api/request_helpers.go` | **P0** |
| T1.3 | 抽出 `normalizeBaseURL` 共享 helper，修掉尾斜杠 `/v1/v1`；`base_url` 为空改为显式报错而非静默回落官方 | `openai_compat.go:27-37`、`:66-75`、`factory.go:366-369` | P1 |
| T1.4 | 为 `openai_compat` 增加 context window 探测（读 `/v1/models` 元数据，vLLM 返回 `max_model_len`） | `pkg/provider/context_window.go:86-133` | P1 |
| T1.5 | 允许 `api_key` 为空（仅 openai_compat，不引入 env 回退以免误读 `OPENAI_API_KEY`） | `factory.go:362-365` | P2 |

### Track 2 — PostgreSQL 后端（`S2`）

| # | 改动 | 文件 | 优先级 |
|---|---|---|---|
| T2.1 | compose 换镜像 `pgvector/pgvector:pg17` | `deploy/compose.yaml` | **P0** |
| T2.2 | `POSTGRES_DB` 设为 `postgres`（**不预建 platform 库**），让 bootstrap 走"库不存在"分支完成全量建表 | `deploy/compose.yaml` + `deploy/env.example` | **P0** |
| T2.3 | 必须设 `storage.postgres.instance_suffix`（绕开 B3） | `deploy/config.example.yaml` | **P0** |
| T2.4 | **修 B3 本体**：统一 org 库命名，让 `tenant_router.orgDBName` 与 `config.OrgDBName` 一致 | `pkg/store/entstore/tenant_router.go:115-121` | P1 |
| T2.5 | `detectDialect` 遇到空 DSN 显式报错，不静默降 SQLite | `pkg/store/entstore/entstore.go:283-289` | P1 |
| T2.6 | `troupe platform status` 支持 `TROUPE_PLATFORM_DSN` 回退 | `cmd/troupe/platform.go:52-55`、`:366` | P2 |
| T2.7 | 提供 `deploy/scripts/init-db.sh`：显式跑一次 bootstrap 并校验 24 张表（不依赖"库不存在"这个隐式前提） | `deploy/scripts/`（新增） | P1 |

### Track 3 — deploy 资产（`S2`）

| # | 改动 | 说明 |
|---|---|---|
| T3.1 | `deploy/compose.yaml` 改为 PG 为默认后端（D1），SQLite 降为可选 profile | 含 `troupe` / `postgres` / `nginx` 三个服务 |
| T3.2 | `deploy/config.example.yaml` 切换为 `storage.backend: postgres` + `platform_dsn` + `instance_suffix` | 保留 SQLite 段作为注释 |
| T3.3 | `deploy/env.example` 增加 `POSTGRES_*` 与 `TROUPE_PLATFORM_DSN` 说明 | 明确 `POSTGRES_DB=postgres` 的原因 |
| T3.4 | `deploy/nginx/nginx.conf` 复核并补 PG 就绪后的 `start_period` | 冷启动首次 bootstrap 会明显变慢 |
| T3.5 | 新增 `deploy/scripts/init-db.sh`、`deploy/scripts/smoke.sh` | 建库校验 + 端到端冒烟 |
| T3.6 | 更新 `deploy/DEPLOY_STORY.md`，合并本文档结论 | 避免两份文档打架 |

### Track 4 — 品牌与命名（`S2` 做部署相关，`S1` 做文案）

| # | 改动 | 归属 | 优先级 |
|---|---|---|---|
| T4.1 | 配置目录 `astonish` → `troupe` + **旧目录兼容回落** | `app_config.go:1675/1684/1689`（D3） | **P0** |
| T4.2 | PG 库名/角色前缀 `astonish_` → `troupe_` | `tenant_router.go`、`bootstrap.go`、`pgutil.go`、`app_config.go`（D4） | P1 |
| T4.3 | 根 `docker-compose.yml`：**已直接删除**（它当时完全不可用，见 B1–B4）；唯一入口为 `deploy/compose.yaml` | 当前 5 处 `docker/astonish` 失效引用 | **P0** ✅ |
| T4.4 | `Makefile:553/559` 产物名加进 `.gitignore`（现只忽略 `astonish-linux-*`） | 避免 100MB 产物污染 git | P1 |
| T4.5 | 前端 `AstonishLogo.tsx` → `TroupeLogo.tsx`、`LoginPage` / `SetupWizard` / `TopBar` 文案 | D5，若选 ① 则只改这几个 | P1 |
| T4.6 | STORY-001 Track A/B 剩余（1072 行文案、`astonish.io/` 79、`astonish-sandbox` 107 等） | 归 STORY-001，本 story 只登记不动 | 后续 |

> **边界说明**：`REUSE.toml` / `LICENSE` 的 `SAP SE` 与作者署名**不动**（Apache-2.0 要求，STORY-001 §1.5 已明确）。

### Track 5 — 验收（`S2`）

| # | 项 | 方法 |
|---|---|---|
| T5.1 | 构建 | `docker compose -f deploy/compose.yaml build`（首次 5–10 分钟） |
| T5.2 | 库校验 | 连 PG 数 `information_schema.tables`，**必须 24 张**；关键 4 张 `users/organizations/platform_settings/platform_secrets` 必须存在 |
| T5.3 | 服务健康 | `/api/healthz` → `{"status":"ok"}`；`/api/readyz` 通 |
| T5.4 | Web 注册 | 浏览器开 `http://localhost:8080` → 注册首个用户 → 自动 superadmin + 自动建 org 库 + 默认 team |
| T5.5 | **模型接入** | UI 加 `OpenAI Compatible` provider → Test Connection → 拉模型列表 → 设为默认 |
| T5.6 | **端到端对话** | 发一条消息，确认流式返回；再用 `curl -N` 直连 API 确认 SSE 逐块到达（验证 Nginx 未缓冲） |
| T5.7 | 回归 | `go build ./...`、`go test ./pkg/config ./pkg/provider ./pkg/api`、`cd web && npm test` |

---

## 4. 怎么做（执行顺序与理由）

```
阶段 0  决策确认（D1–D6）                        ← 阻塞全部
阶段 1  代码修复 Track 1（T1.1 / T1.2 为 P0）
        └─ 理由：这两个是"自建模型能不能对话"的决定性因素，且改动面最小（3 个文件）
阶段 2  PG 后端 Track 2（T2.1 / T2.2 / T2.3 为 P0）
        └─ 理由：先让 PG 能正确 bootstrap，否则后面所有验证都是假的
阶段 3  deploy 资产 Track 3（T3.1–T3.6）
        └─ 理由：依赖阶段 2 的结论（DSN / suffix / 镜像）
阶段 4  品牌与命名 Track 4（T4.1 / T4.3 为 P0）
        └─ 理由：T4.1 会改配置目录，必须在验证之前定，否则验证完再改目录等于重验一遍
阶段 5  端到端验收 Track 5 + T3.6 文档合并
```

**为什么 T4.1（配置目录改名）放在验证之前**：它决定了 `deploy/state/config` 挂载到容器内的路径。若在验证完成后再改，所有持久化数据路径都要重来。

**每个阶段独立提交**，便于二分回滚（沿用 STORY-001 §3.4 的约定）。

### 4.1 风险

| 风险 | 触发条件 | 缓解 |
|---|---|---|
| PG 只建 10/24 表（静默） | `POSTGRES_DB` 预建了 platform 库 | T2.2 设 `POSTGRES_DB=postgres`；T2.7 脚本显式校验 24 张表 |
| Bootstrap 破坏性 | 对已有数据的库运行 | 全新部署无害；文档注明 `WithDropColumn(true)` 语义，禁止对存量库跑 |
| 改了配置目录导致老用户失联 | D3 选"改" | 加旧目录兼容回落 + warning |
| 改了 org 库命名导致已有库找不到 | D4 选"改"且本地已有数据 | 全新部署无影响；如需保留，先用 `instance_suffix` 隔离 |
| 本地模型仍 400 | T1.1 未生效或模型 `max_model_len` 极小 | 验收阶段用真实模型跑一次；保留 `max_output_tokens` 作为兜底旋钮 |
| Nginx 缓冲导致 SSE 卡住 | 配置被"简化" | `proxy_buffering off` + `proxy_read_timeout 3600s`；验收用 `curl -N` 逐块确认 |

### 4.2 关于"Docker daemon"

上次会话实测 **Docker Desktop 未运行**（`docker info` 连不上 `/Users/louis/.docker/run/docker.sock`），所以我只做了 `docker compose config` 的语法校验（通过），**没有实机构建过镜像**。
开工前需要你先启动 Docker Desktop；否则 T5.1 之后的验证无法进行。

---

## 5. Todo list tree

```
STORY-002 Mac 本地 Docker Compose 部署
│
├── 0. 决策（阻塞全部）                                        ← 等你的确认
│   ├── [ ] D1 默认后端：PostgreSQL / SQLite
│   ├── [ ] D2 Redis：不引入 / 只放容器 / 实现共享限流
│   ├── [ ] D3 配置目录 astonish → troupe（带兼容回落）
│   ├── [ ] D4 PG 库名与角色前缀 astonish_ → troupe_
│   ├── [ ] D5 品牌文案：只改部署相关 / 全量改
│   └── [ ] D6 sandbox：先关 / 开（需挂 docker.sock）
│
├── 1. OpenAI 兼容模型可用性
│   ├── [ ] T1.1 max_completion_tokens 可配置（P0）        factory.go:361-373 + openai_compat.go:24-50
│   ├── [ ] T1.2 默认 provider/model 条件覆盖（P0）        request_helpers.go:317-321 + 375-379
│   ├── [ ] T1.3 base_url 规范化 + 空值显式报错（P1）       openai_compat.go:27-37/66-75 + factory.go:366-369
│   ├── [ ] T1.4 openai_compat context window 探测（P1）    context_window.go:86-133
│   └── [ ] T1.5 允许空 api_key（P2）                       factory.go:362-365
│
├── 2. PostgreSQL 后端
│   ├── [ ] T2.1 镜像换 pgvector/pgvector:pg17（P0）
│   ├── [ ] T2.2 POSTGRES_DB=postgres，不预建 platform 库（P0）
│   ├── [ ] T2.3 必须设 instance_suffix（P0）
│   ├── [ ] T2.4 统一 org 库命名，修 B3（P1）                tenant_router.go:115-121
│   ├── [ ] T2.5 detectDialect 空 DSN 显式报错（P1）         entstore.go:283-289
│   ├── [ ] T2.6 platform status 支持 TROUPE_PLATFORM_DSN（P2）
│   └── [ ] T2.7 deploy/scripts/init-db.sh 校验 24 张表（P1）
│
├── 3. deploy 资产
│   ├── [ ] T3.1 compose.yaml：PG 为默认 + nginx（+ SQLite 可选 profile）
│   ├── [ ] T3.2 config.example.yaml：切 postgres + dsn + suffix
│   ├── [ ] T3.3 env.example：POSTGRES_* 与 TROUPE_PLATFORM_DSN
│   ├── [ ] T3.4 nginx.conf：复核 start_period 与 SSE 设置
│   ├── [ ] T3.5 scripts/init-db.sh + scripts/smoke.sh
│   └── [ ] T3.6 合并进 deploy/DEPLOY_STORY.md
│
├── 4. 品牌与命名
│   ├── [ ] T4.1 配置目录 → troupe + 兼容回落（P0）          app_config.go:1675/1684/1689
│   ├── [ ] T4.2 PG 库名/角色前缀 → troupe_（P1）            依赖 D4
│   ├── [x] T4.3 根 docker-compose.yml 已删除，只保留 deploy/compose.yaml（P0）
│   ├── [ ] T4.4 .gitignore 补 troupe-linux-*（P1）
│   ├── [ ] T4.5 前端 Logo / 登录页 / 向导文案（P1）         依赖 D5
│   └── [ ] T4.6 登记 STORY-001 剩余项（不动）
│
└── 5. 验收
    ├── [ ] T5.1 docker compose build
    ├── [ ] T5.2 PG 24 张表校验
    ├── [ ] T5.3 healthz / readyz
    ├── [ ] T5.4 Web 注册首个用户
    ├── [ ] T5.5 UI 配置 OpenAI Compatible + Test Connection
    ├── [ ] T5.6 端到端对话 + curl -N 验证 SSE 逐块
    └── [ ] T5.7 go build / go test / npm test
```

---

## 已完成修复记录（2026-10-11 下午）

### Fix: Settings「Select Model」对话框 Failed to fetch models

- **现象**：UI `GET /api/providers/local-llm/models-metadata` 返回 500，上游 401（`{"error":"Unauthorized"}`）。
- **根因**：加密凭据存储 `~/.config/troupe/credentials.enc` 中存在一条错误密钥
  `provider.local-llm.api_key = "EMPTY"`（5 字符字面量）。`resolveProviderSecret`
  的取值顺序是 凭据存储 → config.yaml → 环境变量，非空占位值抢在 config.yaml
  里正确 key 之前被发给网关 → 401。聊天路径之所以正常，是因为
  `InjectProviderSecretsToConfig` 只在 config 缺 key 时才注入 store 值。
- **修复**：
  1. `docker exec troupe troupe credential remove provider.local-llm.api_key` + 重启容器（即时生效）。
  2. 代码防护（防复发）：`resolveProviderSecret`（pkg/api/settings_handlers.go）与
     `InjectProviderSecretsToConfig` / `SetupAllProviderEnvFromStore`（pkg/config/provider_env.go）
     现在跳过 `"EMPTY"` 与 `"****"` 前缀（掩码回显）两类占位值，`isPlaceholderSecret()`。
- **验证**：`models-metadata` → 200（Qwen/Qwen3.8-27B-FP8）；`go test ./pkg/api ./pkg/config` 全绿；healthz 200。
- **部署方式备注**：本次因 buildx 在沙箱 shell 下 rename ~/.docker/buildx/activity 失败，
  采用宿主机交叉编译（CGO_ENABLED=0 GOOS=linux GOARCH=arm64）+ `docker cp` 替换
  /usr/local/bin/troupe 并重启容器。下次在正常终端执行
  `cd deploy && docker compose up -d --build` 会把修复固化进镜像。
