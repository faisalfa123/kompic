// kompic Modern Frontend Application Controller
let authToken = sessionStorage.getItem("kompic_token") || "";

document.addEventListener("DOMContentLoaded", () => {
  checkAuth();
  startMetricsPolling();
  initNewsWidget();
});

function checkAuth() {
  fetch("/api/auth/status" + (authToken ? `?token=${encodeURIComponent(authToken)}` : ""))
    .then(r => r.json())
    .then(data => {
      if (data.auth_required && !data.authenticated) {
        document.getElementById("authModal").style.display = "flex";
      } else {
        document.getElementById("authModal").style.display = "none";
        refreshDashboard();
        loadSettings();
      }
    })
    .catch(() => {});
}

function verifyPassword() {
  const pwd = document.getElementById("authPassword").value;
  if (!pwd) return;

  fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password: pwd })
  })
    .then(r => r.json())
    .then(data => {
      if (data.success && data.token) {
        authToken = data.token;
        sessionStorage.setItem("kompic_token", authToken);
        document.getElementById("authModal").style.display = "none";
        document.getElementById("authError").style.display = "none";
        refreshDashboard();
        loadSettings();
      } else {
        document.getElementById("authError").style.display = "block";
      }
    })
    .catch(() => {
      document.getElementById("authError").style.display = "block";
    });
}

function authFetch(url, options = {}) {
  options.headers = options.headers || {};
  if (authToken) {
    options.headers["Authorization"] = `Bearer ${authToken}`;
  }
  return fetch(url, options);
}

function switchTab(tabId) {
  document.querySelectorAll(".tab-pane").forEach(el => el.classList.remove("active"));
  document.querySelectorAll(".nav-link").forEach(el => el.classList.remove("active"));

  const targetTab = document.getElementById(`tab-${tabId}`);
  if (targetTab) targetTab.classList.add("active");

  const links = document.querySelectorAll(".nav-link");
  links.forEach(l => {
    if (l.getAttribute("onclick") && l.getAttribute("onclick").includes(tabId)) {
      l.classList.add("active");
    }
  });

  const titles = {
    dashboard: "Dashboard Ringkasan",
    pipeline: "n8n Workflow Eksekusi",
    activity: "Log Aktivitas Sistem",
    settings: "Pengaturan & Tema Berita"
  };
  document.getElementById("pageTitle").innerText = titles[tabId] || "kompic";

  if (tabId === "activity") {
    loadRawLogs();
  }
}

function startMetricsPolling() {
  setInterval(() => {
    if (document.getElementById("authModal").style.display === "none") {
      refreshDashboard();
    }
  }, 3000);
}

function refreshDashboard() {
  // 1. Stats Utama
  authFetch("/api/stats")
    .then(r => r.json())
    .then(data => {
      const savedBytes = data.total_saved_bytes || 0;
      document.getElementById("valSaved").innerText = formatBytes(savedBytes);

      const totalOrig = data.total_orig_bytes || data.total_original_bytes || 1;
      const ratio = totalOrig > 0 ? ((savedBytes / totalOrig) * 100).toFixed(1) : 0;
      const totalCount = data.total_files || (data.success + data.skipped + data.error + data.pending) || 0;
      document.getElementById("subSavedRatio").innerText = `Hemat ${ratio}% (Koleksi: ${totalCount} berkas)`;

      const pending = data.pending || 0;
      const processing = data.processing || (data.active_jobs ? data.active_jobs.length : 0);
      document.getElementById("valQueue").innerText = pending + processing;
      document.getElementById("subWorkers").innerText = `Aktif: ${processing} file (Max 3 Threads)`;

      document.getElementById("valSuccess").innerText = data.success || 0;
      document.getElementById("valSkipped").innerText = data.skipped || 0;
      document.getElementById("valError").innerText = data.error || 0;

      // Update Node pada n8n Workflow
      const activeNode = data.active_node || "start";
      updateWorkflowNode(activeNode);
    })
    .catch(() => {});

  // 2. Hardware Suhu S905X
  authFetch("/api/system")
    .then(r => r.json())
    .then(hw => {
      const temp = hw.cpu_temp_c || 0;
      document.getElementById("valTemp").innerText = `${temp.toFixed(1)} °C`;
      document.getElementById("headerTemp").innerText = `${temp.toFixed(1)} °C`;

      const dot = document.getElementById("thermalDot");
      if (temp > 68) {
        dot.style.background = "var(--danger)";
      } else if (temp > 60) {
        dot.style.background = "var(--warning)";
      } else {
        dot.style.background = "var(--success)";
      }
    })
    .catch(() => {});

  // 3. Tabel Log Media Lengkap
  authFetch("/api/files?limit=40")
    .then(r => r.json())
    .then(files => {
      renderTable(files);
    })
    .catch(() => {});
}

