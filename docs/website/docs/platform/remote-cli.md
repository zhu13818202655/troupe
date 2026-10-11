# Remote CLI

The same `troupe` binary works both locally and against a remote platform server. After logging in, CLI commands execute in your platform context with access to shared memory, team resources, and cascading configuration.

## Logging In

```bash
# Authenticate with a platform server
troupe login https://troupe.acme.corp
```

This prompts for your email and password interactively. For SSO-enabled or OAuth-based environments:

```bash
# OAuth login (opens browser for sign-in)
troupe login https://troupe.acme.corp --sso
```

The `--sso` flag initiates an **OAuth Authorization Code + PKCE** flow: the CLI starts a temporary loopback HTTP server on `127.0.0.1`, opens your browser to the Troupe authorization endpoint, and receives the authorization code on callback. No device code or manual code entry is required — sign-in completes automatically when the browser redirects back.

The CLI requests only the scopes it needs: `openid`, `offline_access`, and `chat`. It does not request `tool:execute` or other privileged scopes.

You can also pre-select your org and team to skip interactive prompts:

```bash
troupe login https://troupe.acme.corp --org acme --team backend
```

Credentials are stored locally in `~/.config/troupe/remote.yaml`. If a stale login is detected, `troupe login` automatically replaces the old credentials without requiring a manual `troupe logout` first.

## Checking Status

```bash
troupe status
```

Shows your current connection info: server URL, user, org, and team.

## Selecting Org and Team

Org and team context is set during login. To switch, log out and log back in:

```bash
troupe logout
troupe login https://troupe.acme.corp --org acme --team frontend
```

If you omit `--org` and `--team`, the login flow will prompt you to select from available options interactively.

## Available Commands in Remote Mode

Once logged in, these commands work against the platform:

```bash
# Chat (session stored on the platform)
troupe chat
troupe chat --resume <session-id>

# Sessions
troupe sessions list
troupe sessions show <session-id>
troupe sessions delete <session-id>

# Flows
troupe flows list
troupe flows run <name>
troupe flows show <name>

# Team and org context
troupe team list
troupe org list
troupe status
```

## Configuration

Remote CLI settings are stored in `~/.config/troupe/remote.yaml`:

```yaml
url: https://troupe.acme.corp
org: acme
team: backend
user_email: alice@acme.corp
```

This file is created automatically by `troupe login` and removed by `troupe logout`.

## Logging Out

```bash
troupe logout
```

This removes the `remote.yaml` file and revokes the stored session.

## Next Steps

- [Platform Overview](./index) — understanding the platform architecture
- [Organizations & Teams](./organizations-and-teams) — the org/team context model
- [Authentication](../security/authentication) — full details on OAuth, OIDC, and token lifecycle
