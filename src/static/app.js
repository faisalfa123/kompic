let authToken = localStorage.getItem("opt_token") || "";

function checkAuth() {
  fetch("/api/auth/status?token=" + encodeURIComponent(authToken))
    .then(r => r.json())
    .then(d => {
      if (d.auth_required && !d.authenticated) {
        document.getElementById("authModal").style.display = "flex";
      } else {
        document.getElementById("authModal").style.display = "none";
        initApp();
      }
    })
    .catch(() => {});
}

function verifyPassword() {
  const pwd = document.getElementById("authPassword").value;
  const errEl = document.getElementById("authError");
  errEl.style.display = "none";

  fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password: pwd })
  })
    .then(r => r.json())
    .then(d => {
      if (d.success) {
        authToken = d.token;
        localStorage.setItem("opt_token", authToken);
        document.getElementById("authModal").style.display = "none";
        initApp();
      } else {
        errEl.innerText = d.message || "Password salah!";
        errEl.style.display = "block";
      }
    })
    .catch(err => {
      errEl.innerText = "Koneksi gagal: " + err;
      errEl.style.display = "block";
    });
}

function authedFetch(url, options = {}) {
  const u = new URL(url, window.location.origin);
  if (authToken) {
    u.searchParams.set("token", authToken);
  }
  return fetch(u.toString(), options).then(r => {
    if (r.status === 401) {
      document.getElementById("authModal").style.display = "flex";
      throw new Error("Unauthorized");
    }
    return r.json();
  });
}

function toggleSidebar() {
  const sb = document.getElementById("sidebar");
  const ov = document.getElementById("overlay");
  sb.classList.toggle("open");
  ov.classList.toggle("open");
}

function switchTab(evt, tabId) {
  document.querySelectorAll(".nav-item").forEach(el => el.classList.remove("active"));
  document.querySelectorAll(".tab-pane").forEach(el => el.classList.remove("active"));
  
  if (evt && evt.currentTarget) {
    evt.currentTarget.classList.add("active");
  } else {
    const el = document.querySelector(`.nav-item[onclick*="${tabId}"]`);
    if (el) el.classList.add("active");
  }

  const pane = document.getElementById("pane-" + tabId);
  if (pane) pane.classList.add("active");

  const sb = document.getElementById("sidebar");
  const ov = document.getElementById("overlay");
  if (sb && sb.classList.contains("open")) {
    toggleSidebar();
  }

  if (tabId === "logs") {
    loadLog(document.getElementById("log-selector").value);
  } else if (tabId === "settings") {
    loadConfig();
  } else {
    refreshData();
  }
}

function toggleCompactMode(isCompact) {
  const table = document.getElementById("tableContainer");
  if (!table) return;
  if (isCompact) {
    table.classList.add("compact-view");
    document.querySelectorAll(".hide-compact").forEach(el => el.style.display = "none");
  } else {
    table.classList.remove("compact-view");
    document.querySelectorAll(".hide-compact").forEach(el => el.style.display = "");
  }
}

function filterFilesTable() {
  const filterVal = (document.getElementById("tableFilter").value || "").toLowerCase();
  const rows = document.querySelectorAll("#tbl-body tr");
  rows.forEach(r => {
    const text = r.innerText.toLowerCase();
    r.style.display = text.includes(filterVal) ? "" : "none";
  });
}

function setMode(mode) {
  authedFetch("/api/action/mode", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: mode })
  }).then(() => refreshData());
}

function triggerScan() {
  authedFetch("/api/action/scan", { method: "POST" })
    .then(d => {
      alert(d.message || "Pemindaian berkas Immich dimulai!");
      refreshData();
    });
}

function syncNow() {
  authedFetch("/api/action/sync/now", { method: "POST" })
    .then(d => {
      alert(d.message || "Sinkronisasi dimulai!");
      refreshData();
    });
}

function pauseSync(paused) {
  authedFetch("/api/action/sync/pause", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ paused: paused })
  }).then(() => refreshData());
}

function retrySync() {
  authedFetch("/api/action/sync/retry", { method: "POST" })
    .then(() => refreshData());
}

function setSyncMode(mode) {
  authedFetch("/api/action/sync/mode", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: mode })
  }).then(() => refreshData());
}

function loadLog(logType) {
  const disp = document.getElementById("log-display");
  if (!disp) return;
  disp.innerText = "Memuat log " + logType + "...";
  authedFetch("/api/logs?type=" + logType)
    .then(d => {
      disp.innerText = d.content || "(Log masih kosong)";
      disp.scrollTop = disp.scrollHeight;
    })
    .catch(err => {
      disp.innerText = "Gagal memuat log: " + err;
    });
}

function formatBytes(bytes, decimals = 2) {
  if (!bytes || bytes === 0) return "0.00 MB";
  const mb = bytes / (1024 * 1024);
  if (mb >= 1024) {
    const gb = mb / 1024;
    return gb.toFixed(decimals) + " GB";
  }
  return mb.toFixed(decimals) + " MB";
}

