#!/bin/bash
set -e

echo "🚀 Setting up dipa-auto..."

if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3 is required but not installed."
    exit 1
fi

VENV_DIR="venv"
SERVICE_NAME="dipa-auto"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HASH_DIR="/var/lib/dipa-auto"

if [ "$1" ]; then
    github_token="$1"
else
    read -sp "GitHub PAT: " github_token
    echo
fi

python3 -m venv "$VENV_DIR"
source "$VENV_DIR/bin/activate"

pip install requests

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

def get_branch_hash(branch):
    response = requests.get(
        f"https://ipa.aspy.dev/discord/{branch}/",
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
Environment="REPO_PAT=$github_token"
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

