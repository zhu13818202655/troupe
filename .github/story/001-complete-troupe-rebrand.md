# STORY-001 — 完成 troupe 品牌改名

> **状态**：✅ 已完成（2026-10-11，见文末「完成报告」）
> **创建**：2026-10-07
> **前置提交**：`cf51ae01`（仓库标识替换 + deploy 资产删除 + 3 个缺陷修复）、`b73a5094`（ent 代码重新生成）
> **相关文档**：`docs/architecture/runtime-distribution-plane.md`（运行时层的技术清单）

---

## 0. 完成报告（2026-10-11）

Track A（用户可见文案）、Track B（运行时标识符）与兼容性敏感项已全量落地，`go build ./...` 通过，`go test ./...` 76 包全绿（唯一失败 `pkg/a2a TestSkillsFromCapabilities` 为技能 ID 排序断言——`astonish-core` → `troupe-core` 改变了字典序位置，已按新序修正）。共改动约 780 个文件。

### 兼容性敏感项的处理（新名写入 + 旧名读取）

| 项 | 新值 | 旧值兼容 |
|---|---|---|
| 团队选择头 | `X-Troupe-Team` | 服务端同时接受 `X-Astonish-Team`（`auth_platform_middleware.go`、`memory_health.go`，含新增测试） |
| 内存模式头 | `X-Troupe-Memory-Mode` | 接受 `X-Astonish-Memory-Mode` |
| 会话 Cookie | `troupe_session` / `troupe_access` / `troupe_refresh` | 读取回退旧名；`clearAuthCookies` 同时过期新旧 Cookie（老会话不断登录） |
| SSO 续跳 Cookie | `troupe_oauth_continue_*` | 读取回退旧前缀 |
| 应用名 | `troupe` / `troupe-extension` | `normalizeAppName` 把 `astonish` / `astonish-extension` 归一化到新值 |
| Slack 命令前缀 | `/troupe-*` | 入口归一化接受 `/astonish-*` 与 `@Astonish`（老 Slack App 安装可用） |
| OAuth client ID | `troupe-chrome-extension` / `troupe-cli` | 校验同时接受旧 ID |
| 会话状态键 | `_troupe_parent_id` / `troupe_plan_lifecycle` / `troupe_memory_type` | 读取回退旧键（存量转录/记忆不失效） |
| 栅栏语言 ID | `troupe-report` / `troupe-app` / `troupe-page-edit` / `troupe-page-tool` | 所有解析正则双接受 |
| 转录标记 | `[Troupe Per-Turn Context …]` | 读取双前缀（`tui_code.go`） |
| JWT issuer | `troupe` | 校验接受 `astonish` |

### 特意保留不变的旧名（有理由，不是遗漏）

- `pkg/config/app_config.go`：`legacyConfigDirName = "astonish"` 与 `resolveConfigDir` 的状态感知回退——老安装的配置/凭据平滑迁移。
- `pkg/backup/`：`ArchiveFormat = "astonish.backup"`、JSON 标签 `json:"astonish"`、加密魔数 `ASTONISH-BACKUP-ENC-v1`——写入备份文件头的格式标识，改动会让旧备份无法恢复（已加注释）。
- `pkg/memory/scenario_identity.go`：对含 `astonish` 的主机/路径的记忆启发式匹配——兼容存量数据。
- `.gitignore` 的旧产物条目（`./astonish`、`astonish-linux-*` 等）——防止历史未跟踪文件混入。

### 已知的遗留（低风险，另开一轮）

1. **`.github/workflows/build.yml` 的 CI 断言**仍引用 `libastonish-treesitter.so`（见 `docs/architecture/runtime-distribution-plane.md:121-124`）——CI 未在本轮验证，下次跑 CI 时需核对。
2. **docs/website 的 VitePress `base: '/astonish/'`**——改了会导致已部署文档站 404，需要与文档站重新发布一起做。
3. **SPDX 版权头**（`REUSE.toml`、源码 `SPDX-License-Identifier` 行）——法律声明，涉及版权主体变更，未动。
4. **`github.com/astonish-clawhub` 外链**——无法确认是否第三方服务，保持原样。
5. **邮件 logo URL**（`pkg/mailer/message.go`）指向 `sap.github.io/troupe/...`——该站点不存在，需要先有真实的品牌资源托管（原 B6.4 项）。
6. 本地化部署使用的镜像 `troupe-local:latest` 需要重新 `docker compose build` 才能吃到本轮改动（见 STORY-002）。

