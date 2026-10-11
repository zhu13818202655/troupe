# Daemon & Scheduler

The daemon runs background services (Studio UI, channel listeners, scheduled flows), and the scheduler manages recurring tasks.

## Daemon

The daemon is a long-running background process that serves Studio at `http://localhost:9393` and powers channels, scheduled flows, and other services.

### Commands

```bash
# Install daemon as a system service
troupe daemon install [--port 9393]

# Uninstall the system service
troupe daemon uninstall

# Start the daemon
troupe daemon start

# Stop the daemon
troupe daemon stop

# Restart the daemon
troupe daemon restart

# Check daemon status
troupe daemon status

# Run in foreground (for debugging)
troupe daemon run [--port 9393]

# View daemon logs
troupe daemon logs [-f] [-n 50]
```

### Log Flags

| Flag | Description |
|------|-------------|
| `-f` | Follow log output (tail) |
| `-n` | Number of lines to show (default: 50) |

## Scheduler

The scheduler executes flows on recurring schedules. It runs within the daemon process.

::: info
Jobs are created through chat — ask the AI to schedule a task and it will create the job for you. The CLI is for managing existing jobs.
:::

### Commands

```bash
# List all scheduled jobs
troupe scheduler list

# Show scheduler status
troupe scheduler status

# Enable a disabled job
troupe scheduler enable <name>

# Disable a job (pauses execution)
troupe scheduler disable <name>

# Trigger immediate execution of a job
troupe scheduler run <name>

# Remove a scheduled job
troupe scheduler remove <name>
```

The `<name>` argument can be a full job ID, a partial ID prefix (must be unambiguous), or a job name (case-insensitive).

### Aliases

- `scheduler list` → `scheduler ls`
- `scheduler remove` → `scheduler rm`

### Examples

```bash
# List all jobs
troupe scheduler ls

# Trigger a job manually
troupe scheduler run daily-report

# Disable a job temporarily
troupe scheduler disable health-check

# Re-enable it
troupe scheduler enable health-check

# Remove a job permanently
troupe scheduler rm old-job
```
