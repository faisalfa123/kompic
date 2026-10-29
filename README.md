# 🎬 Kompres — Kompresor Video & Gambar (Aspek Rasio Terjaga)

Alat baris-perintah (CLI) berbasis Python + FFmpeg untuk mengompres **video** dan **gambar** dengan jaminan:

> ✅ **Aspek rasio output selalu sama dengan aslinya** — video tidak akan pernah gepeng, melar, atau terpotong.

---

## ✨ Fitur

| Fitur | Keterangan |
|---|---|
| 🔒 Aspek rasio terjaga | Lebar dihitung otomatis dari rasio asli (`lebar = tinggi × rasio_asli`) |
| 📐 3 mode resolusi | Resolusi asli / tinggi target / skala persen |
| 🎚️ Kontrol kualitas | CRF untuk video, quality untuk gambar, preset kecepatan encoder |
| 🧠 Verifikasi ganda | Deviasi rasio dihitung **sebelum** dan **sesudah** kompresi; gagal jika > 0,5% |
| 🖼️ Video & gambar | MP4/MKV/MOV/AVI/WebM dll., serta JPG/PNG/WebP/BMP/TIFF |
| 📊 Laporan ringkas | Ukuran awal vs akhir, % penghematan, rasio asli vs output (bisa `--json`) |

---

## 🛠️ Persyaratan