---

## 1. 背景

### 1.1 为什么改名会分成三层

改名的目标标识符按「改了之后会不会立刻报错」自然分成三层，之前只处理完了第一层：

| 层 | 内容 | 改错的后果 | 状态 |
|---|---|---|---|
| 第 1 层 | 小写 `astonish` 标识符：模块路径、`package` 名、`ASTONISH_` 环境变量、CLI 命令名、k8s 命名空间默认值 | 编译失败 / 链接失效 / 命令找不到——**风险自暴露** | ✅ 已完成 |
| 第 2 层 | 大写 `Astonish` 产品名：CLI 帮助、错误文案、邮件模板、机器人自我介绍、前端 UI、文档 | 不影响运行，**但用户会看到** | ❌ 未开始（本文档 Track A） |
| 第 3 层 | 运行时标识符：容器镜像、k8s 标签前缀、Pod/容器名前缀、原生库名 | **静默改变运行行为**：拉不到镜像、GC 认不出 Pod、旧数据失联 | ❌ 未开始（本文档 Track B） |

第 1 层之所以先做完，是因为它会自己暴露错误；第 3 层之所以一直没动，是因为**改错的代价最高且不会立刻报警**——它是唯一需要集群验证的一层。

### 1.2 第 1 层已完成的证据（不要重复做）

| 检查项 | 结果 |
|---|---|
| `SAP/astonish` 旧仓库地址 | 0 处 |
| `package astonish` | 0 处（`package troupe` 47 处） |
| `ASTONISH_` 环境变量 | 0 处（`TROUPE_` 276 处） |
| `cmd/` 内小写 `astonish` 命令文本 | 0 处 |
| k8s 命名空间默认值 | `troupe-sandboxes` |
| CI 构建产物名 | `troupe`（与 `Makefile:12` 的 `BINARY_NAME` 一致） |
| `deploy/helm`、`deploy/k8s` | 已删除，断链已清理 |

### 1.3 现状数据（2026-10-07 实测，可作为验收基线）

**Track A — 大写 `Astonish`：1144 处 / 382 文件**

| 目录 | 处数 | 文件数 |
|---|---|---|
| `docs/` | 522 | 112 |
| `pkg/` | 422 | 179 |
| `web/` | 76 | 37 |
| 仓库根 | 39 | 7 |
| `cmd/` | 32 | 15 |
| `tests/` | 21 | 17 |
| `extension/` | 18 | 8 |
| `docker/` | 8 | 3 |
| `scripts/` | 4 | 2 |
| `ent/`、`tools/` | 各 1 | 各 1 |

其中**代码内字符串（疑似用户可见）约 232 处**，例如：

```
cmd/troupe/version.go:24          Name = "Astonish AI Companion"        ← 版本输出里的产品名
cmd/troupe/setup.go:1690          Title("How would you like to run Astonish?")
cmd/troupe/browser_setup.go:40    huh.NewOption("Default (Chromium, auto-downloaded by Astonish)", "default")
cmd/troupe/code.go:92             fmt.Println("Run Astonish as a local coding tool. ...")
cmd/troupe/platform_backup.go:293 fmt.Println("Create, inspect, and verify Astonish backup archives.")
pkg/mailer/templates.go:22        fmt.Sprintf("You've been added to %s on Astonish", m.OrgName)
pkg/daemon/run.go:1731            fmt.Printf("Astonish daemon running (pid: %d, port: %d)\n", ...)
pkg/channels/telegram/telegram.go:547  fmt.Sprintf("Hey %s! I'm Astonish, your AI assistant. ...", ...)
pkg/fleet/github_reporter.go:119  sb.WriteString(fmt.Sprintf("*Processed by [Astonish Fleet](...)*", ...))
```

**Track B — 运行时标识符**

| 标识符 | 处数 | 文件数 | 关键位置 |
|---|---|---|---|
| `ghcr.io/sap` 容器镜像 | 72 | 27 | 生产默认值 3 处见下方 |
| `astonish-sandbox` 镜像名 | 112 | 36 | — |
| `astn-` Pod/容器名前缀 | 192 | 36 | `k8s/session.go:143`、`k8s/fleet.go:50`、`k8s/template.go:68` |
| `astonish.io/` k8s 标签 | 86 | 17 | `k8s/session.go:58-61`、`k8s/fleet.go:45` |
| `troupe.io/` k8s 标签 | 10 | 4 | `cmd/troupe/sandbox.go:841,881,963`、`cmd/troupe/platform_sandbox_audit.go:350,485` ← **与 `astonish.io/` 混用** |
| `libastonish` 原生库 | 51 | 20 | `codeintel/internal/treesitter/bindings.go:17` |
| `astonish-session` 容器名 | 45 | 21 | `docker/backend.go:257`、`docker/session.go:543` |
| `sap.github.io` 文档站/邮件 logo | 10 | 4 | `pkg/mailer/message.go:15` ← **运行时引用** |

