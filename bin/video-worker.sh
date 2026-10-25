#!/bin/bash
FILE_PATH="$1"
B="/opt/media-optimizer/bin"
DB="/opt/media-optimizer/db/media-optimizer.sqlite"
LOG="/opt/media-optimizer/logs/video-worker.log"
log() { echo "$(date '+%H:%M:%S') [VIDEO-X265] $*" >> "$LOG"; }
ESC=${FILE_PATH//\'/\'\'}

report() {
  local pct="$1" stage="$2" fn; fn=$(basename "$FILE_PATH")
  "$B/state_set" current_compression_node "{\"file\":\"$fn\",\"type\":\"video\",\"stage\":\"$stage\",\"percent\":$pct,\"source\":\"video-worker-x265\"}"
  sqlite3 "$DB" "UPDATE files SET last_progress=$pct, updated_at=datetime('now') WHERE path='$ESC';"
}

ORIG_KB=$(du -k "$FILE_PATH" 2>/dev/null | cut -f1); ORIG_KB=${ORIG_KB:-0}
ORIG_MB=$(awk "BEGIN{printf \"%.2f\", $ORIG_KB/1024}")
sqlite3 "$DB" "UPDATE files SET status='PROCESSING', original_size_mb=$ORIG_MB, updated_at=datetime('now') WHERE path='$ESC';"
report 3 probing

DUR=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$FILE_PATH" 2>/dev/null | cut -d. -f1)
[[ "$DUR" =~ ^[0-9]+$ ]] || DUR=0
OUT="${FILE_PATH}.tmp_opt.mp4"
PF="/tmp/.ffp_$$"

# --- X265 CONFIGURATION FOR ODROID C2 ---
# Preset: ultrafast (Agar CPU tidak terlalu berat, meski hasil kurang optimal dibanding slower presets)
# CRF: 28 (Kualitas standar arsip)
# Tune: zerolatency (Opsional, tapi membantu stabilitas stream)
ffmpeg -y -i "$FILE_PATH" \
    -c:v libx265 -preset ultrafast -crf 28 \
    -c:a aac -b:a 128k \
    -movflags +faststart \
    -progress "$PF" -nostats \
    "$OUT" >/dev/null 2>&1 &
FFPID=$!

(
  while true; do
    if [ -f "$PF" ]; then
      MS=$(grep -oE 'out_time_ms=[0-9]+' "$PF" | tail -1 | cut -d= -f2)
      if [ -n "$MS" ] && [ "$DUR" -gt 0 ]; then
        CUR=$((MS/1000000)); PCT=$((CUR*90/DUR))
        [ $PCT -gt 90 ] && PCT=90; [ $PCT -lt 5 ] && PCT=5
        report $PCT encoding
      fi
    fi
    kill -0 $FFPID 2>/dev/null || break
    sleep 2
  done
) &
PARSEPID=$!

wait $FFPID; CODE=$?
kill $PARSEPID 2>/dev/null
rm -f "$PF"

if [ $CODE -ne 0 ] || [ ! -f "$OUT" ]; then
  log "Encode FAILED ($CODE) $FILE_PATH"; rm -f "$OUT"
  sqlite3 "$DB" "UPDATE files SET status='ERROR', category='ENCODE_FAIL_X265' WHERE path='$ESC';"
  "$B/state_set" current_compression_node "{}"; exit 1
fi

report 95 verifying
FIN_KB=$(du -k "$OUT" 2>/dev/null | cut -f1); FIN_KB=${FIN_KB:-0}
FIN_MB=$(awk "BEGIN{printf \"%.2f\", $FIN_KB/1024}")

# --- CATEGORIZATION LOGIC (SAME AS BEFORE) ---
CATEGORY="EFISIEN"
SAVED_BYTES=$((ORIG_KB - FIN_KB))
PERCENT_SAVED=0
if [ $ORIG_KB -gt 0 ]; then PERCENT_SAVED=$((SAVED_BYTES * 100 / ORIG_KB)); fi

if [ $FIN_KB -ge $ORIG_KB ]; then
    CATEGORY="BOROS"
elif [ $FIN_KB -lt 800 ]; then
    CATEGORY="IRIT"
elif [ $FIN_KB -lt 1024 ]; then
    CATEGORY="HEMAT_PLUS"
elif [ $FIN_KB -lt 2048 ]; then
    CATEGORY="HEMAT"
else
    CATEGORY="EFISIEN"
fi

if [ "$CATEGORY" = "BOROS" ]; then
    rm -f "$OUT"
    sqlite3 "$DB" "UPDATE files SET status='SKIPPED_BOROS', final_size_mb=$FIN_MB, category='BOROS', percent_saved=$PERCENT_SAVED, updated_at=datetime('now') WHERE path='$ESC';"
    log "SKIPPED (Boros): $FILE_PATH Orig:${ORIG_KB}KB New:${FIN_KB}KB. Original preserved."
else
    mv -f "$OUT" "$FILE_PATH"
    sqlite3 "$DB" "UPDATE files SET status='DONE_COMPRESSED', final_size_mb=$FIN_MB, category='$CATEGORY', percent_saved=$PERCENT_SAVED, needs_backup=1, backup_status=NULL, last_progress=100, updated_at=datetime('now') WHERE path='$ESC';"
    log "SUCCESS [$CATEGORY]: $FILE_PATH ${ORIG_MB}MB -> ${FIN_MB}MB (Saved ${PERCENT_SAVED}%) [X265]"
fi

"$B/state_set" current_compression_node "{}"
