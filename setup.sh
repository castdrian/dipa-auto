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

if [ -n "$GITHUB_ACTIONS" ]; then
    echo "🧪 Running in GitHub Actions environment..."
    GITHUB_PAT="$github_token" python3 "$SCRIPT_DIR/mock_test.py"
    exit 0
fi

SERVICE_CONTENT="[Unit]
Description=dipa-auto service
After=network.target

[Service]
Type=simple
User=$USER
Environment=GITHUB_PAT=$github_token
WorkingDirectory=$SCRIPT_DIR
ExecStart=$SCRIPT_DIR/$VENV_DIR/bin/python3 -m dipa_auto.checker
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target"

echo "📝 Creating systemd service file..."
echo "$SERVICE_CONTENT" | sudo tee "/etc/systemd/system/$SERVICE_NAME.service" > /dev/null

echo "🔄 Reloading systemd daemon..."
sudo systemctl daemon-reload

echo "▶️ Starting service..."
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl start "$SERVICE_NAME"

echo "✅ Setup complete!"

