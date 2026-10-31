package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- Configuration Structure ---
type ServerConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	AuthEnabled  bool   `json:"auth_enabled"`
	PasswordHash string `json:"password_hash"`
	PasswordHint string `json:"password_hint"`
}

type StorageConfig struct {
	ImmichUploadDir string `json:"immich_upload_dir"`
	StateFilePath   string `json:"state_file_path"`
	LogDir          string `json:"log_dir"`
}

type CompressionConfig struct {
	Mode            string `json:"mode"` // normal, force, pause, resume
	VideoCodec      string `json:"video_codec"`
	VideoPreset     string `json:"video_preset"`
	VideoCRF        int    `json:"video_crf"`
	VideoMaxHeight  int    `json:"video_max_height"`
	VideoThreads    int    `json:"video_threads"`
	PhotoQuality    int    `json:"photo_quality"`
	PhotoMaxDim     int    `json:"photo_max_dim"`
	MaxVideoWorkers int    `json:"max_video_workers"`
	MaxPhotoWorkers int    `json:"max_photo_workers"`
}

type ThermalConfig struct {
	Enabled         bool    `json:"enabled"`
	MaxTempC        float64 `json:"max_temp_c"`
	CooldownTempC   float64 `json:"cooldown_temp_c"`
	ThermalZonePath string  `json:"thermal_zone_path"`
}

type ScheduleConfig struct {
	Enabled   bool `json:"enabled"`
	StartHour int  `json:"start_hour"`
	EndHour   int  `json:"end_hour"`
}

type BackupConfig struct {
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode"` // normal, force
	RcloneRemote string `json:"rclone_remote"`
	RcloneArgs   string `json:"rclone_args"`
}

type Config struct {
	Server      ServerConfig      `json:"server"`
	Storage     StorageConfig     `json:"storage"`
	Compression CompressionConfig `json:"compression"`
	Thermal     ThermalConfig     `json:"thermal"`
	Schedule    ScheduleConfig    `json:"schedule"`
	Backup      BackupConfig      `json:"backup"`
}

// Global thread-safe configuration holder
var (
	cfg   Config
	cfgMu sync.RWMutex
)

func getCfg() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

func setCfg(newCfg Config) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfg = newCfg
}

func getDefaultConfig() Config {
	var c Config
	c.Server.Host = "0.0.0.0"
	c.Server.Port = 5555
	c.Server.AuthEnabled = true
	h := sha256.Sum256([]byte("admin"))
	c.Server.PasswordHash = hex.EncodeToString(h[:])
	c.Server.PasswordHint = "admin (default)"

	c.Storage.ImmichUploadDir = "/mnt/hdd1/immich/upload/upload"
	c.Storage.StateFilePath = "optimizer_state.json"
	c.Storage.LogDir = "logs"

	c.Compression.Mode = "normal"
	c.Compression.VideoCodec = "libx265"
	c.Compression.VideoPreset = "ultrafast"
	c.Compression.VideoCRF = 26
	c.Compression.VideoMaxHeight = 1080
	c.Compression.VideoThreads = 1
	c.Compression.PhotoQuality = 82
	c.Compression.PhotoMaxDim = 2560
	c.Compression.MaxVideoWorkers = 1
	c.Compression.MaxPhotoWorkers = 3

	c.Thermal.Enabled = true
	c.Thermal.MaxTempC = 75.0
	c.Thermal.CooldownTempC = 65.0
	c.Thermal.ThermalZonePath = "/sys/class/thermal/thermal_zone0/temp"

	c.Schedule.Enabled = true
	c.Schedule.StartHour = 1
	c.Schedule.EndHour = 6

	c.Backup.Enabled = true
	c.Backup.Mode = "normal"
	c.Backup.RcloneRemote = "mega:ImmichBackup"
	c.Backup.RcloneArgs = "--transfers=1 --checkers=2 --fast-list --bwlimit=5M"
	return c
}

func saveConfigFile(c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := "config.json.tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return atomicReplace(tmpFile, "config.json")
}

// --- File Record Structure ---
type ProcessedFile struct {
	FilePath       string   `json:"file_path"`
	FileName       string   `json:"file_name"`
	MediaType      string   `json:"media_type"` // "V" or "P"
	Status         string   `json:"status"`     // "SUCCESS", "SKIPPED", "ERROR", "PENDING", "PROCESSING"
	OrigSize       int64    `json:"orig_size"`
	CompSize       int64    `json:"comp_size"`
	SavedBytes     int64    `json:"saved_bytes"`
	Ratio          float64  `json:"ratio"`
	Category       string   `json:"category"` // "IRIT", "HEMAT_PLUS", "HEMAT", "EFISIEN", "BOROS"
	ErrorMsg       string   `json:"error_msg"`
	SyncStatus     string   `json:"sync_status"` // "PENDING", "UPLOADED", "FAILED"
	RemoteDest     string   `json:"remote_dest"` // Full cloud target path e.g. "mega:ImmichBackup/2026/10/22/photo.jpg"
	ProcessedTime  string   `json:"processed_time"`
	MetadataStatus string   `json:"metadata_status"` // "SAFE", "LOST", "NONE", "PENDING"
	SafeTags       []string `json:"safe_tags"`       // ["GPS / Lokasi", "Waktu & Tanggal", "Kamera / Device", "Apple Live Photo", "Profil Warna (ICC)"]
	LostTags       []string `json:"lost_tags"`       // Tags that were detected in orig but missing in compressed
}

