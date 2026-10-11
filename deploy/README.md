# Troupe — 本地 Docker Compose 部署（Apple Silicon Mac）

一页纸操作手册。设计取舍与背景见
[`../.github/story/002-mac-local-compose-deployment.md`](../.github/story/002-mac-local-compose-deployment.md)。

---

## 组成

| 服务 | 镜像 | 作用 |
|---|---|---|
| `postgres` | `pgvector/pgvector:pg17` | 平台 + 每组织数据库（**必须带 pgvector**） |
| `troupe` | 本地构建，`deploy/Dockerfile` | 单二进制：Studio UI + REST/SSE API |
| `nginx` | `nginx:1.27-alpine` | 反向代理，入口 `:8080`；SSE 需要 `proxy_buffering off` |

访问入口：<http://localhost:8080>

---

## 首次启动

```bash
cd deploy
cp env.example .env                          # 已有 .env 则跳过
mkdir -p state/config state/postgres nginx/certs
cp config.example.yaml state/config/config.yaml
$EDITOR state/config/config.yaml             # 至少改 providers.<name>.base_url / default_model
docker compose up -d --build
```

必须在 `.env` 里填两个密钥（否则每次重启都会失效）：

```bash
openssl rand -hex 32     # -> TROUPE_MASTER_KEY
openssl rand -hex 32     # -> TROUPE_JWT_SECRET
```

> **`deploy/` 是唯一的启动入口。** 根目录**不再**保留 `docker-compose.yml`
> （曾经有一个，但它引用不存在的 `docker/troupe/Dockerfile`、跑不存在的
> `studio` 子命令、还挂载了 rebrand 前的配置路径，已删除）。
> 所有命令都在 `deploy/` 下执行，project name 为 `troupe-local`。

---

## 接入自建 OpenAI 兼容模型

`state/config/config.yaml` 每项字段的详细说明见文件内注释，要点：

```yaml
general:
  default_provider: local-llm
  default_model: <你的模型 id>
  context_length: 32768          # 必须显式设置，否则按 200000 估算

providers:
  local-llm:
    type: openai_compat
    api_key: "EMPTY"             # 不能为空；本地推理服务会忽略它
    base_url: "http://host.docker.internal:11434/v1"
```

- `base_url` 会自动补齐 `/v1`，并剔除误粘贴的 `/chat/completions`、`/models` 后缀。
- 容器内访问宿主机用 `host.docker.internal`；访问 compose 网络内其他服务用服务名。
- `memory.embedding` 若指向不可用的 embedding 端点，**语义检索会失败**（不影响启动，
  但相关功能报错）。不确定就先注释掉整块，走默认本地 embedder。

改完重启：`docker compose restart troupe`

> 平台模式下 provider 也可在 **Settings → Providers** 里配置，写入数据库；
> 同名时数据库的值优先于 `config.yaml`。

---

## 验证

```bash
cd deploy
./scripts/smoke.sh                    # 容器 / 健康检查 / SPA / nginx 缓冲
./scripts/init-db.sh                  # 24 张表 + pgvector 是否齐备
docker compose logs -f troupe
```

Web 端：打开 <http://localhost:8080>，注册第一个用户 ——
**首个注册用户会跳过邮箱验证，并成为组织 owner + 平台超级管理员**。

---

## 排错

**1. 只建了 10 张表，缺 `users` / `organizations`**
平台库必须在**不存在**的前提下由守护进程自建；预先建库只会创建 10 张白名单表。
`./scripts/init-db.sh` 会检出并打印恢复命令（会清空平台库）。

**2. `CREATE EXTENSION vector` 失败**
镜像不是 pgvector。换成 `pgvector/pgvector:pg17`，普通 `postgres:17` 没有该扩展。

**3. 构建期网络问题 —— 有「三个」互相独立的通道，别混为一谈**

镜像、apt、npm 走的是完全不同的网络路径，**一个通不代表另一个通**：

| 通道 | 现象 | 解决 |
|---|---|---|
| 镜像拉取 | `failed to resolve source metadata` / `context deadline exceeded` | Docker Desktop → *Settings → Docker Engine* 加 `"registry-mirrors": ["https://docker.m.daocloud.io"]`，Apply & Restart |
| apt（构建期） | `E: Failed to fetch http://deb.debian.org/… Connection failed [IP: …]` | `deploy/.env` 里设 `APT_MIRROR=http://mirrors.aliyun.com` |
| npm（构建期） | `npm error code ECONNRESET` / `socket disconnected before secure TLS connection` | `deploy/.env` 里设 `NPM_REGISTRY=https://registry.npmmirror.com` |

两个构建期变量的取值分别注意：

- `APT_MIRROR` **必须是 `http://` 而非 `https://`**：`ca-certificates` 正是由同一个
  `RUN` 步骤安装的，抓包时镜像里还没有信任库，走 TLS 一定失败
  （bookworm-slim 基础镜像确认不带 `ca-certificates.crt`）。
- `NPM_REGISTRY` 用 `https://` 是安全的：node 基础镜像自带 CA 证书。

另注意 Docker Desktop 跑在虚拟机里，宿主机 VPN 不一定覆盖它的流量，
需要在 *Settings → Resources → Proxies* 里单独配。

**4. SSE 聊天流卡住 / 不逐字返回**
确认走的是 nginx（`:8080`）而不是直连 `:9393`，且 nginx 内 `proxy_buffering off`。

**5. 沙箱会话起不来**
`TROUPE_SANDBOX_IMAGE` 默认指向一个 ghcr 镜像。需本地构建：
`make sandbox-entrypoint build-linux-arm64 docker-sandbox-base`。
沙箱不可用时普通聊天不受影响。

---

## 沙箱说明

`compose.yaml` 挂了 `/var/run/docker.sock`（DooD），因为沙箱默认开启。
会话容器默认以降权方式运行。不需要容器隔离时把 `config.yaml` 的
`sandbox.enabled` 置为 `false`，并可移除该挂载。
