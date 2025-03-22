#!/bin/bash
set -e

echo "🚀 Setting up dipa-auto..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "📦 Go not found, installing..."
    if command -v apt &> /dev/null; then
        sudo apt update
        sudo apt install -y golang
    elif command -v dnf &> /dev/null; then
        sudo dnf install -y golang
    elif command -v pacman &> /dev/null; then
        sudo pacman -Sy go
    elif command -v brew &> /dev/null; then
        brew install go
    else
        echo "❌ Could not find a package manager to install Go. Please install Go manually."
        exit 1
    fi
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.18"
if [[ $(echo -e "$GO_VERSION\n$REQUIRED_VERSION" | sort -V | head -n1) != "$REQUIRED_VERSION" ]]; then
    echo "⚠️ Go version $GO_VERSION detected. dipa-auto requires at least Go $REQUIRED_VERSION."
    echo "Please update Go manually."
    exit 1
fi

SERVICE_NAME="dipa-auto"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HASH_DIR="/var/lib/dipa-auto"
CONFIG_FILE="$SCRIPT_DIR/config.toml"
BIN_DIR="/usr/local/bin"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Config file not found at $CONFIG_FILE"
    exit 1
fi

echo "📝 Building dipa-auto..."
cd "$SCRIPT_DIR"
go build -o $SERVICE_NAME ./src

echo "📝 Creating hash directory..."
sudo mkdir -p "$HASH_DIR"
sudo chown -R $USER:$USER "$HASH_DIR"

echo "📝 Installing dipa-auto binary..."
sudo cp "$SCRIPT_DIR/$SERVICE_NAME" "$BIN_DIR/$SERVICE_NAME"

echo "📝 Creating systemd service..."
cat > /tmp/dipa-auto.service << END
[Unit]
Description=dipa-auto service
After=network.target

[Service]
Type=simple
User=$USER
Environment="CONFIG_PATH=$CONFIG_FILE"
ExecStart=$BIN_DIR/$SERVICE_NAME
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
END

if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    echo "🔄 Updating existing service..."
    sudo systemctl stop $SERVICE_NAME
else
    echo "✨ Creating new service..."
fi

sudo mv /tmp/dipa-auto.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable $SERVICE_NAME
sudo systemctl start $SERVICE_NAME

echo "✅ Service installed and started successfully!"
echo "📊 Check status with: sudo systemctl status $SERVICE_NAME"
echo "📜 View logs with: sudo journalctl -u $SERVICE_NAME -f"