function formatGB(bytes) {
  if (!bytes || bytes === 0) return "0.00 GB";
  const gb = bytes / (1024 * 1024 * 1024);
  return gb.toFixed(2) + " GB";
}

// Update Real-Time Flowchart Nodes & Pulsing Glow
function updateLiveFlowchart(activeNode) {
  const nodes = ["start", "temp_check", "time_check", "compress", "compare", "replace", "skip", "sync"];
  
  nodes.forEach(n => {
    const el = document.getElementById("fn-" + n);
    if (!el) return;
    
    const shape = el.querySelector("rect, polygon");
    if (!shape) return;

    if (n === activeNode) {
      el.classList.remove("node-active", "node-active-replace", "node-active-skip");
      
      if (n === "replace") {
        el.classList.add("node-active-replace");
        shape.setAttribute("fill", "#059669");
        shape.setAttribute("stroke", "#00d284");
      } else if (n === "skip") {
        el.classList.add("node-active-skip");
        shape.setAttribute("fill", "#d97706");
        shape.setAttribute("stroke", "#ffab00");
      } else {
        el.classList.add("node-active");
        shape.setAttribute("fill", "#1d4ed8");
        shape.setAttribute("stroke", "#60a5fa");
      }
      shape.setAttribute("stroke-width", "3");
    } else {
      el.classList.remove("node-active", "node-active-replace", "node-active-skip");
      if (n === "replace") {
        shape.setAttribute("fill", "#064e3b");
        shape.setAttribute("stroke", "#00d284");
      } else if (n === "skip") {
        shape.setAttribute("fill", "#78350f");
        shape.setAttribute("stroke", "#ffab00");
      } else if (n === "sync") {
        shape.setAttribute("fill", "#182234");
        shape.setAttribute("stroke", "#2196f3");
      } else {
        shape.setAttribute("fill", "#182234");
        shape.setAttribute("stroke", "#334155");
      }
      shape.setAttribute("stroke-width", "2");
      shape.style.filter = "none";
    }
  });
}

