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
mkdir -p "$TARGET/bin" "$TARGET/ui"
mkdir -p "$TARGET/db" "$TARGET/logs"
cp -a "$TMP/code/bin/." "$TARGET/bin/"
cp -a "$TMP/code/ui/." "$TARGET/ui/"
if [ "$TARGET" = "/opt/media-optimizer" ]; then
  SVC=/etc/systemd/system
  cp -a "$TMP/code/systemd/." "$SVC/" 2>/dev/null
  systemctl daemon-reload 2>/dev/null
fi
DBF="$TARGET/db/media-optimizer.sqlite"
SQL="$TMP/data/db/backup.sql"
if [ -f "$SQL" ]; then
  [ -f "$DBF" ] && mv "$DBF" "$DBF.pre-$(date +%s)"
  sqlite3 "$DBF" < "$SQL" 2>/dev/null
  [ $? -ne 0 ] && echo "WARN: load dump gagal"
fi
chmod +x "$TARGET"/bin/*.sh 2>/dev/null
echo "RESTORE DONE -> $TARGET"
mkdir -p "$TARGET/bin" "$TARGET/ui"
mkdir -p "$TARGET/db" "$TARGET/logs"
cp -a "$TMP/code/bin/." "$TARGET/bin/"
cp -a "$TMP/code/ui/." "$TARGET/ui/"
if [ "$TARGET" = "/opt/media-optimizer" ]; then
  SVC=/etc/systemd/system
  cp -a "$TMP/code/systemd/." "$SVC/" 2>/dev/null
  systemctl daemon-reload 2>/dev/null
fi
DBF="$TARGET/db/media-optimizer.sqlite"
SQL="$TMP/data/db/backup.sql"
if [ -f "$SQL" ]; then
  [ -f "$DBF" ] && mv "$DBF" "$DBF.pre-$(date +%s)"
  sqlite3 "$DBF" < "$SQL" 2>/dev/null
  [ $? -ne 0 ] && echo "WARN: load dump gagal"
fi
chmod +x "$TARGET"/bin/*.sh 2>/dev/null
echo "RESTORE DONE -> $TARGET"
