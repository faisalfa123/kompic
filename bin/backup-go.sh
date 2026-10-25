#!/bin/bash
set -uo pipefail
GS=/root/kompresor
BK=/opt/mo-backup
GC="$BK/gocode"; GST="$BK/gostate"
CFG="$BK/config"; LOG="$GS/logs/backup.log"
KEY="/home/dietpi/.ssh/mo_backup_key"
CB=backup-go; SB=backup-go-state
[ -f "$CFG" ] && . "$CFG"
: "${GH_REPO:?missing}"
export GIT_SSH_COMMAND="ssh -i $KEY -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
ts(){ date '+%F %T'; }
log(){ echo "$(ts) [GOBAK] $*" >> "$LOG"; }
fail(){ log "ERR $*"; echo "ERR $*" >&2; exit 1; }
mkdir -p "$BK" "$GS/logs"
if [ ! -d "$GC/.git" ]; then
  git init -q "$GC" || fail "init gc"
  git -C "$GC" remote add origin "$GH_REPO" 2>/dev/null
  git -C "$GC" remote set-url origin "$GH_REPO" 2>/dev/null
  git -C "$GC" config user.name mo-backup
  git -C "$GC" config user.email mo-backup@localhost
  git -C "$GC" symbolic-ref HEAD "refs/heads/$CB"
fi
rm -rf "$GC/src"
mkdir -p "$GC/src"
cp -a "$GS"/*.go "$GS/go.mod" "$GC/src/" 2>>"$LOG"
for F in config.json install-service.sh onerun.sh README.md; do
  cp -a "$GS/$F" "$GC/src/" 2>>"$LOG"
done
cp -a "$GS/static" "$GC/src/static" 2>>"$LOG"
git -C "$GC" add -A
if ! git -C "$GC" diff --cached --quiet; then
  git -C "$GC" commit -q -m "go-code $(ts)" || fail "commit gc"
  git -C "$GC" push -q origin "$CB" || fail "push gc"
  log "go-code pushed ($CB)"
else log "go-code unchanged"; fi
if [ ! -d "$GST/.git" ]; then
  git init -q "$GST" || fail "init gst"
  git -C "$GST" remote add origin "$GH_REPO" 2>/dev/null
  git -C "$GST" remote set-url origin "$GH_REPO" 2>/dev/null
  git -C "$GST" config user.name mo-backup
  git -C "$GST" config user.email mo-backup@localhost
  git -C "$GST" symbolic-ref HEAD "refs/heads/$SB"
fi
cp -a "$GS/optimizer_state.json" "$GST/state.json" 2>>"$LOG"
SZ=$(stat -c%s "$GST/state.json" 2>/dev/null || echo 0)
C1=$(head -c 20 "$GST/state.json" 2>/dev/null | tr -d '[:space:]' | head -c1)
C2=$(tail -c 20 "$GST/state.json" 2>/dev/null | tr -d '[:space:]' | tail -c1)
OK=1
[ "$SZ" -gt 100 ] || OK=0
case "$C1" in '{'|'[') ;; *) OK=0 ;; esac
case "$C2" in '}'|']') ;; *) OK=0 ;; esac
if [ "$OK" = 1 ]; then
  git -C "$GST" add -A
  T="$(git -C "$GST" write-tree)"
  C="$(git -C "$GST" commit-tree "$T" -m "go-state $(ts)")"
  git -C "$GST" update-ref "refs/heads/$SB" "$C"
  git -C "$GST" symbolic-ref HEAD "refs/heads/$SB"
  git -C "$GST" push -q --force origin "$SB" || fail "push gst"
  log "go-state pushed ($SB force)"
else
  log "WARN state torn/invalid, skip"
fi
log "GO DONE"