镜像的 3 处**生产默认值**（运行时真正生效，不是注释）：

```
pkg/sandbox/docker/backend.go:91       c.SandboxImage = "ghcr.io/sap/astonish-sandbox-base:latest"
pkg/sandbox/k8s/backend.go:252         c.SandboxImage = "ghcr.io/sap/astonish-sandbox-base:latest"
pkg/sandbox/openshell/backend.go:70    c.SandboxImage = "ghcr.io/sap/astonish-sandbox-openshell:latest"
```

### 1.4 已知的既有不一致（顺带修掉）

1. **镜像名构建/拉取不匹配**：`Makefile` 构建推送 `$(DOCKER_REGISTRY)/troupe-sandbox-base`，而 Go 运行时默认拉 `ghcr.io/sap/astonish-sandbox-base:latest`——本地构建出来的镜像**永远不会被使用**，除非显式覆盖 `sandbox.image`。
2. **标签前缀混用**：`astonish.io/*`（86）与 `troupe.io/*`（10）并存，任何混合使用两者的选择器都不会命中。

### 1.5 不属于本 story 的范围

- **版权署名**：`SAP SE`（4 处，`REUSE.toml` / `README.md` / `LICENSE` 等）与作者字段 `Rafael Schardosin Silva` **必须原样保留**——这是 Apache-2.0 的署名要求，不是改名目标。
- **小写 `astonish` 标识符**：已改完，不要回改。

---

## 2. 预期

### 2.1 开工前必须先定的决策

这两项决定了后面所有改动，**必须先定**：

| # | 决策 | 选项 | 建议 |
|---|---|---|---|
| D1 | 目标产品名 | `Troupe` / `Troupe AI` / 其他 | 与模块名一致用 `Troupe` |
| D2 | 副标题如何处理 | `cmd/troupe/version.go:24` 现为 `"Astonish AI Companion"` | → `"Troupe AI Companion"` 或直接 `"Troupe"` |
| D3 | 镜像 registry 与镜像名 | 是否发布自有镜像？registry 用哪个？镜像名是否保留 `-sandbox-base` / `-sandbox-openshell` 后缀？ | 不发布则只改文档，不改生产默认值 |
| D4 | k8s 标签前缀 | 保留 `astonish.io/*`（已是运维公开契约）还是迁到 `troupe.io/*` | 二选一，**不可混用** |
| D5 | k8s 命名空间单复数 | `troupe-sandbox` 还是 `troupe-sandboxes` | 现生产默认是复数 |

### 2.2 Track A 完成定义

- [ ] 用户可见的产品名统一为 `Troupe`（D1/D2 决策后）
- [ ] `cmd/`、`pkg/`、`web/`、`extension/` 内代码字符串中的 `Astonish` 归零
- [ ] `docs/` 内叙述性 `Astonish` 归零（追溯历史的表述除外，如 ADR 里"原名 Astonish"）
- [ ] `tests/` 内夹具字符串同步，测试全绿
- [ ] 全量验证通过：`go build` / `go test ./...` / `go vet` / `golangci-lint`

### 2.3 Track B 完成定义

- [ ] 生产默认的镜像引用指向**实际存在**的 registry 与镜像名，且与 `Makefile` 构建产物一致
- [ ] k8s 标签前缀全仓库统一（D4 决策）
- [ ] 命名空间单复数全仓库一致（D5 决策）
- [ ] 原生库改名后：产物名、embed tarball、镜像 `COPY`、`DefaultLibraryPath`、CI 断言**同一提交内**全部更新
- [ ] Pod/容器名前缀如需改，GC reconciler 的 `HasPrefix` 匹配逻辑同步更新并有测试覆盖
- [ ] 在**真实集群**上验证：创建会话 → Pod 以预期前缀/标签/命名空间出现 → GC 能回收

### 2.4 非目标（明确不做）

- 不改版权署名
- 不引入新的部署方案（那是另一个 story）
- 不追求 `docs/` 历史叙述中的 `Astonish` 归零——记录"改名前的名字"是合理的

