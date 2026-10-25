# Immich Media Optimizer & Web Monitor (ODROID-C2 - Golang Edition)

Aplikasi kompresi media otomatis dan Web Monitoring profesional yang dibangun khusus untuk server mini **ODROID-C2** (Amlogic S905, 4-core Cortex-A53, 2GB RAM) yang menjalankan **Immich Self-Hosted** sebagai cloud foto pribadi (Google Photos lokal mandiri).

Dibangun dengan **Golang (Go)** murni tanpa runtime Python guna memastikan memori RAM tetap sangat hemat (~10–15 MB) dan mencegah crash / OOM pada perangkat berdaya rendah.

---

## 🌟 Fitur Utama

1. **🎨 Tampilan Web UI Bergaya Adminator Admin Dashboard**:
   - Desain modern modular dark slate & navy (`#111827`, `#182234`) terinspirasi langsung dari template **Adminator**.
   - **Sidebar Navigasi Elegan**: Akses instan ke Dashboard, Flowchart Monitor, Storage Analytics, Live Settings, System Logs, dan Cloud Backup.
   - **Header Atas (Top Navbar)**: Mode switcher, status engine real-time, widget monitor suhu CPU (`#00d284` normal, `#ffab00` warm, `#f83245` hot), RAM gauge visual bar, dan status tag ODROID-C2.
   - **Kartu Metrik Beraksen (Accent Borders)**: Total Storage Saved, Antrean & Worker State, Sukses Kompresi, Skip Boros (perlindungan file asli), dan Error rate.
   - **Interactive SVG Flowchart & Pulse Glow**: Diagram alur kerja real-time yang menyala mengikuti node prosesor aktif.
2. **⚡ Zero Python Runtime & Ultra Low RAM**: Seluruh backend, supervisor, scanner, API, dan HTTP server berjalan sebagai **single binary Go**. Konsumsi memori hanya ~10–15 MB RAM (meninggalkan sisa RAM melimpah untuk database Immich & PostgreSQL).
3. **⚙️ Kontrol Penuh via Web UI & REST API (`/api/settings`)**:
   - Seluruh parameter konfigurasi dapat diubah langsung dari Web UI (Tab **Live Settings**) tanpa perlu mengedit file secara manual: Port server, ganti password admin (auto SHA-256), direktori upload Immich, ambang batas suhu maksimal & cooldown, rentang jam operasional (jadwal kompresi), codec video (`libx265`/`libx264`), preset encoder, CRF (24–28), batas resolusi (1080p), thread CPU, kualitas foto (80–85), dimensi maksimal (2560px/3840px), hingga remote dan argumen Rclone.
   - Dilengkapi REST API:
     - `GET /api/settings`: Mengambil konfigurasi aktif saat ini.
     - `POST /api/settings`: Menyimpan perubahan ke `config.json` dan memuat ulang runtime secara dinamis.
4. **📍 Preservasi Penuh Geolokasi (GPS) & Metadata Foto/Video (Zero GPS Loss Protection)**:
   - **Foto (JPEG, HEIC, PNG)**: Seluruh metadata EXIF GPS (`GPSLatitude`, `GPSLongitude`, `GPSAltitude`, `GPSTimeStamp`, arah kompas), XMP, dan IPTC dipindahkan utuh.
   - **Video (MOV, MP4, M4V)**: Transcoding FFmpeg menggunakan flag `-map_metadata 0` dan `-movflags +use_metadata_tags` agar container track tidak hilang, diikuti transfer tag QuickTime mendalam via ExifTool (`Keys:GPSCoordinates`, `UserData:GPSCoordinates`, `QuickTime:GPSCoordinates`, dan `CreationDate` UTC).
   - **Sistem Verifikasi Otomatis**: Jika file asli memiliki koordinat lokasi, engine wajib memverifikasi bahwa file hasil kompresi juga memiliki koordinat lokasi. Jika gagal, file asli **TIDAK AKAN DITIMPA** demi menjaga data lokasi tetap 100% aman dan utuh di peta Immich!
5. **📷 Preservasi Penuh iPhone Live Photos**:
   - Metadata Apple QuickTime (`ContentIdentifier`, `MediaGroupUUID`, dan `MakerNotes`) disalin otomatis ke hasil kompresi via `exiftool`. Foto bergerak Live Photos tetap berjalan interaktif di aplikasi iOS dan Immich Web.
6. **🛡️ In-Place Atomic Replace (Skip Boros)**:
   - File hasil kompresi hanya akan menggantikan file asli jika ukurannya terbukti **lebih kecil**. Jika hasil lebih besar/boros, file asli tetap dipertahankan utuh (*Skip Boros*).
