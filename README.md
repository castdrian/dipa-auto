# dipa-auto

A Linux service that monitors Discord IPA releases and triggers automated workflows.

![screenshot](https://adriancastro.dev/s347b2zng5g6.png)

## Features

- Monitors both stable and testflight branches
- Hourly checks for new versions
- Automatic GitHub workflow dispatch
- Systemd service integration

## Setup

```sh
git clone https://github.com/castdrian/dipa-auto
cd dipa-auto && sudo chmod +x setup.sh && sudo ./setup.sh
```

## License

© Adrian Castro 2024. All rights reserved.\
© magic-ipa-source-caddy-server person 2024. All rights reserved.
