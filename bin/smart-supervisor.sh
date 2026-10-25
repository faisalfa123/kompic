#!/bin/bash
B="/opt/media-optimizer/bin"
DB="/opt/media-optimizer/db/media-optimizer.sqlite"
LOG="/opt/media-optimizer/logs/supervisor.log"
mkdir -p "$(dirname "$LOG")"
GT="$B/get_temp.sh"
log() { echo "$(date '+%H:%M:%S') [SUP] $*" >> "$LOG"; }
get_val() { "$B/get_setting" "$1" "$2"; }

# --- KONFIGURASI LIMIT WORKER ---
MAX_PHOTO=3
MAX_VIDEO=1

while true; do
    HEARTBEAT=$(date +%s)
    "$B/state_set" heartbeat "$HEARTBEAT"
    
    # --- METRICS COLLECTION ---
    TEMP=$($GT)
    RAM_TOTAL=$(free -m | awk '/^Mem:/{print $2}')
    RAM_USED=$(free -m | awk '/^Mem:/{print $3}')
    CPU_LOAD=$("$B/get_cpu_percent.sh")
    
    "$B/state_set" cpu_temp "$TEMP"
    "$B/state_set" ram_used "$RAM_USED"
    "$B/state_set" ram_total "$RAM_TOTAL"
    "$B/state_set" cpu_load "$CPU_LOAD"
    
    ROOT_PATH=$(get_val local_root "/mnt/hdd1")
    DF_INFO=$(df -BM "$ROOT_PATH" 2>/dev/null | tail -1)
    STORAGE_FREE_GB=$(echo "$DF_INFO" | awk '{gsub(/M/,"",$4); print int($4/1024)}')
    STORAGE_TOTAL_GB=$(echo "$DF_INFO" | awk '{gsub(/M/,"",$2); print int($2/1024)}')
    "$B/state_set" storage_free_gb "$STORAGE_FREE_GB"
    "$B/state_set" storage_total_gb "$STORAGE_TOTAL_GB"

    # --- MANUAL PAUSE CHECK ---
    MP=$(get_val manual_pause "false")
    if [[ "$MP" == "true" ]]; then
        "$B/state_set" state "MANUAL_PAUSE"
        pkill -TERM -f "photo-worker.sh" || true
        pkill -TERM -f "video-worker.sh" || true
        pkill -TERM -f "rclone-backup-worker.sh" || true
        sleep 5
        continue
    fi

    # --- THERMAL GUARD ---
    PAUSE_TEMP=$(get_val temp_pause 80)
    RESUME_TEMP=$(get_val temp_resume 65)
    RAW_STATE=$(sqlite3 "$DB" "SELECT value FROM runtime_state WHERE key='state';")
    [[ -z "$RAW_STATE" ]] && RAW_STATE="IDLE"

    if (( TEMP >= PAUSE_TEMP )); then
        if [[ "$RAW_STATE" != "OVERHEAT" ]]; then
            log "🔥 OVERHEAT: ${TEMP}C >= ${PAUSE_TEMP}C. Pausing Compression."
            "$B/state_set" state "OVERHEAT"
            pkill -TERM -f "photo-worker.sh" || true
            pkill -TERM -f "video-worker.sh" || true
            "$B/state_set" current_compression_node "{\"status\":\"paused_thermal\"}"
        fi
        
        NEEDS_BACKUP=$(sqlite3 "$DB" "SELECT id FROM files WHERE needs_backup=1 AND backup_status!='UPLOADING' ORDER BY id ASC LIMIT 1;")
        if [[ -n "$NEEDS_BACKUP" ]] && ! pgrep -f "rclone-backup-worker.sh" > /dev/null; then
            log "💾 Thermal Pause. Spawning Backup for ID $NEEDS_BACKUP."
            nohup "$B/rclone-backup-worker.sh" "$NEEDS_BACKUP" >/dev/null 2>&1 &
        fi
        sleep 15
        continue
    elif (( TEMP <= RESUME_TEMP )); then
        if [[ "$RAW_STATE" == "OVERHEAT" ]]; then
            log "️ Temp OK (${TEMP}C). Resuming."
            "$B/state_set" state "PROCESSING_QUEUE"
            "$B/state_set" current_compression_node "{}"
        fi
    fi
    
    # --- SCHEDULE & FORCE MODE LOGIC (FIXED) ---
    FORCE_MODE=$(get_val force_mode "false")
# --- BACKUP SYNC MODE (NORMAL = saat kompresi idle, FORCE = selalu) ---
RAM_PCT=0
if (( RAM_TOTAL > 0 )); then RAM_PCT=$((RAM_USED * 100 / RAM_TOTAL)); fi
BACKUP_PAUSE=$(get_val backup_pause "false")
SYNC_FORCE=$(get_val sync_force_mode "false")
RUNNING_BACKUP=$(pgrep -fc "rclone-backup-worker.sh" 2>/dev/null); : "${RUNNING_BACKUP:=0}"
RUNNING_COMP=$(pgrep -fc "(photo|video)-worker.sh" 2>/dev/null); : "${RUNNING_COMP:=0}"
if [[ "$BACKUP_PAUSE" != "true" ]] && (( RUNNING_BACKUP < 1 )) && (( RAM_PCT < 92 )); then
    if [[ "$SYNC_FORCE" == "true" ]] || (( RUNNING_COMP == 0 )); then
        NEEDS_BACKUP=$(sqlite3 "$DB" "SELECT id FROM files WHERE needs_backup=1 AND (backup_status IS NULL OR backup_status='' OR backup_status LIKE '%FAIL%' OR backup_status LIKE '%ERROR%') ORDER BY id ASC LIMIT 1;")
        if [[ -n "$NEEDS_BACKUP" ]]; then
            if [[ "$SYNC_FORCE" == "true" ]]; then SL="FORCE"; else SL="NORMAL"; fi
            log "☁️ Spawn Backup (mode: $SL): ID $NEEDS_BACKUP"
            nohup "$B/rclone-backup-worker.sh" "$NEEDS_BACKUP" >/dev/null 2>&1 &
        fi
    fi
