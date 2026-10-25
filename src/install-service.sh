#!/usr/bin/env bash
# ==============================================================================
# Immich Media Optimizer - Auto-Run On Boot Installer (systemd service)
# Specially crafted for ODROID-C2 (ARM64 / DietPi / Debian Linux, 2GB RAM)
# ==============================================================================

set -e

SERVICE_NAME="immich-optimizer"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd "$APP_DIR"

SUDO_CMD=""
if [ "$EUID" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        SUDO_CMD="sudo"
    else
        echo "❌ Script ini memerlukan hak akses root/sudo. Silakan jalankan dengan: sudo $0"
        exit 1
    fi
fi

# Print banner
echo "=========================================================="
echo "🛡️  Immich Media Optimizer - Auto-Run Systemd Service Installer"
echo "📂 Lokasi Direktori : $APP_DIR"
echo "=========================================================="

show_help() {
    echo "Penggunaan:"
    echo "  $0                  Pasang dan aktifkan service auto-run tiap boot"
    echo "  $0 --install        Pasang dan aktifkan service auto-run tiap boot"
    echo "  $0 --uninstall      Hentikan dan copot service dari sistem"
    echo "  $0 --status         Cek status service yang sedang berjalan"
    echo "  $0 --restart        Restart service optimizer"
    echo "  $0 --logs           Tampilkan log real-time service (journalctl)"
    echo "=========================================================="
}

action_status() {
    echo "🔍 Mengecek status ${SERVICE_NAME}.service..."
    $SUDO_CMD systemctl status "${SERVICE_NAME}.service" --no-pager || true
}

action_logs() {
    echo "📜 Menampilkan log real-time ${SERVICE_NAME} (tekan Ctrl+C untuk keluar)..."
    $SUDO_CMD journalctl -u "${SERVICE_NAME}.service" -f
}

action_restart() {
    echo "🔄 Merestart ${SERVICE_NAME}.service..."
    $SUDO_CMD systemctl restart "${SERVICE_NAME}.service"
    echo "✅ Service berhasil direstart."
    action_status
}

action_uninstall() {
    echo "🛑 Menghentikan dan mencopot ${SERVICE_NAME}.service..."
    $SUDO_CMD systemctl stop "${SERVICE_NAME}.service" 2>/dev/null || true
    $SUDO_CMD systemctl disable "${SERVICE_NAME}.service" 2>/dev/null || true
    if [ -f "$SERVICE_FILE" ]; then
        $SUDO_CMD rm -f "$SERVICE_FILE"
        $SUDO_CMD systemctl daemon-reload
        echo "✅ File service $SERVICE_FILE berhasil dihapus."
    fi
    echo "🗑️  Auto-run service berhasil dicopot sepenuhnya."
    exit 0
}

# Tangani argumen
case "$1" in
    --uninstall|-u)
        action_uninstall
        ;;
    --status|-s)
        action_status
        exit 0
        ;;
    --logs|-l)
        action_logs
        exit 0
        ;;
    --restart|-r)
        action_restart
        exit 0
        ;;
    --help|-h)
        show_help
        exit 0
        ;;
esac

# 1. Pengecekan & Pemasangan Dependensi
echo "📦 1. Memeriksa dependensi sistem..."
MISSING_PKGS=""
if ! command -v ffmpeg >/dev/null 2>&1; then
    MISSING_PKGS="$MISSING_PKGS ffmpeg"
fi
if ! command -v exiftool >/dev/null 2>&1; then
    MISSING_PKGS="$MISSING_PKGS libimage-exiftool-perl"
fi
if ! command -v rclone >/dev/null 2>&1; then
    MISSING_PKGS="$MISSING_PKGS rclone"
fi
if ! command -v magick >/dev/null 2>&1 && ! command -v convert >/dev/null 2>&1; then
    MISSING_PKGS="$MISSING_PKGS imagemagick"
fi
if ! dpkg -s libheif-examples >/dev/null 2>&1 && ! command -v heif-enc >/dev/null 2>&1; then
    MISSING_PKGS="$MISSING_PKGS libheif-examples"
fi
if [ ! -f "optimizer" ]; then
    if ! command -v go >/dev/null 2>&1; then
        MISSING_PKGS="$MISSING_PKGS golang-go"
    fi
fi

if [ -n "$MISSING_PKGS" ]; then
    echo "⚠️  Memasang paket yang belum tersedia:$MISSING_PKGS..."
    $SUDO_CMD apt-get update -qq
    $SUDO_CMD apt-get install -y -qq $MISSING_PKGS
    echo "✅ Semua paket dependensi berhasil dipasang."
