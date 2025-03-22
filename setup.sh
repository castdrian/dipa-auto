#!/bin/bash
set -e

# Detect if running in Docker
if [ -f "/.dockerenv" ] || grep -q docker /proc/1/cgroup 2>/dev/null; then
    echo "🐳 Running in Docker environment..."
    HASH_DIR="/var/lib/dipa-auto"
    CONFIG_PATH="${CONFIG_PATH:-/app/config.toml}"

    # Ensure proper permissions on hash directory
    mkdir -p "$HASH_DIR"
    chmod 755 "$HASH_DIR"

    # Validate config file exists
    if [ ! -f "$CONFIG_PATH" ]; then
        echo "❌ Config file not found at $CONFIG_PATH"
        echo "Please mount your config.toml file to $CONFIG_PATH"
        exit 1
    fi

    # Handle hash file migration
    if [ -f "$HASH_DIR/branch_hashes.json" ]; then
        echo "🔍 Checking if branch hashes need migration..."
        
        # Check if JSON already has the new format
        if ! grep -q '"branches"' "$HASH_DIR/branch_hashes.json"; then
            echo "🔄 Migrating hash file format..."
            cp "$HASH_DIR/branch_hashes.json" "$HASH_DIR/branch_hashes.backup.json"
            
            # Initialize with new format
            echo '{"branches":{"stable":{"hash":"","dispatches":{}},"testflight":{"hash":"","dispatches":{}}}}' > "$HASH_DIR/branch_hashes.json"
            echo "✅ Hash file migration complete"
        else
            echo "✅ Hash file already in current format"
        fi
    else
        echo "📝 Creating initial branch hashes file..."
        echo '{"branches":{"stable":{"hash":"","dispatches":{}},"testflight":{"hash":"","dispatches":{}}}}' > "$HASH_DIR/branch_hashes.json"
    fi

    echo "✅ Initialization complete, starting dipa-auto..."
    echo "----------------------------------------------"

    # Execute the main binary (used in Docker)
    exec "/app/dipa-auto" "$@"
    
else
    # Standard installation for systemd environments
    echo "🚀 Setting up dipa-auto for standard installation..."

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

    echo "📝 Ensuring dependencies are up-to-date..."
    cd "$SCRIPT_DIR"
    go mod tidy

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
fi
