#!/bin/bash
set -uo pipefail
BK=/opt/mo-backup
CFG="$BK/config"
[ -f "$CFG" ] && . "$CFG"
: "${GH_REPO:?missing}"
KEY="/home/dietpi/.ssh/mo_backup_key"
CB=backup-code
DB=backup-data
TARGET="${1:-/opt/media-optimizer}"
GSC="ssh -i $KEY -o IdentitiesOnly=yes"
GSC="$GSC -o StrictHostKeyChecking=accept-new"
export GIT_SSH_COMMAND="$GSC"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
git clone -q --branch "$CB" "$GH_REPO" "$TMP/code"
[ $? -ne 0 ] && { echo "ERR: no $CB"; exit 1; }
git clone -q --branch "$DB" "$GH_REPO" "$TMP/data"
[ $? -ne 0 ] && { echo "ERR: no $DB"; exit 1; }