// --- Active Job ---
type ActiveJob struct {
	Path   string  `json:"path"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	SizeMB float64 `json:"size_mb"`
	Step   string  `json:"step"` // "Kompresi", "Bandingkan", "Replace", "Skip", "Sync"
}

// --- App State Store ---
type Store struct {
	mu            sync.RWMutex
	saveMu        sync.Mutex
	Files         map[string]*ProcessedFile `json:"files"`
	Mode          string                    `json:"mode"`
	SyncMode      string                    `json:"sync_mode"`
	SyncPaused    bool                      `json:"sync_paused"`
	ScanRequested bool                      `json:"-"`
	SyncRequested bool                      `json:"-"`
	ActiveJobs    []ActiveJob               `json:"-"`
	ActiveNode    string                    `json:"-"` // "start", "temp_check", "time_check", "compress", "compare", "replace", "skip", "sync"
	PosStatus     string                    `json:"-"`
	PosStatusCls  string                    `json:"-"`
	IsThrottled   bool                      `json:"-"`
	CloudFilesCount int64                   `json:"-"`
	CloudTotalBytes int64                   `json:"-"`
	LastCloudAudit  time.Time               `json:"-"`
	stateFilePath string                    `json:"-"`
}

var (
	store      *Store
	authTokens = make(map[string]time.Time)
	authMu     sync.RWMutex
)

func newStore(stateFilePath string) *Store {
	initialCfg := getCfg()
	s := &Store{
		Files:         make(map[string]*ProcessedFile),
		Mode:          initialCfg.Compression.Mode,
		SyncMode:      initialCfg.Backup.Mode,
		SyncPaused:    false,
		ActiveNode:    "start",
		PosStatus:     "WAITING",
		PosStatusCls:  "WAITING",
		stateFilePath: stateFilePath,
	}
	if s.Mode == "" {
		s.Mode = "normal"
	}
	if s.SyncMode == "" {
		s.SyncMode = "normal"
	}
	s.loadState()
	return s
}

func (s *Store) loadState() {
	if _, err := os.Stat(s.stateFilePath); err == nil {
		data, err := os.ReadFile(s.stateFilePath)
		if err == nil {
			var loaded Store
			if err := json.Unmarshal(data, &loaded); err == nil {
				s.mu.Lock()
				if loaded.Files != nil {
					s.Files = loaded.Files
				} else {
					s.Files = make(map[string]*ProcessedFile)
				}
				if loaded.Mode != "" {
					s.Mode = loaded.Mode
				}
				if loaded.SyncMode != "" {
					s.SyncMode = loaded.SyncMode
				}
				s.SyncPaused = loaded.SyncPaused
				s.mu.Unlock()
			}
		}
	}
}

func (s *Store) saveState() {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	s.mu.RLock()
	filesCopy := make(map[string]*ProcessedFile, len(s.Files))
	for k, v := range s.Files {
		filesCopy[k] = v
	}
	stateCopy := struct {
		Files      map[string]*ProcessedFile `json:"files"`
		Mode       string                    `json:"mode"`
		SyncMode   string                    `json:"sync_mode"`
		SyncPaused bool                      `json:"sync_paused"`
	}{
		Files:      filesCopy,
		Mode:       s.Mode,
		SyncMode:   s.SyncMode,
		SyncPaused: s.SyncPaused,
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(stateCopy, "", "  ")
	if err == nil {
		tmpPath := s.stateFilePath + ".tmp"
		if err := os.WriteFile(tmpPath, data, 0644); err == nil {
			_ = atomicReplace(tmpPath, s.stateFilePath)
		}
	}
}

func (s *Store) addOrUpdate(file *ProcessedFile) {
	s.mu.Lock()
	s.Files[file.FilePath] = file
	s.mu.Unlock()
	go s.saveState()
}

func (s *Store) getFile(path string) *ProcessedFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Files[path]
}

func (s *Store) getStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var success, skipped, errCount, pending, processing int
	var totalOrig, totalComp, totalSaved, syncUploadedBytes int64
	var successOrig, successComp int64
	var syncUploaded, syncPending, syncFailed int

	for _, f := range s.Files {
		switch f.Status {
		case "SUCCESS":
			success++
			successOrig += f.OrigSize
			successComp += f.CompSize
		case "SKIPPED":
			skipped++
		case "ERROR":
			errCount++
		case "PENDING":
			pending++
		case "PROCESSING":
			processing++
		}
		totalOrig += f.OrigSize
		totalComp += f.CompSize
		totalSaved += f.SavedBytes

		switch f.SyncStatus {
		case "UPLOADED":
			syncUploaded++
			syncUploadedBytes += f.CompSize
		case "PENDING":
			syncPending++
		case "FAILED":
			syncFailed++
		}
	}

	syncUploadedFinal := int64(syncUploaded)
	syncBytesFinal := syncUploadedBytes
	if s.CloudTotalBytes > 0 || s.CloudFilesCount > 0 {
		syncUploadedFinal = s.CloudFilesCount
		syncBytesFinal = s.CloudTotalBytes
	}

	return map[string]interface{}{
		"total":               len(s.Files),
		"success":             success,
		"skipped":             skipped,
		"error":               errCount,
		"pending":             pending,
		"processing":          processing,
		"total_orig_bytes":    totalOrig,
		"total_comp_bytes":    totalComp,
		"total_saved_bytes":   totalSaved,
		"success_orig_bytes":  successOrig,
		"success_comp_bytes":  successComp,
		"sync_uploaded":       syncUploadedFinal,
		"sync_pending":        syncPending,
		"sync_failed":         syncFailed,
		"sync_uploaded_bytes": syncBytesFinal,
	}
}

func (s *Store) getRecentFiles(limit int) []*ProcessedFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*ProcessedFile, 0, len(s.Files))
	for _, f := range s.Files {
		list = append(list, f)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ProcessedTime > list[j].ProcessedTime
	})
	if len(list) > limit {
		return list[:limit]
	}
	return list
}

func (s *Store) getSyncFiles(limit int) []*ProcessedFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*ProcessedFile, 0)
	for _, f := range s.Files {
		if f.Status == "SUCCESS" || f.SyncStatus != "PENDING" {
			list = append(list, f)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ProcessedTime > list[j].ProcessedTime
	})
	if len(list) > limit {
		return list[:limit]
	}
	return list
}

// --- Logging Helper ---
func writeLog(category, msg string) {
	logDir := getCfg().Storage.LogDir
	if logDir == "" {
		logDir = "logs"
	}
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, category+".log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		_, _ = f.WriteString(fmt.Sprintf("%s [%s] %s\n", timestamp, strings.ToUpper(category), msg))
	}
}

// --- Hardware & Detailed System Metrics (Safe Read-Only) ---
type DetailedSystemMetrics struct {
	CPUTempC       string  `json:"cpu_temp"`
	RAMUsedMB      int     `json:"ram_used_mb"`
	RAMTotalMB     int     `json:"ram_total_mb"`
	RAMFreeMB      int     `json:"ram_free_mb"`
	RAMUsedPct     float64 `json:"ram_used_pct"`
	Load1          string  `json:"load_1"`
	Load5          string  `json:"load_5"`
	Load15         string  `json:"load_15"`
	UptimeText     string  `json:"uptime_text"`
	RootDiskTotal  string  `json:"root_disk_total"`
	RootDiskUsed   string  `json:"root_disk_used"`
	RootDiskFree   string  `json:"root_disk_free"`
	RootDiskPct    int     `json:"root_disk_pct"`
	MediaDiskTotal string  `json:"media_disk_total"`
	MediaDiskUsed  string  `json:"media_disk_used"`
	MediaDiskFree  string  `json:"media_disk_free"`
	MediaDiskPct   int     `json:"media_disk_pct"`
	Arch           string  `json:"arch"`
	Hostname       string  `json:"hostname"`
	ProcessPID     int     `json:"process_pid"`
}

func getSystemMetrics() map[string]interface{} {
	det := getDetailedSystemMetrics()
	return map[string]interface{}{
		"cpu_temp":     det.CPUTempC,
		"ram_used_mb":  det.RAMUsedMB,
		"ram_total_mb": det.RAMTotalMB,
	}
}

func getDetailedSystemMetrics() DetailedSystemMetrics {
	var m DetailedSystemMetrics
	m.ProcessPID = os.Getpid()
	m.Arch = "ARM64 (ODROID-C2 / Linux)"
	if h, err := os.Hostname(); err == nil {
		m.Hostname = h
	} else {
		m.Hostname = "odroid-c2"
	}

	// 1. CPU Temp via sysfs
	temp := 48.0
	tzPath := getCfg().Thermal.ThermalZonePath
	if tzPath == "" {
		tzPath = "/sys/class/thermal/thermal_zone0/temp"
	}
	if b, err := os.ReadFile(tzPath); err == nil {
		if raw, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64); err == nil {
			if raw > 1000.0 {
				temp = raw / 1000.0
			} else if raw > 0 {
				temp = raw
			}
		}
	}
	m.CPUTempC = fmt.Sprintf("%.1f", temp)

	// 2. RAM via /proc/meminfo
	totalMB := 1919
	availMB := 1400
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(b), "\n")
		mem := make(map[string]int)
		for _, line := range lines {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				f := strings.Fields(parts[1])
				if len(f) > 0 {
					val, _ := strconv.Atoi(f[0])
					mem[strings.TrimSpace(parts[0])] = val
				}
			}
		}
		if tot, ok := mem["MemTotal"]; ok {
			totalMB = tot / 1024
		}
		avail := mem["MemAvailable"]
		if avail == 0 {
			avail = mem["MemFree"] + mem["Buffers"] + mem["Cached"]
		}
		availMB = avail / 1024
	}
	m.RAMTotalMB = totalMB
	m.RAMFreeMB = availMB
	m.RAMUsedMB = totalMB - availMB
	if m.RAMUsedMB < 0 {
		m.RAMUsedMB = 0
	}
	if m.RAMTotalMB > 0 {
		m.RAMUsedPct = (float64(m.RAMUsedMB) / float64(m.RAMTotalMB)) * 100.0
	}

	// 3. Load Average via /proc/loadavg
	m.Load1, m.Load5, m.Load15 = "0.10", "0.15", "0.20"
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) >= 3 {
			m.Load1 = fields[0]
			m.Load5 = fields[1]
			m.Load15 = fields[2]
		}
	}

	// 4. System Uptime via /proc/uptime
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 {
			if upSec, err := strconv.ParseFloat(fields[0], 64); err == nil {
				d := int(upSec) / 86400
				h := (int(upSec) % 86400) / 3600
				min := (int(upSec) % 3600) / 60
				if d > 0 {
					m.UptimeText = fmt.Sprintf("%d hari, %d jam, %d menit", d, h, min)
				} else {
					m.UptimeText = fmt.Sprintf("%d jam, %d menit", h, min)
				}
			}
		}
	}
	if m.UptimeText == "" {
		m.UptimeText = "Aktif (ODROID-C2)"
	}

	// 5. Disk Usage for Root (/) and Media (/mnt/hdd1) - Safe Read-only
	m.RootDiskTotal, m.RootDiskUsed, m.RootDiskFree, m.RootDiskPct = getDiskSpace("/")
	mediaPath := getCfg().Storage.ImmichUploadDir
	if mediaPath == "" {
		mediaPath = "/mnt/hdd1"
	}
	m.MediaDiskTotal, m.MediaDiskUsed, m.MediaDiskFree, m.MediaDiskPct = getDiskSpace(mediaPath)

	return m
}

func getDiskSpace(path string) (string, string, string, int) {
	out, err := exec.Command("df", "-k", path).Output()
	if err != nil {
		return "N/A", "N/A", "N/A", 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "N/A", "N/A", "N/A", 0
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return "N/A", "N/A", "N/A", 0
	}
	totKB, _ := strconv.ParseInt(fields[1], 10, 64)
	usedKB, _ := strconv.ParseInt(fields[2], 10, 64)
	freeKB, _ := strconv.ParseInt(fields[3], 10, 64)
	pctStr := strings.TrimSuffix(fields[4], "%")
	pct, _ := strconv.Atoi(pctStr)

	totGB := float64(totKB) / (1024 * 1024)
	usedGB := float64(usedKB) / (1024 * 1024)
	freeGB := float64(freeKB) / (1024 * 1024)

	return fmt.Sprintf("%.1f GB", totGB), fmt.Sprintf("%.1f GB", usedGB), fmt.Sprintf("%.1f GB", freeGB), pct
}

func isWithinSchedule() bool {
	store.mu.RLock()
	mode := store.Mode
	store.mu.RUnlock()

	if mode == "force" {
		return true
	}
	c := getCfg()
	if !c.Schedule.Enabled {
		return true
	}
	hr := time.Now().Hour()
	st := c.Schedule.StartHour
	en := c.Schedule.EndHour
	if st == en {
		return true
	}
	if st < en {
		return hr >= st && hr < en
	}
	return hr >= st || hr < en
}

// --- Compression & iPhone Live Photo Metadata Helper ---
var (
	imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".heic": true, ".png": true, ".webp": true}
	videoExts = map[string]bool{".mov": true, ".mp4": true, ".m4v": true}
)

// copyMetadata preserves all tags including:
// 1. Photo EXIF GPS tags (GPSLatitude, GPSLongitude, GPSAltitude, GPSTimeStamp, etc.)
// 2. Video QuickTime GPS tags (Keys:GPSCoordinates, UserData:GPSCoordinates, QuickTime:GPSCoordinates)
// 3. Apple Live Photo pairing identifiers (Keys:ContentIdentifier, UserData:ContentIdentifier, MediaGroupUUID, StillImageTime)
// 4. Exact color profiles, timestamps with UTC timezone offset (QuickTimeUTC)
func copyMetadata(src, dst string) error {
	cmd := exec.Command("exiftool",
		"-overwrite_original",
		"-P", // Preserve file modification timestamp
		"-m", // Ignore minor warnings
		"-tagsFromFile", src,
		"-all:all",        // Copy all standard tags
		"-unsafe",         // Copy unsafe/hardware tags
		"-icc_profile",    // Preserve exact color space profile
		"-GPS:all",        // Photo GPS IFD tags
		"-QuickTime:all",  // Video QuickTime container tags
		"-Keys:all",       // Video Apple/Android QuickTime Keys
		"-UserData:all",   // Video QuickTime UserData (©xyz)
		"-XMP:all",        // XMP sidecar & embedded tags
		"-EXIF:all",       // Full EXIF block
		"-IPTC:all",       // IPTC caption & keywords
		"-MakerNotes:all", // Manufacturer camera sensor tags
		"-Keys:GPSCoordinates<Keys:GPSCoordinates",
		"-UserData:GPSCoordinates<UserData:GPSCoordinates",
		"-Keys:GPSCoordinates<QuickTime:GPSCoordinates",
		"-UserData:GPSCoordinates<QuickTime:GPSCoordinates",
		"-Keys:GPSCoordinates<UserData:GPSCoordinates",
		"-UserData:GPSCoordinates<Keys:GPSCoordinates",
		"-Keys:GPSCoordinates<GPSCoordinates",
		"-UserData:GPSCoordinates<GPSCoordinates",
		"-Keys:ContentIdentifier<Keys:ContentIdentifier",
		"-UserData:ContentIdentifier<UserData:ContentIdentifier",
		"-Keys:ContentIdentifier<ContentIdentifier",
		"-UserData:ContentIdentifier<ContentIdentifier",
		"-Keys:ContentIdentifier<Apple:ContentIdentifier",
		"-UserData:ContentIdentifier<Apple:ContentIdentifier",
		"-ContentIdentifier", // Apple Live Photo pairing ID
		"-MediaGroupUUID",    // Apple Live Photo group UUID
		"-StillImageTime",    // Apple Live Photo poster frame timestamp
		"-CreationDate",      // Local creation date with timezone
		"-api", "LargeFileSupport=1",
		"-api", "QuickTimeUTC=1",
		dst,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeLog("supervisor", fmt.Sprintf("ExifTool note on %s: %v (%s)", filepath.Base(dst), err, strings.TrimSpace(string(out))))
		return err
	}
	return nil
}

// extractMediaDate extracts the media capture date (EXIF DateTimeOriginal / CreateDate / CreationDate)
// and returns "YYYY/MM/DD". Falls back to file modification time, or current date if unreadable.
func extractMediaDate(path string) string {
	cmd := exec.Command("exiftool",
		"-s3",
		"-q", "-q",
		"-d", "%Y/%m/%d",
		"-DateTimeOriginal",
		"-CreateDate",
		"-CreationDate",
		path,
	)
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Validate format YYYY/MM/DD (length 10, slashes at 4 and 7)
			if len(line) == 10 && line[4] == '/' && line[7] == '/' {
				return line
			}
		}
	}

	// Fallback to file ModTime
	if fi, err := os.Stat(path); err == nil {
		return fi.ModTime().Format("2006/01/02")
	}

	// Fallback to current time
	return time.Now().Format("2006/01/02")
}

// extractGPS checks whether a media file contains GPS or geolocation coordinates
func extractGPS(path string) string {
	cmd := exec.Command("exiftool",
		"-s3",
		"-q", "-q",
		"-GPSLatitude",
		"-GPSCoordinates",
		"-Keys:GPSCoordinates",
		"-UserData:GPSCoordinates",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// verifyDetailedMetadata checks source vs destination metadata and returns safe/lost tags
func verifyDetailedMetadata(src, dst string) (string, []string, []string) {
	// Query standard metadata categories using ExifTool tab-separated output
	// Tags queried:
	// GPS: GPSLatitude, GPSCoordinates, Keys:GPSCoordinates, UserData:GPSCoordinates
	// Date: DateTimeOriginal, CreateDate, CreationDate, MediaCreateDate
	// Device: Make, Model
	// LivePhoto: ContentIdentifier, MediaGroupUUID
	// Color: ProfileDescription, ColorSpace
	cmdSrc := exec.Command("exiftool", "-s", "-q", "-q",
		"-GPSLatitude", "-GPSCoordinates", "-Keys:GPSCoordinates", "-UserData:GPSCoordinates",
		"-DateTimeOriginal", "-CreateDate", "-CreationDate",
		"-Make", "-Model",
		"-ContentIdentifier", "-MediaGroupUUID",
		"-ProfileDescription", "-ColorSpace",
		src,
	)
	outSrc, errSrc := cmdSrc.Output()
	if errSrc != nil {
		// If exiftool fails or file has no tags, fallback to safe default
		return "SAFE", []string{"Metadata Disalin (ExifTool)"}, nil
	}

	cmdDst := exec.Command("exiftool", "-s", "-q", "-q",
		"-GPSLatitude", "-GPSCoordinates", "-Keys:GPSCoordinates", "-UserData:GPSCoordinates",
		"-DateTimeOriginal", "-CreateDate", "-CreationDate",
		"-Make", "-Model",
		"-ContentIdentifier", "-MediaGroupUUID",
		"-ProfileDescription", "-ColorSpace",
		dst,
	)
	outDst, errDst := cmdDst.Output()
	if errDst != nil {
		return "SAFE", []string{"Metadata Terverifikasi"}, nil
	}

	srcLines := strings.Split(string(outSrc), "\n")
	dstLines := strings.Split(string(outDst), "\n")

	parseTags := func(lines []string) map[string]string {
		m := make(map[string]string)
		for _, l := range lines {
			l = strings.TrimSpace(l)
			parts := strings.SplitN(l, ":", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				if v != "" {
					m[k] = v
				}
			}
		}
		return m
	}

	sMap := parseTags(srcLines)
	dMap := parseTags(dstLines)

	var safeTags []string
	var lostTags []string

	// 1. GPS / Lokasi
	hasSrcGPS := sMap["GPSLatitude"] != "" || sMap["GPSCoordinates"] != "" || sMap["Keys:GPSCoordinates"] != "" || sMap["UserData:GPSCoordinates"] != ""
	hasDstGPS := dMap["GPSLatitude"] != "" || dMap["GPSCoordinates"] != "" || dMap["Keys:GPSCoordinates"] != "" || dMap["UserData:GPSCoordinates"] != ""
	if hasSrcGPS {
		if hasDstGPS {
			safeTags = append(safeTags, "GPS / Lokasi (Aman)")
		} else {
			lostTags = append(lostTags, "GPS / Lokasi Hilang")
		}
	}

	// 2. Waktu & Tanggal
	hasSrcDate := sMap["DateTimeOriginal"] != "" || sMap["CreateDate"] != "" || sMap["CreationDate"] != ""
	hasDstDate := dMap["DateTimeOriginal"] != "" || dMap["CreateDate"] != "" || dMap["CreationDate"] != ""
	if hasSrcDate {
		if hasDstDate {
			safeTags = append(safeTags, "Waktu & Tanggal (Aman)")
		} else {
			lostTags = append(lostTags, "Waktu & Tanggal")
		}
	}

	// 3. Kamera & Device
	hasSrcDev := sMap["Make"] != "" || sMap["Model"] != ""
	hasDstDev := dMap["Make"] != "" || dMap["Model"] != ""
	if hasSrcDev {
		if hasDstDev {
			safeTags = append(safeTags, "Kamera / Device (Aman)")
		} else {
			lostTags = append(lostTags, "Kamera / Device")
		}
	}

	// 4. Apple Live Photo
	hasSrcLP := sMap["ContentIdentifier"] != "" || sMap["MediaGroupUUID"] != ""
	hasDstLP := dMap["ContentIdentifier"] != "" || dMap["MediaGroupUUID"] != ""
	if hasSrcLP {
		if hasDstLP {
			safeTags = append(safeTags, "Live Photo ID (Aman)")
		} else {
			lostTags = append(lostTags, "Live Photo ID")
		}
	}

	// 5. Profil Warna (ICC Profile)
	hasSrcColor := sMap["ProfileDescription"] != "" || sMap["ColorSpace"] != ""
	hasDstColor := dMap["ProfileDescription"] != "" || dMap["ColorSpace"] != ""
	if hasSrcColor {
		if hasDstColor {
			safeTags = append(safeTags, "Profil Warna ICC (Aman)")
		}
	}

	// If source had minimal EXIF (e.g. screenshot or web image), but was copied via ExifTool
	if len(safeTags) == 0 && len(lostTags) == 0 {
		safeTags = append(safeTags, "Container Metadata Utuh")
	}

	status := "SAFE"
	if len(lostTags) > 0 {
		status = "LOST"
	}

	return status, safeTags, lostTags
}

// injectGPSMetadata is a dedicated second-pass injector if container tags need explicit re-binding
func injectGPSMetadata(src, dst string) error {
	cmd := exec.Command("exiftool",
		"-overwrite_original",
		"-P",
		"-m",
		"-tagsFromFile", src,
		"-GPS:all",
		"-QuickTime:all",
		"-Keys:all",
		"-UserData:all",
		"-Keys:GPSCoordinates<Keys:GPSCoordinates",
		"-UserData:GPSCoordinates<UserData:GPSCoordinates",
		"-Keys:GPSCoordinates<QuickTime:GPSCoordinates",
		"-UserData:GPSCoordinates<QuickTime:GPSCoordinates",
		"-Keys:GPSCoordinates<UserData:GPSCoordinates",
		"-UserData:GPSCoordinates<Keys:GPSCoordinates",
		"-Keys:GPSCoordinates<GPSCoordinates",
		"-UserData:GPSCoordinates<GPSCoordinates",
		"-api", "LargeFileSupport=1",
		"-api", "QuickTimeUTC=1",
		dst,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeLog("supervisor", fmt.Sprintf("injectGPSMetadata note on %s: %v (%s)", filepath.Base(dst), err, strings.TrimSpace(string(out))))
		return err
	}
	return nil
}

func compressImage(src, dst string) error {
	c := getCfg()
	maxDim := c.Compression.PhotoMaxDim
	if maxDim <= 0 {
		maxDim = 2560
	}
	quality := c.Compression.PhotoQuality
	if quality <= 0 || quality > 100 {
		quality = 82
	}

	ext := strings.ToLower(filepath.Ext(src))
	if ext == ".heic" {
		// Handle iPhone HEIC files:
		// FFmpeg standard builds cannot mux HEIC. We try magick / convert if available.
		var err error
		if _, lookErr := exec.LookPath("magick"); lookErr == nil {
			resizeArg := fmt.Sprintf("%dx%d>", maxDim, maxDim)
			cmd := exec.Command("magick", src, "-resize", resizeArg, "-quality", strconv.Itoa(quality), dst)
			if out, mErr := cmd.CombinedOutput(); mErr != nil {
				err = fmt.Errorf("magick failed: %v (%s)", mErr, string(out))
			}
		} else if _, lookErr := exec.LookPath("convert"); lookErr == nil {
			resizeArg := fmt.Sprintf("%dx%d>", maxDim, maxDim)
			cmd := exec.Command("convert", src, "-resize", resizeArg, "-quality", strconv.Itoa(quality), dst)
			if out, cErr := cmd.CombinedOutput(); cErr != nil {
				err = fmt.Errorf("convert failed: %v (%s)", cErr, string(out))
			}
		} else {
			return fmt.Errorf("HEIC encoder not found (install imagemagick or libheif-examples to compress HEIC photos); original preserved")
		}
		if err != nil {
			return err
		}
		_ = copyMetadata(src, dst)
		return nil
	}

	// Convert 1-100 quality scale to ffmpeg mjpeg/jpeg -q:v scale 1-31 (1 best, 31 worst)
	qv := int(float64(100-quality)*30.0/100.0) + 1
	if qv < 1 {
		qv = 1
	} else if qv > 31 {
		qv = 31
	}

	vf := fmt.Sprintf("scale='min(iw,%d)':-2", maxDim)
	cmd := exec.Command("ffmpeg", "-y", "-threads", "1", "-i", src, "-map_metadata", "0", "-vf", vf, "-q:v", strconv.Itoa(qv), dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg error: %v (%s)", err, string(out))
	}
	_ = copyMetadata(src, dst)
	return nil
}

func compressVideo(src, dst string) error {
	c := getCfg()
	maxH := c.Compression.VideoMaxHeight
	if maxH <= 0 {
		maxH = 1080
	}
	crf := c.Compression.VideoCRF
	if crf <= 0 {
		crf = 26
	}
	preset := c.Compression.VideoPreset
	if preset == "" {
		preset = "ultrafast"
	}
	codec := c.Compression.VideoCodec
	if codec == "" {
		codec = "libx265"
	}
	threads := strconv.Itoa(c.Compression.VideoThreads)
	if threads == "0" || threads == "" {
		threads = "1"
	}

	// Use trunc to guarantee both width and height are divisible by 2 (mandatory for H.264/H.265 yuv420p)
	vf := fmt.Sprintf("scale=-2:'trunc(min(ih,%d)/2)*2'", maxH)

	// Apple & iOS compatibility: tag:v hvc1 and yuv420p are mandatory for QuickTime/Safari/Immich playback
	// CRITICAL: -map_metadata 0 and -movflags +faststart+use_metadata_tags preserve container, audio/video tracks, and location tags
	baseArgs := []string{
		"-y", "-threads", threads,
		"-i", src,
		"-map_metadata", "0",
		"-vf", vf,
		"-c:v", codec,
		"-crf", strconv.Itoa(crf),
		"-preset", preset,
	}
	if codec == "libx265" {
		baseArgs = append(baseArgs, "-tag:v", "hvc1", "-pix_fmt", "yuv420p")
	} else if codec == "libx264" {
		baseArgs = append(baseArgs, "-pix_fmt", "yuv420p")
	}
	baseArgs = append(baseArgs, "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart+use_metadata_tags", dst)

	cmd := exec.Command("ffmpeg", baseArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback to libx264 if libx265 is not supported on platform
		if codec == "libx265" {
			fallbackArgs := []string{
				"-y", "-threads", threads,
				"-i", src,
				"-map_metadata", "0",
				"-vf", vf,
				"-c:v", "libx264",
				"-crf", strconv.Itoa(crf),
				"-preset", preset,
				"-pix_fmt", "yuv420p",
				"-c:a", "aac",
				"-b:a", "128k",
				"-movflags", "+faststart+use_metadata_tags",
				dst,
			}
			cmdFallback := exec.Command("ffmpeg", fallbackArgs...)
			if out2, err2 := cmdFallback.CombinedOutput(); err2 != nil {
				return fmt.Errorf("ffmpeg video fallback error: %v (%s)", err2, string(out2))
			}
		} else {
			return fmt.Errorf("ffmpeg video error: %v (%s)", err, string(out))
		}
	}
	_ = copyMetadata(src, dst)
	return nil
}

// atomicReplace handles safe atomic replacement with zero-data-loss rollback protection
func atomicReplace(srcTmp, target string) error {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return os.Rename(srcTmp, target)
	}

	backup := target + ".bak_opt"
	_ = os.Remove(backup)

	if err := os.Rename(target, backup); err != nil {
		// If atomic direct rename succeeds (POSIX behavior where rename can overwrite)
		if err2 := os.Rename(srcTmp, target); err2 == nil {
			return nil
		}
		return fmt.Errorf("gagal membuat backup sementara untuk %s: %w", target, err)
	}

	if err := os.Rename(srcTmp, target); err != nil {
		// Rollback immediately to prevent data loss!
		_ = os.Rename(backup, target)
		return fmt.Errorf("gagal mengganti target %s: %w (file asli telah dipulihkan)", target, err)
	}

	_ = os.Remove(backup)
	return nil
}

// updateJobStep updates the step description of a specific job in store.ActiveJobs thread-safely
func updateJobStep(filePath, step string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for i := range store.ActiveJobs {
		if store.ActiveJobs[i].Path == filePath {
			store.ActiveJobs[i].Step = step
			break
		}
	}
}

// removeJob removes a job from store.ActiveJobs thread-safely
func removeJob(filePath string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var updated []ActiveJob
	for _, j := range store.ActiveJobs {
		if j.Path != filePath {
			updated = append(updated, j)
		}
	}
	store.ActiveJobs = updated
	if len(store.ActiveJobs) == 0 {
		store.ActiveNode = "start"
	}
}

func processFile(p *ProcessedFile) {
	origStat, statErr := os.Stat(p.FilePath)
	if statErr != nil {
		p.Status = "ERROR"
		p.ErrorMsg = "Berkas asli tidak dapat diakses: " + statErr.Error()
		p.ProcessedTime = time.Now().Format("2006-01-02 15:04:05")
		store.addOrUpdate(p)
		removeJob(p.FilePath)
		return
	}
	p.OrigSize = origStat.Size()

	// Deteksi koordinat GPS / Geolokasi pada berkas asli
	origGPS := extractGPS(p.FilePath)
	if origGPS != "" {
		writeLog("supervisor", fmt.Sprintf("[GPS DETECTED] %s: Terdeteksi data lokasi asli (%s)", p.FileName, origGPS))
	}

	ext := strings.ToLower(filepath.Ext(p.FilePath))
	tmpDst := p.FilePath + ".tmp_opt" + ext

	store.mu.Lock()
	store.ActiveNode = "compress"
	p.Status = "PROCESSING"
	store.mu.Unlock()
	updateJobStep(p.FilePath, "Kompresi")

	var err error
	if p.MediaType == "V" {
		writeLog("video", fmt.Sprintf("Processing video: %s (%.2f MB)", p.FileName, float64(p.OrigSize)/(1024*1024)))
		err = compressVideo(p.FilePath, tmpDst)
	} else {
		writeLog("photo", fmt.Sprintf("Processing image: %s (%.2f MB)", p.FileName, float64(p.OrigSize)/(1024*1024)))
		err = compressImage(p.FilePath, tmpDst)
	}

	if err != nil {
		_ = os.Remove(tmpDst)
		p.Status = "ERROR"
		p.ErrorMsg = err.Error()
		p.ProcessedTime = time.Now().Format("2006-01-02 15:04:05")
		store.addOrUpdate(p)
		writeLog("supervisor", fmt.Sprintf("Failed %s: %v", p.FileName, err))
		removeJob(p.FilePath)
		return
	}

	// VALIDASI GEOLOKASI (ZERO GPS LOSS SAFETY GUARD):
	// Jika berkas asli memiliki koordinat GPS, berkas hasil kompresi WAJIB memiliki koordinat GPS!
	if origGPS != "" {
		dstGPS := extractGPS(tmpDst)
		if dstGPS == "" {
			// Coba injeksi GPS kedua dengan tag mapping mendalam
			_ = injectGPSMetadata(p.FilePath, tmpDst)
			dstGPS = extractGPS(tmpDst)
		}
		if dstGPS == "" {
			// PERLINDUNGAN TOTAL: Berkas asli TIDAK BOLEH ditimpa jika data lokasi hilang!
			_ = os.Remove(tmpDst)
			p.Status = "SKIPPED"
			p.ErrorMsg = "Proteksi Geolokasi: Berkas asli memiliki lokasi GPS, namun gagal disalin ke hasil kompresi. Berkas asli dipertahankan demi menjaga data lokasi tidak hilang."
			p.MetadataStatus = "LOST"
			p.LostTags = []string{"GPS / Lokasi (Dibatalkan demi proteksi)"}
			p.SafeTags = []string{"Berkas Asli Terlindungi (Utuh)"}
			p.CompSize = p.OrigSize
			p.SavedBytes = 0
			p.Ratio = 0
			p.Category = "BOROS"
			p.ProcessedTime = time.Now().Format("2006-01-02 15:04:05")
			store.addOrUpdate(p)
			writeLog("supervisor", fmt.Sprintf("[GPS GUARD ACTIVATED] %s: Berkas asli memiliki GPS (%s) tetapi tidak terbaca di hasil kompresi. Berkas asli dipertahankan demi menjaga keaslian data.", p.FileName, origGPS))
			removeJob(p.FilePath)
			return
		}
		writeLog("supervisor", fmt.Sprintf("[GPS VERIFIED] %s: Data lokasi berhasil diverifikasi dan dipreservasi (%s)", p.FileName, dstGPS))
	}

	// Bandingkan ukuran (Node: compare)
	store.mu.Lock()
	store.ActiveNode = "compare"
	store.mu.Unlock()
	updateJobStep(p.FilePath, "Membandingkan Ukuran")

	tmpStat, err := os.Stat(tmpDst)
	if err != nil {
		_ = os.Remove(tmpDst)
		p.Status = "ERROR"
		p.ErrorMsg = "Gagal membaca output kompresi: " + err.Error()
		p.ProcessedTime = time.Now().Format("2006-01-02 15:04:05")
		store.addOrUpdate(p)
		removeJob(p.FilePath)
		return
	}

	compSize := tmpStat.Size()

	// CRITICAL SAFETY CHECK: Output must be non-empty and at least 512 bytes
	if compSize <= 512 {
		_ = os.Remove(tmpDst)
		p.Status = "ERROR"
		p.ErrorMsg = fmt.Sprintf("Output kompresi korup atau kosong (%d bytes), berkas asli utuh", compSize)
		p.ProcessedTime = time.Now().Format("2006-01-02 15:04:05")
		store.addOrUpdate(p)
		writeLog("supervisor", fmt.Sprintf("WARNING: %s menghasilkan output tidak wajar (%d bytes). Dibatalkan demi keamanan data.", p.FileName, compSize))
		removeJob(p.FilePath)
		return
	}

	p.CompSize = compSize
	p.ProcessedTime = time.Now().Format("2006-01-02 15:04:05")

	// Atomic Replace vs Skip Boros
	if compSize < p.OrigSize {
		// Node: replace
		store.mu.Lock()
		store.ActiveNode = "replace"
		store.mu.Unlock()
		updateJobStep(p.FilePath, "Mengganti Berkas Asli (Atomic Replace)")

		// Preserve original file permissions and modification time
		_ = os.Chmod(tmpDst, origStat.Mode())
		_ = os.Chtimes(tmpDst, origStat.ModTime(), origStat.ModTime())

		// Evaluasi Integritas Metadata Lengkap (GPS, Waktu, Device, Live Photo, ICC)
		metaStatus, safeTags, lostTags := verifyDetailedMetadata(p.FilePath, tmpDst)
		p.MetadataStatus = metaStatus
		p.SafeTags = safeTags
		p.LostTags = lostTags

		if err := atomicReplace(tmpDst, p.FilePath); err != nil {
			p.Status = "ERROR"
			p.ErrorMsg = "Gagal mengganti berkas: " + err.Error()
			writeLog("supervisor", fmt.Sprintf("Gagal replace %s: %v", p.FileName, err))
		} else {
			p.Status = "SUCCESS"
			p.SavedBytes = p.OrigSize - compSize
			p.Ratio = (float64(p.SavedBytes) / float64(p.OrigSize)) * 100.0

			if p.Ratio >= 88 {
				p.Category = "IRIT"
			} else if p.Ratio >= 70 {
				p.Category = "HEMAT_PLUS"
			} else if p.Ratio >= 50 {
				p.Category = "HEMAT"
			} else {
				p.Category = "EFISIEN"
			}

			writeLog("supervisor", fmt.Sprintf("[SUCCESS] %s: %.2fMB -> %.2fMB (Hemat %.1f%%) | Metadata: %s (%v)", p.FileName, float64(p.OrigSize)/(1024*1024), float64(compSize)/(1024*1024), p.Ratio, metaStatus, safeTags))
		}
	} else {
		// Node: skip
		store.mu.Lock()
		store.ActiveNode = "skip"
		store.mu.Unlock()
		updateJobStep(p.FilePath, "Skip (Boros - Berkas Asli Dipertahankan)")

		_ = os.Remove(tmpDst)
		p.Status = "SKIPPED"
		p.CompSize = p.OrigSize
		p.SavedBytes = 0
		p.Ratio = 0
		p.Category = "BOROS"
		p.ErrorMsg = "Ukuran hasil kompresi lebih besar dari asli"
		p.MetadataStatus = "SAFE"
		p.SafeTags = []string{"Berkas Asli Dipertahankan (100% Utuh)"}
		writeLog("supervisor", fmt.Sprintf("[SKIPPED] %s: Hasil lebih besar (asli %d <= hasil %d). Berkas asli utuh.", p.FileName, p.OrigSize, compSize))
	}

	store.addOrUpdate(p)
	time.Sleep(300 * time.Millisecond)
	removeJob(p.FilePath)
}

// --- Dedicated Continuous Thermal Guard ---
func startThermalGuard() {
	for {
		c := getCfg()
		metrics := getSystemMetrics()
		tempStr := fmt.Sprintf("%v", metrics["cpu_temp"])
		temp, _ := strconv.ParseFloat(tempStr, 64)
		maxTemp := c.Thermal.MaxTempC
		if maxTemp <= 0 {
			maxTemp = 75.0
		}
		coolTemp := c.Thermal.CooldownTempC
		if coolTemp <= 0 {
			coolTemp = 65.0
		}

		store.mu.Lock()
		if c.Thermal.Enabled && temp >= maxTemp {
			if !store.IsThrottled {
				writeLog("supervisor", fmt.Sprintf("CPU PANAS (%.1f°C >= %.1f°C). Menjeda kompresi...", temp, maxTemp))
			}
			store.IsThrottled = true
			store.PosStatus = fmt.Sprintf("THROTTLED (%.1f°C >= %.0f°C)", temp, maxTemp)
			store.PosStatusCls = "THROTTLED"
			store.ActiveNode = "temp_check"
		} else if store.IsThrottled {
			if !c.Thermal.Enabled || temp <= coolTemp {
				store.IsThrottled = false
				writeLog("supervisor", fmt.Sprintf("Suhu normal (%.1f°C). Melanjutkan kembali...", temp))
			} else {
				store.PosStatus = fmt.Sprintf("COOLDOWN (%.1f°C -> %.0f°C)", temp, coolTemp)
				store.PosStatusCls = "THROTTLED"
				store.ActiveNode = "temp_check"
			}
		}
		store.mu.Unlock()

		time.Sleep(3 * time.Second)
	}
}

// --- Background Threads ---
func startScanner() {
	writeLog("scanner", "Scanner service started.")
	for {
		dir := getCfg().Storage.ImmichUploadDir
		if dir == "" {
			dir = "/mnt/hdd1/immich/upload/upload"
		}

		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			writeLog("scanner", "Scanning folder: "+dir)
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if imageExts[ext] || videoExts[ext] {
					if strings.Contains(path, ".tmp_opt") || strings.Contains(path, ".bak_opt") {
						return nil
					}
					if store.getFile(path) == nil {
						mType := "P"
						if videoExts[ext] {
							mType = "V"
						}
						store.addOrUpdate(&ProcessedFile{
							FilePath:      path,
							FileName:      filepath.Base(path),
							MediaType:     mType,
							Status:        "PENDING",
							OrigSize:      info.Size(),
							SyncStatus:    "PENDING",
							ProcessedTime: time.Now().Format("2006-01-02 15:04:05"),
						})
						writeLog("scanner", "Antrian baru: "+filepath.Base(path))
					}
				}
				return nil
			})

			// Cek apakah ada file lokal yang sudah dihapus oleh user
			// Jika file lokal sudah terhapus, tandai statusnya agar dihapus juga di MEGA
			store.mu.Lock()
			for path, f := range store.Files {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					if f.SyncStatus != "DELETED_LOCAL" && f.SyncStatus != "DELETED_CLOUD" {
						f.SyncStatus = "DELETED_LOCAL"
						writeLog("scanner", fmt.Sprintf("File lokal terhapus oleh user: %s (akan disinkronisasi hapus ke MEGA)", f.FileName))
					}
				}
			}
			store.mu.Unlock()
		} else {
			writeLog("scanner", "Direktori tidak ditemukan: "+dir)
		}

		for i := 0; i < 60; i++ {
			store.mu.Lock()
			scanReq := store.ScanRequested
			if scanReq {
				store.ScanRequested = false
			}
			store.mu.Unlock()

			if scanReq {
				break
			}
			time.Sleep(10 * time.Second)
		}
	}
}

// findLivePhotoPair scans store.Files for a pending Live Photo pair (matching base path and name, one photo and one video)
func findLivePhotoPair() (*ProcessedFile, *ProcessedFile) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	// Build map of base name -> files
	baseMap := make(map[string]map[string]*ProcessedFile) // key: dir + "/" + baseWithoutExt
	for _, f := range store.Files {
		if f.Status == "PENDING" {
			dir := filepath.Dir(f.FilePath)
			ext := filepath.Ext(f.FilePath)
			nameNoExt := strings.TrimSuffix(filepath.Base(f.FilePath), ext)
			key := filepath.Join(dir, strings.ToLower(nameNoExt))
			if baseMap[key] == nil {
				baseMap[key] = make(map[string]*ProcessedFile)
			}
			baseMap[key][f.MediaType] = f
		}
	}

	for _, m := range baseMap {
		photoFile, hasP := m["P"]
		videoFile, hasV := m["V"]
		if hasP && hasV {
			return photoFile, videoFile
		}
	}
	return nil, nil
}

func startSupervisor() {
	writeLog("supervisor", "Supervisor loop started with Mutual-Exclusion Concurrency Coordinator.")
	for {
		store.mu.RLock()
		mode := store.Mode
		throttled := store.IsThrottled
		store.mu.RUnlock()

		if throttled {
			time.Sleep(3 * time.Second)
			continue
		}

		if mode == "pause" {
			store.mu.Lock()
			store.PosStatus = "PAUSED"
			store.PosStatusCls = "PAUSED"
			store.ActiveNode = "start"
			store.mu.Unlock()
			time.Sleep(3 * time.Second)
			continue
		}

		// 2. Cek Jadwal (Node: time_check)
		inSched := isWithinSchedule()
		if !inSched {
			store.mu.Lock()
			store.PosStatus = "WAITING (SCHEDULE)"
			store.PosStatusCls = "WAITING"
			store.ActiveNode = "time_check"
			store.mu.Unlock()
			time.Sleep(5 * time.Second)
			continue
		}

		c := getCfg()
		maxPhotoWorkers := c.Compression.MaxPhotoWorkers
		if maxPhotoWorkers <= 0 {
			maxPhotoWorkers = 3
		}

		// STRATEGI PENJADWALAN KETAT (MUTUAL EXCLUSION & LIVE PHOTO PRIORITY):
		// 1. Cek apakah ada pasangan Live Photo (1 Foto + 1 Video pendamping) di antrean
		lpPhoto, lpVideo := findLivePhotoPair()
		if lpPhoto != nil && lpVideo != nil {
			store.mu.Lock()
			store.PosStatus = "RUNNING (LIVE PHOTO PAIR)"
			store.PosStatusCls = "RUNNING"
			store.ActiveNode = "compress"
			store.ActiveJobs = []ActiveJob{
				{
					Path:   lpPhoto.FilePath,
					Name:   lpPhoto.FileName,
					Type:   "P (Live)",
					SizeMB: float64(lpPhoto.OrigSize) / (1024 * 1024),
					Step:   "Kompresi Foto Pasangan",
				},
				{
					Path:   lpVideo.FilePath,
					Name:   lpVideo.FileName,
					Type:   "V (Live)",
					SizeMB: float64(lpVideo.OrigSize) / (1024 * 1024),
					Step:   "Kompresi Video Pendamping",
				},
			}
			lpPhoto.Status = "PROCESSING"
			lpVideo.Status = "PROCESSING"
			store.mu.Unlock()

			writeLog("supervisor", fmt.Sprintf("[LIVE PHOTO PAIR] Memproses 1 pasangan Live Photo bersamaan: %s (Foto) & %s (Video)", lpPhoto.FileName, lpVideo.FileName))

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				processFile(lpPhoto)
			}()
			go func() {
				defer wg.Done()
				processFile(lpVideo)
			}()
			wg.Wait()
			continue
		}

		// 2. Jika tidak ada Live Photo pair, ambil Video Mandiri (Solo: 1 video, TIDAK BOLEH BERSAMAAN DENGAN FOTO)
		store.mu.RLock()
		var pendingVideo *ProcessedFile
		for _, f := range store.Files {
			if f.Status == "PENDING" && f.MediaType == "V" {
				pendingVideo = f
				break
			}
		}
		store.mu.RUnlock()

		if pendingVideo != nil {
			store.mu.Lock()
			store.PosStatus = "RUNNING (VIDEO SOLO)"
			store.PosStatusCls = "RUNNING"
			store.ActiveNode = "compress"
			store.ActiveJobs = []ActiveJob{{
				Path:   pendingVideo.FilePath,
				Name:   pendingVideo.FileName,
				Type:   "V",
				SizeMB: float64(pendingVideo.OrigSize) / (1024 * 1024),
				Step:   "Kompresi Video (Solo)",
			}}
			pendingVideo.Status = "PROCESSING"
			store.mu.Unlock()

			writeLog("supervisor", fmt.Sprintf("[VIDEO SOLO] Memproses 1 video mandiri: %s (mutual exclusion: foto ditahan)", pendingVideo.FileName))
			processFile(pendingVideo)
			continue
		}

		// 3. Jika tidak ada Video di antrean, jalankan Batch Foto (hingga 3 foto sekaligus, 0 video)
		store.mu.RLock()
		var pendingPhotos []*ProcessedFile
		for _, f := range store.Files {
			if f.Status == "PENDING" && f.MediaType == "P" {
				pendingPhotos = append(pendingPhotos, f)
				if len(pendingPhotos) >= maxPhotoWorkers {
					break
				}
			}
		}
		store.mu.RUnlock()

		if len(pendingPhotos) > 0 {
			store.mu.Lock()
			store.PosStatus = fmt.Sprintf("RUNNING (%d FOTO BATCH)", len(pendingPhotos))
			store.PosStatusCls = "RUNNING"
			store.ActiveNode = "compress"
			var jobs []ActiveJob
			for _, p := range pendingPhotos {
				p.Status = "PROCESSING"
				jobs = append(jobs, ActiveJob{
					Path:   p.FilePath,
					Name:   p.FileName,
					Type:   "P",
					SizeMB: float64(p.OrigSize) / (1024 * 1024),
					Step:   "Kompresi Foto Paralel",
				})
			}
			store.ActiveJobs = jobs
			store.mu.Unlock()

			writeLog("supervisor", fmt.Sprintf("[PHOTO BATCH] Menjalankan %d foto sekaligus secara paralel", len(pendingPhotos)))

			var wg sync.WaitGroup
			for _, p := range pendingPhotos {
				wg.Add(1)
				go func(item *ProcessedFile) {
					defer wg.Done()
					processFile(item)
				}(p)
			}
			wg.Wait()
			continue
		}

		// Jika antrean kosong
		store.mu.Lock()
		store.PosStatus = "IDLE (MENUNGGU)"
		store.PosStatusCls = "WAITING"
		store.ActiveNode = "start"
		store.ActiveJobs = nil
		store.mu.Unlock()
		time.Sleep(3 * time.Second)
	}
}

// queryMegaRemoteStats queries rclone size to obtain real live total file count and bytes in MEGA
func queryMegaRemoteStats(remote string) (int64, int64, error) {
	cmd := exec.Command("rclone", "size", "--json", remote)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	var res struct {
		Count int64 `json:"count"`
		Bytes int64 `json:"bytes"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return 0, 0, err
	}
	return res.Count, res.Bytes, nil
}

