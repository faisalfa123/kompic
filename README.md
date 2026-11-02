# 🚀 Kompic (Immich Media Optimizer & Ecosystem Monitor)

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/Platform-Linux%20ARM64%20%7C%20AMD64-blue)](#arsitektur-perangkat)
[![Immich Compatible](https://img.shields.io/badge/Immich-v1.120+-success?logo=photopea)](https://immich.app)
[![Zero Crop](https://img.shields.io/badge/Aspect%20Ratio-100%25%20Zero%20Crop-brightgreen)](#zero-crop-guarantee)
[![Cloud Vault](https://img.shields.io/badge/Cloud-MEGA%20Vault%20Sync-red)](#integrasi-cloud-mega-vault)

**Kompic** adalah mesin pengoptimal media latar belakang (*background media optimizer*) dan dasbor pemantau ekosistem mandiri berperforma tinggi yang dirancang khusus untuk server mandiri dan Single Board Computer (SBC) seperti **ODROID-C2 (Amlogic S905X ARM64, RAM 2GB)** dan sistem **DietPi Linux**.

Kompic memindai folder penyimpanan foto & video Immich secara otomatis, mengompresi media dengan menjaga rasio aspek asli 100% (*Zero Crop*), mempertahankan metadata iPhone Live Photos secara presisi, dan menyinkronkan file teringan ke brankas cloud cadangan (*MEGA Vault*).

---

## 📑 Daftar Isi
1. [Fitur Utama](#-fitur-utama)
2. [Aturan Ambang Efisiensi Kompresi Baru](#-aturan-ambang-efisiensi-kompresi-baru-marginal-gain-guard)
3. [Arsitektur Tab & Dasbor Web](#-arsitektur-tab--dasbor-web)
4. [Tab Baru: Immich Suite & Health Monitor](#-tab-immich-suite--health-monitor)
5. [Pembuatan & Regenerasi Thumbnail Immich](#-pembuatan--regenerasi-thumbnail-immich)
6. [Wallpaper Aktif Pixel Art Interaktif](#-wallpaper-aktif-pixel-art-interaktif)
7. [Panduan Instalasi Cepat (1-Klik)](#-panduan-instalasi-cepat-1-klik)
8. [TUI Setup Wizard Interaktif](#-tui-setup-wizard-interaktif)
9. [Struktur Konfigurasi (`config.json`)](#-struktur-konfigurasi-configjson)
10. [Troubleshooting & Solusi Kendala](#-troubleshooting--solusi-kendala)

---

## ✨ Fitur Utama

- **Zero Crop Guarantee (Aspek Rasio 100% Utuh)**:
  Kompresi foto dan video mempertahankan dimensi dan orientasi asli tanpa terpotong (*no cropping*), menjaga komposisi visual fotografi Anda.
- **iPhone Live Photo & Metadata Preservasi Penuh**:
  Mendeteksi dan melindungi tag krusial Apple Live Photos seperti `ContentIdentifier` dan `MediaGroupUUID`, koordinat GPS, profil warna ICC, tanggal pembuatan, dan model kamera via `exiftool`.
- **Hemat Daya & Thermal Guard**:
  Dibatasi maksimal 3 thread CPU (`nice 15`, `ionice 3` class idle) dengan pemantau suhu otomatis (*Continuous Thermal Guard*) agar perangkat SBC tidak mengalami *overheating* atau *throttling*.
- **Brankas Cloud Vault (MEGA Sync)**:
  Mengunggah hanya versi file teringan ke remote cloud `mega:ImmichBackup` dengan struktur folder terorganisir per tahun/bulan/hari.
- **Dukungan Format Lengkap**:
  - Foto: JPEG, PNG, WEBP, HEIC (iPhone via ImageMagick/libheif).
  - Video: MP4, MOV, MKV, AVI, WebM (H.264 / H.265 CRF 26 ultrafast).

---

## 🛡️ Aturan Ambang Efisiensi Kompresi Baru (*Marginal Gain Guard*)

Demi menghemat daya CPU dan mencegah degradasi kualitas yang tidak sebanding, Kompic menerapkan aturan efisiensi pintar:

> **Aturan Evaluasi:**
> Jika hasil kompresi lebih kecil dari berkas asli, **TETAPI** penghematan yang didapatkan **kurang dari 10% DAN selisih ukurannya kurang dari 750 KB**, maka Kompic akan:
> 1. Membatalkan (*SKIP*) proses penggantian file.
> 2. Menghapus file kompresi sementara.
> 3. Mempertahankan berkas asli **100% original**.
> 4. Mencatat log: `[SKIPPED MARGINAL] Penghematan marjinal (<10% & <750KB). Berkas asli dipertahankan.`

---

## 📊 Arsitektur Tab & Dasbor Web

Dasbor web Kompic dapat diakses di port `http://<IP-SERVER>:5555` dan memiliki navigasi lengkap:

| Tab | Nama Tab | Deskripsi & Fungsi |
|---|---|---|
| 1 | **Dashboard** | Ringkasan statistik media, penyimpanan hemat, progress antrean, dan kecepatan pemrosesan. |
| 2 | **Flowchart Monitor** | Visualisasi alur kerja logika mesin (Linier, Cabang Foto vs Video, Prioritas Antrean). |
| 3 | **Storage Analytics** | Grafik rincian penggunaan storage, perbandingan sebelum vs sesudah kompresi. |
| 4 | **Device Monitor** | Pantauan realtime 4 Core CPU, suhu SoC Amlogic, memori RAM, disk I/O, dan thread aktif. |
| 5 | **Live Settings** | Pengaturan parameter kompresi foto/video, batas thermal, dan akun login tanpa restart. |
| 6 | **System Logs** | Log langsung dari worker pengawas, kompresi foto, kompresi video, dan cloud sync. |
| 7 | **Cloud Backup** | Kontrol sinkronisasi berkas ke remote MEGA Vault dengan status pending/sukses. |
| 8 | **Immich Suite** | Tab pemantau kesehatan server Immich, PostgreSQL, Redis, dan tombol kontrol service. |
| 9 | **Panduan & Help** | Dokumentasi teknis arsitektur perangkat ODROID-C2, panduan alur, dan FAQ. |

---

## 🖼️ Tab: Immich Suite & Health Monitor

Tab khusus yang mengintegrasikan ekosistem Immich secara langsung ke dalam Kompic:
- **Status Layanan Realtime**: Memantau status aktif dan PID dari `immich.service`, database `postgresql`, dan `redis`.
- **Indikator Port 2283**: Memastikan web server Immich merespon status `HTTP 200 OK`.
- **Auto-Fix Geodata Symlink (`/build/geodata`)**: Mencegah dan memperbaiki bug startup crash reverse geocoding Immich hanya dengan 1 klik.
- **Aksi Cepat Service**: Tombol untuk melakukan Restart, Stop, atau Start Immich langsung dari dasbor web.
- **Log Streaming Jurnal**: Menampilkan 30 baris log `journalctl -u immich` terkini secara langsung.

---

## ⚡ Pembuatan & Regenerasi Thumbnail Immich

Pembuatan thumbnail dapat dijalankan melalui **3 antarmuka sekaligus**:

### 1. Melalui Web Dashboard Kompic
Buka Tab **"Immich Suite"**, lalu klik tombol hijau **`🖼️ Generate Thumbnails`**. Kompic akan secara otomatis mengirimkan perintah antrean pembuatan thumbnail ke Immich Server API.

### 2. Melalui Terminal / TUI Bash
Jalankan perintah langsung via terminal STB:
```bash
curl -s -X PUT http://127.0.0.1:2283/api/jobs/thumbnail \
     -H "Content-Type: application/json" \
     -d '{"command":"start","force":false}'
```
Atau jalankan wizard terpadu:
```bash
sudo ./install-wizard.sh
```
*(Wizard akan menanyakan pada langkah [5/5] apakah Anda ingin langsung merestart antrean thumbnail).*

### 3. Melalui Web Resmi Immich
Buka Web Immich di port `:2283` -> Buka menu **Administration** -> **Jobs** -> Klik **Generate All** pada baris **Generate Thumbnails**.

---

## 🎨 Wallpaper Aktif Pixel Art Interaktif

Pada bilah sisi (*sidebar*) kiri bawah, terdapat widget **Active Pixel Art Wallpaper**:
- **4 Lanskap Unik Nusantara**:
  1. 🏛️ **Bundaran HI**: Patung Selamat Datang, air mancur menyembur dinamis, dan siluet kota senja.
  2. 🌧️ **Hujan & Petir**: Lembah pegunungan tropis dengan hujan deras dan kilatan petir.
  3. 📈 **Pasar & IHSG**: Matrix candlestick cyberpunk dinamis dengan indikator tren pasar.
  4. 🌾 **Sawah Hijau**: Pematang sawah terasering berundak dengan pondok petani fajar.
- **Interaksi**:
  - **Klik Kotak Widget**: Berganti tema lanskap secara instan.
  - **Tombol `🖥️ Full Wallpaper: ON/OFF`**: Mengaktifkan atau menonaktifkan kanvas latar belakang dinamis di seluruh layar dashboard.

---

## 📥 Panduan Instalasi Cepat (1-Klik)

Untuk memperbarui atau memasang Kompic versi terbaru di DietPi / STB Anda:

```bash
sudo bash -c '
cd /root/kompresor
TS=$(date +%s)
echo "=== 1. Mengunduh Kompic Binary & Aset Web ==="
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/kompic-linux-arm64?v=${TS}" -o optimizer
chmod +x optimizer
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/src/static/index.html?v=${TS}" -o static/index.html
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/src/static/style.css?v=${TS}" -o static/style.css
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/src/static/app.js?v=${TS}" -o static/app.js
curl -sL "https://raw.githubusercontent.com/faisalfa123/kompic/backup-go/install-wizard.sh?v=${TS}" -o install-wizard.sh
chmod +x install-wizard.sh

echo "=== 2. Merestart Layanan ==="
systemctl restart immich-optimizer
sleep 2
systemctl status immich-optimizer --no-pager | head -n 12
'
```

---

## 🧙 TUI Setup Wizard Interaktif

Kompic menyediakan TUI Wizard interaktif yang memandu Anda mengatur folder media dan memeriksa integritas sistem:

```bash
# Jalankan via Script Installer
sudo ./install-wizard.sh

# Atau jalankan via Binary Flag
sudo ./optimizer --wizard
```

**Alur Wizard:**
1. **Deteksi Immich**: Mengecek apakah Immich sudah terpasang. Jika belum, menawarkan instalasi otomatis (*DietPi Software ID 202* atau Docker).
2. **Folder Foto/Media**: Menentukan path folder upload Immich (default: `/mnt/hdd1/immich/upload`).
3. **Validasi Geodata**: Mengamankan symlink `/build/geodata` untuk mencegah kegagalan startup Immich.
4. **Konfigurasi Kompic**: Menyimpan pengaturan secara otomatis ke file `config.json`.
5. **Regenerasi Thumbnail**: Opsi opsional langsung membuat thumbnail media Immich.

---

## ⚙️ Struktur Konfigurasi (`config.json`)

Contoh berkas konfigurasi default `/root/kompresor/config.json`:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 5555,
    "auth_enabled": false,
    "password_hash": "8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918",
    "password_hint": "admin (default)"
  },
  "storage": {
    "immich_upload_dir": "/mnt/hdd1/immich/upload/upload",
    "state_file_path": "optimizer_state.json"
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
    "max_photo_workers": 3
  },
  "thermal": {
    "temp_limit_celsius": 75.0,
    "throttle_sleep_sec": 30
  },
  "backup": {
    "rclone_remote": "mega:ImmichBackup"
  }
}
```

---

## 🔧 Troubleshooting & Solusi Kendala

### 1. Immich Gagal Start: `ENOENT /build/geodata/geodata-date.txt`
- **Penyebab**: Immich mencari folder geodata di `/build/geodata/`, sementara pada sistem bare-metal berada di `/opt/immich/geodata`.
- **Solusi**: Klik tombol **`🛠️ Auto Fix Geodata Symlink`** di tab **Immich Suite** pada dashboard Kompic, atau jalankan:
  ```bash
  sudo mkdir -p /build
  sudo ln -sfn /opt/immich/geodata /build/geodata
  sudo chown -R immich:immich /build
  sudo systemctl restart immich
  ```

### 2. Login Web UI Terkunci
- **Solusi**: Default password adalah `admin`. Jika ingin menonaktifkan password login, buka `config.json`, ubah `"auth_enabled": false`, lalu restart service:
  ```bash
  sudo systemctl restart immich-optimizer
  ```

---

## 📄 Lisensi & Kredit

- Dikembangkan untuk komunitas self-hosted Immich & SBC ARM64.
- Kompatibel penuh dengan [Immich Application](https://github.com/immich-app/immich).
