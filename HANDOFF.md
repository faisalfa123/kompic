# HANDOFF — Media Optimizer v0.9.6-beta (untuk agen AI penerus)

## 0. Cara pakai file ini
Baca SELURUH file ini SEBELUM mengubah apa pun. Bagian 7 (SOP deploy) dan
bagian 8-9 (larangan regresi & edge-case terbuka) adalah yang paling sering
dilanggar dan paling mahal akibatnya. Kode sumber ada di branch `backup-code`
repo ini; file ini adalah "kenapa"-nya, yang TIDAK bisa diekstrak dari kode.

## 1. Identitas sistem & path
- OS: DietPi v10.7.2 pada Odroid C2 (aarch64). Debian-based, bash-only.
- App root : /opt/media-optimizer
- Bin      : /opt/media-optimizer/bin   (semua script + tool backup/restore)
- UI       : /opt/media-optimizer/ui    (index.php, app.js, style.css, flow.js)
- DB       : /opt/media-optimizer/db/media-optimizer.sqlite
- Logs     : /opt/media-optimizer/logs  (supervisor, video-worker, photo-worker,
               backup-rclone, scanner, backup.log)
- Staging  : /opt/mo-backup             (working tree git backup: code/ & data/)
- Config   : /opt/mo-backup/config      (GH_REPO=git@github.com/faisalfa123/kompic.git)
- SSH key  : /home/dietpi/.ssh/mo_backup_key (.pub terdaftar sebagai DEPLOY KEY
               dengan ALLOW WRITE di repo kompik). Private key TIDAK PERNAH masuk repo.
- Unit     : media-optimizer.service (supervisor), media-optimizer-ui.service (UI)
- UI port  : 8090 (disajikan lighttpd + php-fpm; terlihat di htop)
- Media    : /mnt/hdd1/immich/... (library Immich; postgres+redis di htop = stack
               Immich, BUKAN bagian optimizer). Media TIDAK ikut backup git.

## 2. Arsitektur runtime (smart-supervisor.sh)
Loop `while true` + sleep 5. Setiap iterasi:
1. Tulis heartbeat & metrik (suhu via get_temp.sh, RAM via free -m, CPU via
   get_cpu_percent.sh) ke runtime_state.
2. Gerbang MANUAL PAUSE (settings.manual_pause) -> kill worker, lanjut.
3. Gerbang THERMAL: jika suhu >= temp_pause (default 80) -> state OVERHEAT,
   bunuh worker kompresi, PAKSA spawn backup, sleep pendek. Jika <= temp_resume
   (default 65) dan sebelumnya OVERHEAT -> pulihkan state.
4. Gerbang JADWAL: proses hanya jika force_mode=true ATAU jam 00:00-09:00.
5. Gerbang RAM: spawn hanya jika RAM% < 90.
6. Spawn kompresi: MAX_PHOTO=3, MAX_VIDEO=1 (baris 11-12). Pilih PENDING dengan
   retry_count<3, tandai PROCESSING, nohup worker &.
7. Zombie cleanup: file PROCESSING >15 menit -> retry_count++; jika >=3 -> ERROR.
8. Spawn backup (blok "BACKUP SYNC MODE"): mode NORMAL = hanya saat tidak ada
   kompresi jalan (RUNNING_COMP==0); mode FORCE = selalu (maks 1 worker) selama
   RAM<92% & tidak paused. Baca settings.sync_force_mode.

Worker (script terpisah, di-spawn asinkron):
- video-worker.sh : ffprobe -> ffmpeg -c:v libx265 -preset ultrafast -crf 28
  -c:a aac -b:a 128k -movflags +faststart -> bandingkan size -> jika final>=asli
  -> SKIPPED (buang .tmp, pertahankan asli); else replace + hitung percent_saved
  & category -> DONE_COMPRESSED, needs_backup=1.
- photo-worker.sh : JPG q82 / PNG lossless / WebP q80 (magick/cwebp), logika sama.
- rclone-backup-worker.sh : rclone copy ke MEGA, update backup_status.

## 3. Skema DB (tabel utama)
- files(id, path, media_type['video'|'photo'], status, original_size_mb,
  final_size_mb, category, percent_saved, retry_count, needs_backup,
  backup_status, updated_at)
  status: PENDING | PROCESSING | DONE_COMPRESSED | SKIPPED_BOROS |
          SKIPPED_LARGER | ERROR
  category: IRIT | HEMAT_PLUS | HEMAT | EFISIEN | BOROS | GAGAL | UNKNOWN
- settings(key, value)  : force_mode, manual_pause, backup_pause,
          sync_force_mode, temp_pause, temp_resume
- runtime_state(key, value) : cpu_temp, ram_used, ram_total, state,
          position_status, heartbeat, current_compression_node (JSON),
          current_backup_node (JSON)

