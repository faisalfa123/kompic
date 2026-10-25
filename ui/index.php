<?php
$dbPath='/opt/media-optimizer/db/media-optimizer.sqlite';
$action=$_GET['api']??'';
if($action!==''){header('Content-Type: application/json; charset=utf-8');}else{header('Content-Type: text/html; charset=utf-8');}
try{$db=new PDO("sqlite:$dbPath");$db->setAttribute(PDO::ATTR_ERRMODE,PDO::ERRMODE_EXCEPTION);}
catch(PDOException $e){echo json_encode(['ok'=>false,'error'=>'DB Connection Failed']);exit;}
function getV($p,$k,$d=''){ $s=$p->prepare("SELECT value FROM settings WHERE key=?"); $s->execute([$k]); return $s->fetchColumn() ?: $d; }
function getS($p,$k,$d=''){ $s=$p->prepare("SELECT value FROM runtime_state WHERE key=?"); $s->execute([$k]); return $s->fetchColumn() ?: $d; }
if($action==='dashboard_data'){
 $sys=['temp_c'=>(int)getS($db,'cpu_temp',0),'ram_used_mb'=>(int)getS($db,'ram_used',0),'ram_total_mb'=>(int)getS($db,'ram_total',1),'global_state'=>getS($db,'state','IDLE'),'force_mode'=>filter_var(getV($db,'force_mode','false'),FILTER_VALIDATE_BOOLEAN),'manual_pause'=>filter_var(getV($db,'manual_pause','false'),FILTER_VALIDATE_BOOLEAN),'backup_pause'=>filter_var(getV($db,'backup_pause','false'),FILTER_VALIDATE_BOOLEAN),'sync_force'=>filter_var(getV($db,'sync_force_mode','false'),FILTER_VALIDATE_BOOLEAN),'position_status'=>getS($db,'position_status','PENDING')];
 $em='NORMAL';if($sys['manual_pause'])$em='PAUSED';elseif($sys['force_mode'])$em='FORCE';elseif($sys['global_state']==='OVERHEAT')$em='COOLDOWN';$sys['effective_mode']=$em;
 $agg=$db->query("SELECT COALESCE(SUM(original_size_mb),0) t_orig,COALESCE(SUM(final_size_mb),0) t_fin,COALESCE(SUM(CASE WHEN status='DONE_COMPRESSED' THEN 1 ELSE 0 END),0) s_succ,COALESCE(SUM(CASE WHEN status='SKIPPED_BOROS' THEN 1 ELSE 0 END),0) s_skip,COALESCE(SUM(CASE WHEN status='ERROR' THEN 1 ELSE 0 END),0) s_err,COALESCE(SUM(CASE WHEN status='PENDING' THEN 1 ELSE 0 END),0) s_pend,COALESCE(SUM(CASE WHEN status='PROCESSING' THEN 1 ELSE 0 END),0) s_proc FROM files")->fetch(PDO::FETCH_ASSOC);
 $savedMb=max(0,$agg['t_orig']-$agg['t_fin']);
 $stats=['total_orig_mb'=>round($agg['t_orig'],2),'saved_mb'=>round($savedMb,2),'success_count'=>(int)$agg['s_succ'],'skipped_boros_count'=>(int)$agg['s_skip'],'error_count'=>(int)$agg['s_err'],'pending_count'=>(int)$agg['s_pend'],'processing_count'=>(int)$agg['s_proc']];
 $pb=$db->query("SELECT media_type,COUNT(*) c FROM files WHERE status='PROCESSING' GROUP BY media_type")->fetchAll(PDO::FETCH_KEY_PAIR);
 $stats['processing_video']=(int)($pb['video']??0);$stats['processing_photo']=(int)($pb['photo']??0);
 $rf=$db->query("SELECT path,media_type,status,original_size_mb,final_size_mb,category,percent_saved,updated_at FROM files ORDER BY updated_at DESC LIMIT 50")->fetchAll(PDO::FETCH_ASSOC);
 $bq=$db->query("SELECT path,media_type,original_size_mb,final_size_mb,backup_status,updated_at FROM files WHERE needs_backup=1 OR backup_status IS NOT NULL ORDER BY updated_at DESC LIMIT 50")->fetchAll(PDO::FETCH_ASSOC);
 $compRaw=json_decode(getS($db,'current_compression_node','{}'),true)?:[];$bakRaw=json_decode(getS($db,'current_backup_node','{}'),true)?:[];
 echo json_encode(['ok'=>true,'system'=>$sys,'stats'=>$stats,'recent_files'=>$rf,'backup_queue'=>$bq,'active_processes'=>['compression'=>$compRaw,'backup'=>$bakRaw]]);exit;
}
if($action==='set_mode'){
 $mode=$_GET['mode']??'normal';
 if($mode==='pause'){$db->exec("UPDATE settings SET value='true' WHERE key='manual_pause'");$db->exec("UPDATE settings SET value='false' WHERE key='force_mode'");}
 elseif($mode==='force'){$db->exec("UPDATE settings SET value='false' WHERE key='manual_pause'");$db->exec("UPDATE settings SET value='true' WHERE key='force_mode'");}
 elseif($mode==='resume'){$db->exec("UPDATE settings SET value='false' WHERE key='manual_pause'");}
 else{$db->exec("UPDATE settings SET value='false' WHERE key='manual_pause'");$db->exec("UPDATE settings SET value='false' WHERE key='force_mode'");}
 echo json_encode(['ok'=>true,'current_mode'=>$mode]);exit;
}
if($action==='set_sync_mode'){
 $mode=$_GET['mode']??'normal';$val=($mode==='force')?'true':'false';
 $db->exec("INSERT INTO settings(key,value) SELECT 'sync_force_mode','false' WHERE NOT EXISTS(SELECT 1 FROM settings WHERE key='sync_force_mode')");
 $db->exec("UPDATE settings SET value='$val' WHERE key='sync_force_mode'");
 echo json_encode(['ok'=>true,'sync_force'=>($mode==='force')]);exit;
}
if($action==='scan'){exec('/opt/media-optimizer/bin/scanner.sh >/dev/null 2>&1 &');echo json_encode(['ok'=>true]);exit;}
if($action==='set_backup'){
 $st=$_GET['state']??'resume';
 $db->exec("UPDATE settings SET value='".($st==='pause'?'true':'false')."' WHERE key='backup_pause'");
 echo json_encode(['ok'=>true,'backup_pause'=>($st==='pause')]);exit;
}
if($action==='sync_now'){
 if(filter_var(getV($db,'backup_pause','false'),FILTER_VALIDATE_BOOLEAN)){echo json_encode(['ok'=>false,'error'=>'Sync di-pause']);exit;}
 $id=$db->query("SELECT id FROM files WHERE needs_backup=1 AND (backup_status IS NULL OR backup_status='' OR backup_status LIKE '%FAIL%' OR backup_status LIKE '%ERROR%') ORDER BY id LIMIT 1")->fetchColumn();
 if(!$id){echo json_encode(['ok'=>false,'error'=>'Tidak ada file antrian backup']);exit;}
 exec('/opt/media-optimizer/bin/rclone-backup-worker.sh '.(int)$id.' >/dev/null 2>&1 &');
 echo json_encode(['ok'=>true,'spawned_id'=>(int)$id]);exit;
}
if($action==='retry_sync'){
 $n=$db->exec("UPDATE files SET backup_status=NULL WHERE backup_status LIKE '%FAIL%' OR backup_status LIKE '%ERROR%'");
 echo json_encode(['ok'=>true,'reset'=>$n]);exit;
}
if($action==='get_log'){
 $t=$_GET['type']??'supervisor';$n=(int)($_GET['lines']??50);
 $m=['supervisor'=>'/opt/media-optimizer/logs/supervisor.log','video'=>'/opt/media-optimizer/logs/video-worker.log','photo'=>'/opt/media-optimizer/logs/photo-worker.log','backup'=>'/opt/media-optimizer/logs/backup-rclone.log','scanner'=>'/opt/media-optimizer/logs/scanner.log'];
 $f=$m[$t]??$m['supervisor'];
 if(!file_exists($f)){echo json_encode(['ok'=>true,'content'=>"Log belum ada.\n","type"=>$t]);exit;}
 echo json_encode(['ok'=>true,'content'=>shell_exec("tail -n $n ".escapeshellarg($f))?:'(Kosong)','type'=>$t]);exit;
}
?>
<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Media Optimizer v0.9.6-beta</title><link rel="stylesheet" href="style.css"></head><body>
<div class="overlay" id="overlay" onclick="toggleSidebar()"></div>
<div class="sidebar" id="sidebar">
<div class="brand">🛡️ Media Opt v0.9.6-beta</div>
<div class="nav-item active" onclick="switchTab(event,'dash')">📊 Dashboard</div>
<div class="nav-item" onclick="switchTab(event,'node')">🕸️ Live Flow</div>
<div class="nav-item" onclick="switchTab(event,'flow')">🗺️ Alur Sistem</div>
<div class="nav-item" onclick="switchTab(event,'logs')">📜 Logs</div>
<div class="nav-item" onclick="switchTab(event,'sync')">☁️ Sync to Mega</div>
<div class="nav-item" onclick="switchTab(event,'help')">❓ Bantuan</div>
</div>
<div class="main-content">
<div class="top-bar"><div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
<button class="mobile-menu-btn" onclick="toggleSidebar()">☰</button>
<span id="top-mode-badge" class="mode-badge mode-NORMAL">NORMAL</span>
<span id="top-pos-status" class="pos-status pos-PENDING">PENDING</span>
<span id="top-sync-badge" class="sync-badge">SYNC</span>
</div><div class="sys-metrics-top"><span>Temp: <b id="top-temp" class="t-ok">--°C</b></span><span>RAM: <b id="top-ram">-- MB</b></span></div></div>
<div id="pane-dash" class="tab-pane active">
<div class="stats-grid">
<div class="stat-card"><div class="stat-val c-green" id="hs-saved-mb">0 MB</div><div class="stat-lbl" id="hs-saved-comp">Dari Asli: 0 MB</div></div>
<div class="stat-card"><div class="stat-val" id="hs-success">0</div><div class="stat-lbl">Sukses Kompres</div></div>
<div class="stat-card"><div class="stat-val c-yellow" id="hs-skipped">0</div><div class="stat-lbl">Skip (Boros)</div></div>
<div class="stat-card"><div class="stat-val c-red" id="hs-error">0</div><div class="stat-lbl">Gagal</div></div>
<div class="stat-card"><div class="stat-val" id="hs-processing">0</div><div class="stat-lbl">Sedang Proses (V:<span id="sub-v">0</span> P:<span id="sub-p">0</span>)</div></div>
<div class="stat-card"><div class="stat-val" id="hs-pending">0</div><div class="stat-lbl">Antrian (Pending)</div></div>
</div>
<div class="controls-row"><div class="shortcuts-panel">
<button class="shortcut-btn" id="btn-resume" onclick="setMode('resume')">▶️ Lanjut</button>
<button class="shortcut-btn" id="btn-pause" onclick="setMode('pause')">⏸️ Pause</button>
<button class="shortcut-btn" id="btn-normal" onclick="setMode('normal')">⚙️ Normal</button>
<button class="shortcut-btn" id="btn-force" onclick="setMode('force')">🔥 Force</button>
<button class="shortcut-btn" onclick="triggerScan()">🔄 Scan</button>
</div><div class="view-toggle"><span class="toggle-label">Ringkas</span><label class="toggle-switch"><input type="checkbox" id="compactToggle" onchange="toggleCompactMode(this.checked)"><span class="slider"></span></label></div></div>
<div class="files-table-container" id="tableContainer"><table><thead><tr><th>Nama File</th><th>Tipe</th><th>Status</th><th class="hide-compact">Asli (MB)</th><th>Kompres (MB)</th><th>Hemat (%)</th><th>Kategori</th><th class="hide-compact">Waktu</th></tr></thead><tbody id="tbl-body"></tbody></table></div>
</div>
<div id="pane-node" class="tab-pane">
<div class="fc-wrap"><div class="fc-title">🔄 Alur Kerja Sistem (Bypass & Cabang)</div><svg class="fc-svg" id="live-svg" viewBox="0 0 920 280" preserveAspectRatio="xMidYMid meet"></svg></div>
<div class="panel"><div class="fc-title">📍 Detail Proses Aktif</div><div id="process-details-content"><div class="empty-note">Tidak ada proses aktif saat ini.</div></div></div>
</div>
<div id="pane-flow" class="tab-pane">
<div class="fc-wrap"><div class="fc-title">🗺️ Alur Sistem — Bahasa Manusia (mengalir sesuai kondisi nyata)</div><div id="stepper"><div class="empty-note">Memuat alur…</div></div></div>
</div>
<div id="pane-logs" class="tab-pane" style="padding:10px"><div class="log-viewer-wrapper"><div class="log-header">
<select id="log-selector" onchange="loadLog(this.value)"><option value="supervisor">Supervisor</option><option value="video">Video Worker</option><option value="photo">Photo Worker</option><option value="backup">Backup Rclone</option><option value="scanner">Scanner</option></select>
<button onclick="loadLog(document.getElementById('log-selector').value)">Refresh</button></div><div class="log-content" id="log-display">Loading...</div></div></div>
<div id="pane-sync" class="tab-pane">
<div class="fc-wrap"><div class="fc-title">☁️ Alur Kerja Backup MEGA</div><svg class="fc-svg" id="sync-svg" viewBox="0 0 560 120" preserveAspectRatio="xMidYMid meet" style="min-width:480px"></svg></div>
<div class="sync-controls">
<button class="shortcut-btn" onclick="syncNow()">☁️ Sync Sekarang</button>
<button class="shortcut-btn" id="btn-sync-pause" onclick="pauseSync(true)">⏸️ Pause Sync</button>
<button class="shortcut-btn" id="btn-sync-resume" onclick="pauseSync(false)">▶️ Resume Sync</button>
<button class="shortcut-btn" onclick="retrySync()">🔁 Retry Yang Gagal</button>
<button class="shortcut-btn" id="btn-sync-normal" onclick="setSyncMode('normal')">⚙️ Sync Normal</button>
<button class="shortcut-btn" id="btn-sync-force" onclick="setSyncMode('force')">🔥 Sync Force</button>
</div>
<div class="stats-grid">
<div class="stat-card"><div class="stat-val c-blue" id="sync-total-files">0</div><div class="stat-lbl">Total Ter-Backup</div></div>
<div class="stat-card"><div class="stat-val c-green" id="sync-total-size">0 GB</div><div class="stat-lbl">Ukuran di Cloud</div></div>
<div class="stat-card"><div class="stat-val c-yellow" id="sync-queue">0</div><div class="stat-lbl">Antrian Upload</div></div>
<div class="stat-card"><div class="stat-val c-red" id="sync-errors">0</div><div class="stat-lbl">Gagal Upload</div></div>
</div>
<div class="files-table-container"><table><thead><tr><th>Nama File</th><th>Tipe</th><th>Ukuran Akhir</th><th>Status Backup</th><th>Waktu</th></tr></thead><tbody id="tbl-sync-body"></tbody></table></div>
</div>
<div id="pane-help" class="tab-pane">
<div class="panel"><h3 class="h-blue">🛠️ Panduan</h3><ul><li><b>Dashboard:</b> statistik penghematan & antrian kompresi.</li><li><b>Live Flow:</b> flowchart node + bypass Force + cabang Replace/Skip.</li><li><b>Alur Sistem:</b> cerita langkah‑demi‑langkah berbahasa awam yang menyala sesuai kondisi runtime.</li><li><b>Sync to Mega:</b> Normal = upload saat kompresi idle; Force = upload terus (maks 1 worker).</li></ul></div>
<div class="panel"><h3 class="h-yellow">⚠️ Penting</h3><ul><li>Video <b>X265 ultrafast</b>; Foto JPG q82 / PNG lossless / WebP q80.</li><li>Limit aman Odroid C2: <b>3 Foto / 1 Video</b>; Sync Force tambah maks <b>1 Rclone</b>.</li><li>Top bar: mode kompresi + posisi + mode sync + suhu berwarna + RAM MB.</li><li>Warna suhu: hijau normal, kuning kerja, merah dekat overheat.</li></ul></div>
</div>
</div>
<script src="app.js"></script>
<script src="flow.js"></script>
</body></html>
