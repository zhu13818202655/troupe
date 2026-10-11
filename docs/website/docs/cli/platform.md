# Platform Commands

The `troupe platform` command manages multi-tenant platform operations — database initialization, organization management, and user administration.

## Usage

```bash
troupe platform <subcommand> [flags]
```

## Subcommands

### Initialize Platform Database

```bash
troupe platform init \
  --host <postgres-host> \
  --password <postgres-admin-password>
```

This creates the platform database and runs migrations. It prints the connection DSN for use in Helm values or config.

| Flag | Default | Env Fallback | Description |
|------|---------|--------------|-------------|
| `--host` | (required) | `PGHOST` | PostgreSQL hostname |
| `--port` | `5432` | `PGPORT` | PostgreSQL port |
| `--user` | `postgres` | `PGUSER` | PostgreSQL admin user |
| `--password` | (required) | `PGPASSWORD` | PostgreSQL admin password |
| `--sslmode` | `prefer` | `PGSSLMODE` | SSL mode |
| `--suffix` | auto-generated | — | Fixed instance suffix |

### Generate Secret

```bash
troupe platform gen-secret
```

Generates a cryptographically secure random secret (64-character hex string) suitable for use as `masterKey` or `jwtSecret`.

### Platform Status

```bash
troupe platform status
```

Displays organization count, user count, lists organizations with team counts, and shows the PostgreSQL version.

### Organization Management

```bash
# Create a new organization
troupe platform org create --name "Acme Corp" --slug acme-corp

# Create with an existing user as owner
troupe platform org create --name "Acme Corp" --slug acme-corp --owner-email admin@acme.com

# List all organizations
troupe platform org list

# Invite a user to an organization
troupe platform org invite \
  --org acme-corp \
  --email user@acme.com \
  --role member

# Invite with password prompt (instead of generated password)
troupe platform org invite \
  --org acme-corp \
  --email admin@acme.com \
  --role owner \
  --password
```

#### `org create` Flags

| Flag | Description |
|------|-------------|
| `--name` | Organization display name (required) |
| `--slug` | URL-safe identifier (required) |
| `--owner-email` | Set an existing user as org owner |

#### `org invite` Flags

| Flag | Description |
|------|-------------|
| `--org` | Organization slug (required) |
| `--email` | User's email address (required) |
| `--role` | Role: `owner`, `admin`, `member` (default: member) |
| `--name` | Display name for new users |
| `--team` | Also add to this team (default: general) |
| `--password` | Prompt for password instead of generating one |

### User Management

```bash
# List all users (optionally filter by org)
troupe platform user list [--org <slug>]

# Show user details
troupe platform user show <email>

# Delete a user
troupe platform user delete <email>

# Set a user's password interactively
troupe platform user set-password <email>

# Disable/enable a user account
troupe platform user disable <email>
troupe platform user enable <email>

# Promote/demote platform superadmin
troupe platform user promote <email>
troupe platform user demote <email>
```

### Issue Access Token

```bash
# Interactive browser-based login
troupe platform issue-token --server https://troupe.example.com --sso

# Direct password authentication
troupe platform issue-token \
  --server https://troupe.example.com \
  --email admin@example.com \
  --password

# Output as JSON
troupe platform issue-token --server https://troupe.example.com --json
```

### Sandbox Audit

```bash
troupe platform sandbox-audit
```

Audits sandbox PVCs for orphaned data.

## Roles

| Role | Permissions |
|------|-------------|
| `owner` | Full org control, manage all members and teams |
| `admin` | Manage members, teams, settings |
| `member` | Use agents, view team resources |
