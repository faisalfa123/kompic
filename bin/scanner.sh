#!/usr/bin/env bash
B="/opt/media-optimizer/bin"
DB="/opt/media-optimizer/db/media-optimizer.sqlite"
LOG="$B/../logs/scanner.log"

log() { echo "$(date '+%Y-%m-%d %H:%M:%S') [SCANNER] $*" >> "$LOG"; }

get_val() { $B/get_setting "$1" "$2"; }

ROOT_PATH=$(get_val local_root "/mnt/hdd1")
EXT_PHOTO=$(get_val extensions_photo ".jpg,.jpeg,.png,.heic,.webp")
EXT_VIDEO=$(get_val extensions_video ".mp4,.mov,.mkv,.avi")
MIN_SIZE_KB=$(get_val min_size_kb "100")

log "Starting Scan on: $ROOT_PATH"

if [[ ! -d "$ROOT_PATH" ]]; then
    log "ERROR: Root path $ROOT_PATH does not exist."
    exit 1
fi

build_find_args() {
    local exts="$1"
    local args=""
    IFS=',' read -ra ADDR <<< "$exts"
    for i in "${ADDR[@]}"; do
        i="${i// /}"
        [[ -z "$i" ]] && continue
        args+=" -iname '*${i}' -o"
    done
    args="${args% -o}"
    echo "$args"
}

PHOTO_ARGS=$(build_find_args "$EXT_PHOTO")
VIDEO_ARGS=$(build_find_args "$EXT_VIDEO")

process_files() {
    local type_label=$1
    shift
    local search_cmd=$@
    
    eval "$search_cmd" | while read -r FILE; do
        [[ -z "$FILE" ]] && continue
        
        SIZE_KB=$(du -k "$FILE" | cut -f1)
        
        if (( SIZE_KB < MIN_SIZE_KB )); then
            continue
        fi
        
        EXISTS=$(sqlite3 "$DB" "SELECT count(*) FROM files WHERE path='$FILE';")
        
        if (( EXISTS == 0 )); then
            ORIG_MB=$(awk "BEGIN {printf \"%.2f\", $SIZE_KB/1024}")
            sqlite3 "$DB" "INSERT INTO files (path, media_type, status, original_size_mb, created_at, updated_at) VALUES ('$FILE', '$type_label', 'PENDING', $ORIG_MB, datetime('now'), datetime('now'));"
            log "Added NEW: [$type_label] $FILE (${SIZE_KB}KB)"
        fi
    done
}

if [[ -n "$PHOTO_ARGS" ]]; then
    FIND_CMD="find \"$ROOT_PATH\" -type f \( $PHOTO_ARGS \)"
    process_files "photo" "$FIND_CMD"
fi

if [[ -n "$VIDEO_ARGS" ]]; then
    FIND_CMD="find \"$ROOT_PATH\" -type f \( $VIDEO_ARGS \)"
    process_files "video" "$FIND_CMD"
fi

log "Scan Complete."
