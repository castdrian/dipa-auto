#!/bin/bash
set -e

if [ ! -f .secrets ]; then
    echo "Error: .secrets file not found"
    exit 1
fi

act push -W .github/workflows/test-setup.yml --container-architecture linux/arm64 --secret-file .secrets 