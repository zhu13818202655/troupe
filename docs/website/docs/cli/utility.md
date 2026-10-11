# Utility Commands

Miscellaneous commands for setup, status, and system management.

## `troupe setup`

Interactive first-run wizard that configures your Troupe installation:

```bash
troupe setup
```

Walks you through:
- Selecting a storage backend (SQLite or PostgreSQL)
- Selecting an AI provider and entering API keys
- Choosing a default model
- Configuring web search tools
- Setting up browser automation
- Initializing container sandboxes (optional)

## `troupe status`

Display the current state of all Troupe subsystems:

```bash
troupe status
```

Shows provider configuration, daemon status, MCP servers, memory state, and more.

## `troupe --version`

Print version and build information:

```bash
troupe --version
# or
troupe -v
```

::: info
This is a flag, not a subcommand. Use `--version` or `-v`.
:::

## `troupe login`

Authenticate with a remote platform instance:

```bash
troupe login
```

Used when connecting the CLI to a cloud-deployed Troupe platform. After login, commands like `chat`, `flows`, and `scheduler` operate against the remote server.

## `troupe logout`

Disconnect from the remote platform:

```bash
troupe logout
```

## `troupe daemon`

Manage the background daemon service and Studio web UI (local-only):

```bash
# Install as a system service (auto-starts on login)
troupe daemon install

# Start / stop / restart the service
troupe daemon start
troupe daemon stop
troupe daemon restart

# Check service status
troupe daemon status

# Run in the foreground (starts the Studio HTTP server on :9393)
troupe daemon run

# View logs
troupe daemon logs

# Uninstall the service
troupe daemon uninstall
```

`troupe daemon run` starts the Studio web interface at `http://localhost:9393`. The installed service runs this automatically in the background on login.

## `troupe config`

Manage configuration:

```bash
troupe config
```

## `troupe tools`

Manage MCP servers and tools:

```bash
# List all available tools
troupe tools list

# List MCP servers
troupe tools servers

# Refresh tool cache
troupe tools refresh

# Enable/disable an MCP server
troupe tools enable <name>
troupe tools disable <name>

# Edit MCP configuration
troupe tools edit

# Browse MCP server store
troupe tools store

# Search tools
troupe tools search <query>
```

## `troupe sessions`

Manage chat sessions:

```bash
troupe sessions
```

## `troupe credentials`

Manage the encrypted credential store (local-only):

```bash
troupe credentials
```

## `troupe sandbox`

Manage container sandboxes (local-only):

```bash
troupe sandbox
```

## Deprecated Commands

### `troupe memory`

::: warning Deprecated
The `memory` CLI command is no longer available. Memory is managed through the agent's built-in memory tools during chat sessions, or via Studio Settings.
:::

See [Studio Overview](../studio/) for details on the web interface.
