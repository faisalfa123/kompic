#!/bin/bash
for z in /sys/class/thermal/thermal_zone*/temp; do
  [[ -r "$z" ]] || continue
  raw=$(cat "$z" 2>/dev/null)
  [[ "$raw" =~ ^[0-9]+$ ]] || continue
  (( raw > 1000 )) && echo $((raw/1000)) || echo "$raw"
  exit 0
done
echo 0
