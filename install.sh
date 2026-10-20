#!/usr/bin/env bash
# Crush + Harness² Root Installation Entrypoint
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$DIR/scripts/install.sh" "$@"
