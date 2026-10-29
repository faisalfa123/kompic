#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
kompres.py — Kompresor Video & Gambar (CLI)

Fitur utama:
  * Aspek rasio output SELALU sama dengan aslinya (tidak pernah gepeng/melar).
  * Pilihan mode resolusi:
      - "asli"   : tidak mengubah resolusi sama sekali (default)
      - "target" : tinggi target, lebar dihitung otomatis agar rasio tetap
                   (nilai ganjil dibulatkan ke genap karena kebutuhan encoder)
      - "persen" : skala persentase dari ukuran asli (rasio tetap)
  * Kualitas (CRF), preset kecepatan, codec, dan opsi tanpa-audio.
  * Validasi rasio asli vs rasio hasil kompresi + laporan ringkas.

Contoh:
    python kompres.py input.mp4                       # resolusi asli, kualitas seimbang
    python kompres.py input.mp4 -t 720                # tinggi 720p, lebar mengikuti rasio
    python kompres.py input.mp4 -s 50                 # skala 50% (rasio tetap)
    python kompres.py input.mp4 -c 28 -p slow         # file lebih kecil, encoding lebih lama
    python kompres.py foto.jpg -o foto_kecil.jpg -q 70
"""

from __future__ import annotations

import argparse
import json
import math
import os
import re
import shutil
import subprocess
import sys

# ---------------------------------------------------------------------------
# Utilitas dasar
# ---------------------------------------------------------------------------

def die(msg: str) -> None:
    print(f"[ERROR] {msg}", file=sys.stderr)
    sys.exit(1)


def require_ffmpeg() -> str:
    ffmpeg = shutil.which("ffmpeg")
    if not ffmpeg:
        die("ffmpeg tidak ditemukan di PATH. Instal ffmpeg terlebih dahulu.")
    return ffmpeg


def run(cmd: list[str], capture: bool = False) -> subprocess.CompletedProcess:
    proc = subprocess.run(cmd, text=True, capture_output=capture)
    if proc.returncode != 0 and capture:
        raise RuntimeError(proc.stderr.strip().splitlines()[-1] if proc.stderr else "ffmpeg gagal")
    return proc


def even(n: float) -> int:
    """Bulatkan ke integer GENAP terdekat (kebutuhan libx264/x265 + yuv420p)."""
    v = int(round(n))
    if v % 2 != 0:
        v += 1 if v >= int(n) else -1  # arahkan ke genap terdekat
    if v <= 0:
        v = 2
    return max(v, 2) if v % 2 == 0 else max(v + 1, 2)


# ---------------------------------------------------------------------------
# Probing
# ---------------------------------------------------------------------------

def probe(ffmpeg: str, path: str) -> dict:
    """Ambil data stream via `ffmpeg -i` (tanpa perlu ffprobe)."""
    proc = subprocess.run([ffmpeg, "-hide_banner", "-i", path],
                          capture_output=True, text=True)
    info = {"width": None, "height": None, "duration": None,
            "fps": None, "has_audio": False, "bitrate_kbps": None}

    m = re.search(r"Duration:\s*(\d+):(\d+):(\d+(?:\.\d+)?)", proc.stderr)
    if m:
        info["duration"] = int(m.group(1)) * 3600 + int(m.group(2)) * 60 + float(m.group(3))

    m = re.search(r"bitrate:\s*(\d+)\s*kb/s", proc.stderr)
    if m:
        info["bitrate_kbps"] = int(m.group(1))

    for line in proc.stderr.splitlines():
        if "Video:" in line and "Stream #" in line:
            mm = re.search(r"(\d{2,5})x(\d{2,5})", line)
            if mm:
                info["width"], info["height"] = int(mm.group(1)), int(mm.group(2))
            fm = re.search(r"([\d.]+)\s*fps", line)
            if fm:
                info["fps"] = float(fm.group(1))
        if "Audio:" in line and "Stream #" in line:
            info["has_audio"] = True

    if info["width"] is None or info["height"] is None:
        raise ValueError(f"Resolusi tidak terbaca dari: {path}")
    return info


# ---------------------------------------------------------------------------
# Perhitungan aspek rasio
# ---------------------------------------------------------------------------

def round_ratio(w: int, h: int) -> tuple[int, int]:
    """Rasio sederhana mis. 1920x1080 -> (16, 9)."""
    g = math.gcd(w, h)
    return w // g, h // g


def calc_output_size(orig_w: int, orig_h: int, args: argparse.Namespace) -> tuple[int, int, str]:
    """
    Hitung dimensi output yang MENJAGA aspek rasio asli.
    Mengembalikan (lebar, tinggi, deskripsi_mode).
    """
    ratio = orig_w / orig_h

    if args.mode == "asli":
        return orig_w, orig_h, f"resolusi asli ({orig_w}x{orig_h})"

    if args.mode == "target":
        th = args.target_height
        tw = even(th * ratio)          # lebar = tinggi x rasio  -> rasio terjaga
        th = even(th)
        return tw, th, f"tinggi target {th}px, lebar mengikuti rasio -> {tw}x{th}"

    # mode persen
    pct = min(max(args.scale_pct, 10), 100)
    tw = even(orig_w * pct / 100.0)
    th = even(orig_h * pct / 100.0)
    return tw, th, f"skala {pct}% -> {tw}x{th}"


def ratio_deviation_percent(ow: int, oh: int, nw: int, nh: int) -> float:
    """Selisih rasio asli vs output dalam persen (harus ~0)."""
    orig = ow / oh
    new = nw / nh
    return abs(new - orig) / orig * 100.0


# ---------------------------------------------------------------------------
# Kompresi video
# ---------------------------------------------------------------------------

SUPPORTED_EXT = {".mp4", ".mkv", ".mov", ".avi", ".webm", ".flv", ".m4v", ".wmv"}
IMAGE_EXT = {".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff"}


def default_output(inp: str, suffix: str = "_compressed", ext: str | None = None) -> str:
    base, old_ext = os.path.splitext(inp)
    ext = ext or old_ext
    return f"{base}{suffix}{ext}"


def compress_video(ffmpeg: str, inp: str, out: str, nw: int, nh: int,
                   crf: int, preset: str, codec: str, no_audio: bool) -> None:
    vf = f"scale={nw}:{nh}:flags=lanczos,setsar=1"
    cmd = [ffmpeg, "-y", "-i", inp, "-vf", vf,
           "-c:v", codec, "-crf", str(crf), "-preset", preset]
    if no_audio:
        cmd += ["-an"]
    else:
        cmd += ["-c:a", "aac", "-b:a", "128k"]
    cmd += ["-pix_fmt", "yuv420p", out]
    try:
        run(cmd, capture=True)
    except RuntimeError as e:
        die(f"Kompresi video gagal: {e}")


def compress_image(inp: str, out: str, nw: int, nh: int, quality: int) -> None:
    try:
        from PIL import Image
    except ImportError:
        die("Untuk mengompres gambar, instal Pillow: pip install Pillow")
    img = Image.open(inp)
    img = img.resize((nw, nh), Image.LANCZOS)  # rasio dijaga oleh perhitungan di atas
    save_kwargs = {}
    ext = os.path.splitext(out)[1].lower()
    if ext in (".jpg", ".jpeg"):
        save_kwargs = {"quality": quality, "optimize": True}
        if img.mode in ("RGBA", "P"):
            img = img.convert("RGB")
    elif ext == ".webp":
        save_kwargs = {"quality": quality}
    img.save(out, **save_kwargs)


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="kompres.py",
        description="Kompres video/gambar TANPA mengubah aspek rasio aslinya.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__.split("Contoh:")[-1],
    )
    p.add_argument("input", help="File video/gambar yang akan dikompres")

    g = p.add_mutually_exclusive_group()
    g.add_argument("-t", "--target-height", type=int, metavar="PX", dest="target_height",
                   help="Tinggi target piksel; LEBAR dihitung otomatis agar rasio tetap "
                        "(mis. -t 720 pada video 1920x1080 menghasilkan 1280x720)")
    g.add_argument("-s", "--scale", type=int, metavar="PERSEN", dest="scale_pct",
                   help="Skala persentase dari resolusi asli (mis. -s 50), rasio tetap")
    g.add_argument("--original", action="store_true",
                   help="Pertahankan resolusi asli apa adanya (hanya kurangi bitrate/kualitas)")

    p.add_argument("-c", "--crf", type=int, default=23, metavar="0-51",
                   help="Kualitas video CRF: makin besar makin kecil & makin buruk "
                        "(default 23; 18≈visual-lossless, 28≈hemat ruang)")
    p.add_argument("-q", "--quality", type=int, default=75, metavar="1-100",
                   help="Kualitas gambar JPG/WebP (default 75)")
    p.add_argument("-p", "--preset", default="medium",
                   choices=["ultrafast", "superfast", "veryfast", "faster", "fast",
                            "medium", "slow", "slower", "veryslow"],
                   help="Kecepatan vs efisiensi kompresi video (default medium)")
    p.add_argument("--codec", default="libx264", choices=["libx264", "libx265", "libvpx-vp9"],
                   help="Codec video (default libx264)")
    p.add_argument("--no-audio", action="store_true", help="Buang jalur audio")
    p.add_argument("-o", "--output", help="Nama file output (default: <nama>_compressed.<ekstensi>)")
    p.add_argument("--json", action="store_true", help="Cetak laporan akhir dalam format JSON")
    return p


def main() -> None:
    args = build_parser().parse_args()

    if not os.path.isfile(args.input):
        die(f"File tidak ditemukan: {args.input}")

    ext = os.path.splitext(args.input)[1].lower()
    is_image = ext in IMAGE_EXT
    is_video = ext in SUPPORTED_EXT
    if not (is_image or is_video):
        die(f"Format tidak didukung: {ext}")

    # Tentukan mode resolusi — inti perbaikan: rasio selalu mengikuti aslinya
    if args.target_height:
        if args.target_height < 2:
            die("Tinggi target terlalu kecil (minimal 2 px)")
        mode = "target"
    elif args.scale_pct:
        mode = "persen"
    else:
        mode = "asli"
    args.mode = mode

    src_size = os.path.getsize(args.input)

    if is_image:
        try:
            from PIL import Image
            with Image.open(args.input) as im:
                ow, oh = im.size
        except ImportError:
            die("Untuk mengompres gambar, instal Pillow: pip install Pillow")
        nw, nh, desc = calc_output_size(ow, oh, args)
        out = args.output or default_output(args.input)
        print(f"[INFO] Asli   : {ow}x{oh}  (rasio {round_ratio(ow, oh)})")
        print(f"[INFO] Mode   : {desc}")
        print(f"[INFO] Output : {nw}x{nh}  (rasio {round_ratio(nw, nh)})")
        compress_image(args.input, out, nw, nh, args.quality)
    else:
        ffmpeg = require_ffmpeg()
        info = probe(ffmpeg, args.input)
        ow, oh = info["width"], info["height"]
        nw, nh, desc = calc_output_size(ow, oh, args)
        out = args.output or default_output(args.input)

        dev = ratio_deviation_percent(ow, oh, nw, nh)
        print(f"[INFO] Asli     : {ow}x{oh} @ {info['fps']} fps, "
              f"durasi {info['duration']:.1f}s, rasio {round_ratio(ow, oh)}")
        print(f"[INFO] Mode     : {desc}")
        print(f"[INFO] Output   : {nw}x{nh} (rasio {round_ratio(nw, nh)}, "
              f"deviasi rasio {dev:.3f}%)")
        print(f"[INFO] Encoder  : {args.codec}, CRF {args.crf}, preset {args.preset}, "
              f"audio {'OFF' if args.no_audio or not info['has_audio'] else 'AAC 128k'}")

        if dev > 0.5:
            die(f"Deviasi rasio {dev:.2f}% terlalu besar — periksa parameter.")

        print("[RUN ] Sedang mengompres ...")
        compress_video(ffmpeg, args.input, out, nw, nh,
                       args.crf, args.preset, args.codec, args.no_audio)

        oinfo = probe(ffmpeg, out)
        final_dev = ratio_deviation_percent(ow, oh, oinfo["width"], oinfo["height"])
        print(f"[OK  ] Hasil terverifikasi: {oinfo['width']}x{oinfo['height']} "
              f"(deviasi rasio {final_dev:.3f}%)")

    dst_size = os.path.getsize(out)
    saved = (1 - dst_size / src_size) * 100 if src_size else 0
    fmt = lambda b: f"{b/1024/1024:.2f} MB" if b >= 1024*1024 else f"{b/1024:.1f} KB"
    report = {
        "input": args.input, "output": out,
        "resolusi_asli": f"{ow}x{oh}", "resolusi_output": f"{nw}x{nh}",
        "rasio_asli": "{}:{}".format(*round_ratio(ow, oh)),
        "rasio_output": "{}:{}".format(*round_ratio(nw, nh)),
        "ukuran_asli": fmt(src_size), "ukuran_baru": fmt(dst_size),
        "penghematan": f"{saved:.1f}%",
    }
    if args.json:
        print(json.dumps(report, ensure_ascii=False, indent=2))
    else:
        print("\n===== RINGKASAN =====")
        for k, v in report.items():
            print(f"  {k:<16}: {v}")


if __name__ == "__main__":
    main()
