# Runtime & Distribution Plane

> **Status:** Inventory complete — decisions pending  
> **Scope:** Container images, binary name, namespaces, container/pod name prefixes,  
> Kubernetes label prefixes, the native tree-sitter library, and the docs-site /  
> email assets.  
> **Context:** The Helm chart (`deploy/helm`) and `deploy/k8s` were removed on  
> 2026-10-07. Deployment assets will be rebuilt against our own requirements  
> (see §3).

## 1. Why this plane cannot be find-and-replaced

The `SAP/astonish` → `zhu13818202655/troupe` rename was a **text-level** change:  
URLs, comments, test fixtures. Every value in this document is different — it is  
**executed**:

| Value                       | Consumed by                                             |
| --------------------------- | ------------------------------------------------------- |
| Container image reference   | containerd / kubelet `imagePull`                        |
| Binary name                 | what users type; what a package manager installs        |
| Namespace                   | created by the API server; RBAC / PVCs are scoped to it |
| Container / pod name prefix | GC reconcilers match on it; operators grep it           |
| Label prefix                | label selectors, NetworkPolicies, GC sweeps             |
| Native library path         | `dlopen` at runtime                                     |

So unlike the GitHub-reference cleanup, changing any of these changes **behaviour**.  
A blind find-and-replace is a breaking change, not a cleanup.

## 2. Current state — the plane is internally inconsistent

The rename was applied to some layers and not others. Several values below are  
currently **inconsistent with each other**, which is worth resolving before adding  
new deployment tooling.

### 2.1 Container images — build output does not match the runtime default

`Makefile:15` sets `DOCKER_REGISTRY ?= ghcr.io/sap`, and the build targets push  
**`troupe`-named** images:

| Artefact                     | Makefile builds & pushes                                                     | Go runtime default                              |
| ---------------------------- | ---------------------------------------------------------------------------- | ----------------------------------------------- |
| control plane (api + worker) | `$(DOCKER_REGISTRY)/troupe` (`Makefile:611-694`)                             | —                                               |
| sandbox base                 | `$(DOCKER_REGISTRY)/troupe-sandbox-base` (`Makefile:590`, `626`, `677`)      | `ghcr.io/sap/astonish-sandbox-base:latest`      |
| sandbox OpenShell            | `$(DOCKER_REGISTRY)/troupe-sandbox-openshell` (`Makefile:597`, `638`, `689`) | `ghcr.io/sap/astonish-sandbox-openshell:latest` |

The Go defaults that actually drive pulls:

- `pkg/sandbox/docker/backend.go:91` — `ghcr.io/sap/astonish-sandbox-base:latest`
- `pkg/sandbox/k8s/backend.go:252` — `ghcr.io/sap/astonish-sandbox-base:latest`
- `pkg/sandbox/openshell/backend.go:70` — `ghcr.io/sap/astonish-sandbox-openshell:latest`

**Consequence:** `make docker-sandbox-base` produces `ghcr.io/sap/troupe-sandbox-base`,  
but a sandbox created with default config tries to pull  
`ghcr.io/sap/astonish-sandbox-base:latest`. Locally built images are never used  
unless `sandbox.image` is overridden explicitly.

> Note there are **two independent axes** here: the registry (`ghcr.io/sap` →  
> a registry we control) and the image name (`astonish-*` → `troupe-*`). They can  
> be decided separately.

### 2.2 Binary name

| Source                            | Value                                       |
| --------------------------------- | ------------------------------------------- |
| `Makefile:12` (`BINARY_NAME`)     | `troupe`                                    |
| `.github/workflows/build.yml:54`  | `go build -v -o astonish .`                 |
| `cmd/troupe/root.go` upgrade hint | `brew upgrade zhu13818202655/troupe/troupe` |

`make build` produces `./troupe`; CI produces `./astonish`. Same mismatch family  
as §2.1.

### 2.3 Namespaces

| Consumer                          | Namespace                                             |
| --------------------------------- | ----------------------------------------------------- |
| Go k8s backend default            | `troupe-sandboxes` (`pkg/sandbox/k8s/backend.go:227`) |
| `Makefile` → `make platform-init` | `-n troupe` (`Makefile:531`)                          |
| e2e preflight defaults            | `troupe` (control plane), `troupe-sandbox` (sandbox)  |

The Helm chart previously reconciled these via `namespaces.prefix`; with the chart  
gone, the namespaces must be provisioned by whatever replaces it. Note the Go  
default is `troupe-sandboxes` (plural) while the Makefile/e2e defaults use  
`troupe-sandbox` (singular) — pick one.

