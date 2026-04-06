#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-latest}"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="marmot"
REPO="pol-cova/marmot-cli"

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)         ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Function to check if version is a pre-release
is_prerelease() {
  local ver="$1"
  if [[ "$ver" =~ -(alpha|beta|rc) ]]; then
    return 0
  fi
  return 1
}

# Resolve latest STABLE version from GitHub API (skips pre-releases)
get_latest_stable() {
  local releases_url="https://api.github.com/repos/${REPO}/releases"
  
  if command -v curl &>/dev/null; then
    curl -fsSL "$releases_url" \
      | grep '"tag_name"' \
      | grep -v '\-alpha' \
      | grep -v '\-beta' \
      | grep -v '\-rc' \
      | head -1 \
      | sed 's/.*"tag_name": *"\(.*\)".*/\1/'
  elif command -v wget &>/dev/null; then
    wget -qO- "$releases_url" \
      | grep '"tag_name"' \
      | grep -v '\-alpha' \
      | grep -v '\-beta' \
      | grep -v '\-rc' \
      | head -1 \
      | sed 's/.*"tag_name": *"\(.*\)".*/\1/'
  else
    echo "Error: curl or wget is required"; exit 1
  fi
}

# Resolve version
if [ "$VERSION" = "latest" ]; then
  VERSION="$(get_latest_stable)"
  
  if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest stable version"; exit 1
  fi
  
  echo -e "${GREEN}Latest stable version: ${VERSION}${NC}"
elif [ "$VERSION" = "--help" ] || [ "$VERSION" = "-h" ]; then
  echo "Marmot Installer"
  echo ""
  echo "Usage:"
  echo "  curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash"
  echo "  curl -fsSL https://raw.githubusercontent.com/pol-cova/marmot-cli/main/install.sh | bash -s [VERSION]"
  echo ""
  echo "Examples:"
  echo "  # Install latest stable release (recommended)"
  echo "  curl -fsSL .../install.sh | bash"
  echo ""
  echo "  # Install specific version"
  echo "  curl -fsSL .../install.sh | bash -s v0.2.0"
  echo ""
  echo "  # Install pre-release (for testing)"
  echo "  curl -fsSL .../install.sh | bash -s v0.3.0-alpha"
  echo ""
  echo "Version formats:"
  echo "  v0.2.0              Stable release"
  echo "  v0.3.0-alpha        Alpha pre-release (testing)"
  echo "  v0.3.0-beta.1       Beta pre-release (testing)"
  echo "  v0.3.0-rc.1         Release candidate (testing)"
  echo ""
  echo "For release information, see: https://github.com/${REPO}/blob/main/RELEASES.md"
  exit 0
fi

# Check if installing a pre-release
if is_prerelease "$VERSION"; then
  echo ""
  echo -e "${YELLOW}⚠️  WARNING: You are installing a PRE-RELEASE version (${VERSION})${NC}"
  echo -e "${YELLOW}   This version may have bugs, incomplete features, or breaking changes.${NC}"
  echo -e "${YELLOW}   Do NOT use in production. Use for testing only.${NC}"
  echo ""
  echo "   Stable releases do NOT contain -alpha, -beta, or -rc in the version."
  echo ""
  
  # Ask for confirmation (unless --yes flag is passed as $2)
  if [ "${2:-}" != "--yes" ]; then
    read -p "Are you sure you want to continue? (y/N) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      echo "Installation cancelled."
      echo ""
      echo "To install the latest stable version, run:"
      echo "  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | bash"
      exit 1
    fi
  fi
  echo ""
fi

FILENAME="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

echo "Installing Marmot ${VERSION} for ${OS}/${ARCH}..."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if command -v curl &>/dev/null; then
  curl -fsSL "$URL" -o "${TMP}/${FILENAME}"
else
  wget -qO "${TMP}/${FILENAME}" "$URL"
fi

tar -xzf "${TMP}/${FILENAME}" -C "$TMP"

if [ -w "$INSTALL_DIR" ]; then
  install -m 755 "${TMP}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
  sudo install -m 755 "${TMP}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo ""
echo -e "${GREEN}Marmot ${VERSION} installed to ${INSTALL_DIR}/${BINARY_NAME}${NC}"
echo ""

if is_prerelease "$VERSION"; then
  echo -e "${YELLOW}⚠️  This is a PRE-RELEASE version. Please report any issues:${NC}"
  echo "   https://github.com/${REPO}/issues"
  echo ""
fi

echo "Next steps:"
echo "  marmot init          # Configure Marmot (Cloud or Local storage)"
echo "  marmot backup --all  # Run your first backup"
echo "  marmot key export    # Store encryption key safely outside this server!"
echo ""
echo "For documentation: https://github.com/${REPO}#readme"
echo "For releases info: https://github.com/${REPO}/blob/main/RELEASES.md"
echo ""
