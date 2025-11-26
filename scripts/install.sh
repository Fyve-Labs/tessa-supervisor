#!/usr/bin/env bash

# Check for help flag
case "$1" in
-h | --help)
  printf "Tessa installation script\n\n"
  printf "Usage: ./install.sh [options]\n\n"
  printf "Options: \n"
  printf "  -n, --name            : Device name\n"
  printf "  -t, --token           : Token\n"
  printf "  -v, --version         : Version to install (default: latest)\n"
  printf "  -u, --uninstall       : Uninstall Tessa\n"
  printf "  -h, --help            : Display this help message\n"
  exit 0
  ;;
esac

# Build sudo args by properly quoting everything
build_sudo_args() {
  QUOTED_ARGS=""
  while [ $# -gt 0 ]; do
    if [ -n "$QUOTED_ARGS" ]; then
      QUOTED_ARGS="$QUOTED_ARGS "
    fi
    QUOTED_ARGS="$QUOTED_ARGS'$(echo "$1" | sed "s/'/'\\\\''/g")'"
    shift
  done
  echo "$QUOTED_ARGS"
}

# Check if running as root and re-execute with sudo if needed
if [ "$(id -u)" != "0" ]; then
  if command -v sudo >/dev/null 2>&1; then
    if [ -f "$0" ] && [ "$0" != "sh" ]; then
      exec sudo -- "$0" "$@"
    else
      exec sudo -E sh -s -- "$@"
    fi
  else
    echo "This script must be run as root. Please either:"
    echo "1. Run this script as root (su root)"
    echo "2. Install sudo and run with sudo"
    exit 1
  fi
fi

# Default values
UNINSTALL=false
GITHUB_URL="https://github.com"
GITHUB_REPO="${GITHUB_REPO:-Fyve-Labs/tessa-supervisor}"
TOKEN=""
DEVICE_NAME=""
VERSION="latest"

# Parse arguments
while [ $# -gt 0 ]; do
  case "$1" in
  -t | --token)
    shift
    TOKEN="$1"
    ;;
  -n | --device-name)
    shift
    DEVICE_NAME="$1"
    ;;
  -v | --version)
    shift
    VERSION="$1"
    ;;
  -u | --uninstall)
    UNINSTALL=true
    ;;
  *)
    echo "Invalid option: $1" >&2
    exit 1
    ;;
  esac
  shift
done

# Detect system architecture
detect_architecture() {
  arch=$(uname -m)

  case "$arch" in
    x86_64)
      arch="amd64"
      ;;
    armv6l|armv7l)
      arch="arm"
      ;;
    aarch64)
      arch="arm64"
      ;;
  esac

  echo "$arch"
}

# Set paths based on operating system
DATA_DIR="/etc/tessad"
BIN_PATH="/usr/local/bin/tessad"

# Uninstall process
if [ "$UNINSTALL" = true ]; then
  echo "Stopping and disabling the service..."
  systemctl stop tessad.service
  systemctl disable tessad.service

  echo "Removing the systemd service file..."
  rm /etc/systemd/system/tessad.service

  # Remove the update timer and service if they exist
  echo "Removing the daily update service and timer..."
  systemctl stop tessad-update.timer 2>/dev/null
  systemctl disable tessad-update.timer 2>/dev/null
  rm -f /etc/systemd/system/tessad-update.service
  rm -f /etc/systemd/system/tessad-update.timer

  systemctl daemon-reload

  echo "Removing the tessad directory..."
  rm -rf "$DATA_DIR"

  echo "tessad has been uninstalled successfully!"
  exit 0
fi

if [ -z "$TOKEN" ]; then
  echo "Token is required"
  echo "Re-run install.sh -t <token>"
  exit 1
fi

# Check if a package is installed
package_installed() {
  command -v "$1" >/dev/null 2>&1
}

# Check for package manager and install necessary packages if not installed
if package_installed apt-get; then
  if ! package_installed tar || ! package_installed curl || ! package_installed sha256sum; then
    apt-get update
    apt-get install -y tar curl coreutils
  fi
else
  echo "Warning: Please ensure 'tar' and 'curl' and 'sha256sum (coreutils)' are installed."
fi

# Remove newlines from TOKEN
TOKEN=$(echo "$TOKEN" | tr -d '\n')

# Verify checksum
if command -v sha256sum >/dev/null; then
  CHECK_CMD="sha256sum"
elif command -v md5 >/dev/null; then
  CHECK_CMD="md5 -q"
else
  echo "No MD5 checksum utility found"
  exit 1
fi


# Create the directory for the Tessa Agent
if [ ! -d "$DATA_DIR" ]; then
  echo "Creating the directory for the Tessa Agent..."
  mkdir -p "$DATA_DIR"
  chmod 755 "$DATA_DIR"