---

## 3. 方案

### 3.1 总体策略

**Track A 可以批量替换，Track B 只能逐项决策 + 集群验证。**

理由：Track A 的改动不影响行为，出错由测试兜住；Track B 每个标识符都有外部副作用（registry 里有没有镜像、集群里有没有同名资源、GC 靠前缀认领 Pod），必须逐项判断后再动。

### 3.2 Track A：分四级递进，先代码后文档

```
A1 一次性字符串（改动最集中、收益最直接）
    └─ cmd/ 32 处 + pkg/ 422 处
A2 前端与扩展（用户直接看到）
    └─ web/ 76 处 + extension/ 18 处
A3 文档（量大但零风险）
    └─ docs/ 522 处
A4 测试夹具与校验
    └─ tests/ 21 处 + 全量回归
```

顺序理由：A1 决定"产品名长什么样"，A2/A3 必须跟随 A1 的结论；A4 最后做，避免夹具改了两遍。

**执行方式**：每级用一次脚本化替换 + 人工抽查 `git diff`。**不要**一次性全仓库 `sed`——`docs/` 里有需要保留的历史叙述，`REUSE.toml` 里有版权署名。

需要人工判断的例外（不要脚本化）：
- `REUSE.toml` / `LICENSE` / `README.md` 的版权与作者段
- `docs/` 中明确在讲历史的句子（如"原名 Astonish""从 Astonish 迁移"）
- 一切出现在 URL、包名、文件路径里的 `Astonish`（那属于 Track B 或已完成的第 1 层）

### 3.3 Track B：按风险从低到高

```
B0 决策
    └─ D3 registry/镜像名、D4 标签前缀、D5 命名空间单复数
B1 镜像引用（低风险，但必须先确认镜像真实存在）
    ├─ Makefile 构建/推送目标
    ├─ 3 处 Go 生产默认值
    └─ docker/ 下的 Dockerfile 与注释
B2 k8s 标签前缀统一（中风险，运维可见契约）
    └─ 决定保留哪一侧后，另一侧全量迁移 + 加迁移说明
B3 命名空间单复数统一（中风险）
    └─ 生产默认值 + e2e 默认值 + Makefile
B4 原生库改名（高风险，需重建产物）
    ├─ pkg/codeintel/native/Makefile 的 OUT
    ├─ embed 的 tarball
    ├─ docker/sandbox-base/Dockerfile 的 COPY
    ├─ bindings.go 的 DefaultLibraryPath 与文件名搜索列表
    └─ .github/workflows/build.yml 的断言
B5 Pod/容器名前缀（最高风险，放最后）
    ├─ docker 的 astonish-session-
    ├─ k8s 的 astn-sess- / astn-fleet- / astn-tmpl- / astn-org- / astn-gc-*
    └─ GC reconciler 的 HasPrefix 匹配 + 测试
B6 文档站与邮件 logo（需先有替代托管）
    ├─ README 徽章与链接
    ├─ web/src/components/UpgradeDialog.tsx 的安装命令
    └─ pkg/mailer/message.go 的 logoURL
```

### 3.4 风险与回滚

| 风险 | 触发条件 | 缓解 |
|---|---|---|
| 镜像拉取失败 | B1 改了生产默认值但镜像不存在 | B1 前先确认 registry 与镜像真实可拉；改完在本地跑一次沙箱创建 |
| GC 回收失效 / 误删 | B5 改了前缀但漏改 matcher | 一次只改一个前缀，每改一个跑一次 GC 测试；在真实集群验证 |
| 旧数据失联 | B2/B3 改标签或命名空间后，历史 Pod 不再被识别 | 改前先盘点集群内现有资源；必要时加过渡期双标签 |
| 原生库 `dlopen` 失败 | B4 只改了产物名没改搜索列表 | 产物名、tarball、COPY、DefaultLibraryPath、CI 断言同一提交 |
| 邮件 logo 404 | B6 改了 `logoURL` 但没托管替代图 | 先托管再改；或保留该 URL 不动 |

回滚：Track A 全部是纯文本改动，`git revert` 即可。Track B 的 B1/B6 涉及外部资源（registry 镜像、托管图片），回滚需要外部侧配合——因此**每个阶段独立提交**，便于二分。

### 3.5 验证方法

每阶段结束跑：

