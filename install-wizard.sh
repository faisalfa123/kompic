#!/usr/bin/env bash
# ==============================================================================
# Kompic & Immich Suite - Unified Setup & TUI Wizard Installer
# ==============================================================================
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

clear
echo -e "${CYAN}"
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║             KOMPIC & IMMICH UNIFIED SETUP WIZARD                 ║"
echo "║          Smart Media Optimizer, Vault Backup & Monitor           ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check Root
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}[ERROR] Harap jalankan script ini dengan akses root (sudo bash install-wizard.sh)${NC}"
  exit 1
fi

# 1. CEK / DETEKSI IMMICH
echo -e "${BLUE}=== [1/4] Pemeriksaan Instalasi Immich ===${NC}"
IMMICH_INSTALLED=false
if [ -d "/opt/immich" ] || [ -f "/etc/systemd/system/immich.service" ]; then
    IMMICH_INSTALLED=true
    echo -e "${GREEN}[OK] Immich terdeteksi terpasang di sistem (/opt/immich atau immich.service).${NC}"
else
    echo -e "${YELLOW}[!] Immich belum terpasang di perangkat ini.${NC}"
    read -p "Apakah Anda ingin memasang Immich sekarang? (y/n) [default: y]: " INSTALL_IMMICH_CHOICE
    INSTALL_IMMICH_CHOICE=${INSTALL_IMMICH_CHOICE:-y}

    if [[ "$INSTALL_IMMICH_CHOICE" =~ ^[Yy]$ ]]; then
        if [ -f "/boot/dietpi/func/dietpi-globals" ] || command -v dietpi-software &>/dev/null; then
            echo -e "${GREEN}--> Terdeteksi DietPi OS! Memasang Immich Native via dietpi-software (Software ID: 202)...${NC}"
            dietpi-software install 202
        else
            echo -e "${YELLOW}--> Sistem Linux Standar terdeteksi.${NC}"
            echo -e "Memasang dependensi docker jika belum tersedia..."
            if ! command -v docker &>/dev/null; then
                curl -fsSL https://get.docker.com | sh
            fi
            mkdir -p /opt/immich-app
            cd /opt/immich-app
            curl -sL https://github.com/immich-app/immich/releases/latest/download/docker-compose.yml -o docker-compose.yml
            curl -sL https://github.com/immich-app/immich/releases/latest/download/example.env -o .env
            echo -e "${GREEN}Docker-compose Immich berhasil diunduh ke /opt/immich-app/${NC}"
        fi
    fi
fi

# 2. LOKASI FOLDER MEDIA FOTO IMMICH
echo ""
echo -e "${BLUE}=== [2/4] Konfigurasi Folder Media & Foto Immich ===${NC}"
DEFAULT_MEDIA_PATH="/mnt/hdd1/immich/upload"
if [ ! -d "/mnt/hdd1" ] && [ -d "/mnt/dietpi_userdata/immich" ]; then
    DEFAULT_MEDIA_PATH="/mnt/dietpi_userdata/immich/upload"
fi

echo -e "Folder ini digunakan Immich untuk menyimpan foto dan dipindai Kompic untuk dioptimasi."
read -p "Masukkan path folder upload Immich [default: ${DEFAULT_MEDIA_PATH}]: " USER_MEDIA_PATH
USER_MEDIA_PATH=${USER_MEDIA_PATH:-$DEFAULT_MEDIA_PATH}

mkdir -p "$USER_MEDIA_PATH"
# Izin user immich jika ada
if id "immich" &>/dev/null; then
    chown -R immich:immich "$USER_MEDIA_PATH" 2>/dev/null || true
fi
chmod 775 "$USER_MEDIA_PATH"
echo -e "${GREEN}[OK] Folder media dikonfigurasi di: ${USER_MEDIA_PATH}${NC}"

