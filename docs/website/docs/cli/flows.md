# Flow Commands

The `troupe flows` command manages agent flow pipelines from the command line.

## Usage

```bash
troupe flows <subcommand> [flags]
```

## Subcommands

### List Flows

```bash
troupe flows list
```

### Run a Flow

```bash
# Run a flow by name
troupe flows run <flow-name>

# Run with parameters
troupe flows run my-flow -p file=data.csv -p format=json

# Run with a specific model
troupe flows run my-flow --provider openai --model gpt-4o

# Auto-approve all tool calls
troupe flows run my-flow --auto-approve
```

### Show Flow Structure

```bash
# Visualize a flow's node structure
troupe flows show <flow-name>
```

### Edit a Flow

```bash
# Open flow YAML file for editing
troupe flows edit <flow-name>
```

### Import a Flow

```bash
# Import a flow from a local YAML file
troupe flows import <path-to-flow.yaml>
```

### Remove a Flow

```bash
troupe flows remove <flow-name>
```

### Flow Store

Browse and install flows from configured taps:

```bash
# List available flows from all taps
troupe flows store list

# Search for flows
troupe flows store search <query>

# Install a flow from a tap
troupe flows store install <tap-name>/<flow-name>

# Uninstall a tap flow
troupe flows store uninstall <tap-name>/<flow-name>

# Update all tap manifests
troupe flows store update
```

## Flags for `run`

| Flag | Short | Description |
|------|-------|-------------|
| `--provider` | | AI provider to use |
| `--model` | | Model name |
| `-p` | | Parameter in `key=value` format (repeatable) |
| `--auto-approve` | | Auto-approve all tool executions |
| `--browser` | | Launch with embedded web browser UI |
| `--port` | | Port for web server (with --browser, default: 8080) |
| `--debug` | | Enable debug mode |

## Scheduling

Flows can be scheduled for recurring execution. Ask the agent to schedule a flow, or manage existing schedules with the [scheduler](./daemon-scheduler.md).