## 4. UI (index.php = backend PHP + HTML; app.js/style.css/flow.js = frontend)
Endpoint API (?api=...): dashboard_data, set_mode, set_sync_mode, scan,
set_backup, sync_now, retry_sync, get_log. Polling 2s (app.js) / 2.5s (flow.js).
Tab: Dashboard, Live Flow, Alur Sistem, Logs, Sync to Mega, Bantuan.
Top bar global: mode kompresi + posisi + badge SYNC (NORMAL/FORCE) + suhu
berwarna (hijau<65, kuning 65-77, merah>=78) + RAM dalam MB.

## 5. Flowchart — KENAPA SVG viewBox hardcoded + IIFE (JANGAN REFACTOR)
Dua tab flowchart (Live Flow di app.js, Alur Sistem di flow.js) memakai
SVG dengan `viewBox` TETAP (koordinat hardcoded), BUKAN mengukur clientWidth.
Alasan historis: versi yang mengukur lebar container PECAH TOTAL saat tab masih
display:none (lebar=0 -> viewBox dari 0 -> diagram menyusut jadi titik). viewBox
tetap kebal terhadap ganti-tab/resize. JANGAN ganti ke measure-based.
flow.js dibungkus IIFE `(function(){...})();` SUPAYA TIDAK BOCOR ke global.
Pelajaran: dulu flow.js mendefinisikan fungsi global `elbow` yang MENIMPA
`elbow` milik app.js -> renderUI crash -> Live Flow & Sync kosong. IIFE mencegah
kelas bug tabrakan nama global selamanya. JANGAN buang IIFE / JANGAN tambah
var/fungsi global di flow.js.
Sengaja TIDAK pakai library node-graph (Vue Flow dll) karena butuh bundler/build;
DietPi didesain "Bash scripts only, no building or compiling" & "minimal CPU/RAM"
(lihat README MichaIng/DietPi). Menambah toolchain build = titik gagal baru di SBC
yang di-deploy via paste SSH.

## 6. Backup/restore GitHub (repo privat faisalfa123/kompic)
Tiga branch, JANGAN tertukar:
- main        : milik USER. JANGAN PERNAH push/force ke sini. (SHA awal
                878905d9...; verifikasi tidak berubah setelah tiap backup.)
- backup-code : snapshot bin/ ui/ systemd/ logs-snap/ + HANDOFF.md. Push NORMAL
                (riwayat code terjaga). Di-update oleh backup-to-github.sh.
- backup-data : dump SQLite (db/backup.sql). ORPHAN + FORCE-PUSH -> selalu TEPAT
                1 commit, riwayat tidak membengkak. commit-tree tanpa -p.
Script: bin/backup-to-github.sh (snapshot+push), bin/restore-from-github.sh
(argumen target opsional -> dry-run ke /tmp tanpa sentuh produksi; restore
non-destruktif: DB lama di-mv ke .pre-<epoch> sebelum load dump).
Cron: /etc/cron.d/mo-backup, `0 */12 * * * root .../backup-to-github.sh`.
Deploy key scoped 1 repo, ALLOW WRITE. GIT_SSH_COMMAND pakai PATH ABSOLUT key
(karena cron jalan sebagai root, key hidup di /home/dietpi/.ssh).
RESTORE DISASTER: ulangi setup key+config, lalu
  /opt/media-optimizer/bin/restore-from-github.sh
  systemctl daemon-reload && systemctl enable --now media-optimizer media-optimizer-ui

## 7. SOP DEPLOY ANTI-KORUP (WAJIB sebelum edit apa pun via SSH)
Terminal SSH mesin ini MENJATUHKAN KARAKTER & newline saat paste panjang
(sudah terbukti 8+ kali: heredoc terpotong di tengah, delimiter tak terbaca,
perintah tergabung jadi satu baris). Aturan keras:
a. Masuk root: `sudo -i` ; matikan history expansion: `set +H` (kode penuh `!`).
b. Blok heredoc MAKSIMAL ~14 baris, tidak ada baris >80 char.
c. SETELAH tiap blok, verifikasi: `wc -l <file>` harus bertambah PERSIS sejumlah
   baris blok, dan `tail -1 <file>` harus = baris terakhir yang diharapkan.
   Kalau meleset -> blok korup -> ULANGI HANYA BLOK ITU (jangan lanjut buta).
d. `bash -n`/`node --check`/`php -l` hanya cek SINTAKS, BUKAN logika/semantik.
   Mereka BUTA terhadap duplikasi blok & variabel salah-arti. Selalu kombinasikan
   dengan `wc -l` + grep whole-word (`grep -cw`) untuk bukti semantik.
e. File BESAR (skrip >~20 baris, HANDOFF.md): JANGAN paste. Buat di Windows,
   transfer via scp/WinSCP (editor GUI = paste atomic, tidak lewat buffer SSH).
