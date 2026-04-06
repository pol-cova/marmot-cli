# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0-alpha] - 2024-12-19

**⚠️ This is an ALPHA pre-release. Not suitable for production use.**

This alpha release introduces the new S3-only architecture with local storage mode. It is intended for testing and feedback before the stable v0.3.0 release.

### 🚀 Major Changes

#### Removed Hub Dependency
- **BREAKING**: Removed Marmot Hub dependency - now uses direct S3-compatible storage
- Removed `internal/client/hub.go` and Hub-related code
- Removed `marmot connect` command (no longer needed)
- Simplified architecture: backups go directly to your storage

#### Added S3-Compatible Storage Support
- Support for AWS S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO
- Direct storage provider integration with AWS SDK v2
- Optimized for each provider's specific requirements:
  - R2: Uses `auto` region
  - B2: Region-specific endpoints
  - Wasabi: Default `us-east-1` region
  - MinIO: Path-style addressing

### ✨ New Features

#### Interactive Init Command
- Complete rewrite of `marmot init` with better DX
- Interactive provider selection (R2, S3, B2, Wasabi, MinIO)
- Smart endpoint prompts based on provider
- Credential input with confirmation
- Connection testing during setup
- Clear next steps after initialization

#### Performance Optimizations
- **99% reduction in API calls** for `ListBackups` (eliminated N+1 problem)
- Memory-efficient streaming uploads (no `io.ReadAll`)
- Regex-based metadata extraction from object keys
- Smart size detection using `io.ReadSeeker`

#### MongoDB Support
- Added MongoDB backup support (mongodump/mongorestore)
- Archive format with BSON parsing
- Verification support for MongoDB archives

### 🔧 Technical Improvements

#### Configuration
- Simplified config structure (removed HubConfig)
- Environment variable support for all storage options
- Provider-specific validation and defaults

#### Code Quality
- Removed ~788 lines of Hub-related code
- Added ~560 lines of optimized S3 implementation
- Clean separation of storage providers
- Better error handling and context

### 📚 Documentation
- Complete rewrite of README.md for S3 focus
- Provider-specific configuration examples
- Environment variable documentation
- Security best practices (key management)

### 🐛 Bug Fixes
- Fixed memory overhead on large backup uploads
- Fixed N+1 API call issue in backup listing
- Correct region handling for R2 (`auto` vs `us-east-1`)

## [0.2.0] - 2024-11-XX

### Added
- MongoDB support with mongodump/mongorestore
- Backup verification command (`marmot verify`)
- Dry-run restore option
- BSON archive parsing for MongoDB
- Improved error messages in restore operations

### Fixed
- Restore from local file path issues
- Better handling of database authentication
- Fixed race condition in upload queue

## [0.1-alpha] - 2024

### Added
- Initial alpha release
- Automatic database discovery in Docker containers
- Support for MySQL and PostgreSQL databases
- AES-256-GCM encryption for backups
- Gzip compression
- Scheduled backups with cron expressions
- Upload queue with SQLite persistence
- Automatic retry with exponential backoff
- Restore functionality from Hub or local files
- CLI interface with Cobra
- systemd service support
- Key export/import for disaster recovery
- Configuration management with Viper

### Features
- **Backup Pipeline**: Dump → Compress → Encrypt → Save → Queue → Upload
- **Database Support**: MySQL (mysqldump) and PostgreSQL (pg_dump)
- **Security**: AES-256-GCM encryption, secure file permissions (0600)
- **Reliability**: Upload queue, automatic retry (max 5 attempts), local persistence
- **Cross-platform**: Linux, macOS, Windows support
- **Docker Integration**: Automatic container discovery and backup

### Technical
- Go 1.25
- Cobra CLI framework
- Viper configuration
- Docker SDK
- SQLite for queue persistence
- robfig/cron for scheduling

[Unreleased]: https://github.com/pol-cova/marmot-cli/compare/v0.3.0...HEAD
[0.3.0-alpha]: https://github.com/pol-cova/marmot-cli/releases/tag/v0.3.0-alpha
[0.2.0]: https://github.com/pol-cova/marmot-cli/releases/tag/v0.2.0
[0.1-alpha]: https://github.com/pol-cova/marmot-cli/releases/tag/v0.1-alpha
