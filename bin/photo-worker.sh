#!/bin/bash
FILE_PATH="$1"
B="/opt/media-optimizer/bin"
DB="/opt/media-optimizer/db/media-optimizer.sqlite"
LOG="/opt/media-optimizer/logs/photo-worker.log"
log() { echo "$(date '+%H:%M:%S') [PHOTO] $*" >> "$LOG"; }
ESC=${FILE_PATH//\'/\'\'}

report() {
  local pct="$1" stage="$2" fn; fn=$(basename "$FILE_PATH")
  "$B/state_set" current_compression_node "{\"file\":\"$fn\",\"type\":\"photo\",\"stage\":\"$stage\",\"percent\":$pct,\"source\":\"photo-worker\"}"
  sqlite3 "$DB" "UPDATE files SET last_progress=$pct, updated_at=datetime('now') WHERE path='$ESC';"
}

ORIG_KB=$(du -k "$FILE_PATH" 2>/dev/null | cut -f1); ORIG_KB=${ORIG_KB:-0}
ORIG_MB=$(awk "BEGIN{printf \"%.2f\", $ORIG_KB/1024}")
sqlite3 "$DB" "UPDATE files SET status='PROCESSING', original_size_mb=$ORIG_MB, updated_at=datetime('now') WHERE path='$ESC';"
report 5 reading

EXT="${FILE_PATH##*.}"; EXT=$(echo "$EXT" | tr '[:upper:]' '[:lower:]')
OUT="${FILE_PATH}.tmp_opt"
report 40 converting

case "$EXT" in
  jpg|jpeg) magick "$FILE_PATH" -quality 82 -strip "$OUT" >/dev/null 2>&1 ;;
  png)      magick "$FILE_PATH" -strip -define png:compression-level=9 "$OUT" >/dev/null 2>&1 ;;
  webp)     cwebp -quiet -q 80 "$FILE_PATH" -o "$OUT" >/dev/null 2>&1 ;;
  heic)     magick "$FILE_PATH" -quality 85 -strip "$OUT" >/dev/null 2>&1 ;;
  *) log "Unsupported ext $EXT"; sqlite3 "$DB" "UPDATE files SET status='ERROR', category='UNSUPPORTED_EXT' WHERE path='$ESC';"; "$B/state_set" current_compression_node "{}"; exit 1 ;;
esac
CODE=$?

if [ $CODE -ne 0 ] || [ ! -f "$OUT" ]; then
  log "Convert failed ($CODE) $FILE_PATH"; rm -f "$OUT"
  # Increment retry handled by supervisor zombie check, but here we mark error temporarily
  sqlite3 "$DB" "UPDATE files SET status='ERROR', category='CONVERT_FAIL' WHERE path='$ESC';"
  "$B/state_set" current_compression_node "{}"; exit 1
fi

report 90 verifying
FIN_KB=$(du -k "$OUT" 2>/dev/null | cut -f1); FIN_KB=${FIN_KB:-0}
FIN_MB=$(awk "BEGIN{printf \"%.2f\", $FIN_KB/1024}")

# --- CATEGORIZATION LOGIC ---
CATEGORY="EFISIEN" # Default if smaller
SAVED_BYTES=$((ORIG_KB - FIN_KB))
PERCENT_SAVED=0
if [ $ORIG_KB -gt 0 ]; then PERCENT_SAVED=$((SAVED_BYTES * 100 / ORIG_KB)); fi

if [ $FIN_KB -ge $ORIG_KB ]; then
    CATEGORY="BOROS"
elif [ $FIN_KB -lt 800 ]; then
    CATEGORY="IRIT"
elif [ $FIN_KB -lt 1024 ]; then # < 1MB
    CATEGORY="HEMAT_PLUS"
elif [ $FIN_KB -lt 2048 ]; then # < 2MB
    CATEGORY="HEMAT"
else
    CATEGORY="EFISIEN"
fi

if [ "$CATEGORY" = "BOROS" ]; then
    # KEEP ORIGINAL, DELETE TMP, MARK STATUS
    rm -f "$OUT"
    sqlite3 "$DB" "UPDATE files SET status='SKIPPED_BOROS', final_size_mb=$FIN_MB, category='BOROS', percent_saved=$PERCENT_SAVED, updated_at=datetime('now') WHERE path='$ESC';"
    log "SKIPPED (Boros): $FILE_PATH Orig:${ORIG_KB}KB New:${FIN_KB}KB. Original preserved."
else
    # REPLACE ORIGINAL WITH COMPRESSED
    mv -f "$OUT" "$FILE_PATH"
    sqlite3 "$DB" "UPDATE files SET status='DONE_COMPRESSED', final_size_mb=$FIN_MB, category='$CATEGORY', percent_saved=$PERCENT_SAVED, needs_backup=1, backup_status=NULL, last_progress=100, updated_at=datetime('now') WHERE path='$ESC';"
    log "SUCCESS [$CATEGORY]: $FILE_PATH ${ORIG_MB}MB -> ${FIN_MB}MB (Saved ${PERCENT_SAVED}%)"
fi

"$B/state_set" current_compression_node "{}"