fi
    HOUR=$(date +%H); MIN=$(date +%M)
    TIME_NOW_HM=$(printf "%02d:%02d" $HOUR $MIN)
    START_SCHED=$(get_val schedule_start "00:00")
    END_SCHED=$(get_val schedule_end "09:00")
    
    SHOULD_RUN=false
    POSITION_STATUS="PENDING" # Default position status

    if [[ "$FORCE_MODE" == "true" ]]; then 
        SHOULD_RUN=true
        POSITION_STATUS="RUNNING (FORCE)"
    else
        # Check if current time is within window
        if [[ "$TIME_NOW_HM" > "$START_SCHED" || "$TIME_NOW_HM" == "$START_SCHED" ]] && [[ "$TIME_NOW_HM" < "$END_SCHED" ]]; then 
            SHOULD_RUN=true
            POSITION_STATUS="RUNNING (SCHEDULE)"
        else
            SHOULD_RUN=false
            POSITION_STATUS="WAITING (SCHEDULE)"
        fi
    fi
    
    # Update Position Status in DB for UI
    "$B/state_set" position_status "$POSITION_STATUS"

    if [[ "$SHOULD_RUN" == false ]]; then
        "$B/state_set" state "IDLE"
        sleep 30
        continue
    fi
    
    # --- SPAWN WORKERS WITH NEW LIMITS (3 Photo / 1 Video) ---
    RAM_PCT=0
    if (( RAM_TOTAL > 0 )); then RAM_PCT=$((RAM_USED * 100 / RAM_TOTAL)); fi
    
    RUNNING_PHOTO=$(pgrep -fc "photo-worker.sh" || true)
    RUNNING_VIDEO=$(pgrep -fc "video-worker.sh" || true)
    
    if (( RAM_PCT < 90 )); then
        if (( RUNNING_PHOTO < MAX_PHOTO )); then
            P_PHOTO=$(sqlite3 "$DB" "SELECT path FROM files WHERE status='PENDING' AND media_type='photo' AND retry_count < 3 LIMIT 1;")
            if [[ -n "$P_PHOTO" ]] && ! pgrep -f "photo-worker.sh.*$P_PHOTO" > /dev/null; then
                log "📸 Spawn Photo ($((RUNNING_PHOTO+1))/$MAX_PHOTO): $P_PHOTO"
                sqlite3 "$DB" "UPDATE files SET status='PROCESSING', updated_at=datetime('now') WHERE path='$P_PHOTO';"
                nohup "$B/photo-worker.sh" "$P_PHOTO" >/dev/null 2>&1 &
            fi
        fi
        
        if (( RUNNING_VIDEO < MAX_VIDEO )); then
            P_VIDEO=$(sqlite3 "$DB" "SELECT path FROM files WHERE status='PENDING' AND media_type='video' AND retry_count < 3 LIMIT 1;")
            if [[ -n "$P_VIDEO" ]] && ! pgrep -f "video-worker.sh.*$P_VIDEO" > /dev/null; then
                log "🎬 Spawn Video ($((RUNNING_VIDEO+1))/$MAX_VIDEO): $P_VIDEO"
                sqlite3 "$DB" "UPDATE files SET status='PROCESSING', updated_at=datetime('now') WHERE path='$P_VIDEO';"
                nohup "$B/video-worker.sh" "$P_VIDEO" >/dev/null 2>&1 &
            fi
        fi
    else
        log "⚠️ High RAM Usage (${RAM_PCT}%). Holding off new spawns."
    fi
    
    # --- ZOMBIE CLEANUP ---
    STUCK_FILES=$(sqlite3 "$DB" "SELECT id, path, retry_count FROM files WHERE status='PROCESSING' AND updated_at < datetime('now', '-15 minutes');")
    if [[ -n "$STUCK_FILES" ]]; then
        while IFS='|' read -r FID FPATH RCOUNT; do
            NEW_RC=$((RCOUNT + 1))
            if (( NEW_RC >= 3 )); then
                log "⚠️ Max Retries Reached for $FPATH. Marking as ERROR."
                sqlite3 "$DB" "UPDATE files SET status='ERROR', category='GAGAL_MAX_RETRY', retry_count=$NEW_RC WHERE id=$FID;"
            else
                log "♻️ Resetting Stuck File $FPATH (Retry $NEW_RC/3)."
                sqlite3 "$DB" "UPDATE files SET status='PENDING', retry_count=$NEW_RC WHERE id=$FID;"
            fi
        done <<< "$STUCK_FILES"
    fi
    
    RUNNING_TOTAL=$(pgrep -fc "(photo|video)-worker.sh" || true)
    if (( RUNNING_TOTAL > 0 )); then "$B/state_set" state "PROCESSING_QUEUE"
    else "$B/state_set" state "IDLE"; fi
    
    sleep 5
done