7. **🌡️ Thermal Guard Otomatis**:
   - Memantau sensor suhu prosesor ODROID-C2 (`/sys/class/thermal/thermal_zone0/temp`). Jika CPU mencapai ambang batas (default 75°C), kompresi otomatis dijeda hingga dingin kembali (< 65°C).
8. **🔒 Proteksi Password Web UI (Port 5555)**:
   - Dilindungi modal autentikasi password dengan verifikasi hash SHA-256. Password dapat diganti sewaktu-waktu langsung dari tab Live Settings di Web UI.
9. **🚀 Auto-Run Tiap Boot (Systemd Service)**:
   - Dilengkapi script installer service mandiri `install-service.sh` sehingga binary optimizer otomatis aktif di latar belakang setiap kali ODROID-C2 dinyalakan ulang (*auto-start on boot*).
10. **☁️ Rclone MEGA Backup**:
   - Sinkronisasi otomatis file yang telah dioptimasi ke remote cloud MEGA saat prosesor idle.

---

## 🚀 Cara Menjalankan di ODROID-C2

### Opsi A: Pasang Sebagai Auto-Run Service Tiap Boot (Sangat Direkomendasikan)
Agar optimizer langsung aktif otomatis setiap kali server reboot tanpa perlu menjalankan script manual:

```bash
chmod +x install-service.sh
sudo ./install-service.sh
```

Script akan otomatis:
- Memeriksa dan memasang paket dependensi sistem (`ffmpeg`, `exiftool`, `imagemagick`, `libheif-examples`, `rclone`, `golang-go`).
- Mengompilasi binary `optimizer` berukuran kecil (`-ldflags="-s -w"`).
- Memasang unit service `/etc/systemd/system/immich-optimizer.service`.
- Mengaktifkan dan menjalankan service pada sistem boot.

#### Perintah Manajemen Service:
```bash
sudo systemctl status immich-optimizer   # Cek status
sudo systemctl restart immich-optimizer  # Restart service
sudo systemctl stop immich-optimizer     # Stop sementara
sudo journalctl -u immich-optimizer -f   # Pantau log live
sudo ./install-service.sh --uninstall    # Copot service
```

---

### Opsi B: Jalankan Secara Manual di Konsol (`onerun.sh`)
Jika Anda hanya ingin menjalankan langsung di terminal konsol untuk pengujian:

```bash
chmod +x onerun.sh
./onerun.sh
```
*(Skrip juga akan menawarkan pemasangan systemd service secara interaktif).*

---

## 🌐 Akses Web UI

Buka browser dari perangkat di jaringan yang sama:
```text
http://<IP-ODROID-C2>:5555
```
- **Password Default**: `admin` *(Dapat langsung diubah pada Tab Pengaturan di Web UI)*.

---

## ⚙️ Ringkasan Konfigurasi (`config.json`)

| Bagian | Parameter | Deskripsi | Default |
| :--- | :--- | :--- | :--- |
| **server** | `port` | Port HTTP Web UI | `5555` |
| | `host` | Host binding IP | `0.0.0.0` |
| | `auth_enabled` | Aktifkan verifikasi password | `true` |
| | `password_hash` | SHA-256 hash password | (Hash `admin`) |
| **storage** | `immich_upload_dir` | Lokasi upload folder Immich | `/mnt/hdd1/immich/upload/upload` |
| | `state_file_path` | File database state | `optimizer_state.json` |
| | `log_dir` | Folder penyimpanan log | `logs` |
| **thermal** | `enabled` | Proteksi panas CPU | `true` |
| | `max_temp_c` | Suhu jeda kompresi | `75` (°C) |
| | `cooldown_temp_c` | Suhu lanjut kompresi | `65` (°C) |
| **schedule**| `enabled` | Batasi waktu kompresi | `true` |
| | `start_hour` / `end_hour` | Jam operasional malam | `1` s/d `6` (01:00 - 06:00) |
| **compression**| `video_codec` | Codec video FFmpeg | `libx265` (H.265) |
| | `video_preset` | Kecepatan encoder | `ultrafast` |
| | `video_crf` | Kualitas vs kompresi | `26` |
| | `video_threads` | Thread CPU video | `1` (Optimal ODROID-C2) |
| | `photo_quality` | Kualitas foto | `82` |
| | `photo_max_dim` | Maksimal dimensi foto | `2560` (px) |
| **backup** | `enabled` | Sinkronisasi Cloud | `true` |
| | `rclone_remote` | Remote target rclone | `mega:ImmichBackup` |
| | `rclone_args` | Parameter rclone | `--transfers=1 ...` |