### 2.4 Container and pod name prefixes

| Prefix                                          | Location                                                                 | Notes                       |
| ----------------------------------------------- | ------------------------------------------------------------------------ | --------------------------- |
| `astonish-session-`                             | `pkg/sandbox/docker/backend.go:257`, `pkg/sandbox/docker/session.go:543` | Docker container names      |
| `astn-sess-`                                    | `pkg/sandbox/k8s/session.go:143`                                         | k8s session pods            |
| `astn-fleet-`                                   | `pkg/sandbox/k8s/fleet.go:50`                                            | fleet pods                  |
| `astn-tmpl-`                                    | `pkg/sandbox/k8s/template.go:68`                                         | template-builder pods       |
| `astn-org-`                                     | `pkg/sandbox/k8s/network.go:52`                                          | per-org NetworkPolicy names |
| `astn-gc-*`, `astn-upper-gc-`, `astn-layer-gc-` | `pkg/sandbox/k8s/gc_reconciler.go`, `session.go:802`, `template.go:289`  | GC pods                     |

**This is the highest-risk group.** The GC reconcilers match these prefixes with  
`strings.HasPrefix`, and overlay directories are named from session IDs. Renaming  
a prefix without updating every matcher silently orphans cluster resources. Treat  
as a separate, well-tested change — not part of a bulk rename.

### 2.5 Label prefixes

- `astonish.io/...` — the vast majority (76 occurrences); the operative constants  
  are in `pkg/sandbox/k8s/session.go:58-61` and `fleet.go:45`.  
  `pkg/sandbox/k8s/session.go:55` explicitly calls this a **public contract with  
  operators** (`kubectl get pods -l astonish.io/...`).
- `troupe.io/...` — a smaller, newer set (6 occurrences):  
  `cmd/troupe/sandbox.go:841,881,963` (`troupe.io/purpose`),  
  `cmd/troupe/platform_sandbox_audit.go:350,485` (`troupe.io/type`),  
  `cmd/troupe/sandbox_k8s_smoke.go:21` (doc comment).

Because the two prefixes coexist, any selector that mixes them will not match.  
**Verify which prefix each selector/GC sweep actually looks for before changing  
either side** — this needs a code read of the GC and template-builder paths, not  
a grep.

### 2.6 Native tree-sitter library

| Aspect               | Value                                         | Location                                                                    |
| -------------------- | --------------------------------------------- | --------------------------------------------------------------------------- |
| Artefact name        | `libastonish-treesitter.so` / `.dylib`        | `pkg/codeintel/native/Makefile:11`, `pkg/codeintel/native/builder.go:39-41` |
| Default install path | `/usr/lib/astonish/libastonish-treesitter.so` | `pkg/codeintel/internal/treesitter/bindings.go:17`                          |
| Runtime override     | `TROUPE_TREESITTER_LIB`                       | `pkg/codeintel/internal/treesitter/bindings.go:150`                         |
| CI assertions        | `libastonish-treesitter.so`                   | `.github/workflows/build.yml:63,68`                                         |
| Image build          | `make -C pkg/codeintel/native`                | `docker/sandbox-base/Dockerfile:45`                                         |

Note the env var was already renamed to `TROUPE_TREESITTER_LIB` while the artefact  
and directory kept the old name. A rename must change the file name, the embedded  
tarball, the `COPY` in the image, the default path, and the CI assertion **in one  
commit** — the bindings resolve by filename and will fail at `dlopen` otherwise.

### 2.7 Docs site and email assets

| Location                                  | Reference                                                      |
| ----------------------------------------- | -------------------------------------------------------------- |
| `README.md:10,216,366`                    | `https://sap.github.io/astonish/` (badges, links)              |
| `web/src/components/UpgradeDialog.tsx:21` | `curl -sSL https://sap.github.io/astonish/install.sh \| bash`  |
| `pkg/mailer/message.go:15`                | `logoURL = "https://sap.github.io/astonish/astonish-logo.png"` |

`pkg/mailer/message.go` is a **runtime** reference: every outgoing email embeds  
that image. Changing or removing it without hosting a replacement yields broken  
images in mail.

`web/src/components/AstonishLogo.tsx` uses locally served `/astonish-logo-*.svg`  
assets, so the in-app logo is self-hosted and low-risk.

## 3. What the 2026-10-07 cleanup removed

Removed:

