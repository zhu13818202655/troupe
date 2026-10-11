# Choose Your Interface

Troupe provides multiple interfaces for different workflows and environments. Platform-based interfaces connect to the same platform and share sessions, memory, and flows. **Troupe Code** stands alone as a fully local coding tool that requires no platform.

## Interfaces

::: tip Recommended Starting Point
New to Troupe? Start with **Studio** for the full visual experience, or **Troupe Code** if you want a local AI pair programmer in the terminal right now — no setup beyond an API key.
:::

### Troupe Code (Local Coding Tool)

A fully local, in-process coding agent — like Claude Code, Codex, or OpenCode. Runs directly in your project directory with no daemon, no platform, and no login required. Tools execute with your own permissions on the host filesystem.

```bash
troupe code                          # Start coding in current directory
troupe code -m anthropic:claude-4    # Pin a specific model
troupe code -C ./my-project          # Operate in a different directory
troupe code --auto-approve           # Bypass authorization prompts
```

Features unique to code mode:
- **Plan mode** — codegraph-powered planning that uses a pre-computed knowledge graph for highly efficient, dependency-aware plans in a fraction of the tool calls
- **Ask mode** — research-only Q&A to explore architecture and discuss approaches without changing files
- **Rollback** — revert both conversation and file changes to any earlier point
- **Dual-backend switching** (`Ctrl+\`) — switch to platform chat without leaving the TUI
- **AGENTS.md project guidance** — loads project conventions automatically
- **Per-project sessions** — each directory keeps its own conversation history

Best for: coding tasks, refactoring, local development, developers who want an AI pair programmer in the terminal.

See [Code Mode documentation](../cli/code.md) for full details.

### Studio (Web UI)

The full visual interface running at `http://localhost:9393`. Includes Chat, a visual flow designer, generative applications, [Slides](../slides/) generation and presentation, settings management, and real-time agent execution display with token tracking. Studio is served automatically by the daemon — just open `http://localhost:9393` in your browser.

Best for: creating and presenting slide decks, flow design, generative UI, managing apps and settings, and visual execution monitoring.

<!-- IMAGE: Studio interface showing chat, Slides, flow designer, and apps -->

### CLI (Platform Chat)

A rich terminal chat interface with colors, markdown rendering, and interactive elements. Supports all agent capabilities including tool use, memory, and flow execution. Requires authentication via `troupe login` before use.

```bash
troupe login http://localhost:9393    # Authenticate against the platform
troupe chat                           # New session
troupe chat -p openai -m gpt-4o      # Specific provider/model
troupe chat --resume                  # Resume last session
```

Best for: platform-backed interactions, team memory, sandboxed execution, developers who prefer the terminal.

### Remote CLI

The same CLI used for local access, pointed at a remote server. Authenticates via password or SSO, then provides the full CLI experience against the remote platform.

```bash
troupe login https://platform.yourcompany.com
troupe chat
troupe flows list
troupe status
```

Best for: team members accessing the shared platform remotely, CI/CD integration.

### Telegram

Bot integration for mobile and desktop access. Supports database-backed allowlists and dynamic per-message routing in cloud deployments. Switch org and team context with in-channel commands.

Best for: quick questions on mobile, notifications, async interactions.

### Email

Send messages to the agent via email. Supports plus-addressing for per-org routing (`bot+orgname@domain.com`). Responses are delivered back to the sender.

Best for: async workflows, forwarding content for processing, users who prefer email.

### Slack

Workspace integration with team-scoped routing. Messages route to the correct org and team context based on the Slack workspace and channel configuration.

Best for: team collaboration, integrating agent responses into existing Slack workflows.

## Comparison

| Interface | Deployment | Real-time | Visual | Mobile | Requires Platform |
|-----------|-----------|-----------|--------|--------|-------------------|
| Code | Local only | Yes | No | No | No |
| Studio | Local / Cloud | Yes | Yes | No | Yes |
| CLI | Local / Cloud | Yes | No | No | Yes |
| Remote CLI | Cloud only | Yes | No | No | Yes |
| Telegram | Local / Cloud | Yes | No | Yes | Yes |
| Email | Local / Cloud | No (async) | No | Yes | Yes |
| Slack | Cloud only | Yes | No | Yes | Yes |

## Running Multiple Interfaces

Interfaces are not mutually exclusive. You can run Studio for visual work while using the CLI for quick tasks, and have Telegram configured for mobile access. All platform-based interfaces share the same sessions, memory, and platform context.

**Troupe Code** is standalone — it does not require the daemon and can run alongside any other interface. When logged in to a platform, press `Ctrl+\` inside code mode to switch to the platform chat panel without leaving the TUI.

The daemon (`troupe daemon start`) must be running for platform-based interfaces (Studio, CLI, Remote CLI, Channels). The CLI authenticates against the daemon via `troupe login`.