# Backup & Recovery

Troupe backup archives are designed for two jobs:

1. Recover platform data after an operator error or infrastructure failure.
2. Port data from one deployment location or backend to another.

The current implementation supports the archive format, SQLite and PostgreSQL logical data export, archive verification, passphrase-encrypted archives, and logical restore into clean SQLite or PostgreSQL targets. SQLite targets can also be explicitly reset before restore.

## Archive Files

A backup archive uses the `.troupe-backup` extension. It is one user-facing file, gzip-compressed by default, with multiple logical files inside. It contains:

- `manifest.json` — archive metadata, source backend, exported scopes, schema versions, and payload entries.
- `checksums.json` — SHA-256 checksums for every payload file.
- Payload files — logical JSONL data for exported entities. Backups export data, not physical database files.

Archive verification checks the manifest, checksum file, compressed stream, and payload integrity before any restore workflow trusts the archive. Checksums are calculated over the uncompressed logical payload bytes.

## Create a Logical Backup Archive

```bash
troupe platform backup create --output backup.troupe-backup
```

The default archive is gzip-compressed. To create an uncompressed tar-compatible archive for debugging, use:

```bash
troupe platform backup create --output backup.troupe-backup --compression none
```

For SQLite and PostgreSQL deployments, the current `create` command exports logical JSONL rows across platform, org, team, and personal scopes. This includes durable rows for:

- Flows.
- Apps and app state.
- Fleet templates, plans, setup data, and runtime tables.
- Drill reports.
- Sessions and session events.
- Memories.
- Settings and MCP servers.
- Credentials and scheduled jobs.
- Users, organizations, teams, and memberships.

The archive does not contain physical `.db` files. Recovery backups preserve stored protected values by default. Add `--redact-secrets` only when creating a portable/support export that should not be used for full recovery. Add `--passphrase <secret>` to encrypt the complete archive file, including manifest and checksums. The command requires `storage.backend: sqlite` or `storage.backend: postgres`; empty/legacy backend values are treated as SQLite zero-config platform mode.

## Inspect an Archive

```bash
troupe platform backup inspect backup.troupe-backup
```

Use JSON output for scripts:

```bash
troupe platform backup inspect backup.troupe-backup --json
troupe platform backup inspect encrypted.troupe-backup --passphrase "$BACKUP_PASSPHRASE"
```

Inspection reads the manifest and checksum metadata. It also shows whether the archive is passphrase-encrypted. It does not prove payload integrity; use `verify` for that.

## Verify an Archive

```bash
troupe platform backup verify backup.troupe-backup
```

Verification fails if:

- The archive format or version is unsupported.
- `manifest.json` or `checksums.json` is missing.
- A payload file is missing.
- A payload checksum or size does not match.
- The archive contains unchecked payload files.
- The compressed archive stream is malformed.
- The archive contains unsafe paths.

For automation:

```bash
troupe platform backup verify backup.troupe-backup --json
troupe platform backup verify encrypted.troupe-backup --passphrase "$BACKUP_PASSPHRASE"
```

## Recover a Fresh Installation

Restore targets a clean SQLite or PostgreSQL platform installation. Configure the new installation's storage first, stop the daemon if it is running, then verify and dry-run the archive:

```bash
troupe daemon stop
troupe platform backup verify backup.troupe-backup
troupe platform restore backup.troupe-backup --dry-run
```

If the dry-run has no blockers, restore the archive:

```bash
troupe platform restore backup.troupe-backup --confirm
troupe daemon start
```

The safest target is an empty data directory. If `troupe setup` already created throwaway users, orgs, or teams, either create a fresh data directory or explicitly reset the SQLite target during restore:

```bash
troupe platform restore backup.troupe-backup --confirm --reset-target
```

`--reset-target` is destructive: it deletes the target SQLite platform database, tenant database tree, and legacy file-backed runtime directories under the configured data directory before bootstrapping a new empty target and importing the archive. The command still requires `--confirm` for writes.