- `deploy/helm/troupe/` — 32 files (chart, templates, 4 values files)
- `deploy/k8s/README.md` — 1 file (already-deprecated pointer to the chart)
- `pkg/sandbox/k8s/manifest_deploy_test.go` and `manifest_dryrun_test.go` —  
  chart-rendering validation tests (dead once the chart is gone, and already  
  pointing at the pre-rename `deploy/helm/astonish`)
- Makefile targets that existed only to install the chart:  
  `e2e-k8s-up`, `e2e-k8s-down`, `e2e-openshell-up`, `e2e-openshell-down`,  
  `helm-deps` — plus the now-dead `E2E_K8S_*` variables, help lines, `.PHONY`  
  entries, and the hint messages that told operators to run them
- Dead ignore rules in `.gitignore` and `.dockerignore`

Kept deliberately:

- `E2E_OS_*` variables — still used by `make test-e2e-openshell`
- `make test-e2e`, `make test-e2e-sqlite`, `make test-e2e-inspect` — they now  
  expect a **manually provisioned** cluster and say so in their error messages
- `docker/` image definitions — independent of the chart

**Consequence:** there is currently no supported way to provision a cluster. The  
e2e suites that need Kubernetes will fail their preflight until deployment tooling  
is rebuilt.

## 4. Decisions required before touching this plane

1. **Image registry and names.** Do we publish our own images at all? If yes:  
   which registry, and do we keep the `-sandbox-base` / `-sandbox-openshell`  
   suffixes? This single decision unblocks §2.1 and §2.2.
2. **Binary name.** `troupe` everywhere (matching `Makefile:12`), and update CI.
3. **Namespace.** `troupe-sandbox` (singular, matching the Makefile/e2e defaults)  
   or `troupe-sandboxes` (plural, matching the current Go default) — pick one and  
   make all consumers agree.
4. **Label prefix.** Whether `astonish.io/*` stays (it is a documented operator  
   contract, so keeping it is defensible) or migrates to `troupe.io/*` with a  
   transition period. Do **not** mix.
5. **Name prefixes** (§2.4). Decide last; requires GC-aware testing.
6. **Docs site.** Whether `sap.github.io/astonish` keeps being referenced or a  
   replacement is hosted before the references change.

## 5. Suggested staging

| Phase | Change                                                                          | Why this order                                                                |
| ----- | ------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| 0     | Decide §4                                                                       | Everything below depends on the registry/name choice                          |
| 1     | Build & publish images; update `Makefile` + the 3 Go defaults **in one commit** | Fixes the current build/pull mismatch (§2.1)                                  |
| 2     | Binary name: `Makefile` + `.github/workflows/build.yml`                         | Purely build-side, low risk                                                   |
| 3     | Namespace unification                                                           | Touches config + e2e defaults; no runtime data migration if no clusters exist |
| 4     | Docs site / email assets                                                        | Needs a hosting decision first                                                |
| 5     | Label prefix                                                                    | Operator-visible contract; needs a migration note                             |
| 6     | Container/pod name prefixes                                                     | Highest risk — GC reconcilers, one prefix per commit, each with tests         |

Each phase should be a separate commit so a regression can be bisected.

## 6. Verification

```bash
# Build and unit tests
go build ./...
go test ./pkg/sandbox/...

# Inventory of everything still on the old naming (should trend to zero)
grep -rn --exclude-dir=.git --exclude-dir=node_modules -E "ghcr\.io/sap|astonish-session|astn-|libastonish|astonish\.io/" .

# Prove the runtime default matches what the build produces
make docker-sandbox-base
grep -rn "SandboxImage = \"ghcr" pkg/sandbox/*/backend.go
```


````

For phases 3–6, verify against a real cluster: create a session, confirm pods
appear with the expected prefix/labels/namespace, and confirm the GC reconciler
still finds and reaps them.

## 7. Rollback

All changes in this plane are plain file edits under version control; there is no
runtime state to migrate (no clusters are currently deployed from this repo). A
`git revert` of the offending commit is sufficient. The one exception is a
published container image or namespace rename — those leave artefacts behind and
should be staged behind an alias where possible.

## 8. Appendix — how this inventory was produced

```bash
# Images
grep -rn --exclude-dir=.git --exclude-dir=node_modules "ghcr.io/sap" .

# Namespaces / prefixes / labels
grep -rnE --exclude-dir=.git --exclude-dir=node_modules \
  "astonish-sandboxes|astonish-session|astn-|astonish\.io/|troupe\.io/" .

# Docs site / email
grep -rn --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=dist \
  "sap.github.io" .

# Native library
grep -rn --exclude-dir=.git "libastonish" .
````

> `grep` on this host does not honour BRE `\|` alternation; use `grep -E`.