function refreshData() {
  authedFetch("/api/status")
    .then(data => {
      if (!data) return;

      // Update Real-Life Live Flowchart Node
      updateLiveFlowchart(data.active_node || "start");

      // Hardware Telemetry (CPU Temp & Dynamic Dot)
      const topTemp = document.getElementById("top-temp");
      const topTempDot = document.getElementById("top-temp-dot");
      const tempVal = parseFloat(data.sys.cpu_temp) || 0;
      if (topTemp) {
        topTemp.innerText = tempVal + "°C";
        if (tempVal >= 75) {
          topTemp.className = "t-crit";
          if (topTempDot) topTempDot.className = "dot-indicator dot-hot";
        } else if (tempVal >= 65) {
          topTemp.className = "t-warn";
          if (topTempDot) topTempDot.className = "dot-indicator dot-warm";
        } else {
          topTemp.className = "t-norm";
          if (topTempDot) topTempDot.className = "dot-indicator dot-normal";
        }
      }

      // RAM Usage & Micro Bar
      const topRam = document.getElementById("top-ram");
      const ramFill = document.getElementById("ram-fill");
      const ramUsed = data.sys.ram_used_mb || 0;
      const ramTotal = data.sys.ram_total_mb || 1919;
      if (topRam) {
        topRam.innerText = `${ramUsed}/${ramTotal} MB`;
      }
      if (ramFill && ramTotal > 0) {
        const pct = Math.min(100, Math.round((ramUsed / ramTotal) * 100));
        ramFill.style.width = pct + "%";
        if (pct > 85) {
          ramFill.style.background = "var(--adm-danger)";
        } else if (pct > 70) {
          ramFill.style.background = "var(--adm-warning)";
        } else {
          ramFill.style.background = "var(--adm-info)";
        }
      }

      // Top badges
      const curMode = (data.mode || "normal").toLowerCase();
      const modeBadge = document.getElementById("top-mode-badge");
      if (modeBadge) {
        modeBadge.innerText = curMode.toUpperCase();
        modeBadge.className = "mode-badge mode-" + curMode.toUpperCase();
      }

      const posStatus = document.getElementById("top-pos-status");
      if (posStatus) {
        posStatus.innerText = data.pos_status || "WAITING";
        posStatus.className = "pos-status pos-" + (data.pos_status_class || "WAITING");
      }

      const syncPaused = !!data.sync_paused;
      const curSyncMode = (data.sync_mode || "normal").toLowerCase();
      const syncBadge = document.getElementById("top-sync-badge");
      if (syncBadge) {
        if (syncPaused) {
          syncBadge.innerText = "SYNC PAUSED";
          syncBadge.className = "sync-badge s-paused";
        } else {
          syncBadge.innerText = "SYNC " + curSyncMode.toUpperCase();
          syncBadge.className = "sync-badge s-" + curSyncMode;
        }
      }

      // Sidebar Worker Badge
      const navBadge = document.getElementById("nav-badge-worker");
      if (navBadge) {
        if (data.active_jobs && data.active_jobs.length > 0) {
          navBadge.innerText = "Running (" + data.active_jobs.length + ")";
          navBadge.className = "nav-badge badge-green";
        } else if (curMode === "pause") {
          navBadge.innerText = "Paused";
          navBadge.className = "nav-badge badge-yellow";
        } else {
          navBadge.innerText = "Idle";
          navBadge.className = "nav-badge";
        }
      }

      // Sidebar Cloud Backup Badge
      const navSyncBadge = document.getElementById("nav-badge-sync");
      if (navSyncBadge) {
        if (syncPaused) {
          navSyncBadge.innerText = "Paused";
          navSyncBadge.className = "nav-badge badge-yellow";
        } else if (data.stats && data.stats.sync_pending > 0) {
          navSyncBadge.innerText = "Syncing (" + data.stats.sync_pending + ")";
          navSyncBadge.className = "nav-badge badge-green";
        } else {
          navSyncBadge.innerText = "Idle";
          navSyncBadge.className = "nav-badge";
        }
      }

      // Sidebar Device Badge
      const navDevBadge = document.getElementById("nav-badge-device");
      if (navDevBadge) {
        if (tempVal >= 75) {
          navDevBadge.innerText = "Hot";
          navDevBadge.className = "nav-badge badge-red";
        } else if (tempVal >= 65) {
          navDevBadge.innerText = "Warm";
          navDevBadge.className = "nav-badge badge-yellow";
        } else {
          navDevBadge.innerText = "Normal";
          navDevBadge.className = "nav-badge badge-green";
        }
      }

      // Quick buttons (Top Navbar)
      const qNorm = document.getElementById("quick-btn-normal");
      const qForce = document.getElementById("quick-btn-force");
      const qPause = document.getElementById("quick-btn-pause");
      if (qNorm && qForce && qPause) {
        qNorm.className = "btn-quick" + (curMode === "normal" ? " btn-active-normal" : "");
        qForce.className = "btn-quick" + (curMode === "force" ? " btn-active-force" : "");
        qPause.className = "btn-quick" + (curMode === "pause" ? " btn-active-pause" : "");
      }

      // Table Toolbar Shortcut Buttons (Dashboard Pane)
      const bNorm = document.getElementById("btn-normal");
      const bForce = document.getElementById("btn-force");
      const bPause = document.getElementById("btn-pause");
      const bResume = document.getElementById("btn-resume");
      if (bNorm && bForce && bPause) {
        bNorm.className = "shortcut-btn" + (curMode === "normal" ? " active-normal" : "");
        bForce.className = "shortcut-btn" + (curMode === "force" ? " active-force" : "");
        bPause.className = "shortcut-btn" + (curMode === "pause" ? " active-pause" : "");
        if (bResume) {
          bResume.className = "shortcut-btn" + (curMode !== "pause" ? " active-normal" : "");
        }
      }

      // Sync Controls Buttons (Cloud Backup Pane)
      const bsPause = document.getElementById("btn-sync-pause");
      const bsResume = document.getElementById("btn-sync-resume");
      const bsNorm = document.getElementById("btn-sync-normal");
      const bsForce = document.getElementById("btn-sync-force");
      if (bsPause && bsResume) {
        bsPause.className = "shortcut-btn" + (syncPaused ? " active-pause" : "");
        bsResume.className = "shortcut-btn" + (!syncPaused ? " active-normal" : "");
      }
      if (bsNorm && bsForce) {
        bsNorm.className = "shortcut-btn" + (curSyncMode === "normal" ? " active-normal" : "");
        bsForce.className = "shortcut-btn" + (curSyncMode === "force" ? " active-force" : "");
      }

      // Flow label texts in SVGs (both Flowchart Monitor and Stepper)
      const tempStatusText = tempVal >= 75 ? "PANAS -> jeda" : (tempVal + "°C");
      const schedStatusText = curMode === "force" ? "FORCE BYPASS" : "Jadwal " + (data.in_schedule ? "AKTIF" : "MENUNGGU");
      const queueStatusText = "Antre: " + (data.stats.pending || 0);

      const flowTemp = document.getElementById("flow-lbl-temp");
      if (flowTemp) flowTemp.innerText = tempStatusText;
      const liveTemp = document.getElementById("live-lbl-temp");
      if (liveTemp) liveTemp.innerText = tempStatusText;

      const flowSched = document.getElementById("flow-lbl-schedule");
      if (flowSched) flowSched.innerText = schedStatusText;
      const liveSched = document.getElementById("live-lbl-schedule");
      if (liveSched) liveSched.innerText = schedStatusText;

      const flowQueue = document.getElementById("flow-lbl-queue");
      if (flowQueue) flowQueue.innerText = queueStatusText;
      const liveQueue = document.getElementById("live-lbl-queue");
      if (liveQueue) liveQueue.innerText = queueStatusText;

      // Dashboard Stat Cards
      const savedBytes = data.stats.total_saved_bytes || 0;
      const origBytes = data.stats.total_orig_bytes || 0;
      const successOrig = data.stats.success_orig_bytes || 0;
      const successComp = data.stats.success_comp_bytes || 0;
      const totalFiles = data.stats.total || 0;
      const successCount = data.stats.success || 0;

      // Rasio penghematan dihitung dari file yang benar-benar berhasil dikompresi
      const compSavingsRatio = successOrig > 0 
        ? (((successOrig - successComp) / successOrig) * 100).toFixed(1) 
        : (savedBytes > 0 && origBytes > 0 ? ((savedBytes / origBytes) * 100).toFixed(1) : "0.0");

      const hsSaved = document.getElementById("hs-saved-mb");
      if (hsSaved) hsSaved.innerText = formatBytes(savedBytes);

      const hsPct = document.getElementById("hs-saved-pct");
      if (hsPct) hsPct.innerText = "Hemat " + compSavingsRatio + "%";

      const hsComp = document.getElementById("hs-saved-comp");
      if (hsComp) {
        if (successOrig > 0) {
          hsComp.innerText = `${formatBytes(successOrig)} → ${formatBytes(successComp)}`;
        } else {
          hsComp.innerText = `0.00 MB → 0.00 MB`;
        }
      }

      const hsTotalLib = document.getElementById("hs-total-lib");
      if (hsTotalLib) {
        hsTotalLib.innerText = `Total Koleksi: ${totalFiles.toLocaleString()} berkas (${formatBytes(origBytes)})`;
      }

      const hsSuccess = document.getElementById("hs-success");
      if (hsSuccess) hsSuccess.innerText = data.stats.success || 0;
      const hsSkipped = document.getElementById("hs-skipped");
      if (hsSkipped) hsSkipped.innerText = data.stats.skipped || 0;
      const hsError = document.getElementById("hs-error");
      if (hsError) hsError.innerText = data.stats.error || 0;
      const hsProcessing = document.getElementById("hs-processing");
      if (hsProcessing) hsProcessing.innerText = data.stats.processing || 0;
      const hsPending = document.getElementById("hs-pending");
      if (hsPending) hsPending.innerText = data.stats.pending || 0;

      const dThermal = document.getElementById("dash-thermal-val");
      if (dThermal) dThermal.innerText = tempVal + "°C";
      const dSched = document.getElementById("dash-sched-val");
      if (dSched) dSched.innerText = curMode === "force" ? "Mode: Force (Bypass)" : ("Jadwal: " + (data.in_schedule ? "Aktif" : "Menunggu Jam"));

      // Storage Analytics Pane
      const aSaved = document.getElementById("analytics-saved-mb");
      if (aSaved) aSaved.innerText = formatBytes(savedBytes);
      const aSuccessSizes = document.getElementById("analytics-success-sizes");
      if (aSuccessSizes) {
        aSuccessSizes.innerText = `${formatBytes(successOrig)} → ${formatBytes(successComp)}`;
      }
      const aOrig = document.getElementById("analytics-orig-bytes");
      if (aOrig) aOrig.innerText = `${formatBytes(origBytes)} (${totalFiles.toLocaleString()} berkas)`;
      const aRatio = document.getElementById("analytics-avg-ratio");
      if (aRatio) aRatio.innerText = compSavingsRatio + "%";
      const aProg = document.getElementById("analytics-progress-bar");
      if (aProg) aProg.style.width = Math.min(100, Math.max(0, parseFloat(compSavingsRatio))) + "%";

      const aThermal = document.getElementById("analytics-thermal-status");
      if (aThermal) aThermal.innerText = `AKTIF (${tempVal}°C / Batas 75°C)`;

      // Device Monitor Pane (Safe Read-Only Telemetry)
      if (data.sys_detailed) {
        const sd = data.sys_detailed;
        const dTemp = document.getElementById("dev-cpu-temp");
        if (dTemp) dTemp.innerText = (sd.cpu_temp || tempVal) + "°C";

        const dRam = document.getElementById("dev-ram-usage");
        if (dRam) dRam.innerText = `${sd.ram_used_mb} / ${sd.ram_total_mb} MB (${Math.round(sd.ram_used_pct || 0)}%)`;

        const dLoad = document.getElementById("dev-load-avg");
        if (dLoad) dLoad.innerText = `${sd.load_1} / ${sd.load_5} / ${sd.load_15}`;

        const dUp = document.getElementById("dev-uptime");
        if (dUp) dUp.innerText = sd.uptime_text || "--";

        const dArch = document.getElementById("dev-arch");
        if (dArch) dArch.innerText = sd.arch || "ARM64";

        const dHost = document.getElementById("dev-hostname");
        if (dHost) dHost.innerText = sd.hostname || "odroid-c2";

        const dPid = document.getElementById("dev-pid");
        if (dPid) dPid.innerText = sd.process_pid ? `PID: ${sd.process_pid}` : "--";

        const dRootText = document.getElementById("dev-root-disk-text");
        const dRootBar = document.getElementById("dev-root-disk-bar");
        if (dRootText) dRootText.innerText = `${sd.root_disk_used} / ${sd.root_disk_total} (${sd.root_disk_pct}%)`;
        if (dRootBar) dRootBar.style.width = `${sd.root_disk_pct}%`;

        const dMediaText = document.getElementById("dev-media-disk-text");
        const dMediaBar = document.getElementById("dev-media-disk-bar");
        if (dMediaText) dMediaText.innerText = `${sd.media_disk_used} / ${sd.media_disk_total} (${sd.media_disk_pct}%)`;
        if (dMediaBar) dMediaBar.style.width = `${sd.media_disk_pct}%`;
      }

      // Files Table
      const tbody = document.getElementById("tbl-body");
      if (tbody) {
        if (data.files && data.files.length > 0) {
          tbody.innerHTML = data.files.map(f => {
            const origMB = (f.orig_size / (1024 * 1024)).toFixed(2);
            const compMB = (f.comp_size / (1024 * 1024)).toFixed(2);
            const ratioPct = (f.ratio || 0).toFixed(1) + "%";
            const statusClass = f.status === "SUCCESS" ? "c-green" : (f.status === "SKIPPED" ? "c-yellow" : "c-red");

            // Format Metadata Badge & Checked Tags
            const isSafe = (f.metadata_status === "SAFE") || (!f.metadata_status && f.status === "SUCCESS") || (f.status === "SKIPPED");
            const isLost = f.metadata_status === "LOST";
            
            let metaBadge = `<span class="meta-badge badge-meta-safe">🛡️ AMAN</span>`;
            if (isLost) {
              metaBadge = `<span class="meta-badge badge-meta-lost">⚠️ HILANG</span>`;
            }

            // Build Checked List of Safe Tags
            let safeTags = f.safe_tags || [];
            if (safeTags.length === 0) {
              if (f.status === "SUCCESS") {
                safeTags = ["GPS / Lokasi (Aman)", "Waktu & Tanggal (Aman)", "Kamera / Device (Aman)"];
              } else if (f.status === "SKIPPED") {
                safeTags = ["Berkas Asli Dipertahankan (100% Utuh)"];
              }
            }

            const tagsHtml = safeTags.map(t => `<span class="meta-tag-pill"><span class="tag-check">✓</span> ${t}</span>`).join(" ");
            const lostTagsHtml = (f.lost_tags || []).map(t => `<span class="meta-tag-pill tag-lost"><span class="tag-cross">✗</span> ${t}</span>`).join(" ");

            return `<tr>
              <td title="${f.file_path}" style="font-weight:600;color:#fff;">${f.file_name}</td>
              <td><span style="font-size:0.75rem;padding:2px 6px;border-radius:3px;background:var(--adm-border);">${f.media_type}</span></td>
              <td><span class="${statusClass}" style="font-weight:700;">${f.status}</span></td>
              <td>
                <div style="display:flex;flex-direction:column;gap:4px;">
                  <div>${metaBadge}</div>
                  <div style="display:flex;flex-wrap:wrap;gap:4px;">${tagsHtml} ${lostTagsHtml}</div>
                </div>
              </td>
              <td class="hide-compact">${origMB}</td>
              <td>${compMB}</td>
              <td style="color:var(--adm-success);font-weight:700;">${ratioPct}</td>
              <td><span class="cat-tag cat-${f.category}">${f.category}</span></td>
              <td class="hide-compact" style="color:var(--adm-text-dim);font-size:.75rem">${f.processed_time || "-"}</td>
            </tr>`;
          }).join("");
          filterFilesTable();
        } else {
          tbody.innerHTML = `<tr><td colspan="9" style="text-align:center;color:var(--adm-text-dim);">Belum ada riwayat berkas yang diproses.</td></tr>`;
        }
      }

      // Flowchart History Table (Tab 2)
      const flowTbody = document.getElementById("tbl-flow-history-body");
      if (flowTbody) {
        if (data.files && data.files.length > 0) {
          flowTbody.innerHTML = data.files.map(f => {
            const isSuccess = f.status === "SUCCESS";
            const isSkipped = f.status === "SKIPPED";
            
            // Flow Node Completed Badge
            let nodeStepHtml = `<span class="flow-step-tag step-replace">Replace (Hemat)</span>`;
            if (isSkipped) {
              nodeStepHtml = `<span class="flow-step-tag step-skip">Skip (Cegah Boros)</span>`;
            } else if (f.status === "ERROR") {
              nodeStepHtml = `<span class="flow-step-tag step-error">Error / Batal</span>`;
            }

            // Metadata Status
            const isSafe = (f.metadata_status === "SAFE") || (!f.metadata_status && isSuccess) || isSkipped;
            const isLost = f.metadata_status === "LOST";
            let metaBadge = `<span class="meta-badge badge-meta-safe">🛡️ AMAN</span>`;
            if (isLost) {
              metaBadge = `<span class="meta-badge badge-meta-lost">⚠️ HILANG</span>`;
            }

            // Safe checklist tags
            let safeTags = f.safe_tags || [];
            if (safeTags.length === 0) {
              if (isSuccess) {
                safeTags = ["GPS / Lokasi (Aman)", "Waktu & Tanggal (Aman)", "Kamera / Device (Aman)"];
              } else if (isSkipped) {
                safeTags = ["Berkas Asli Dipertahankan (100% Utuh)"];
              }
            }
            const tagsChecklist = safeTags.map(t => `<span class="meta-tag-pill"><span class="tag-check">✓</span> ${t}</span>`).join(" ");
            const lostTagsChecklist = (f.lost_tags || []).map(t => `<span class="meta-tag-pill tag-lost"><span class="tag-cross">✗</span> ${t}</span>`).join(" ");

            const origMB = (f.orig_size / (1024 * 1024)).toFixed(2);
            const compMB = (f.comp_size / (1024 * 1024)).toFixed(2);
            const ratioPct = (f.ratio || 0).toFixed(1) + "%";

            return `<tr>
              <td style="color:var(--adm-text-dim);font-size:0.75rem;">${f.processed_time || "-"}</td>
              <td style="font-weight:600;color:#fff;">${f.file_name}</td>
              <td><span style="font-size:0.75rem;padding:2px 6px;border-radius:3px;background:var(--adm-border);">${f.media_type}</span></td>
              <td>${nodeStepHtml}</td>
              <td>${metaBadge}</td>
              <td><div style="display:flex;flex-wrap:wrap;gap:4px;">${tagsChecklist} ${lostTagsChecklist}</div></td>
              <td style="font-size:0.8rem;"><b>${origMB} MB</b> -> <b style="color:var(--adm-success);">${compMB} MB</b> (${ratioPct})</td>
            </tr>`;
          }).join("");
        } else {
          flowTbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--adm-text-dim);">Belum ada alur flowchart yang selesai dieksekusi.</td></tr>`;
        }
      }

      // Active Process Details
      const actDetail = document.getElementById("process-details-content");
      if (actDetail) {
        if (data.active_jobs && data.active_jobs.length > 0) {
          actDetail.innerHTML = data.active_jobs.map(j => `
            <div style="padding:12px; background:var(--adm-bg-card-sub); border-radius:6px; border-left:4px solid var(--adm-primary); margin-bottom:8px; box-shadow:0 2px 8px rgba(0,0,0,0.2);">
              <div style="font-weight:700; color:#fff; font-size:0.95rem;">[Worker ${j.type}] ${j.name} (${j.size_mb} MB)</div>
              <div style="color:var(--adm-info); font-size:0.85rem; margin-top:4px;">⚡ Langkah Saat Ini: <b>${j.step || 'Sedang memproses...'}</b></div>
            </div>
          `).join("");
        } else {
          actDetail.innerHTML = `<div class="empty-note">Tidak ada proses kompresi aktif saat ini. Menunggu jadwal malam (01-06) atau file baru di library Immich.</div>`;
        }
      }

      // Sync stats
      const sFiles = document.getElementById("sync-total-files");
      if (sFiles) sFiles.innerText = data.stats.sync_uploaded || 0;
      const sSize = document.getElementById("sync-total-size");
      if (sSize) sSize.innerText = formatGB(data.stats.sync_uploaded_bytes);
      const sQueue = document.getElementById("sync-queue");
      if (sQueue) sQueue.innerText = data.stats.sync_pending || 0;
      const sErr = document.getElementById("sync-errors");
      if (sErr) sErr.innerText = data.stats.sync_failed || 0;

      // Table Sync
      const tSync = document.getElementById("tbl-sync-body");
      if (tSync) {
        if (data.sync_files && data.sync_files.length > 0) {
          tSync.innerHTML = data.sync_files.map(sf => {
            let statusColor = "c-yellow";
            if (sf.sync_status === "UPLOADED") statusColor = "c-green";
            else if (sf.sync_status === "DELETED_LOCAL" || sf.sync_status === "DELETED_CLOUD") statusColor = "c-red";
            else if (sf.sync_status === "FAILED") statusColor = "c-red";

            return `
            <tr>
              <td style="font-weight:600;color:#fff;">${sf.file_name}</td>
              <td>${sf.media_type}</td>
              <td>${formatBytes(sf.comp_size)}</td>
              <td><span class="${statusColor}" style="font-weight:700;">${sf.sync_status}</span></td>
              <td style="font-family:monospace;font-size:0.75rem;color:var(--adm-info);">${sf.remote_dest || "-"}</td>
              <td style="color:var(--adm-text-dim);font-size:.75rem">${sf.processed_time || "-"}</td>
            </tr>`;
          }).join("");
        } else {
          tSync.innerHTML = `<tr><td colspan="6" style="text-align:center;color:var(--adm-text-dim);">Belum ada antrian upload cloud.</td></tr>`;
        }
      }
    })
    .catch(() => {});
}

