# dipa-auto

A Go service that monitors Discord ipa releases and triggers automated workflows.

![screenshot](https://adriancastro.dev/s347b2zng5g6.png)

## Features

- Monitors both stable and testflight branches
- Timed checks for new versions
- Automatic GitHub workflow dispatch
- Systemd service integration
- Written in Go for high performance and low memory usage

## Setup

```sh
git clone https://github.com/castdrian/dipa-auto
cd dipa-auto && sudo chmod +x setup.sh && sudo ./setup.sh
```

## Requirements

- Go 1.18 or higher
- Linux system with systemd

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