- **Python 3.8+**
- **FFmpeg** tersedia di `PATH` (untuk video) → [ffmpeg.org/download.html](https://ffmpeg.org/download.html)
- **Pillow** (hanya untuk kompresi gambar) → `pip install Pillow`

```bash
# Cek ffmpeg
ffmpeg -version
```

---

## 🚀 Cara Penggunaan

```bash
python kompres.py <file_masukan> [opsi]
```

### Contoh umum

```bash
# 1. Kompres dengan resolusi ASLI (hanya turunkan bitrate/kualitas) — default
python kompres.py video.mp4

# 2. Turunkan ke tinggi 720px — LEBAR dihitung otomatis agar rasio tetap
#    1920x1080 (16:9) -> 1280x720   |   1080x1920 (9:16) -> 720x1280
python kompres.py video.mp4 -t 720

# 3. Skala 50% dari resolusi asli (rasio tetap terjaga)
python kompres.py video.mp4 -s 50

# 4. File lebih kecil, encoding lebih lama
python kompres.py video.mp4 -c 28 -p slow

# 5. HEVC (lebih hemat ~40-50% dibanding H.264, encoding lebih lambat)
python kompres.py video.mp4 --codec libx265 -c 26

# 6. Buang audio
python kompres.py video.mp4 --no-audio

# 7. Kompresi gambar
python kompres.py foto.jpg -q 70 -o foto_kecil.jpg
python kompres.py foto.png -s 50            # ubah separuh ukuran, rasio tetap

# 8. Laporan JSON (untuk skrip otomasi)
python kompres.py video.mp4 -t 480 --json
```

### Semua Opsi

| Opsi | Default | Keterangan |
|---|---|---|
| `-t, --target-height PX` | – | Tinggi target piksel; **lebar mengikuti rasio asli** |
| `-s, --scale PERSEN` | – | Skala persentase (10–100) dari resolusi asli |
| `--original` | ✔ | Pertahankan resolusi asli (mode default bila `-t`/`-s` tidak dipakai) |
| `-c, --crf 0-51` | `23` | Kualitas video. Kecil = bagus & besar; 18 ≈ *visual lossless*, 28 ≈ hemat ruang |
| `-q, --quality 1-100` | `75` | Kualitas JPG/WebP |
| `-p, --preset` | `medium` | `ultrafast` … `veryslow` (makin lambat makin efisien) |
| `--codec` | `libx264` | `libx264`, `libx265`, `libvpx-vp9` |
| `--no-audio` | – | Buang jalur audio |
| `-o, --output` | `<nama>_compressed.<ekst>` | Nama file hasil |
| `--json` | – | Cetak laporan akhir sebagai JSON |

> ⚠️ `-t`, `-s`, dan `--original` bersifat **saling eksklusif** (pilih salah satu).

---

## 🔧 Perbaikan yang Dilakukan (Aspek Rasio)

Masalah pada pengaturan sebelumnya: dimensi output bisa dipaksa bebas (mis. `-t 720 -w 1200`) sehingga video jadi **gepeng/melar** karena rasio aslinya berubah.

Perbaikan pada versi ini:

1. **Lebar tidak pernah ditentukan manual.** Lebar selalu dihitung: `lebar_baru = tinggi_baru × (lebar_asli / tinggi_asli)`.
2. **Pembulatan ke angka genap** (`even()`) — wajib untuk encoder `libx264/x265` + `yuv420p`; deviasi akibat pembulatan sangat kecil (< 0,2%).
3. **Filter FFmpeg `setsar=1`** — memastikan *Sample Aspect Ratio* output 1:1 sehingga DAR (Display Aspect Ratio) persis seperti hitungan.
4. **Validasi dua arah**: deviasi rasio diperiksa sebelum eksekusi **dan** setelah hasil dikompres (probe ulang file output). Program berhenti jika deviasi > 0,5%.
5. **Mendukung video potret/vertikal** — mis. 1080×1920 (9:16) dengan `-t 1280` menghasilkan 720×1280, bukan 1280×720.

Hasil uji:

| Asli | Mode | Output | Rasio | Deviasi |
|---|---|---|---|---|
| 1920×1080 (16:9) | `-t 720` | 1280×720 | 16:9 | 0,000% |
| 1080×1920 (9:16) | `-t 1280` | 720×1280 | 9:16 | 0,000% |
| 1920×1080 (16:9) | `-s 50` | 960×540 | 16:9 | 0,000% |
| 720×577 (ganjil) | `-t 360` | 450×360 | ±4:5 | 0,174% |

---

## 🧭 Flowchart Cara Kerja

```mermaid
flowchart TD
    A([🚀 Mulai<br/>python kompres.py file input]) --> B{File ada?}
    B -- Tidak --> E1[❌ Error: file tidak ditemukan] --> Z([Selesai])
    B -- Ya --> C{Cek ekstensi file}

    C -- .jpg/.png/.webp/... --> D1[Baca ukuran asli<br/>lebar x tinggi via Pillow]
    C -- .mp4/.mkv/.mov/... --> D2[Probe FFmpeg:<br/>resolusi, fps, durasi, audio]

    D1 --> F{Mode resolusi?<br/>-t / -s / default}
    D2 --> F

    F -- "-t TINGGI" --> G1["Hitung lebar = tinggi × rasio_asli<br/>(1920×1080, -t 720 → 1280×720)"]
    F -- "-s PERSEN" --> G2["Kali kedua dimensi dengan %<br/>(1920×1080, -s 50 → 960×540)"]
    F -- "default/--original" --> G3[Pertahankan resolusi asli<br/>1920×1080 → 1920×1080]

    G1 --> H[Bulatkan ke angka GENAP<br/>even() — syarat encoder]
    G2 --> H
    G3 --> H

    H --> I{Deviasi rasio<br/>asli vs output ≤ 0,5% ?}
    I -- Tidak --> E2[❌ Hentikan: rasio menyimpang] --> Z
    I -- Ya --> J{Tipe file?}

    J -- Gambar --> K1[Resize Lanczos + simpan<br/>dengan quality -q]
    J -- Video --> K2["FFmpeg encode:<br/>scale=W:H:flags=lanczos,setsar=1<br/>-c:v codec -crf N -preset P<br/>-c:a aac 128k / -an"]

    K2 --> L[Probe ulang file output]
    L --> M{Verifikasi rasio<br/>output akhir OK?}
    M -- Tidak --> E3[⚠️ Peringatan deviasi] --> N
    M -- Ya --> N[💰 Bandingkan ukuran<br/>awal vs akhir + % penghematan]
    K1 --> N

    N --> O[Tampilkan ringkasan<br/>atau JSON bila --json]
    O --> Z([✅ Selesai])

    style G1 fill:#d4edda,stroke:#28a745
    style G2 fill:#d4edda,stroke:#28a745
    style G3 fill:#d4edda,stroke:#28a745
    style I fill:#fff3cd,stroke:#ffc107
    style M fill:#fff3cd,stroke:#ffc107
    style E2 fill:#f8d7da,stroke:#dc3545
```

### Alur teks singkat (bila renderer Mermaid tidak tersedia)

```
Mulai → Validasi file → Baca resolusi asli (probe)
      → Pilih mode (-t / -s / asli)
      → HITUNG lebar baru dari RASIO ASLI → bulatkan genap
      → Cek deviasi rasio ≤ 0,5% ─(tidak)→ Berhenti ❌
      → Encode (FFmpeg scale+setsar=1 / Pillow resize)
      → Probe ulang output → verifikasi rasio ✔
      → Laporkan ukuran & penghematan → Selesai ✅
```

---

## 📁 Struktur Proyek

```
.
├── kompres.py     # Program utama (CLI, tanpa dependensi eksternal selain ffmpeg/Pillow)
└── README.md      # Dokumen ini
```

## ❓ Tips Memilih Parameter

- **Ingin ukuran jauh lebih kecil tanpa peduli resolusi?** → naikkan CRF: `-c 28` lalu `-c 32`.
- **Ingin kualitas terbaik?** → `-c 18 -p slower`.
- **Untuk unggahan media sosial?** → `-t 1080 -c 23` (rasio tetap mengikuti kamera asal).
- **Batch banyak file?** → `for f in *.mp4; do python kompres.py "$f" -t 720; done`

## 📄 Lisensi

MIT — bebas digunakan dan dimodifikasi.