function updateWorkflowNode(nodeName) {
  document.getElementById("headerNodeStatus").innerText = `Node: ${nodeName.toUpperCase()}`;

  document.querySelectorAll(".n8n-node").forEach(n => n.classList.remove("active-pulse"));
  document.querySelectorAll(".n8n-curve").forEach(c => c.classList.remove("curve-active"));

  const nodeEl = document.getElementById(`node-${nodeName}`);
  if (nodeEl) nodeEl.classList.add("active-pulse");

  if (nodeName === "start" || nodeName === "temp_check") {
    document.getElementById("curve-start-temp")?.classList.add("curve-active");
  } else if (nodeName === "compress") {
    document.getElementById("curve-temp-comp")?.classList.add("curve-active");
  } else if (nodeName === "compare") {
    document.getElementById("curve-comp-compare")?.classList.add("curve-active");
  } else if (nodeName === "replace") {
    document.getElementById("curve-compare-replace")?.classList.add("curve-active");
    document.getElementById("curve-replace-cloud")?.classList.add("curve-active");
  } else if (nodeName === "skip") {
    document.getElementById("curve-compare-skip")?.classList.add("curve-active");
  } else if (nodeName === "sync") {
    document.getElementById("curve-replace-cloud")?.classList.add("curve-active");
  }
}

function renderTable(files) {
  const tbody = document.getElementById("filesTableBody");
  if (!files || files.length === 0) {
    tbody.innerHTML = `<tr><td colspan="9" style="text-align: center; color: var(--text-dim); padding: 24px;">Belum ada media yang dioptimasi.</td></tr>`;
    return;
  }

  tbody.innerHTML = files.map(f => {
    const isVideo = f.file_path && f.file_path.match(/\.(mp4|mov|m4v|mkv)$/i);
    const typeBadge = isVideo ? `<span style="background: rgba(99,102,241,0.2); color:#c7d2fe; padding:2px 6px; border-radius:4px; font-weight:700;">V</span>` : `<span style="background: rgba(16,185,129,0.2); color:#a7f3d0; padding:2px 6px; border-radius:4px; font-weight:700;">P</span>`;
    
    let statusText = f.status;
    let statusColor = "var(--text-dim)";
    if (f.status === "SUCCESS") statusColor = "var(--success)";
    else if (f.status === "SKIPPED") statusColor = "var(--warning)";
    else if (f.status === "PENDING") statusColor = "var(--info)";

    // Metadata Badges (Memastikan Live Photo & GPS Terlihat Jelas)
    let metaBadges = `<span class="badge-meta">✓ Metadata Aman</span>`;
    if (f.safe_tags && f.safe_tags.length > 0) {
      metaBadges = f.safe_tags.map(t => `<span class="badge-meta">✓ ${t}</span>`).join(" ");
    }

    const origMB = (f.orig_size / (1024 * 1024)).toFixed(2);
    const compMB = (f.comp_size / (1024 * 1024)).toFixed(2);
    const ratio = f.ratio ? f.ratio.toFixed(1) + "%" : "0.0%";

    // Aturan Kategori Eksplisit Sesuai Permintaan User:
    // < 1MB = EFISIEN, 1-2MB = HEMAT, > 2MB = BETTER, Boros = BOROS
    let cat = f.category || "EFISIEN";
    let catClass = "badge-efisien";
    if (f.status === "SKIPPED" || cat === "BOROS") {
      cat = "BOROS (SKIP)";
      catClass = "badge-boros";
    } else {
      const cSizeMB = f.comp_size / (1024 * 1024);
      if (cSizeMB < 1.0) {
        cat = "EFISIEN";
        catClass = "badge-efisien";
      } else if (cSizeMB <= 2.0) {
        cat = "HEMAT";
        catClass = "badge-hemat";
      } else {
        cat = "BETTER";
        catClass = "badge-better";
      }
    }

    const timeStr = f.processed_time || "--";
    const filename = f.file_name || (f.file_path ? f.file_path.split("/").pop() : "media");

    return `
      <tr>
        <td style="font-family:'JetBrains Mono', monospace; font-weight:600; font-size:0.78rem;">${filename}</td>
        <td>${typeBadge}</td>
        <td style="color:${statusColor}; font-weight:700;">${statusText}</td>
        <td>${metaBadges}</td>
        <td>${origMB}</td>
        <td>${compMB}</td>
        <td style="font-weight:600; color:${f.saved_bytes > 0 ? 'var(--success)' : 'var(--text-dim)'};">${ratio}</td>
        <td><span class="badge-cat ${catClass}">${cat}</span></td>
        <td style="color:var(--text-dim); font-size:0.75rem;">${timeStr}</td>
      </tr>
    `;
  }).join("");
}

function filterTable() {
  const query = document.getElementById("tableFilter").value.toLowerCase();
  const rows = document.querySelectorAll("#filesTableBody tr");
  rows.forEach(r => {
    r.style.display = r.innerText.toLowerCase().includes(query) ? "" : "none";
  });
}

// Quick Actions (Gambar 4)
function setMode(mode) {
  authFetch("/api/control/mode", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode })
  }).then(() => {
    alert(`Mode mesin diubah ke: ${mode.toUpperCase()}`);
    refreshDashboard();
  });
}