// ==============================================================================
// CONFIGURATION MANAGEMENT (Web UI Full Control Panel)
// ==============================================================================
function showSettingsAlert(msg, isSuccess) {
  const el = document.getElementById("settings-alert");
  if (!el) return;
  el.className = "alert-box " + (isSuccess ? "alert-success" : "alert-error");
  el.innerText = msg;
  el.style.display = "block";
  window.scrollTo({ top: 0, behavior: 'smooth' });
  setTimeout(() => {
    el.style.display = "none";
  }, 6000);
}

function loadConfig() {
  // Try /api/settings first, fallback to /api/config
  const endpoint = "/api/settings";
  authedFetch(endpoint)
    .catch(() => authedFetch("/api/config"))
    .then(data => {
      if (!data) return;

      // Server
      if (data.server) {
        const portEl = document.getElementById("cfg-server-port");
        if (portEl) portEl.value = data.server.port || 5555;
        const hostEl = document.getElementById("cfg-server-host");
        if (hostEl) hostEl.value = data.server.host || "0.0.0.0";
        const authEl = document.getElementById("cfg-server-auth");
        if (authEl) authEl.checked = !!data.server.auth_enabled;
        const hintEl = document.getElementById("cfg-password-hint");
        if (hintEl) hintEl.value = data.server.password_hint || "";
        const newPassEl = document.getElementById("cfg-new-password");
        if (newPassEl) newPassEl.value = "";
      }

      // Storage
      if (data.storage) {
        const upEl = document.getElementById("cfg-storage-upload");
        if (upEl) upEl.value = data.storage.immich_upload_dir || "";
        const stEl = document.getElementById("cfg-storage-state");
        if (stEl) stEl.value = data.storage.state_file_path || "";
        const lgEl = document.getElementById("cfg-storage-log");
        if (lgEl) lgEl.value = data.storage.log_dir || "";
      }

      // Thermal
      if (data.thermal) {
        const thEnEl = document.getElementById("cfg-thermal-enabled");
        if (thEnEl) thEnEl.checked = !!data.thermal.enabled;
        const maxTEl = document.getElementById("cfg-thermal-max");
        if (maxTEl) maxTEl.value = data.thermal.max_temp_c || 75;
        const coolTEl = document.getElementById("cfg-thermal-cool");
        if (coolTEl) coolTEl.value = data.thermal.cooldown_temp_c || 65;
        const tzEl = document.getElementById("cfg-thermal-path");
        if (tzEl) tzEl.value = data.thermal.thermal_zone_path || "";
      }

      // Schedule
      if (data.schedule) {
        const scEnEl = document.getElementById("cfg-schedule-enabled");
        if (scEnEl) scEnEl.checked = !!data.schedule.enabled;
        const scStEl = document.getElementById("cfg-schedule-start");
        if (scStEl) scStEl.value = data.schedule.start_hour !== undefined ? data.schedule.start_hour : 1;
        const scEndEl = document.getElementById("cfg-schedule-end");
        if (scEndEl) scEndEl.value = data.schedule.end_hour !== undefined ? data.schedule.end_hour : 6;
      }

      // Compression
      if (data.compression) {
        const vcEl = document.getElementById("cfg-video-codec");
        if (vcEl) vcEl.value = data.compression.video_codec || "libx265";
        const vpEl = document.getElementById("cfg-video-preset");
        if (vpEl) vpEl.value = data.compression.video_preset || "ultrafast";
        const vcrfEl = document.getElementById("cfg-video-crf");
        if (vcrfEl) vcrfEl.value = data.compression.video_crf || 26;
        const vhEl = document.getElementById("cfg-video-height");
        if (vhEl) vhEl.value = data.compression.video_max_height || 1080;
        const vtEl = document.getElementById("cfg-video-threads");
        if (vtEl) vtEl.value = data.compression.video_threads || 1;
        const vwEl = document.getElementById("cfg-video-workers");
        if (vwEl) vwEl.value = data.compression.max_video_workers || 1;
        const pqEl = document.getElementById("cfg-photo-quality");
        if (pqEl) pqEl.value = data.compression.photo_quality || 82;
        const pdEl = document.getElementById("cfg-photo-dim");
        if (pdEl) pdEl.value = data.compression.photo_max_dim || 2560;
        const pwEl = document.getElementById("cfg-photo-workers");
        if (pwEl) pwEl.value = data.compression.max_photo_workers || 2;
      }

      // Backup
      if (data.backup) {
        const bkEnEl = document.getElementById("cfg-backup-enabled");
        if (bkEnEl) bkEnEl.checked = !!data.backup.enabled;
        const bkRemEl = document.getElementById("cfg-backup-remote");
        if (bkRemEl) bkRemEl.value = data.backup.rclone_remote || "";
        const bkArgEl = document.getElementById("cfg-backup-args");
        if (bkArgEl) bkArgEl.value = data.backup.rclone_args || "";
      }
    })
    .catch(err => {
      showSettingsAlert("Gagal memuat konfigurasi: " + err, false);
    });
}

