#!/bin/bash
set -uo pipefail
SRC=/opt/media-optimizer
BK=/opt/mo-backup
CODE="$BK/code"
DATA="$BK/data"
CFG="$BK/config"
LOG="$SRC/logs/backup.log"
KEY="/home/dietpi/.ssh/mo_backup_key"
CB=backup-code
DB=backup-data
[ -f "$CFG" ] && . "$CFG"
: "${GH_REPO:?missing}"
GSC="ssh -i $KEY -o IdentitiesOnly=yes"
GSC="$GSC -o StrictHostKeyChecking=accept-new"
export GIT_SSH_COMMAND="$GSC"
ts(){ date '+%F %T'; }
log(){ echo "$(ts) [BAK] $*" >> "$LOG"; }
fail(){ log "ERROR: $*"; echo "ERROR: $*" >&2; exit 1; }
mkdir -p "$BK" "$SRC/logs"
if [ ! -d "$CODE/.git" ]; then
  git init -q "$CODE" || fail "init code"
  git -C "$CODE" remote add origin "$GH_REPO" 2>/dev/null
  git -C "$CODE" remote set-url origin "$GH_REPO" 2>/dev/null
  git -C "$CODE" config user.name mo-backup
  git -C "$CODE" config user.email mo-backup@localhost
  git -C "$CODE" symbolic-ref HEAD "refs/heads/$CB"
fi
rm -rf "$CODE/bin" "$CODE/ui"
rm -rf "$CODE/systemd" "$CODE/logs-snap"
mkdir -p "$CODE/bin" "$CODE/ui"
mkdir -p "$CODE/systemd" "$CODE/logs-snap"
cp -a "$SRC/bin/." "$CODE/bin/" 2>>"$LOG"
cp -a "$SRC/ui/." "$CODE/ui/" 2>>"$LOG"
SVC=/etc/systemd/system
cp -a "$SVC/media-optimizer.service" "$CODE/systemd/" 2>>"$LOG"
cp -a "$SVC/media-optimizer-ui.service" "$CODE/systemd/" 2>>"$LOG"
for L in supervisor video-worker photo-worker; do
  F="$SRC/logs/$L.log"
  [ -f "$F" ] && tail -n 200 "$F" > "$CODE/logs-snap/$L.log"
done
for L in backup-rclone scanner; do
  F="$SRC/logs/$L.log"
  [ -f "$F" ] && tail -n 200 "$F" > "$CODE/logs-snap/$L.log"
done
git -C "$CODE" add -A
if ! git -C "$CODE" diff --cached --quiet; then
  git -C "$CODE" commit -q -m "code $(ts)" || fail "commit code"
  git -C "$CODE" push -q origin "$CB" || fail "push code"
  log "code pushed ($CB)"
else
  log "code unchanged"
fi
if [ ! -d "$DATA/.git" ]; then
  git init -q "$DATA" || fail "init data"
  git -C "$DATA" remote add origin "$GH_REPO" 2>/dev/null
  git -C "$DATA" remote set-url origin "$GH_REPO" 2>/dev/null
  git -C "$DATA" config user.name mo-backup
  git -C "$DATA" config user.email mo-backup@localhost
  git -C "$DATA" symbolic-ref HEAD "refs/heads/$DB"
fi
mkdir -p "$DATA/db"
TMP="$(mktemp)"
sqlite3 "$SRC/db/media-optimizer.sqlite" ".backup $TMP"
[ $? -ne 0 ] && { rm -f "$TMP"; fail "sqlite backup"; }
sqlite3 "$TMP" .dump > "$DATA/db/backup.sql"
[ $? -ne 0 ] && { rm -f "$TMP"; fail "sqlite dump"; }
rm -f "$TMP"
git -C "$DATA" add -A
TREE="$(git -C "$DATA" write-tree)"
COMMIT="$(git -C "$DATA" commit-tree "$TREE" -m "data $(ts)")"
git -C "$DATA" update-ref "refs/heads/$DB" "$COMMIT"
git -C "$DATA" symbolic-ref HEAD "refs/heads/$DB"
git -C "$DATA" push -q --force origin "$DB" || fail "push data"
log "data pushed ($DB force)"
log "DONE"
