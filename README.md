# Marmot CLI

[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://github.com/pol-cova/marmot-cli/blob/main/LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/pol-cova/marmot-cli/ci.yml?branch=main)](https://github.com/pol-cova/marmot-cli/actions)
[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://golang.org)

**Marmot is a database backup tool that automatically backs up MySQL, PostgreSQL, and MongoDB to S3-compatible storage or local disk.**

Supports AWS S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO, and local-only mode for users who just need automated local backups with retention management.

### 🚀 Release Status

| Version | Status | Channel | Description |
|---------|--------|---------|-------------|
| **v0.2.0** | **Stable** | Production | Current stable release with Hub support |
| **v0.3.0-alpha** | **Alpha** | Testing | New S3-only architecture + local storage (⚠️ Pre-release) |

**Current Recommendation:**
- **Production:** Use [v0.2.0](https://github.com/pol-cova/marmot-cli/releases/tag/v0.2.0) (stable with Hub support)
- **Testing:** Try [v0.3.0-alpha](https://github.com/pol-cova/marmot-cli/releases/tag/v0.3.0-alpha) (new features, may have issues)

See [RELEASES.md](RELEASES.md) for our full release strategy and version compatibility.

---

## Table of Contents

- [Release Status](#-release-status)
- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Supported Storage Providers](#supported-storage-providers)
- [Commands](#commands)
- [Configuration](#configuration)
- [Backup Verification](#backup-verification)
- [Database Management](#database-management)
- [Environment Variables](#environment-variables)
- [Development](#development)
- [License](#license)

---

## Features

- **Multi-Database Support**: MySQL, PostgreSQL, and MongoDB
- **Flexible Storage**: Cloud (S3/R2/B2/Wasabi/MinIO) or Local-only mode
- **Automatic Discovery**: Scans Docker containers to auto-discover databases
- **Scheduled Backups**: Cron-based scheduling for automated backups
- **Retention Management**: Automatic cleanup of old backups (local mode)
- **Encryption**: AES-256-GCM encryption for secure backups
- **Compression**: Gzip compression to reduce storage costs
- **Backup Verification**: Validate backups without restoring
- **Direct Storage**: No hub or middleware required - backups go straight to your storage

---

## Quick Start

### 1. Install Marmot

```bash
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash
```

### 2. Initialize Configuration

```bash
marmot init
```

This interactive wizard will:
- Choose between **Cloud storage** (S3/R2/B2/Wasabi/MinIO) or **Local-only** mode
- For cloud: Ask for provider credentials (once, securely stored)
- For local: Configure retention and disk space management
- Discover databases in Docker containers
- Generate encryption keys
- Test the connection (cloud mode)

### 3. Run Your First Backup

```bash
# Backup all configured databases
marmot backup --all

# Or backup a specific database
marmot backup --db my-postgres
```

### 4. Start the Daemon (Optional)

For automated scheduled backups:

```bash
# Run manually (for testing)
marmot start

# Or install as systemd service (production)
sudo cp marmot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now marmot
```

---

## Installation

### Quick Install (Linux/macOS)

**Latest Stable (Recommended for Production):**
```bash
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash
```

**Specific Version:**
```bash
# Install a specific stable version
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash -s v0.2.0

# View install help
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash -s --help
```

**⚠️ Pre-release Versions (For Testing Only):**
```bash
# Install alpha/beta/rc version (requires confirmation)
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash -s v0.3.0-alpha

# Skip confirmation (use with caution)
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash -s v0.3.0-alpha --yes
```

See [RELEASES.md](RELEASES.md) for our release channel strategy and versioning.

### From Source

```bash
git clone https://github.com/pol-cova/marmot-cli.git
cd marmot
make build
./bin/marmot --version
```

The version is automatically determined from git tags:
- If on a tagged release: shows tag (e.g., `v0.3.0`)
- If on a commit after tag: shows tag+commits (e.g., `v0.3.0-5-gabc123`)
- If no tags exist: shows `dev`

Install system-wide:

```bash
sudo make install
```

### Build for Specific Platform

```bash
make build-linux    # Linux AMD64
make build-darwin   # macOS (Intel and Apple Silicon)
make build-windows  # Windows AMD64
```

---

## Supported Storage Providers

### Local-Only Mode (No Cloud)

For users who just need automated local backups with retention management:

```bash
marmot init
# Select: local
# Configure backup directory (optional, defaults to ~/.local/share/marmot/backups)
# Set retention days (default: 30, 0 = unlimited)
# Set minimum free space warning (default: 10 GB)
```

**Benefits:**
- No cloud credentials needed
- Zero storage costs
- Data never leaves the server
- Automatic retention/cleanup with `marmot cleanup`

**Note:** Backups are stored locally only - ensure you have proper disaster recovery procedures.

### Cloudflare R2 (Recommended - Zero egress fees)

```bash
marmot init
# Select: r2
# Enter: https://<account-id>.r2.cloudflarestorage.com
# Enter: bucket name
# Enter: access key from R2 API tokens
# Enter: secret key from R2 API tokens
```

### AWS S3

```bash
marmot init
# Select: s3
# Enter: bucket name
# Enter: region (e.g., us-east-1)
# Enter: access key ID
# Enter: secret access key
```

### Backblaze B2

```bash
marmot init
# Select: b2
# Enter: https://s3.us-west-004.backblazeb2.com
# Enter: bucket name
# Enter: key ID
# Enter: application key
```

### Wasabi

```bash
marmot init
# Select: wasabi
# Enter: bucket name
# Enter: access key
# Enter: secret key
# Region defaults to us-east-1
```

### MinIO (Self-hosted)

```bash
marmot init
# Select: minio
# Enter: http://localhost:9000 (or your MinIO endpoint)
# Enter: bucket name
# Enter: access key
# Enter: secret key
```

### Other S3-Compatible Services

```bash
marmot init
# Select: other
# Enter: provider name
# Enter: S3-compatible endpoint URL
# Enter: bucket name
# Enter: access key
# Enter: secret key
```

---

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `marmot init` | Interactive setup wizard |
| `marmot backup [db-id]` | Perform manual backup |
| `marmot backup --all` | Backup all databases |
| `marmot start` | Start backup daemon |
| `marmot restore` | Restore from backup |
| `marmot verify` | Verify backup integrity |
| `marmot status` | Show daemon and storage status |
| `marmot cleanup` | Clean up old local backups (local mode) |

### Database Management

| Command | Description |
|---------|-------------|
| `marmot db list` | List configured databases |
| `marmot db add` | Add a database manually |
| `marmot db remove` | Remove a database |

### Utility Commands

| Command | Description |
|---------|-------------|
| `marmot decrypt` | Decrypt backup file locally |
| `marmot queue` | Manage upload queue |
| `marmot service` | Systemd service management |
| `marmot key export` | Export encryption key |
| `marmot key import` | Import encryption key |

---

## Configuration

Configuration is stored in `~/.config/marmot/config.yaml` (Linux/macOS) or `%APPDATA%\marmot\config.yaml` (Windows).

### Example: Local-Only Storage

```yaml
storage_type: local
local:
  path: /backup/marmot                    # Custom backup directory (optional)
  retention_days: 30                      # Keep backups for 30 days (0 = unlimited)
  min_free_space_gb: 10                   # Warn if less than 10 GB free

databases:
  - id: prod-postgres
    type: postgres
    container_id: abc123def456
    name: myapp
    user: postgres
    password: secret
    schedule: "0 2 * * *"  # Daily at 2 AM
    enabled: true
```

**Retention Management:**
```bash
# Preview what would be cleaned up
marmot cleanup --dry-run

# Clean up old backups
marmot cleanup

# Skip confirmation
marmot cleanup --force
```

### Example: Cloudflare R2

```yaml
storage:
  provider: r2
  endpoint: https://<account>.r2.cloudflarestorage.com
  bucket: my-backups
  access_key: <access-key-id>
  secret_key: <secret-access-key>
  server_id: web-01
  prefix: backups

databases:
  - id: prod-postgres
    type: postgres
    container_id: abc123def456
    name: myapp
    user: postgres
    password: secret
    schedule: "0 2 * * *"
    enabled: true
```

### Example: AWS S3

```yaml
storage:
  provider: s3
  bucket: my-backup-bucket
  region: us-east-1
  access_key: AKIAXXXXXXXXXXXXXXXX
  secret_key: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  server_id: web-01

databases:
  - id: prod-mysql
    type: mysql
    dsn: "mysql://user:pass@localhost:3306/mydb"
    schedule: "0 */6 * * *"
    enabled: true
```

---

## Environment Variables

All configuration can be set via environment variables (useful for CI/CD):

### Local Storage

```bash
export MARMOT_STORAGE_TYPE=local
export MARMOT_LOCAL_PATH=/backup/marmot
export MARMOT_LOCAL_RETENTION_DAYS=30
export MARMOT_LOCAL_MIN_FREE_SPACE_GB=10
```

### Cloud Storage

```bash
export MARMOT_STORAGE_TYPE=s3
export MARMOT_S3_PROVIDER=r2
export MARMOT_S3_ENDPOINT=https://<account>.r2.cloudflarestorage.com
export MARMOT_S3_BUCKET=my-backups
export MARMOT_S3_ACCESS_KEY=<access-key>
export MARMOT_S3_SECRET_KEY=<secret-key>
export MARMOT_S3_SERVER_ID=web-01
export MARMOT_S3_PREFIX=backups
```

---

## Backup Verification

The `verify` command validates backup integrity without modifying your database:

```bash
# Verify from storage by backup ID
marmot verify s3://my-bucket/web-01/2024/01/15/mydb-1705312800.enc

# Verify latest backup for a database
marmot verify --db prod-mongo --latest

# Verify local file
marmot verify --file ./backup.enc

# Summary only
marmot verify s3://my-bucket/... --format summary
```

**What it checks:**
- ✅ File integrity (decrypts successfully)
- ✅ Structure validation (valid dump format)
- ✅ Table/collection counts
- ✅ Row/document counts per structure
- ✅ Sample data preview (3 records each)

---

## Database Management

### Adding Databases

**MySQL via Docker:**
```bash
marmot db add --type mysql --id prod-mysql \
  --container mysql-container \
  --name mydb --user root --password secret \
  --schedule "0 2 * * *"
```

**PostgreSQL via DSN:**
```bash
marmot db add --type postgres --id prod-pg \
  --dsn "postgres://user:pass@localhost:5432/mydb" \
  --schedule "0 3 * * *"
```

**MongoDB via DSN:**
```bash
marmot db add --type mongo --id prod-mongo \
  --dsn "mongodb://root:secret@localhost:27017/admin" \
  --schedule "0 4 * * *"
```

---

## Storage Object Structure

Backups are organized in your bucket as:

```
[prefix/]<server-id>/YYYY/MM/DD/<database-name>-<timestamp>.enc
```

Example:
```
backups/web-01/2024/01/15/myapp-1705312800.enc
backups/web-01/2024/01/15/analytics-1705316400.enc
```

Each object includes metadata:
- `server-id`: The server that created the backup
- `database-name`: The database that was backed up
- `timestamp`: ISO 8601 timestamp of the backup

---

## Development

```bash
make version       # Show current version info
make build         # Build binary (auto-detects version from git tags)
make test          # Run tests
make test-coverage # Generate coverage report
make lint          # Run linter
```

**Version Management:**
- Version is automatically set from git tags at build time
- Create a release: `git tag -a v0.3.0 -m "Release v0.3.0" && git push origin v0.3.0`
- The Makefile uses `git describe --tags --always` to get the version
- Commit hash and build time are also embedded in the binary

---

## Security

- **Encryption**: All backups are encrypted with AES-256-GCM before upload
- **Key Management**: Encryption keys are stored locally at `~/.config/marmot/key`
- **Backup Your Key**: Run `marmot key export` and store the key securely offline
  - Without this key, backups **cannot** be decrypted
  - Store it in a password manager or secure location outside your servers
- **Credentials**: Storage credentials are stored in config with 0600 permissions
- **No Data Leakage**: Backups go directly to your S3 bucket - no middleware

---

## Contributing

We welcome contributions! See [Contributing Guidelines](CONTRIBUTING.md).

This project adheres to a [Code of Conduct](CODE_OF_CONDUCT.md).

---

## License

MIT License - see [LICENSE](LICENSE) file.

---

## Support

- **GitHub Issues**: [github.com/pol-cova/marmot-cli/issues](https://github.com/pol-cova/marmot-cli/issues)
- **CLI Help**: `marmot --help`
- **Command Help**: `marmot <command> --help`