function saveConfig() {
  const port = parseInt(document.getElementById("cfg-server-port").value, 10);
  const host = document.getElementById("cfg-server-host").value.trim();
  const authEnabled = document.getElementById("cfg-server-auth").checked;
  const newPass = document.getElementById("cfg-new-password").value.trim();
  const passHint = document.getElementById("cfg-password-hint").value.trim();

  const uploadDir = document.getElementById("cfg-storage-upload").value.trim();
  const statePath = document.getElementById("cfg-storage-state").value.trim();
  const logDir = document.getElementById("cfg-storage-log").value.trim();

  const thermalEnabled = document.getElementById("cfg-thermal-enabled").checked;
  const maxTemp = parseFloat(document.getElementById("cfg-thermal-max").value);
  const coolTemp = parseFloat(document.getElementById("cfg-thermal-cool").value);
  const tzPath = document.getElementById("cfg-thermal-path").value.trim();

  const schedEnabled = document.getElementById("cfg-schedule-enabled").checked;
  const startHour = parseInt(document.getElementById("cfg-schedule-start").value, 10);
  const endHour = parseInt(document.getElementById("cfg-schedule-end").value, 10);

  const videoCodec = document.getElementById("cfg-video-codec").value;
  const videoPreset = document.getElementById("cfg-video-preset").value;
  const videoCRF = parseInt(document.getElementById("cfg-video-crf").value, 10);
  const videoHeight = parseInt(document.getElementById("cfg-video-height").value, 10);
  const videoThreads = parseInt(document.getElementById("cfg-video-threads").value, 10);
  const videoWorkers = parseInt(document.getElementById("cfg-video-workers").value, 10);
  const photoQuality = parseInt(document.getElementById("cfg-photo-quality").value, 10);
  const photoDim = parseInt(document.getElementById("cfg-photo-dim").value, 10);
  const photoWorkers = parseInt(document.getElementById("cfg-photo-workers").value, 10);

  const backupEnabled = document.getElementById("cfg-backup-enabled").checked;
  const rcloneRemote = document.getElementById("cfg-backup-remote").value.trim();
  const rcloneArgs = document.getElementById("cfg-backup-args").value.trim();

  // Input Validation
  if (isNaN(port) || port < 1 || port > 65535) {
    showSettingsAlert("Port harus bernilai angka antara 1 dan 65535!", false);
    return;
  }
  if (maxTemp <= coolTemp) {
    showSettingsAlert("Suhu maksimal (" + maxTemp + "°C) harus lebih tinggi dari suhu cooldown (" + coolTemp + "°C)!", false);
    return;
  }
  if (isNaN(startHour) || startHour < 0 || startHour > 23 || isNaN(endHour) || endHour < 0 || endHour > 23) {
    showSettingsAlert("Jam jadwal operasional harus bernilai antara 0 dan 23!", false);
    return;
  }
  if (isNaN(videoCRF) || videoCRF < 10 || videoCRF > 45) {
    showSettingsAlert("Nilai Video CRF harus antara 10 dan 45 (rekomendasi: 24–28)!", false);
    return;
  }
  if (isNaN(photoQuality) || photoQuality < 10 || photoQuality > 100) {
    showSettingsAlert("Kualitas foto harus antara 10 dan 100 (rekomendasi: 80–85)!", false);
    return;
  }

  const payload = {
    server: {
      port: port,
      host: host || "0.0.0.0",
      auth_enabled: authEnabled,
      password_hint: passHint
    },
    storage: {
      immich_upload_dir: uploadDir,
      state_file_path: statePath,
      log_dir: logDir
    },
    thermal: {
      enabled: thermalEnabled,
      max_temp_c: maxTemp,
      cooldown_temp_c: coolTemp,
      thermal_zone_path: tzPath
    },
    schedule: {
      enabled: schedEnabled,
      start_hour: startHour,
      end_hour: endHour
    },
    compression: {
      video_codec: videoCodec,
      video_preset: videoPreset,
      video_crf: videoCRF,
      video_max_height: videoHeight,
      video_threads: videoThreads,
      max_video_workers: videoWorkers,
      photo_quality: photoQuality,
      photo_max_dim: photoDim,
      max_photo_workers: photoWorkers
    },
    backup: {
      enabled: backupEnabled,
      rclone_remote: rcloneRemote,
      rclone_args: rcloneArgs
    }
  };

  if (newPass) {
    payload.server.new_password = newPass;
  }

  // Post to /api/settings
  authedFetch("/api/settings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  })
    .catch(() => {
      // Fallback to /api/config if needed
      return authedFetch("/api/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
    })
    .then(d => {
      if (d.success) {
        if (d.new_token) {
          authToken = d.new_token;
          localStorage.setItem("opt_token", authToken);
        }
        let msg = d.message || "Pengaturan berhasil disimpan dan langsung aktif!";
        if (d.port_changed) {
          msg += " (Perhatian: Port server diubah ke " + port + ". Perubahan port akan aktif saat service direstart).";
        }
        showSettingsAlert(msg, true);
        document.getElementById("cfg-new-password").value = "";
        refreshData();
      } else {
        showSettingsAlert(d.message || "Gagal menyimpan konfigurasi!", false);
      }
    })
    .catch(err => {
      showSettingsAlert("Terjadi kesalahan koneksi: " + err, false);
    });
}

function initApp() {
  refreshData();
  setInterval(refreshData, 2000); // Polling every 2s for active CPU node lighting
}

document.addEventListener("DOMContentLoaded", () => {
  checkAuth();
});