```bash
go build ./...
go test ./... -count=1
go vet ./...
golangci-lint run

# Track A 归零校验（按目录逐级收敛）
grep -rn --include='*.go' -F 'Astonish' cmd/ pkg/ | grep -v '_test.go'

# ent 生成代码必须幂等（CI 有硬性检查）
make ent-generate && git diff --exit-code ent/
```

Track B 额外需要在真实集群验证（B2/B3/B4/B5 适用）：

```bash
# 创建会话后确认 Pod 前缀、标签、命名空间
kubectl get pods -A -l <label-prefix>/type
# 确认 GC 仍能认领并回收
kubectl get pods -A | grep <pod-prefix>
```

---

## 4. Todo list tree

```
STORY-001 完成 troupe 品牌改名
│
├── 0. 决策（阻塞后续全部工作）
│   ├── [ ] D1 确定目标产品名（暂定 Troupe）
│   ├── [ ] D2 确定副标题（version.go:24 的 "AI Companion" 是否保留）
│   ├── [ ] D3 确定镜像 registry 与镜像名；确认镜像是否真实存在
│   ├── [ ] D4 确定 k8s 标签前缀保留 astonish.io 还是迁 troupe.io
│   └── [ ] D5 确定命名空间单复数（sandbox / sandboxes）
│
├── A. Track A — 产品文案层（1144 处 / 382 文件）
│   ├── A1. Go 代码内用户可见字符串（cmd/ 32 + pkg/ 422）
│   │   ├── [ ] A1.1 cmd/troupe/version.go:24 产品名与副标题
│   │   │        （实测补充：同文件 :30-36 的 `asciiLogo` 是拼作 **ASTONISH** 的
│   │   │          figlet 大字，`troupe --version` 每次都会打印 —— 该处漏统计，
│   │   │          需要一并换成 TROUPE 的字形。）
│   │   ├── [ ] A1.2 cmd/troupe/ 其余 CLI 帮助与错误文案（setup / code / chat /
│   │   │        tools / memory / platform_backup / browser_setup / sandbox）
│   │   ├── [ ] A1.3 pkg/mailer/templates.go 邀请邮件正文（4 处）
│   │   ├── [ ] A1.4 pkg/daemon/run.go:1731 守护进程启动日志
│   │   ├── [ ] A1.5 pkg/channels/telegram/telegram.go:547 机器人自我介绍
│   │   ├── [ ] A1.6 pkg/fleet/github_reporter.go:119 GitHub 评论署名
│   │   ├── [ ] A1.7 pkg/ 其余错误信息与注释中的产品名
│   │   └── [ ] A1.8 抽查 git diff，确认未误改 URL / 包名 / 版权段
│   │
│   ├── A2. 前端与扩展（web/ 76 + extension/ 18）
│   │   ├── [ ] A2.1 web/src/components/LoginPage.tsx（"Welcome to Astonish" 等）
│   │   ├── [ ] A2.2 web/src/components/settings/ChatSettings.tsx 系统提示词
│   │   ├── [ ] A2.3 web/src/components/settings/BrowserSettings.tsx
│   │   ├── [ ] A2.4 web/src/App.tsx:404 升级提示文案
│   │   ├── [ ] A2.5 web/src/ 其余 UI 文案
│   │   ├── [ ] A2.6 extension/ 清单与界面文案
│   │   └── [ ] A2.7 跑前端测试（vitest）
│   │
│   ├── A3. 文档（docs/ 522 处 / 112 文件）
│   │   ├── [ ] A3.1 README.md 叙述部分（保留版权与作者段）
│   │   ├── [ ] A3.2 CONTRIBUTING.md
│   │   ├── [ ] A3.3 docs/website/ 用户文档
│   │   ├── [ ] A3.4 docs/architecture/ 架构文档
│   │   └── [ ] A3.5 标注并跳过"历史叙述"例外（改名溯源句）
│   │
│   └── A4. 测试与回归（tests/ 21 处）
│       ├── [ ] A4.1 tests/ 内夹具字符串
│       ├── [ ] A4.2 全仓库校验：大写 Astonish 在代码目录内归零
│       └── [ ] A4.3 四关卡全跑（build / test / vet / lint）
│
├── B. Track B — 运行时与分发层
│   ├── B1. 镜像引用（依赖 D3）
│   │   ├── [ ] B1.1 确认目标 registry 与镜像名真实可拉
│   │   ├── [ ] B1.2 Makefile 构建/推送目标对齐
│   │   ├── [ ] B1.3 docker/backend.go:91 生产默认值
│   │   ├── [ ] B1.4 k8s/backend.go:252 生产默认值
│   │   ├── [ ] B1.5 openshell/backend.go:70 生产默认值
│   │   ├── [ ] B1.6 docker/ 下 Dockerfile 与注释
│   │   └── [ ] B1.7 本地创建一次沙箱会话，确认镜像真的拉得到
│   │
│   ├── B2. k8s 标签前缀统一（依赖 D4）
│   │   ├── [ ] B2.1 盘点集群内现有 Pod 使用的标签前缀
│   │   ├── [ ] B2.2 统一 pkg/sandbox/k8s/session.go:58-61 与 fleet.go:45
│   │   ├── [ ] B2.3 统一 cmd/troupe/sandbox.go 与 platform_sandbox_audit.go
│   │   ├── [ ] B2.4 更新一切标签选择器与 NetworkPolicy
│   │   └── [ ] B2.5 集群验证：kubectl get pods -l <prefix>/type
│   │
│   ├── B3. 命名空间单复数（依赖 D5）
│   │   ├── [ ] B3.1 pkg/sandbox/k8s/backend.go:227 生产默认值
│   │   ├── [ ] B3.2 pkg/config/app_config.go 注释
│   │   ├── [ ] B3.3 Makefile 的 platform-init 与 e2e 默认值
│   │   └── [ ] B3.4 文档同步
│   │
│   ├── B4. 原生库改名 libastonish → libtroupe
│   │   ├── [ ] B4.1 pkg/codeintel/native/Makefile:11 OUT
│   │   ├── [ ] B4.2 重建产物 + 更新 embed tarball
│   │   ├── [ ] B4.3 bindings.go:17 DefaultLibraryPath 与 :150 文件名搜索列表
│   │   ├── [ ] B4.4 docker/sandbox-base/Dockerfile 的 COPY 与 make 调用
│   │   ├── [ ] B4.5 .github/workflows/build.yml:63,68 断言
│   │   └── [ ] B4.6 验证 dlopen 成功（本地 + 容器内各一次）
│   │
│   ├── B5. Pod/容器名前缀（最高风险，最后做）
│   │   ├── [ ] B5.1 docker/backend.go:257 astonish-session-
│   │   ├── [ ] B5.2 docker/session.go:543 overlay 目录匹配
│   │   ├── [ ] B5.3 k8s/session.go:143 astn-sess-
│   │   ├── [ ] B5.4 k8s/fleet.go:50 astn-fleet-
│   │   ├── [ ] B5.5 k8s/template.go:68 astn-tmpl-
│   │   ├── [ ] B5.6 k8s/network.go:52 astn-org-
│   │   ├── [ ] B5.7 k8s/gc_reconciler.go 的 astn-gc-* / astn-upper-gc / astn-layer-gc
│   │   ├── [ ] B5.8 逐个核对 GC 的 HasPrefix 匹配逻辑与测试
│   │   └── [ ] B5.9 集群验证：建会话 → Pod 前缀正确 → GC 能回收
│   │
│   └── B6. 文档站与邮件 asset（需先有替代托管）
│       ├── [ ] B6.1 托管替代的文档站与 logo 图片
│       ├── [ ] B6.2 README.md:10,216,366 徽章与链接
│       ├── [ ] B6.3 web/src/components/UpgradeDialog.tsx:21 安装命令
│       └── [ ] B6.4 pkg/mailer/message.go:15 logoURL（改前确认新图可访问）
│
└── Z. 收尾
    ├── [ ] Z1. 更新 docs/architecture/runtime-distribution-plane.md 为"已完成"
    ├── [ ] Z2. 全量验证四关卡
    ├── [ ] Z3. 按阶段拆分提交（勿一次性大提交，便于二分回滚）
    └── [ ] Z4. 确认 STORY-001 各勾选项均有对应提交
```

---

## 附录：如何复现本故事的数据基线

```bash
# Track A — 大写产品名
grep -rn --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=dist \
  --exclude-dir=.vitepress --exclude='*.pb.go' -F 'Astonish' . | wc -l

# Track B — 运行时标识符
for p in 'ghcr.io/sap' 'sap.github.io' 'astonish.io/' 'troupe.io/' 'astn-' \
         'astonish-session' 'libastonish' 'astonish-sandbox'; do
  printf '%-22s %s\n' "$p" \
    "$(grep -rn --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=dist \
        --exclude='*.pb.go' -F "$p" . | wc -l)"
done
```

> 注意：本机 `grep` 不识别 BRE 的 `\|` 交替，多模式匹配请用 `grep -E`。
