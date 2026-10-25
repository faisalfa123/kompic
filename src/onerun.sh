#!/usr/bin/env bash
# ==============================================================================
# Immich Media Optimizer (Golang Native) - Single Launch Script (onerun.sh)
# Optimized for ODROID-C2 (Amlogic S905 / ARM64, 2GB RAM)
# ZERO PYTHON RUNTIME - Ultra Low Memory Footprint (~10MB RAM)
# ==============================================================================

set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd "$DIR"

echo "=========================================================="
echo "🛡️  Immich Media Optimizer (Golang Edition) - ODROID-C2"
echo "=========================================================="

# Opsi langsung pasang sebagai systemd service
if [ "$1" == "--install-service" ] || [ "$1" == "-i" ]; then
    chmod +x install-service.sh
    exec ./install-service.sh
fi

# 1. Pengecekan Dependensi Sistem
check_and_install_deps() {
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

    # Cek Go compiler jika binary belum ada
    if [ ! -f "optimizer" ]; then
        if ! command -v go >/dev/null 2>&1; then
            MISSING_PKGS="$MISSING_PKGS golang-go"
        fi
    fi

    if [ -n "$MISSING_PKGS" ]; then
        echo "⚠️  Paket berikut belum terpasang:$MISSING_PKGS"
        echo "📦 Memasang paket secara otomatis dengan apt..."
        if [ "$EUID" -ne 0 ]; then
            sudo apt-get update -qq && sudo apt-get install -y -qq $MISSING_PKGS
        else
            apt-get update -qq && apt-get install -y -qq $MISSING_PKGS
        fi
        echo "✅ Paket dependensi berhasil dipasang."
    else
        echo "✅ Semua dependensi sistem sudah siap."
    fi
}

check_and_install_deps

# 2. Pastikan file konfigurasi ada
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

# 3. Buat folder logs & static jika belum ada
mkdir -p logs static

# 4. Kompilasi binary Go jika belum tersedia
if [ ! -f "optimizer" ]; then
    echo "🔨 Mengompilasi binary Golang mandiri (optimizer)..."
    go build -ldflags="-s -w" -o optimizer main.go
    chmod +x optimizer
    echo "✅ Kompilasi berhasil! Binary berukuran kecil dan siap dieksekusi."
else
    chmod +x optimizer
fi

# 5. Penawaran Pemasangan Auto-Run on Boot
if [ -t 0 ]; then
    echo ""
    echo "💡 TIPS: Apakah Anda ingin memasang optimizer ini sebagai SERVICE AUTO-RUN TIAP BOOT?"
    echo "   (Sehingga program otomatis berjalan di latar belakang setiap server dinyalakan)"
    read -t 5 -p "👉 Pasang sebagai background system service sekarang? [y/N] (Otomatis lanjut dalam 5 detik): " INSTALL_ANS || true
    echo ""
    if [[ "$INSTALL_ANS" =~ ^[yY]$ ]]; then
        chmod +x install-service.sh
        exec ./install-service.sh
    fi
fi

# 6. Informasi Akses Konsol
IP_ADDR=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$IP_ADDR" ]; then
    IP_ADDR="localhost"
fi

echo "----------------------------------------------------------"
echo "🌐 Web UI running on : http://${IP_ADDR}:5555"
echo "🔑 Password Default  : admin"
echo "⚡ Runtime           : Golang Native (~10MB RAM, NO PYTHON)"
echo "🔄 Real-Life Flow    : Live Glow & Step Nodes di Tab Live Flowchart"
echo "⚙️  Full Controls     : Semua kontrol dapat diatur langsung di Web UI"
echo "🛡️  Thermal Guard     : Auto-throttle di atas 75°C"
echo "📷 Live Photo Safe   : Ya (Preservasi ContentIdentifier via exiftool)"
echo "📍 GPS / Geolokasi   : Dijamin 100% Utuh (Auto-Verify Zero GPS Loss)"
echo "----------------------------------------------------------"
echo "🚀 Menjalankan optimizer di konsol (tekan Ctrl+C untuk berhenti)..."
echo "   (Catatan: Untuk auto-run tiap boot, jalankan: ./install-service.sh)"
echo "----------------------------------------------------------"

exec ./optimizer