f. JANGAN pernah append ke file yang sudah diverifikasi hijau kecuali ada bukti
   ia rusak. SELALU `cp file file.bak` sebelum modifikasi; RESTORE .bak dulu
   sebelum menulis ulang.
g. Jangan menjalankan blok yang sama dua kali (gejala: wc -l melonjak 2x).
   Setelah lihat `### OK ###`, langsung ke blok berikutnya.

## 8. Bug yang SUDAH diperbaiki — JANGAN REGRESI
- SVG: viewBox hardcoded, bukan measure clientWidth (lihat §5).
- flow.js: dibungkus IIFE, bukan global (lihat §5).
- Supervisor hitung proses: `pgrep -fc "..." || true` (BUKAN `|| echo 0`).
  `pgrep -c` sudah mencetak 0 saat idle DAN exit 1; `|| echo 0` menambah 0 kedua
  -> variabel "0\n0" -> `(( ))` syntax error -> gerbang spawn SELALU false saat
  idle -> kompresi tidak pernah mulai. Ini pelumpuh utama yang pernah terjadi.
- UI angka: pakai pengecekan `== null` (fungsi num()), BUKAN truthiness JS,
  karena 0 (0% hemat / 0 MB) dianggap falsy dan tampil "-".
- CSS progress bar (.progress-track/.progress-fill/.process-stage) HARUS ada;
  pernah tak sengaja terhapus saat rewrite -> batang hilang meski JS memanggil.
- Dua variabel terpisah di flow.js: `force`=sys.force_mode (kompresi/jadwal/busur
  bypass) vs `syncForce`=sys.sync_force (node Kirim MEGA / badge sync). Dulu
  dicampur satu `force` -> node MEGA salah label. JANGAN gabungkan lagi.
- get_cpu_percent.sh sudah di-deploy (dulu "No such file", spam error).

## 9. Edge-case TERBUKA yang SENGAJA DITUNDA — JANGAN "PERBAIKI" TANPA KONFIRMASI USER
Ini diketahui & didiagnosis, tapi ditahan agar tidak menumpuk perubahan pada
sistem yang baru stabil. Diskusikan dengan user sebelum menyentuh:
1. UI tidak punya WATCHDOG HEARTBEAT: top bar membaca runtime_state tanpa cek
   kesegaran -> saat supervisor macet/error, UI tetap pamer "RUNNING" seolah
   sehat (pernah terjadi: mesin idle tapi dashboard tenang). Fix: hitung umur
   heartbeat di index.php, tampilkan "SUPERVISOR STALE" bila >~30s.
2. Label skip mismatch: DB menyimpan SKIPPED_LARGER, tapi agregat index.php &
   pemetaan app.js mengecek SKIPPED_BOROS -> kartu "Skip (Boros)" selalu 0 &
   kolom status tampil teks mentah. Fix: terima kedua literal.
3. video-worker double-count: worker spawn subshell -> `pgrep -fc video-worker.sh`
   mengembalikan 2 saat 1 encode. Dengan MAX_VIDEO=1 ini kebetulan aman (mencegah
   encode ganda), TAPI RAPUH: jika MAX_VIDEO dinaikkan, sistem salah kira penuh.
   Fix benar: hitung ffmpeg / PID unik ber-parent supervisor, bukan nama script.
4. dietpi-update 10.7.2 -> 10.8 DITUNDA. Release note 10.8 mengubah hal yang
   bersentuhan dengan stack ini (dietpi-software IDs jadi bernama, HAProxy
   packaging sendiri, fix Immich & Immich-ML). JANGAN upgrade OS sampai kompresi
   stabil >=24 jam DAN cron backup terbukti fire otomatis. Setelah itu, buat
   backup manual terakhir dulu (titik pulih di GitHub) baru upgrade.

## 10. State runtime saat handoff (2026-10-25 ~02:00 UTC)
- Kompresi AKTIF: ffmpeg nyata meng-encode (contoh 024572AD...mp4, retry).
- Antrian menurun dari 124 (~119 saat dicek), DONE_COMPRESSED ~21, SKIPPED ~40.
- Mode kompresi FORCE ; mode sync NORMAL ; suhu sempat 65C (hot — pantau warna).
- Backup/restore GitHub lulus end-to-end (BACKUP_EXIT=0, 3 branch, main utuh).

## 11. Rujukan
- Repo kode+data: github.com/faisalfa123/kompic (branch backup-code / backup-data).
- Filosofi platform: README MichaIng/DietPi ("Bash scripts only, no building or
  compiling required"; "highly optimised for minimal CPU and RAM").
- DietPi 10.8 release notes: dietpi.com/docs/releases/v10_8/ (alasan tunda upgrade).