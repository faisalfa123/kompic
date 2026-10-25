#!/bin/bash
FILE_ID="$1"
B="/opt/media-optimizer/bin"
DB="/opt/media-optimizer/db/media-optimizer.sqlite"
LOG="/opt/media-optimizer/logs/backup-rclone.log"
log() { echo "$(date '+%H:%M:%S') [BACKUP] $*" >> "$LOG"; }

PATH_SRC=$(sqlite3 "$DB" "SELECT path FROM files WHERE id=$FILE_ID AND needs_backup=1 LIMIT 1;")
if [ -z "$PATH_SRC" ]; then log "No pending backup for id $FILE_ID"; exit 0; fi
ESC=${PATH_SRC//\'/\'\'}
FN=$(basename "$PATH_SRC")

REMOTE=$("$B/get_setting" rclone_remote "mega:")
LOCAL_ROOT=$("$B/get_setting" local_root "/mnt/hdd1")
REL=${PATH_SRC#$LOCAL_ROOT/}
DEV=$(echo "$REL" | cut -d'/' -f1)
DATESTR=$(date +%Y/%m/%d)
DEST="${REMOTE}Backup_Media_Optimizer/${DEV}/${DATESTR}/${FN}"

log "Upload start: $PATH_SRC -> $DEST"
sqlite3 "$DB" "UPDATE files SET backup_status='UPLOADING', updated_at=datetime('now') WHERE id=$FILE_ID;"

OUT="/tmp/.rclone_out_$$"
rclone copyto "$PATH_SRC" "$DEST" --stats 5s --stats-one-line -v > "$OUT" 2>&1 &
RPID=$!

LAST_PCT=-1
STALL=$(date +%s)
RESULT="ok"
while kill -0 $RPID 2>/dev/null; do
  sleep 3
  PCT=$(grep -oE '[0-9]+%' "$OUT" | tail -1 | tr -d '%')
  [ -z "$PCT" ] && PCT=0
  NOW=$(date +%s)
  if [ "$PCT" != "$LAST_PCT" ]; then LAST_PCT=$PCT; STALL=$NOW
  elif [ $((NOW-STALL)) -ge 60 ]; then
    log "STALL 60s at ${PCT}%, killing rclone $RPID"
    kill -TERM $RPID 2>/dev/null; sleep 2; kill -KILL $RPID 2>/dev/null
    RESULT="timeout"; break
  fi
  "$B/state_set" current_backup_node "{\"file\":\"$FN\",\"percent\":$PCT,\"status\":\"uploading\",\"source\":\"rclone-worker\"}"
done
wait $RPID 2>/dev/null; CODE=$?
rm -f "$OUT"

if [ "$RESULT" = "timeout" ]; then
  sqlite3 "$DB" "UPDATE files SET backup_status='ERROR_T_OUT', needs_backup=1, updated_at=datetime('now') WHERE id=$FILE_ID;"
  "$B/state_set" current_backup_node "{\"error\":\"T_OUT\",\"file\":\"$FN\"}"
  log "TIMEOUT $FN (kept in queue for retry)"
  exit 1
elif [ $CODE -eq 0 ]; then
  sqlite3 "$DB" "UPDATE files SET backup_status='SUCCESS', needs_backup=0, updated_at=datetime('now') WHERE id=$FILE_ID;"
  "$B/state_set" current_backup_node "{}"
  log "SUCCESS $FN -> $DEST"
else
  sqlite3 "$DB" "UPDATE files SET backup_status='ERROR_RCLONE_FAIL', needs_backup=1, updated_at=datetime('now') WHERE id=$FILE_ID;"
  "$B/state_set" current_backup_node "{\"error\":\"FAIL_$CODE\",\"file\":\"$FN\"}"
  log "FAIL($CODE) $FN"
fi