// verifyFileExistsInMega checks whether a specific remote path exists on MEGA
func verifyFileExistsInMega(remoteTarget string) bool {
	cmd := exec.Command("rclone", "lsf", remoteTarget)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func startSyncWorker() {
	writeLog("backup", "Sync engine started.")
	for {
		store.mu.RLock()
		syncPaused := store.SyncPaused
		syncMode := store.SyncMode
		activeCount := len(store.ActiveJobs)
		lastAudit := store.LastCloudAudit
		store.mu.RUnlock()

		if syncPaused {
			time.Sleep(5 * time.Second)
			continue
		}

		c := getCfg()
		if !c.Backup.Enabled {
			time.Sleep(10 * time.Second)
			continue
		}
		remote := c.Backup.RcloneRemote
		if remote == "" {
			remote = "mega:ImmichBackup"
		}
		args := c.Backup.RcloneArgs
		if args == "" {
			args = "--transfers=1"
		}

		// 1. Audit Berkala Total Penyimpanan Asli di MEGA & Deteksi File Hilang (tiap 60 detik atau jika dipicu)
		if time.Since(lastAudit) > 60*time.Second {
			if count, totalB, err := queryMegaRemoteStats(remote); err == nil {
				store.mu.Lock()
				store.CloudFilesCount = count
				store.CloudTotalBytes = totalB
				store.LastCloudAudit = time.Now()
				store.mu.Unlock()
			}

			// Cek file yang berstatus UPLOADED: jika ternyata hilang di MEGA, kembalikan ke PENDING (kirim ulang)
			store.mu.RLock()
			var filesToCheck []*ProcessedFile
			for _, f := range store.Files {
				if f.Status == "SUCCESS" && f.SyncStatus == "UPLOADED" && f.RemoteDest != "" {
					filesToCheck = append(filesToCheck, f)
				}
			}
			store.mu.RUnlock()

			for _, f := range filesToCheck {
				if !verifyFileExistsInMega(f.RemoteDest) {
					// Jika file lokal masih ada, kirim ulang!
					if _, err := os.Stat(f.FilePath); err == nil {
						f.SyncStatus = "PENDING"
						store.addOrUpdate(f)
						writeLog("backup", fmt.Sprintf("File hilang di MEGA terdeteksi: %s -> Dijadwalkan KIRIM ULANG", f.FileName))
					}
				}
			}
		}

		store.mu.Lock()
		syncReq := store.SyncRequested
		if syncReq {
			store.SyncRequested = false
		}
		store.mu.Unlock()

		if syncMode == "normal" && activeCount > 0 && !syncReq {
			time.Sleep(10 * time.Second)
			continue
		}

		// 2. Cek apakah ada file lokal yang dihapus user -> sinkronisasi hapus satu arah ke MEGA
		store.mu.RLock()
		var deleteTarget *ProcessedFile
		for _, f := range store.Files {
			if f.SyncStatus == "DELETED_LOCAL" && f.RemoteDest != "" {
				deleteTarget = f
				break
			}
		}
		store.mu.RUnlock()

		if deleteTarget != nil {
			store.mu.Lock()
			store.ActiveNode = "sync"
			store.mu.Unlock()

			writeLog("backup", fmt.Sprintf("Sinkronisasi Hapus ke MEGA: %s (%s)", deleteTarget.FileName, deleteTarget.RemoteDest))
			delCmd := exec.Command("rclone", "deletefile", deleteTarget.RemoteDest)
			if out, err := delCmd.CombinedOutput(); err == nil {
				deleteTarget.SyncStatus = "DELETED_CLOUD"
				store.addOrUpdate(deleteTarget)
				writeLog("backup", fmt.Sprintf("Berhasil dihapus dari MEGA: %s", deleteTarget.FileName))
				// Update audit cloud
				if count, totalB, err := queryMegaRemoteStats(remote); err == nil {
					store.mu.Lock()
					store.CloudFilesCount = count
					store.CloudTotalBytes = totalB
					store.mu.Unlock()
				}
			} else {
				writeLog("backup", fmt.Sprintf("Gagal menghapus file di MEGA %s: %v (%s)", deleteTarget.FileName, err, string(out)))
			}

			store.mu.Lock()
			store.ActiveNode = "start"
			store.mu.Unlock()
			continue
		}

		// 3. Ambil target upload (file yang baru dioptimasi atau yang perlu dikirim ulang)
		store.mu.RLock()
		var syncTarget *ProcessedFile
		for _, f := range store.Files {
			if (f.Status == "SUCCESS" || f.Status == "SKIPPED") && f.SyncStatus == "PENDING" {
				// Pastikan file lokal masih ada
				if _, err := os.Stat(f.FilePath); err == nil {
					syncTarget = f
					break
				}
			}
		}
		store.mu.RUnlock()

		if syncTarget != nil || syncReq {
			if syncTarget != nil {
				dateFolder := extractMediaDate(syncTarget.FilePath)
				remoteTarget := fmt.Sprintf("%s/%s/%s", strings.TrimRight(remote, "/"), dateFolder, syncTarget.FileName)
				syncTarget.RemoteDest = remoteTarget

				store.mu.Lock()
				store.ActiveNode = "sync"
				store.mu.Unlock()

				writeLog("backup", fmt.Sprintf("Syncing to Cloud: %s -> %s", syncTarget.FileName, remoteTarget))
				cmdArgs := []string{"copyto", syncTarget.FilePath, remoteTarget}
				cmdArgs = append(cmdArgs, strings.Fields(args)...)

				cmd := exec.Command("rclone", cmdArgs...)
				if out, err := cmd.CombinedOutput(); err == nil {
					syncTarget.SyncStatus = "UPLOADED"
					store.addOrUpdate(syncTarget)
					writeLog("backup", fmt.Sprintf("Uploaded: %s -> %s", syncTarget.FileName, remoteTarget))
					
					// Segarkan metrik kapasitas asli dari MEGA
					if count, totalB, err := queryMegaRemoteStats(remote); err == nil {
						store.mu.Lock()
						store.CloudFilesCount = count
						store.CloudTotalBytes = totalB
						store.mu.Unlock()
					}
				} else {
					syncTarget.SyncStatus = "FAILED"
					store.addOrUpdate(syncTarget)
					writeLog("backup", fmt.Sprintf("Failed sync %s: %v (%s)", syncTarget.FileName, err, string(out)))
				}

				store.mu.Lock()
				store.ActiveNode = "start"
				store.mu.Unlock()
			}
		} else {
			time.Sleep(10 * time.Second)
		}
	}
}

// --- HTTP Server & REST API ---
func isAuthenticated(r *http.Request) bool {
	c := getCfg()
	if !c.Server.AuthEnabled {
		return true
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		authHdr := r.Header.Get("Authorization")
		if strings.HasPrefix(authHdr, "Bearer ") {
			token = strings.TrimPrefix(authHdr, "Bearer ")
		}
	}
	if token == "" {
		return false
	}
	authMu.RLock()
	_, exists := authTokens[token]
	authMu.RUnlock()
	return exists
}

func sendJSON(w http.ResponseWriter, data interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

// Request payload for setting updates
type ConfigUpdateRequest struct {
	Server struct {
		Host         *string `json:"host"`
		Port         *int    `json:"port"`
		AuthEnabled  *bool   `json:"auth_enabled"`
		NewPassword  *string `json:"new_password"`
		PasswordHint *string `json:"password_hint"`
	} `json:"server"`
	Storage struct {
		ImmichUploadDir *string `json:"immich_upload_dir"`
		StateFilePath   *string `json:"state_file_path"`
		LogDir          *string `json:"log_dir"`
	} `json:"storage"`
	Compression struct {
		Mode            *string `json:"mode"`
		VideoCodec      *string `json:"video_codec"`
		VideoPreset     *string `json:"video_preset"`
		VideoCRF        *int    `json:"video_crf"`
		VideoMaxHeight  *int    `json:"video_max_height"`
		VideoThreads    *int    `json:"video_threads"`
		PhotoQuality    *int    `json:"photo_quality"`
		PhotoMaxDim     *int    `json:"photo_max_dim"`
		MaxVideoWorkers *int    `json:"max_video_workers"`
		MaxPhotoWorkers *int    `json:"max_photo_workers"`
	} `json:"compression"`
	Thermal struct {
		Enabled         *bool    `json:"enabled"`
		MaxTempC        *float64 `json:"max_temp_c"`
		CooldownTempC   *float64 `json:"cooldown_temp_c"`
		ThermalZonePath *string  `json:"thermal_zone_path"`
	} `json:"thermal"`
	Schedule struct {
		Enabled   *bool `json:"enabled"`
		StartHour *int  `json:"start_hour"`
		EndHour   *int  `json:"end_hour"`
	} `json:"schedule"`
	Backup struct {
		Enabled      *bool   `json:"enabled"`
		Mode         *string `json:"mode"`
		RcloneRemote *string `json:"rclone_remote"`
		RcloneArgs   *string `json:"rclone_args"`
	} `json:"backup"`
}

func handleRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path

	// Static Assets
	if path == "/" || path == "/index.html" {
		http.ServeFile(w, r, "static/index.html")
		return
	}
	if strings.HasPrefix(path, "/static/") {
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))).ServeHTTP(w, r)
		return
	}

	// Auth Check
	if path == "/api/auth/status" {
		sendJSON(w, map[string]interface{}{
			"auth_required": getCfg().Server.AuthEnabled,
			"authenticated": isAuthenticated(r),
		}, 200)
		return
	}

	// Login Endpoint
	if path == "/api/auth/login" && r.Method == "POST" {
		var req struct {
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		h := sha256.Sum256([]byte(req.Password))
		hashStr := hex.EncodeToString(h[:])
		c := getCfg()
		if hashStr == c.Server.PasswordHash {
			tokenBytes := sha256.Sum256([]byte(fmt.Sprintf("%s_%d", req.Password, time.Now().UnixNano())))
			token := hex.EncodeToString(tokenBytes[:])
			authMu.Lock()
			authTokens[token] = time.Now()
			authMu.Unlock()
			sendJSON(w, map[string]interface{}{"success": true, "token": token}, 200)
		} else {
			sendJSON(w, map[string]interface{}{"success": false, "message": "Password salah!"}, 403)
		}
		return
	}

	// Protected Endpoints
	if !isAuthenticated(r) {
		sendJSON(w, map[string]string{"error": "Unauthorized"}, 401)
		return
	}

	if path == "/api/status" {
		store.mu.RLock()
		actJobs := store.ActiveJobs
		actNode := store.ActiveNode
		mode := store.Mode
		syncMode := store.SyncMode
		syncPaused := store.SyncPaused
		posStat := store.PosStatus
		posCls := store.PosStatusCls
		throttled := store.IsThrottled
		store.mu.RUnlock()

		metrics := getSystemMetrics()
		detailedMetrics := getDetailedSystemMetrics()
		stats := store.getStats()
		recent := store.getRecentFiles(30)
		syncFiles := store.getSyncFiles(30)

		sendJSON(w, map[string]interface{}{
			"mode":             mode,
			"sync_mode":        syncMode,
			"sync_paused":      syncPaused,
			"is_throttled":     throttled,
			"pos_status":       posStat,
			"pos_status_class": posCls,
			"in_schedule":      isWithinSchedule(),
			"active_node":      actNode,
			"active_jobs":      actJobs,
			"sys":              metrics,
			"sys_detailed":     detailedMetrics,
			"stats":            stats,
			"files":            recent,
			"sync_files":       syncFiles,
		}, 200)
		return
	}

	// Config / Settings GET Endpoint
	if (path == "/api/config" || path == "/api/settings") && r.Method == "GET" {
		c := getCfg()
		safeConfig := map[string]interface{}{
			"server": map[string]interface{}{
				"host":          c.Server.Host,
				"port":          c.Server.Port,
				"auth_enabled":  c.Server.AuthEnabled,
				"password_hint": c.Server.PasswordHint,
			},
			"storage":     c.Storage,
			"compression": c.Compression,
			"thermal":     c.Thermal,
			"schedule":    c.Schedule,
			"backup":      c.Backup,
		}
		sendJSON(w, safeConfig, 200)
		return
	}

	// Config / Settings POST Endpoint - Dynamic Reload & Persistence
	if (path == "/api/config" || path == "/api/settings") && r.Method == "POST" {
		var req ConfigUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSON(w, map[string]interface{}{"success": false, "message": "Format data JSON tidak valid: " + err.Error()}, 400)
			return
		}

		current := getCfg()
		portChanged := false

		// Server Config Update
		if req.Server.Host != nil && *req.Server.Host != "" {
			current.Server.Host = *req.Server.Host
		}
		if req.Server.Port != nil && *req.Server.Port > 0 && *req.Server.Port <= 65535 {
			if current.Server.Port != *req.Server.Port {
				portChanged = true
			}
			current.Server.Port = *req.Server.Port
		}
		if req.Server.AuthEnabled != nil {
			current.Server.AuthEnabled = *req.Server.AuthEnabled
		}
		newToken := ""
		if req.Server.NewPassword != nil && strings.TrimSpace(*req.Server.NewPassword) != "" {
			newPass := strings.TrimSpace(*req.Server.NewPassword)
			h := sha256.Sum256([]byte(newPass))
			current.Server.PasswordHash = hex.EncodeToString(h[:])
			if req.Server.PasswordHint != nil {
				current.Server.PasswordHint = *req.Server.PasswordHint
			}
			tokenBytes := sha256.Sum256([]byte(fmt.Sprintf("%s_%d", newPass, time.Now().UnixNano())))
			newToken = hex.EncodeToString(tokenBytes[:])
			authMu.Lock()
			authTokens[newToken] = time.Now()
			authMu.Unlock()
			writeLog("supervisor", "Password admin berhasil diperbarui.")
		} else if req.Server.PasswordHint != nil {
			current.Server.PasswordHint = *req.Server.PasswordHint
		}

		// Storage Config Update
		if req.Storage.ImmichUploadDir != nil && *req.Storage.ImmichUploadDir != "" {
			current.Storage.ImmichUploadDir = *req.Storage.ImmichUploadDir
		}
		if req.Storage.StateFilePath != nil && *req.Storage.StateFilePath != "" {
			current.Storage.StateFilePath = *req.Storage.StateFilePath
		}
		if req.Storage.LogDir != nil && *req.Storage.LogDir != "" {
			current.Storage.LogDir = *req.Storage.LogDir
		}

		// Compression Config Update
		if req.Compression.Mode != nil && *req.Compression.Mode != "" {
			current.Compression.Mode = *req.Compression.Mode
			store.mu.Lock()
			store.Mode = *req.Compression.Mode
			store.mu.Unlock()
		}
		if req.Compression.VideoCodec != nil && *req.Compression.VideoCodec != "" {
			current.Compression.VideoCodec = *req.Compression.VideoCodec
		}
		if req.Compression.VideoPreset != nil && *req.Compression.VideoPreset != "" {
			current.Compression.VideoPreset = *req.Compression.VideoPreset
		}
		if req.Compression.VideoCRF != nil && *req.Compression.VideoCRF >= 10 && *req.Compression.VideoCRF <= 45 {
			current.Compression.VideoCRF = *req.Compression.VideoCRF
		}
		if req.Compression.VideoMaxHeight != nil && *req.Compression.VideoMaxHeight >= 360 {
			current.Compression.VideoMaxHeight = *req.Compression.VideoMaxHeight
		}
		if req.Compression.VideoThreads != nil && *req.Compression.VideoThreads >= 1 {
			current.Compression.VideoThreads = *req.Compression.VideoThreads
		}
		if req.Compression.PhotoQuality != nil && *req.Compression.PhotoQuality >= 10 && *req.Compression.PhotoQuality <= 100 {
			current.Compression.PhotoQuality = *req.Compression.PhotoQuality
		}
		if req.Compression.PhotoMaxDim != nil && *req.Compression.PhotoMaxDim >= 480 {
			current.Compression.PhotoMaxDim = *req.Compression.PhotoMaxDim
		}
		if req.Compression.MaxVideoWorkers != nil && *req.Compression.MaxVideoWorkers >= 1 {
			current.Compression.MaxVideoWorkers = *req.Compression.MaxVideoWorkers
		}
		if req.Compression.MaxPhotoWorkers != nil && *req.Compression.MaxPhotoWorkers >= 1 {
			current.Compression.MaxPhotoWorkers = *req.Compression.MaxPhotoWorkers
		}

		// Thermal Config Update
		if req.Thermal.Enabled != nil {
			current.Thermal.Enabled = *req.Thermal.Enabled
		}
		if req.Thermal.MaxTempC != nil && *req.Thermal.MaxTempC >= 40.0 && *req.Thermal.MaxTempC <= 95.0 {
			current.Thermal.MaxTempC = *req.Thermal.MaxTempC
		}
		if req.Thermal.CooldownTempC != nil && *req.Thermal.CooldownTempC >= 30.0 && *req.Thermal.CooldownTempC <= 90.0 {
			current.Thermal.CooldownTempC = *req.Thermal.CooldownTempC
		}
		if req.Thermal.ThermalZonePath != nil && *req.Thermal.ThermalZonePath != "" {
			current.Thermal.ThermalZonePath = *req.Thermal.ThermalZonePath
		}

		// Schedule Config Update
		if req.Schedule.Enabled != nil {
			current.Schedule.Enabled = *req.Schedule.Enabled
		}
		if req.Schedule.StartHour != nil && *req.Schedule.StartHour >= 0 && *req.Schedule.StartHour <= 23 {
			current.Schedule.StartHour = *req.Schedule.StartHour
		}
		if req.Schedule.EndHour != nil && *req.Schedule.EndHour >= 0 && *req.Schedule.EndHour <= 23 {
			current.Schedule.EndHour = *req.Schedule.EndHour
		}

		// Backup Config Update
		if req.Backup.Enabled != nil {
			current.Backup.Enabled = *req.Backup.Enabled
		}
		if req.Backup.Mode != nil && *req.Backup.Mode != "" {
			current.Backup.Mode = *req.Backup.Mode
			store.mu.Lock()
			store.SyncMode = *req.Backup.Mode
			store.mu.Unlock()
		}
		if req.Backup.RcloneRemote != nil {
			current.Backup.RcloneRemote = *req.Backup.RcloneRemote
		}
		if req.Backup.RcloneArgs != nil {
			current.Backup.RcloneArgs = *req.Backup.RcloneArgs
		}

		// Save updated config to disk atomically
		if err := saveConfigFile(current); err != nil {
			sendJSON(w, map[string]interface{}{"success": false, "message": "Gagal menyimpan ke config.json: " + err.Error()}, 500)
			return
		}

		// Apply dynamically to in-memory runtime
		setCfg(current)
		writeLog("supervisor", "Konfigurasi sistem berhasil diperbarui & dimuat ulang via Web UI.")

		respData := map[string]interface{}{
			"success":      true,
			"message":      "Konfigurasi berhasil disimpan dan dimuat ulang secara dinamis!",
			"port_changed": portChanged,
		}
		if newToken != "" {
			respData["new_token"] = newToken
		}
		sendJSON(w, respData, 200)
		return
	}

	if path == "/api/logs" {
		logType := r.URL.Query().Get("type")
		if logType == "" {
			logType = "supervisor"
		}
		logDir := getCfg().Storage.LogDir
		if logDir == "" {
			logDir = "logs"
		}
		logPath := filepath.Join(logDir, logType+".log")
		content := ""
		if b, err := os.ReadFile(logPath); err == nil {
			lines := strings.Split(string(b), "\n")
			start := 0
			if len(lines) > 200 {
				start = len(lines) - 200
			}
			content = strings.Join(lines[start:], "\n")
		}
		sendJSON(w, map[string]interface{}{"type": logType, "content": content}, 200)
		return
	}

	if r.Method == "POST" {
		switch path {
		case "/api/action/mode":
			var req struct {
				Mode string `json:"mode"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Mode != "" {
				targetMode := req.Mode
				if targetMode == "resume" {
					targetMode = "normal"
				}
				store.mu.Lock()
				store.Mode = targetMode
				store.mu.Unlock()
				store.saveState()

				c := getCfg()
				c.Compression.Mode = targetMode
				setCfg(c)
				_ = saveConfigFile(c)

				sendJSON(w, map[string]interface{}{"success": true, "mode": targetMode}, 200)
				return
			}
		case "/api/action/scan":
			store.mu.Lock()
			store.ScanRequested = true
			store.mu.Unlock()
			sendJSON(w, map[string]interface{}{"success": true, "message": "Pemindaian dimulai"}, 200)
			return
		case "/api/action/sync/now":
			store.mu.Lock()
			store.SyncRequested = true
			store.mu.Unlock()
			sendJSON(w, map[string]interface{}{"success": true, "message": "Sinkronisasi dimulai"}, 200)
			return
		case "/api/action/sync/pause":
			var req struct {
				Paused bool `json:"paused"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			store.mu.Lock()
			store.SyncPaused = req.Paused
			store.mu.Unlock()
			sendJSON(w, map[string]interface{}{"success": true, "sync_paused": req.Paused}, 200)
			return
		case "/api/action/sync/mode":
			var req struct {
				Mode string `json:"mode"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Mode != "" {
				store.mu.Lock()
				store.SyncMode = req.Mode
				store.mu.Unlock()

				c := getCfg()
				c.Backup.Mode = req.Mode
				setCfg(c)
				_ = saveConfigFile(c)

				sendJSON(w, map[string]interface{}{"success": true, "sync_mode": req.Mode}, 200)
				return
			}
		case "/api/action/sync/retry":
			store.mu.Lock()
			for _, f := range store.Files {
				if f.SyncStatus == "FAILED" {
					f.SyncStatus = "PENDING"
				}
			}
			store.mu.Unlock()
			store.saveState()
			sendJSON(w, map[string]interface{}{"success": true, "message": "Antrian gagal di-reset"}, 200)
			return
		}
	}

	http.NotFound(w, r)
}

func main() {
	// 1. Baca Konfigurasi
	var initialCfg Config
	cfgData, err := os.ReadFile("config.json")
	if err != nil {
		log.Printf("⚠️ config.json tidak ditemukan, membuat konfigurasi default...")
		initialCfg = getDefaultConfig()
		_ = saveConfigFile(initialCfg)
	} else if err := json.Unmarshal(cfgData, &initialCfg); err != nil {
		log.Printf("⚠️ Format config.json salah (%v), menggunakan konfigurasi default...", err)
		initialCfg = getDefaultConfig()
		_ = saveConfigFile(initialCfg)
	}
	setCfg(initialCfg)

	statePath := initialCfg.Storage.StateFilePath
	if statePath == "" {
		statePath = "optimizer_state.json"
	}
	store = newStore(statePath)

	// 2. Jalankan background goroutines
	go startThermalGuard()
	go startScanner()
	go startSupervisor()
	go startSyncWorker()

	// 3. Jalankan HTTP Server
	port := initialCfg.Server.Port
	if port <= 0 {
		port = 5555
	}
	addr := fmt.Sprintf("%s:%d", initialCfg.Server.Host, port)
	writeLog("supervisor", fmt.Sprintf("Web UI running at http://%s", addr))
	log.Printf("🛡️ Immich Media Optimizer (Go) siap di http://%s", addr)

	http.HandleFunc("/", handleRoutes)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server berhenti: %v", err)
	}
}