function setControl(action) {
  authFetch(`/api/control/${action}`, { method: "POST" })
    .then(() => {
      alert(`Perintah: ${action.toUpperCase()} dikirim ke mesin!`);
      refreshDashboard();
    });
}

function triggerScan() {
  authFetch("/api/scan", { method: "POST" })
    .then(() => {
      alert("Pindai folder media Immich dimulai!");
      refreshDashboard();
    });
}

function loadRawLogs() {
  authFetch("/api/logs?name=supervisor")
    .then(r => r.text())
    .then(txt => {
      document.getElementById("rawLogBox").innerText = txt || "Belum ada catatan log terbaru.";
    })
    .catch(() => {});
}

// Pixel Art Indonesian News Widget Controller
function initNewsWidget() {
  const savedTheme = localStorage.getItem("kompic_news_theme") || "auto";
  changePixelTheme(savedTheme);
}

function changePixelTheme(themeKey) {
  localStorage.setItem("kompic_news_theme", themeKey);
  const icon = document.getElementById("pixelIcon");
  const tag = document.getElementById("pixelTag");
  const headline = document.getElementById("pixelHeadline");

  const themes = {
    demo: { icon: "🚩", tag: "AKSI BUNDARAN HI", text: "Massa masih berorasi mengawal aspirasi di Bundaran HI" },
    hujan: { icon: "🌧️", tag: "CUACA JAKARTA", text: "Hujan merata mengguyur sebagian wilayah Jabodetabek" },
    nasional: { icon: "🇮🇩", tag: "HARI NASIONAL", text: "Semangat persatuan dan kemandirian Indonesia" },
    ekonomi: { icon: "📈", tag: "BURSA & IHSG", text: "Indeks IHSG bergerak positif mengawali perdagangan" },
    santai: { icon: "☕", tag: "SUASANA SANTAI", text: "Kondisi server adem dan santai menikmati kopi" }
  };

  if (themeKey === "auto") {
    // Coba ambil tren berita Indonesia publik via API
    fetch("https://api-berita-indonesia.vercel.app/antara/terbaru")
      .then(r => r.json())
      .then(res => {
        if (res && res.data && res.data.posts && res.data.posts.length > 0) {
          const item = res.data.posts[0];
          tag.innerText = "BERITA INDONESIA TERKINI";
          headline.innerText = item.title;

          const titleLower = item.title.toLowerCase();
          if (titleLower.includes("demo") || titleLower.includes("aksi") || titleLower.includes("politik")) {
            icon.innerText = "🚩";
          } else if (titleLower.includes("hujan") || titleLower.includes("banjir") || titleLower.includes("cuaca")) {
            icon.innerText = "🌧️";
          } else if (titleLower.includes("ihsg") || titleLower.includes("ekonomi") || titleLower.includes("harga")) {
            icon.innerText = "📈";
          } else {
            icon.innerText = "📰";
          }
        }
      })
      .catch(() => {
        // Fallback default demo Bundaran HI sesuai situasi saat ini
        icon.innerText = "🚩";
        tag.innerText = "AKSI BUNDARAN HI";
        headline.innerText = "Massa mengawal aspirasi di kawasan Bundaran HI Jakarta";
      });
  } else if (themes[themeKey]) {
    icon.innerText = themes[themeKey].icon;
    tag.innerText = themes[themeKey].tag;
    headline.innerText = themes[themeKey].text;
  }
}

function loadSettings() {
  authFetch("/api/settings")
    .then(r => r.json())
    .then(cfg => {
      if (!cfg || !cfg.server) return;
      document.getElementById("cfgPort").value = cfg.server.port || 5555;
      document.getElementById("cfgImmichDir").value = cfg.storage.immich_upload_dir || "/mnt/hdd1/immich/upload/upload";
      document.getElementById("cfgMaxTemp").value = cfg.thermal.max_temp_c || 68;
      if (cfg.backup) {
        document.getElementById("cfgRcloneRemote").value = cfg.backup.rclone_remote || "mega:ImmichBackup";
      }
      const savedTheme = localStorage.getItem("kompic_news_theme") || "auto";
      document.getElementById("cfgNewsTheme").value = savedTheme;
    });
}

function saveSettings(e) {
  e.preventDefault();
  const theme = document.getElementById("cfgNewsTheme").value;
  changePixelTheme(theme);

  const payload = {
    server: {
      port: parseInt(document.getElementById("cfgPort").value),
      new_password: document.getElementById("cfgNewPassword").value || ""
    },
    storage: {
      immich_upload_dir: document.getElementById("cfgImmichDir").value
    },
    thermal: {
      max_temp_c: parseFloat(document.getElementById("cfgMaxTemp").value)
    },
    backup: {
      rclone_remote: document.getElementById("cfgRcloneRemote").value
    }
  };

  authFetch("/api/settings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  }).then(() => {
    alert("✅ Konfigurasi dan tema berita berhasil disimpan!");
    loadSettings();
  });
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}
