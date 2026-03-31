#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR/scripts/validation"

exec go run ./replay_block_audit \
    -network=obtcregtest \
    -rpchost=127.0.0.1:18556 \
    -rpcuser=obtc \
    -rpcpass=obtcpass \
    -check-reap-selection \
    "$@"