# Update immich.env jika ada
if [ -f "/mnt/dietpi_userdata/immich/immich.env" ]; then
    if grep -q "IMMICH_MEDIA_LOCATION=" /mnt/dietpi_userdata/immich/immich.env; then
        sed -i "s|^IMMICH_MEDIA_LOCATION=.*|IMMICH_MEDIA_LOCATION='${USER_MEDIA_PATH}'|" /mnt/dietpi_userdata/immich/immich.env
        echo -e "${GREEN}[OK] Diperbarui IMMICH_MEDIA_LOCATION di /mnt/dietpi_userdata/immich/immich.env${NC}"
    fi
fi

# 3. FIX REVERSE GEODATA SYMLINK
echo ""
echo -e "${BLUE}=== [3/4] Validasi Reverse Geocoding Database (/build/geodata) ===${NC}"
if [ -f "/build/geodata/geodata-date.txt" ]; then
    echo -e "${GREEN}[OK] Symlink /build/geodata valid.${NC}"
else
    echo -e "${YELLOW}[!] Memperbaiki symlink /build/geodata untuk Immich...${NC}"
    mkdir -p /build
    if [ -d "/opt/immich/geodata" ]; then
        ln -sfn /opt/immich/geodata /build/geodata
    else
        mkdir -p /build/geodata
    fi
    if id "immich" &>/dev/null; then
        chown -R immich:immich /build 2>/dev/null || true
    fi
    chmod 755 /build
    echo -e "${GREEN}[OK] Symlink /build/geodata berhasil diamankan.${NC}"
fi

# 4. INSTALASI & SETUP KOMPIC OPTIMIZER
echo ""
echo -e "${BLUE}=== [4/4] Konfigurasi Kompic Optimizer Service ===${NC}"
mkdir -p /root/kompresor/static
cd /root/kompresor

TS=$(date +%s)
echo "Mengunduh binary & UI Kompic terbaru dari repo..."
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/kompic-linux-arm64?v=${TS}" -o optimizer
chmod +x optimizer
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/src/static/index.html?v=${TS}" -o static/index.html
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/src/static/style.css?v=${TS}" -o static/style.css
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/src/static/app.js?v=${TS}" -o static/app.js

# Setup / update config.json
if [ -f "/root/kompresor/config.json" ]; then
    # Update field immich_upload_dir
    python3 -c "
import json
try:
    with open('config.json', 'r') as f:
        d = json.load(f)
    d.setdefault('storage', {})['immich_upload_dir'] = '${USER_MEDIA_PATH}'
    with open('config.json', 'w') as f:
        json.dump(d, f, indent=2)
except Exception as e:
    pass
" 2>/dev/null || true
else
    # Inisialisasi awal via binary
    ./optimizer --wizard <<< "" 2>/dev/null || true
fi

# Setup systemd service untuk Kompic jika belum ada
if [ ! -f "/etc/systemd/system/immich-optimizer.service" ]; then
    echo "Membuat service systemd immich-optimizer..."
    cat <<EOF > /etc/systemd/system/immich-optimizer.service
[Unit]
Description=Immich Media Optimizer & Web Monitor Service (Golang)
After=network.target immich.service
Docs=https://github.com/immich-app/immich

[Service]
Type=simple
User=root
WorkingDirectory=/root/kompresor
ExecStart=/root/kompresor/optimizer
Restart=always
RestartSec=5
LimitNOFILE=65535
Nice=15
IOSchedulingClass=idle

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable immich-optimizer
fi

# Restart layanan
echo ""
echo -e "${GREEN}Merestart layanan Immich & Kompic...${NC}"
systemctl restart immich 2>/dev/null || true
systemctl restart immich-optimizer 2>/dev/null || true

echo ""
echo -e "${CYAN}══════════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  INSTALASI & KONFIGURASI SELESAI DENGAN SUKSES! 🚀${NC}"
echo -e "${CYAN}══════════════════════════════════════════════════════════════════${NC}"
echo -e "Dashboard Kompic   : http://$(hostname -I | awk '{print $1}'):5555"
echo -e "Web App Immich     : http://$(hostname -I | awk '{print $1}'):2283"
echo -e "Folder Media Foto  : ${USER_MEDIA_PATH}"
echo ""
