#!/bin/bash
set -e

echo "🚀 Setting up dipa-auto..."

if ! command -v python3 &> /dev/null; then
    echo "📦 Python 3 not found, installing..."
    if command -v apt &> /dev/null; then
        sudo apt update
        sudo apt install -y python3 python3-venv
    elif command -v dnf &> /dev/null; then
        sudo dnf install -y python3 python3-venv
    elif command -v pacman &> /dev/null; then
        sudo pacman -Sy python python-venv
    elif command -v brew &> /dev/null; then
        brew install python
    else
        echo "❌ Could not find a package manager to install Python. Please install Python 3 manually."
        exit 1
    fi
fi

VENV_DIR="venv"
SERVICE_NAME="dipa-auto"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HASH_DIR="/var/lib/dipa-auto"
CONFIG_FILE="$SCRIPT_DIR/config.toml"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Config file not found at $CONFIG_FILE"
    exit 1
fi

python3 -m venv "$VENV_DIR"
source "$VENV_DIR/bin/activate"

pip install requests tomli zon

echo "📝 Validating config..."
python3 << END
import tomli
import zon
import sys

CONFIG_SCHEMA = zon.record({
    "github_token": zon.string().min(1),
    "ipa_base_url": zon.string().url(),
    "repo_name": zon.string().regex(r"^[a-zA-Z0-9-]+/[a-zA-Z0-9-]+$"),
    "refresh_interval": zon.number().int().positive()
})

try:
    with open("$CONFIG_FILE", "rb") as f:
        config = tomli.load(f)
    CONFIG_SCHEMA.validate(config)
    print("✅ Config validation successful")
except Exception as e:
    print(f"❌ Config validation failed: {e}")
    sys.exit(1)
END

sudo mkdir -p "$HASH_DIR"
sudo chown -R $USER:$USER "$HASH_DIR"

echo "📝 Checking branch hashes..."
if [ -f "$HASH_DIR/branch_hashes.json" ]; then
    echo "✨ Using existing branch hashes"
else
    echo "📝 Creating initial branch hashes..."
    python3 << END
import requests
import json
import hashlib
import tomli

with open("$CONFIG_FILE", "rb") as f:
    config = tomli.load(f)

def get_branch_hash(branch):
    response = requests.get(
        f"{config['ipa_base_url']}/{branch}/",
        headers={"Accept": "application/json"}
    )
    data = response.json()
    return hashlib.sha256(json.dumps(data, sort_keys=True).encode()).hexdigest()

hashes = {
    "stable": get_branch_hash("stable"),
    "testflight": get_branch_hash("testflight")
}

with open("$HASH_DIR/branch_hashes.json", "w") as f:
    json.dump(hashes, f)
END
fi

echo "📝 Creating systemd service..."
cat > /tmp/dipa-auto.service << END
[Unit]
Description=dipa-auto service
After=network.target

[Service]
Type=simple
User=$USER
Environment="CONFIG_PATH=$CONFIG_FILE"
WorkingDirectory=$SCRIPT_DIR
ExecStart=$SCRIPT_DIR/$VENV_DIR/bin/python3 -m dipa_auto
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