fi

# Save token to data dir so it can bootstrap on first start
echo -n "$TOKEN" > "$DATA_DIR/token"
chmod 600 "$DATA_DIR/token"

OS=$(uname -s | sed -e 'y/ABCDEFGHIJKLMNOPQRSTUVWXYZ/abcdefghijklmnopqrstuvwxyz/')
ARCH=$(detect_architecture)

# Determine version to install
if [ "$VERSION" = "latest" ]; then
  API_RELEASE_URL="https://api.github.com/repos/$GITHUB_REPO/releases/latest"
  INSTALL_VERSION=$(curl -s "$API_RELEASE_URL" | grep -o '"tag_name": "v[^"]*"' | cut -d'"' -f4 | tr -d 'v')
  if [ -z "$INSTALL_VERSION" ]; then
    echo "Failed to get latest version"
    exit 1
  fi
else
  INSTALL_VERSION="$VERSION"
  # Remove 'v' prefix if present
  INSTALL_VERSION=$(echo "$INSTALL_VERSION" | sed 's/^v//')
fi

echo "Downloading and installing tessad version ${INSTALL_VERSION}..."

FILE_NAME="tessad_${INSTALL_VERSION}_${OS}_${ARCH}.tar.gz"
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR" || exit 1

# Download checksums file
CHECKSUM=$(curl -sL "$GITHUB_URL/$GITHUB_REPO/releases/download/v${INSTALL_VERSION}/checksums.txt" | grep "$FILE_NAME" | cut -d' ' -f1)
if [ -z "$CHECKSUM" ] || ! echo "$CHECKSUM" | grep -qE "^[a-fA-F0-9]{64}$"; then
  echo "Failed to get checksum or invalid checksum format"
  exit 1
fi

if ! curl -#L "$GITHUB_URL/$GITHUB_REPO/releases/download/v${INSTALL_VERSION}/$FILE_NAME" -o "$FILE_NAME"; then
  echo "Failed to download tessad from ""$GITHUB_URL/$GITHUB_REPO/releases/download/v${INSTALL_VERSION}/$FILE_NAME"
  rm -rf "$TEMP_DIR"
  exit 1
fi

if [ "$($CHECK_CMD "$FILE_NAME" | cut -d' ' -f1)" != "$CHECKSUM" ]; then
  echo "Checksum verification failed: $($CHECK_CMD "$FILE_NAME" | cut -d' ' -f1) & $CHECKSUM"
  rm -rf "$TEMP_DIR"
  exit 1
fi

if ! tar -xzf "$FILE_NAME" tessad; then
  echo "Failed to extract the agent"
  rm -rf "$TEMP_DIR"
  exit 1
fi

mv tessad "$BIN_PATH"
chmod 755 "$BIN_PATH"

# Cleanup
rm -rf "$TEMP_DIR"

# Make sure /etc/machine-id exists for persistent fingerprint
if [ ! -f /etc/machine-id ]; then
  cat /proc/sys/kernel/random/uuid | tr -d '-' > /etc/machine-id
fi


# systemd service installation code
echo "Creating the systemd service for tessad..."

cat >/etc/systemd/system/tessad.service <<EOF
[Unit]
Description=Tessa Agent Service
Wants=network-online.target
After=network-online.target

[Service]
Environment="DEVICE_NAME=$DEVICE_NAME"
ExecStart=$BIN_PATH start
Restart=on-failure
RestartSec=5
RuntimeDirectory=tessad
StateDirectory=tessad

[Install]
WantedBy=multi-user.target
EOF

# Load and start the service
printf "\nLoading and starting tessad service...\n"
systemctl daemon-reload
systemctl enable tessad.service
systemctl start tessad.service


echo "Setting up daily automatic updates for tessad..."

# Create systemd service for the daily update
cat >/etc/systemd/system/tessad-update.service <<EOF
[Unit]
Description=Update tessad if needed
Wants=tessad.service

[Service]
Type=oneshot
ExecStart=$BIN_PATH update
EOF

# Create systemd timer for the daily update
cat >/etc/systemd/system/tessad-update.timer <<EOF
[Unit]
Description=Run tessad update daily

[Timer]
OnCalendar=daily
Persistent=true
RandomizedDelaySec=4h

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now tessad-update.timer

printf "\nDaily updates have been enabled.\n"

# Wait for the service to start or fail
if [ "$(systemctl is-active tessad.service)" != "active" ]; then
  echo "Error: The tessad service is not running."
  echo "$(systemctl status tessad.service)"
  exit 1
fi

printf "\n\033[32mtessad has been installed successfully! It is now running.\033[0m\n"