else
    echo "✅ Dependensi sistem lengkap."
fi

# 2. Kompilasi Binary Optimizer
if [ ! -f "optimizer" ]; then
    echo "🔨 2. Mengompilasi binary Golang mandiri (optimizer)..."
    go build -ldflags="-s -w" -o optimizer main.go
    chmod +x optimizer
    echo "✅ Kompilasi berhasil! Ukuran binary ringan (~10MB RAM)."
else
    echo "✅ Binary optimizer sudah siap."
    chmod +x optimizer
fi

# 3. Pastikan config.json dan folder ada
mkdir -p logs static
if [ ! -f "config.json" ]; then
    echo "⚠️  config.json tidak ditemukan, membuat konfigurasi default..."
    cat <<EOF > config.json
{
  "server": {
    "host": "0.0.0.0",
    "port": 5555,
    "auth_enabled": true,
    "password_hash": "8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918",
    "password_hint": "admin (default)"
  },
  "storage": {
    "immich_upload_dir": "/mnt/hdd1/immich/upload/upload",
    "state_file_path": "optimizer_state.json",
    "log_dir": "logs"
  },
  "compression": {
    "mode": "normal",
    "video_codec": "libx265",
    "video_preset": "ultrafast",
    "video_crf": 26,
    "video_max_height": 1080,
    "video_threads": 1,
    "photo_quality": 82,
    "photo_max_dim": 2560,
    "max_video_workers": 1,
    "max_photo_workers": 2
  },
  "thermal": {
    "enabled": true,
    "max_temp_c": 75,
    "cooldown_temp_c": 65,
    "thermal_zone_path": "/sys/class/thermal/thermal_zone0/temp"
  },
  "schedule": {
    "enabled": true,
    "start_hour": 1,
    "end_hour": 6
  },
  "backup": {
    "enabled": true,
    "mode": "normal",
    "rclone_remote": "mega:ImmichBackup",
    "rclone_args": "--transfers=1 --checkers=2 --fast-list --bwlimit=5M"
  }
}
EOF
fi

# 4. Buat systemd Unit File
echo "⚙️  3. Memasang systemd service unit ke $SERVICE_FILE..."
cat <<EOF | $SUDO_CMD tee "$SERVICE_FILE" >/dev/null
[Unit]
Description=Immich Media Optimizer & Web Monitor Service (Golang)
Documentation=https://github.com/immich-app/immich
After=network.target immich-server.service local-fs.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/optimizer
Restart=always
RestartSec=5s
KillMode=process
LimitNOFILE=65536
Environment="PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
StandardOutput=journal
StandardError=journal
SyslogIdentifier=immich-optimizer

# ODROID-C2 Resource Guard (2GB RAM Optimization)
MemoryMax=1600M
CPUWeight=80

[Install]
WantedBy=multi-user.target
EOF

# 5. Aktifkan dan Jalankan Service
echo "🚀 4. Mengaktifkan service untuk auto-run tiap boot..."
$SUDO_CMD systemctl daemon-reload
$SUDO_CMD systemctl enable "${SERVICE_NAME}.service"
$SUDO_CMD systemctl restart "${SERVICE_NAME}.service"

# 6. Tampilkan Hasil dan Informasi Akses
IP_ADDR=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$IP_ADDR" ]; then
    IP_ADDR="localhost"
fi

echo "=========================================================="
echo "🎉 SUKSES! Immich Media Optimizer Telah Berjalan Sebagai Service"
echo "=========================================================="
echo "🌐 Akses Web UI        : http://${IP_ADDR}:5555"
echo "🔑 Password Default    : admin"
echo "⚡ Auto-Run Tiap Boot  : AKTIF (systemd: ${SERVICE_NAME}.service)"
echo "🛡️  Thermal Guard       : Auto-throttle di atas 75°C"
echo "🍏 iPhone Live Photos  : Preservasi otomatis ContentIdentifier"
echo "=========================================================="
echo "Perintah Manajemen:"
echo "  - Cek Status         : sudo systemctl status ${SERVICE_NAME}"
echo "  - Restart Service    : sudo systemctl restart ${SERVICE_NAME}"
echo "  - Stop Service       : sudo systemctl stop ${SERVICE_NAME}"
echo "  - Pantau Log Live    : sudo journalctl -u ${SERVICE_NAME} -f"
echo "  - Copot Service      : ./install-service.sh --uninstall"
echo "=========================================================="
