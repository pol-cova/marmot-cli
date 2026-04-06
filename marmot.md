# Marmot Workflow

## The Big Picture

Two components work together:
- **Marmot (CLI agent)** — runs on each database server, does the actual backup work
- **MarmotHub (API server)** — receives and stores encrypted backups in Cloudflare R2, runs somewhere safe outside your servers

```
[Database Server]          [Internet]         [MarmotHub + R2]
  Marmot CLI          →   encrypted file  →   stores backup
  (dumps, encrypts,        over HTTPS          (your safe copy)
   uploads)
```

---

## Setup Flow (one time per server)

**Option 1: Quick connect (recommended)**

```bash
# 1. Install the binary
curl -fsSL https://raw.githubusercontent.com/pol-cova/Marmot/main/install.sh | sh

# 2. Connect to your Hub (creates config + generates encryption key)
marmot connect https://hub.example.com YOUR_TOKEN --server-id web-01

# 3. ⚠️  CRITICAL: Export and save the key OFF this server
marmot key export
# → copy that base64 string to a password manager or print it

# 4. Add your databases
marmot db add --type postgres --dsn "postgres://app:pass@localhost/prod" --id prod-pg
marmot db add --type mysql --container mycontainer --name orders --user app --password secret --id prod-mysql

# 5. Install the daemon (runs scheduled backups automatically)
marmot service install
```

**Option 2: Interactive wizard**

```bash
marmot init  # Interactive setup with Docker auto-discovery
```

---

## Backup Flow (what happens on each backup)

```
marmot backup --db prod-pg
         │
         ▼
1. DUMP      pg_dump (or docker exec pg_dump) → dump.sql
         │
         ▼
2. COMPRESS  gzip level 9 → dump.sql.gz
         │
         ▼
3. ENCRYPT   AES-256-GCM with your key.bin → backup.enc
         │
         ▼
4. SAVE      writes to local storage (stays even if upload fails)
   - Linux (root): `/var/lib/marmot/backups/`
   - Linux (user): `~/.marmot/backups/`
   - macOS: `~/.marmot/backups/`
   - Windows: `%APPDATA%/marmot/backups/`
         │
         ▼
5. QUEUE     adds entry to local SQLite (upload_queue table)
         │
         ▼
6. UPLOAD    POST /api/v1/backup to Hub with SHA256 checksum header
         │
         ▼
      Hub stores in Cloudflare R2, applies retention policy
      (daily 7d / weekly 4wk / monthly 3mo)
```

- If the upload fails → retries with exponential backoff (2, 4, 8, 16, 32 min)
- If the server is running the daemon → daemon retries automatically every 5 min
- Local .enc file always stays — the backup is never lost even if Hub is unreachable

---

## Scheduled Backup Flow (daemon mode)

```bash
marmot start --foreground  # or via systemd/launchd after `marmot service install`
```

The daemon reads the cron schedule from each database config (e.g. `0 2 * * *` = 2am daily) and runs the same 6-step pipeline automatically. It also runs a retry processor every 5 minutes for any failed uploads.

---

## Restore Flow

**Normal restore (server is alive, you need to rollback):**

```bash
marmot restore --db prod-pg              # uses latest backup from Hub (default)
marmot restore --db prod-pg --backup bkp_abc123  # specific backup ID
marmot restore --db prod-pg --file /path/to/local.enc  # from local file
```

**What happens:**
1. Hub API → download backup.enc
2. decrypt with key.bin
3. decompress
4. pg_restore / mysql (Docker or direct)

**Disaster recovery (server is dead/ransomwared, fresh machine):**

```bash
# On the new server:
curl -fsSL .../install.sh | sh
marmot connect https://hub.example.com TOKEN --server-id web-01
marmot key import "your-saved-base64-key"   # ← why you exported it earlier
marmot restore --db prod-pg --latest --force
```

---

## Why the Key Export Matters

```
Server gets ransomwared
        │
        ├─ key.bin is gone  ──→  backups on Hub = USELESS ENCRYPTED BLOBS
        │
        └─ key.bin exported ──→  new server + marmot key import = full recovery
```

The Hub holds the ciphertext. Your exported key is the only way to decrypt it. They're useless without each other — which is exactly what you want (Hub could be breached, backups stay safe).

---

## Quick Reference

| Command | What it does |
|---------|--------------|
| `marmot init` | Interactive setup wizard |
| `marmot connect <url> <token>` | Quick connect to Hub (with `--server-id`) |
| `marmot key export` | Print key as base64 — store it off-server |
| `marmot key import` | Restore key on a new/recovered server |
| `marmot db add` | Add a database (Docker or DSN) |
| `marmot db list` | Show configured databases |
| `marmot backup --all` | Immediate backup of all databases |
| `marmot restore --db X` | Restore latest backup (default) |
| `marmot restore --db X --backup ID` | Restore specific backup |
| `marmot restore --db X --file path` | Restore from local .enc file |
| `marmot service install` | Register daemon with systemd/launchd |
| `marmot status` | Show Hub connectivity + queue + databases |
| `marmot queue clear` | Clear all pending uploads |
| `marmot queue list` | Show pending/failed uploads |
