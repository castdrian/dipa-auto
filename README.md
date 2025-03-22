# dipa-auto

A Go service that monitors Discord ipa releases and triggers automated workflows.

![screenshot](https://adriancastro.dev/s347b2zng5g6.png)

## Features

- Monitors both stable and testflight branches
- Timed checks for new versions
- Automatic GitHub workflow dispatch
- Systemd service integration
- Written in Go for high performance and low memory usage

## Setup Options

### Standard Installation

```sh
git clone https://github.com/castdrian/dipa-auto
cd dipa-auto && sudo chmod +x setup.sh && sudo ./setup.sh
```

### Docker Installation

1. Clone the repository:
```sh
git clone https://github.com/castdrian/dipa-auto
cd dipa-auto
```

2. Create your configuration file:
```sh
cp example.config.toml config.toml
```

3. Start with Docker Compose:
```sh
docker compose up -d
```

4. Check logs:
```sh
docker compose logs -f
```

The Docker setup includes:
- Automatic initialization and configuration
- Hash file migration (if upgrading from previous versions)
- Persistent storage for tracking dispatched updates
- Healthcheck to ensure the service is running properly

## Migrating from standard to Docker

If you're moving from a standard installation to Docker:

1. Stop the systemd service:
```sh
sudo systemctl stop dipa-auto
sudo systemctl disable dipa-auto
```

2. Edit the compose.yml to mount your existing hash directory:
```yaml
volumes:
  - /var/lib/dipa-auto:/var/lib/dipa-auto
```

3. Start with Docker Compose:
```sh
docker compose up -d
```

## Requirements

### For Standard Installation
- Go 1.18 or higher
- Linux system with systemd

### For Docker Installation
- Docker and Docker Compose

## Configuration

Example configuration (config.toml):

```toml
# ipa service configuration
ipa_base_url = "https://ipa.example.com"

# service configuration
refresh_schedule = "0,15,30,45 * * * *" # every quarter hour (00,15,30,45)

# target repo configuration
[[targets]]
github_repo = "user/repo"
github_token = "github_pat_..."

[[targets]]
github_repo = "org/repo"
github_token = "github_pat_..."
```

## License

© Adrian Castro 2024. All rights reserved.\
© magic-ipa-source-caddy-server person 2024. All rights reserved.
