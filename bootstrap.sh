#!/usr/bin/env bash
set -euo pipefail

# Pinned local bootstrap: runs the install.sh that sits next to this file
# instead of downloading the latest one from GitHub.
# Upstream original: https://github.com/Awassee/wolfbbs/blob/main/bootstrap.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER_PATH="${SCRIPT_DIR}/install.sh"

if [[ ! -f "$INSTALLER_PATH" ]]; then
  echo "WolfBBS bootstrap: no install.sh found next to this script (${SCRIPT_DIR})."
  exit 1
fi

echo "WolfBBS bootstrap: using local installer ${INSTALLER_PATH}"
exec bash "$INSTALLER_PATH" "$@"
