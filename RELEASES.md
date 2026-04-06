# Release Strategy

Marmot follows a dual-track release strategy with stable and pre-release channels.

## Version Numbering

We use [Semantic Versioning](https://semver.org/) with pre-release tags:

- **Stable releases**: `v0.2.0`, `v0.3.0`, `v1.0.0`
- **Alpha releases**: `v0.3.0-alpha` (early testing, may have breaking changes)
- **Beta releases**: `v0.3.0-beta.1`, `v0.3.0-beta.2` (feature-complete, testing phase)
- **Release Candidates**: `v0.3.0-rc.1`, `v0.3.0-rc.2` (final testing before stable)

## Release Channels

### Stable Channel (Recommended for Production)

Stable releases are battle-tested and ready for production use.

```bash
# Install latest stable
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash

# Install specific stable version
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash -s v0.2.0
```

### Pre-release Channel (For Testing)

Pre-releases contain new features but may have bugs or breaking changes.

```bash
# Install latest pre-release (manual download from GitHub releases page)
# Pre-releases are marked with "Pre-release" badge on GitHub
```

**⚠️ Warning**: Pre-release versions may:
- Have bugs or incomplete features
- Change configuration formats
- Break compatibility with previous versions
- Not be suitable for production use

## Current Status

| Version | Status | Channel | Description |
|---------|--------|---------|-------------|
| v0.2.0 | **Stable** | Production | Current stable release |
| v0.3.0-alpha | Pre-release | Testing | New S3-only architecture, local storage mode |
| v0.3.0-beta.x | (future) | Testing | Feature-complete v0.3.0 |
| v0.3.0 | (future) | Production | Next stable release |

## Creating a Release

### Stable Release

```bash
# 1. Ensure all tests pass
make test

# 2. Update CHANGELOG.md
# 3. Update version in README if needed

# 4. Create and push tag
git tag -a v0.3.0 -m "Release v0.3.0"
git push origin v0.3.0

# GitHub Actions will automatically build and create the release
```

### Pre-release (Alpha)

```bash
# For early testing of new features
git tag -a v0.3.0-alpha -m "Alpha: New S3 architecture + local storage"
git push origin v0.3.0-alpha

# GitHub will mark this as "Pre-release" automatically
```

### Pre-release (Beta)

```bash
# When feature-complete but needs testing
git tag -a v0.3.0-beta.1 -m "Beta 1: Feature complete, testing phase"
git push origin v0.3.0-beta.1
```

### Release Candidate

```bash
# Final testing before stable
git tag -a v0.3.0-rc.1 -m "RC 1: Release candidate"
git push origin v0.3.0-rc.1
```

## Automated CI/CD

The repository uses GitHub Actions for automated releases:

1. **On every tag push** (`v*`): Builds binaries for all platforms
2. **Pre-release detection**: Tags containing `alpha`, `beta`, or `rc` are marked as pre-releases on GitHub
3. **Artifacts**: Binaries are attached to the GitHub release
4. **Release notes**: Auto-generated from commit history

## Testing Pre-releases

When testing a pre-release:

1. **Backup your config**: `cp ~/.config/marmot/config.yaml ~/.config/marmot/config.yaml.backup`
2. **Test in staging**: Use a non-production server
3. **Report issues**: Create GitHub issues with the pre-release version
4. **Check compatibility**: Review CHANGELOG for breaking changes

## Downgrading

If you need to downgrade from a pre-release:

```bash
# Download specific stable version
curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash -s v0.2.0

# Restore your config if needed
cp ~/.config/marmot/config.yaml.backup ~/.config/marmot/config.yaml
```

## Version Compatibility

| Marmot Version | Config Format | Storage Types | Breaking Changes |
|----------------|---------------|---------------|------------------|
| v0.1.x | v1 | Hub only | - |
| v0.2.x | v1 | Hub only | - |
| **v0.3.0-alpha** | **v2** | **S3 + Local** | **Yes** - Removed Hub support |
| v0.3.0+ | v2 | S3 + Local | No (from v0.3.0-alpha) |

**v0.3.0 Breaking Changes:**
- Removed Marmot Hub support (migrated to direct S3)
- New configuration format (`storage_type: local|s3`)
- New `init` flow (Cloud vs Local first)
- Added `cleanup` command for local storage
