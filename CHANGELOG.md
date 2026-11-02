# 📋 CHANGELOG - Kompic (Immich Media Optimizer & Ecosystem Suite)

Semua riwayat perubahan, penambahan fitur, peningkatan performa, dan perbaikan bug didokumentasikan secara rinci di berkas ini.

---

## [v1.3.0] - 2026-11-02

### ✨ Fitur Baru & Peningkatan Besar
1. **8 Tema Wallpaper Aktif Pixel Art Asli Nusantara**:
   - Menghapus ketergantungan API berita pihak ketiga dan menggantinya dengan **Active Multi-Layer Pixel Art Wallpaper Engine**.
   - Menyajikan 8 lanskap unik khas Indonesia dengan animasi siklus awan dan partikel:
     1. ⛰️ **BUMIAYU**: *Lembah Asri Bumiayu, Siluet Megah Gunung Slamet & Jembatan Kereta Api Sakalibel (pilar bata merah).*
     2. 🛕 **TANAH JAWA**: *Candi Borobudur, Kabut Fajar Magelang & Stupa Utama.*
     3. 🐂 **PULAU MADURA**: *Arena Pesisir Selat Madura & Karapan Sapi Gagah dengan Joki Berodheng.*
     4. 🌺 **PULAU BALI**: *Danau Beratan Bedugul, Pura Ulun Danu Meru 5 Tingkat & Payung Tedung.*
     5. 🐃 **SUMATRA**: *Tebing Curam Ngarai Sianok & Atap Gonjong Rumah Gadang Minangkabau.*
     6. 🛶 **KALIMANTAN**: *Sungai Barito, Hutan Hujan Tropis Borneo & Perahu Pasar Terapung Muatan Buah.*
     7. 🛖 **SULAWESI**: *Perbukitan Tana Toraja & Rumah Adat Tongkonan Megah.*
     8. 🏝️ **TANAH PAPUA**: *Gugusan Karst Zamrud Raja Ampat & Rumah Adat Honai Tradisional.*
   - Interaktif: Klik kotak widget sidebar untuk berganti tema, dan tombol `🖥️ Full Wallpaper: ON/OFF` untuk mengaktifkan kanvas bergerak di seluruh layar dashboard.

2. **Dual-Mode Pelacak Progres Pembuatan Thumbnail Immich**:
   - Menghubungkan pembuatan thumbnail langsung dari **Web UI Kompic**, **TUI Bash**, dan **Web Immich**.
   - Tersedia opsi ganti tampilan (*toggle view*) antara:
     - **Mode Detailed Progress Bar**: Menampilkan persentase %, file aktif yang sedang diproses, dan jumlah total.
     - **Mode Minimalist Status Badge**: Menampilkan animasi titik hijau berdenyut (*live-dot*) ringkas.

3. **Aturan Ambang Efisiensi Kompresi Pintar (*Marginal Gain Guard*)**:
   - Jika hasil kompresi menghasilkan penghematan **di bawah 10% DAN selisih ukuran kurang dari 750 KB**, Kompic otomatis membatalkan (*SKIP*) penggantian file dan mempertahankan berkas asli **100% original**.

4. **Tab Baru: Immich Suite & Health Monitor**:
   - Pemantau realtime status dan PID dari `immich.service`, database `postgresql`, dan `redis`.
   - Tombol cepat Restart & Stop layanan Immich langsung dari web UI.
   - Fitur **Auto Fix Geodata Symlink** (`/build/geodata`) untuk mencegah dan memperbaiki bug fatal startup crash reverse geocoding Immich.
   - Pengecekan responsivitas Port 2283 dan streaming 30 baris log `journalctl -u immich`.

5. **Dual-Mode TUI Setup Wizard (`install-wizard.sh` & `./optimizer --wizard`)**:
   - Deteksi otomatis instalasi Immich (dukungan native DietPi ID 202 & Docker).
   - Wizard penentuan folder media foto Immich yang terintegrasi langsung dengan scanner Kompic.

### 🐛 Perbaikan Bug (Bug Fixes)
- Memperbaiki `SyntaxError: Unexpected non-whitespace character after JSON` pada endpoint aksi `/api/immich/action` dengan menerapkan otentikasi `authedFetch` resmi.
- Menghapus duplikasi fungsi `initSidebarPixelDiorama()` di `app.js` yang menyebabkan macetnya pembaruan data telemetri di dasbor utama.
- Memperbaiki transparansi CSS kanvas latar belakang `#pixelBackgroundCanvas` agar tembus pandang elegan di seluruh antarmuka dashboard.

---

## [v1.2.0] - 2026-11-01
- Menambahkan tab Device Monitor dengan 4-Core CPU meter realtime & tabel thread inspeksi.
- Menambahkan 3 Mode tampilan interaktif pada Flowchart Monitor (Alur Linier, Cabang Foto vs Video, dan Prioritas Antrean).
- Memindahkan informasi arsitektur mesin ke tab "Panduan & Help".
- Menambahkan tombol pembuka artikel sumber asli pada widget berita.

---

## [v1.1.0] - 2026-10-31
- Memperbaiki gerbang otentikasi modal login agar tidak menutupi dashboard saat `auth_enabled: false`.
- Perlindungan penuh Apple iPhone Live Photos (`ContentIdentifier` dan `MediaGroupUUID`).
- Validasi ketat Zero GPS Loss: file asli tidak akan ditimpa jika koordinat lokasi gagal disalin ke hasil kompresi.

---

## [v1.0.0] - 2026-10-22
- Rilis perdana mesin pengoptimal media Kompic berbasis Golang untuk arsitektur Linux ARM64 (ODROID-C2).
- Dukungan kompresi video H.265 CRF 26 dan foto WEBP/JPEG dengan jaminan *Zero Crop* (rasio aspek 100% utuh).
- Sinkronisasi brankas cloud cadangan ke MEGA Vault (`mega:ImmichBackup`).