Scheduled jobs are restored paused by default to avoid duplicate automation after staging restores or migrations. Use `--enable-scheduled-jobs` only when you intentionally want restored jobs to resume immediately. Login sessions, device sessions, link codes, OAuth caches, and sandbox runtime sessions are skipped by default; use `--include-transient` only for controlled recovery tests.

For full credential recovery, copy the source encryption key to the new installation before restore and before starting the restored system:

```bash
mkdir -p ~/.config/troupe
cp /secure/source/.store_key ~/.config/troupe/.store_key
chmod 600 ~/.config/troupe/.store_key
# or export the same TROUPE_MASTER_KEY used by the source deployment
export TROUPE_MASTER_KEY='...'
```

Restore dry-runs now check encrypted credential material. If the backup contains credentials and the current key is missing or wrong, restore reports a blocker instead of importing credentials that would be visible but unusable. Fix the key first, then run the dry-run again.

Recovery backups preserve stored values by default, including password hashes and encrypted credential blobs, because those values are required for a working restored system. Use `--redact-secrets` only for portable/support exports that intentionally cannot restore protected values:

```bash
troupe platform backup create --output portable.troupe-backup --redact-secrets
```

Redacted fields cannot be recovered and must be reconfigured manually.

## Scoped Export

Backup creation can narrow the export scope:

```bash
troupe platform backup create --org acme --output acme.troupe-backup
troupe platform backup create --org acme --team sre --output acme-sre.troupe-backup
troupe platform backup create --org acme --user alice@example.com --output alice.troupe-backup
```

The restore flow validates archive integrity, schema compatibility, target emptiness, and encrypted credential key compatibility before writing data. Schema mismatch errors usually mean the target needs migrations, a matching Troupe binary, or a backup created from a compatible schema. Use restore-time mapping when you need to port data into renamed scopes:

```bash
troupe platform restore acme.troupe-backup --dry-run --map-org acme:acme-prod
troupe platform restore acme-sre.troupe-backup --confirm --map-org acme:acme-prod --map-team acme/sre:acme-prod/platform
troupe platform restore alice.troupe-backup --confirm --map-user acme/alice@example.com:acme-prod/alice@example.com
```

`--map-org` uses `old:new`. `--map-team` and `--map-user` use `old-org/old-id:new-org/new-id`. Mapping is for fresh-target portability; it is not a merge tool for combining conflicting existing tenants.

## Credentials and Secrets

Credential values remain encrypted in backups. Troupe backup tooling must not export plaintext API keys, OAuth tokens, passwords, or provider secrets.

The logical exporter preserves stored protected values in recovery backups. When `--redact-secrets` is used, it redacts sensitive-looking columns and nested JSON keys such as password, token, secret, API key, JWT, and encrypted-value fields. Restoring encrypted credential payloads into a different deployment requires compatible key material or a future explicit re-key workflow. If the source used `~/.config/troupe/.store_key`, copy that file securely to the target. If the source used `TROUPE_MASTER_KEY`, configure the same value on the target before restore.

## Operational Guidance

After creating an archive, run `verify` before copying it to long-term storage or using it for migration work. Use `--passphrase` for archive-level encryption, and still handle recovery archives as sensitive files.

For strict full-platform consistency, run backup creation during a maintenance window or while writes are paused. PostgreSQL deployments use separate databases per organization, so a complete platform export cannot rely on a single database snapshot. Each restored scope is imported transactionally, but a full PostgreSQL platform restore is not one cross-database transaction. PostgreSQL `--reset-target` is intentionally blocked; restore PostgreSQL archives into a fresh platform database or perform any destructive reset externally using your site's database runbook. App-created SQL schemas remain planned follow-up work.

## Related Documentation

- [Platform Overview](./index)
- [Administration](./administration)
- [Credential Security](../security/credential-security)
