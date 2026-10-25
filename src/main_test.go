package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigPersistenceAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	sampleCfg := getDefaultConfig()
	sampleCfg.Server.Host = "127.0.0.1"
	sampleCfg.Server.Port = 5555
	sampleCfg.Server.AuthEnabled = true
	h := sha256.Sum256([]byte("admin"))
	sampleCfg.Server.PasswordHash = hex.EncodeToString(h[:])
	sampleCfg.Compression.VideoCodec = "libx265"
	sampleCfg.Compression.VideoCRF = 26
	sampleCfg.Compression.MaxVideoWorkers = 1
	sampleCfg.Compression.MaxPhotoWorkers = 2
	sampleCfg.Thermal.MaxTempC = 75.0
	sampleCfg.Thermal.CooldownTempC = 65.0
	sampleCfg.Schedule.StartHour = 1
	sampleCfg.Schedule.EndHour = 6

	if err := saveConfigFile(sampleCfg); err != nil {
		t.Fatalf("saveConfigFile error: %v", err)
	}

	data, err := os.ReadFile("config.json")
	if err != nil {
		t.Fatalf("Failed to read saved config.json: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if loaded.Server.Port != 5555 {
		t.Errorf("Expected port 5555, got %d", loaded.Server.Port)
	}
	if loaded.Compression.VideoCodec != "libx265" {
		t.Errorf("Expected codec libx265, got %s", loaded.Compression.VideoCodec)
	}
	if loaded.Compression.MaxVideoWorkers != 1 {
		t.Errorf("Expected max_video_workers 1, got %d", loaded.Compression.MaxVideoWorkers)
	}
}

func TestStoreStatsAndCategories(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "test_state.json")
	st := newStore(stateFile)

	file1 := &ProcessedFile{
		FilePath:      "/test/img1.jpg",
		FileName:      "img1.jpg",
		MediaType:     "P",
		Status:        "SUCCESS",
		OrigSize:      1000000,
		CompSize:      100000,
		SavedBytes:    900000,
		Ratio:         90.0,
		Category:      "IRIT",
		SyncStatus:    "PENDING",
		ProcessedTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	file2 := &ProcessedFile{
		FilePath:      "/test/video1.mov",
		FileName:      "video1.mov",
		MediaType:     "V",
		Status:        "SKIPPED",
		OrigSize:      5000000,
		CompSize:      5000000,
		SavedBytes:    0,
		Ratio:         0.0,
		Category:      "BOROS",
		SyncStatus:    "PENDING",
		ProcessedTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	st.addOrUpdate(file1)
	st.addOrUpdate(file2)

	stats := st.getStats()
	if stats["total"] != 2 {
		t.Errorf("Expected total 2, got %v", stats["total"])
	}
	if stats["success"] != 1 {
		t.Errorf("Expected success 1, got %v", stats["success"])
	}
	if stats["skipped"] != 1 {
		t.Errorf("Expected skipped 1, got %v", stats["skipped"])
	}
	if stats["total_saved_bytes"] != int64(900000) {
		t.Errorf("Expected saved bytes 900000, got %v", stats["total_saved_bytes"])
	}
}

func TestConfigAPIEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	initC := getDefaultConfig()
	initC.Server.Host = "0.0.0.0"
	initC.Server.Port = 5555
	initC.Server.AuthEnabled = false
	initC.Compression.VideoCodec = "libx265"
	initC.Compression.VideoCRF = 26
	setCfg(initC)
	_ = saveConfigFile(initC)

	store = newStore(filepath.Join(tmpDir, "state.json"))

	// Test GET /api/config
	reqGet := httptest.NewRequest("GET", "/api/config", nil)
	wGet := httptest.NewRecorder()
	handleRoutes(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected GET 200, got %d", wGet.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(wGet.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse config response: %v", err)
	}

	serverPart, ok := resp["server"].(map[string]interface{})
	if !ok || serverPart["port"] != float64(5555) {
		t.Errorf("Unexpected server config response: %v", resp["server"])
	}

	// Password hash should NEVER be leaked in GET response
	if _, hasHash := serverPart["password_hash"]; hasHash {
		t.Errorf("Security risk: password_hash leaked in GET /api/config")
	}

	// Test POST /api/config
	newPort := 6666
	newCRF := 28
	newPass := "supersecret"
	newVideoWorkers := 2
	updatePayload := map[string]interface{}{
		"server": map[string]interface{}{
			"port":         newPort,
			"new_password": newPass,
		},
		"compression": map[string]interface{}{
			"video_crf":         newCRF,
			"max_video_workers": newVideoWorkers,
		},
	}
	bodyBytes, _ := json.Marshal(updatePayload)
	reqPost := httptest.NewRequest("POST", "/api/config", bytes.NewReader(bodyBytes))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	handleRoutes(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("Expected POST 200, got %d: %s", wPost.Code, wPost.Body.String())
	}

	updated := getCfg()
	if updated.Server.Port != 6666 {
		t.Errorf("Expected port 6666, got %d", updated.Server.Port)
	}
	if updated.Compression.VideoCRF != 28 {
		t.Errorf("Expected CRF 28, got %d", updated.Compression.VideoCRF)
	}
	if updated.Compression.MaxVideoWorkers != 2 {
		t.Errorf("Expected MaxVideoWorkers 2, got %d", updated.Compression.MaxVideoWorkers)
	}

	expectedHash := sha256.Sum256([]byte(newPass))
	if updated.Server.PasswordHash != hex.EncodeToString(expectedHash[:]) {
		t.Errorf("Password was not properly hashed and updated")
	}
}

func TestAtomicReplaceSafe(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "original.jpg")
	srcTmp := filepath.Join(tmpDir, "original.jpg.tmp_opt.jpg")

	origContent := []byte("original image content here")
	newContent := []byte("compressed image content")

	if err := os.WriteFile(target, origContent, 0644); err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}
	if err := os.WriteFile(srcTmp, newContent, 0644); err != nil {
		t.Fatalf("Failed to create srcTmp: %v", err)
	}

	if err := atomicReplace(srcTmp, target); err != nil {
		t.Fatalf("atomicReplace failed: %v", err)
	}

	// Verify target now has new content
	readBack, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("Failed to read replaced target: %v", err)
	}
	if string(readBack) != string(newContent) {
		t.Errorf("Expected content %s, got %s", string(newContent), string(readBack))
	}

	// Verify backup was cleaned up
	if _, err := os.Stat(target + ".bak_opt"); !os.IsNotExist(err) {
		t.Errorf("Expected backup to be removed after successful atomic replace")
	}
}

func TestZeroSizeCorruptOutputPrevention(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	target := filepath.Join(tmpDir, "photo.jpg")
	origContent := make([]byte, 1024*1024) // 1MB original file
	for i := range origContent {
		origContent[i] = 0xAA
	}
	if err := os.WriteFile(target, origContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	st := newStore(filepath.Join(tmpDir, "state.json"))
	store = st

	p := &ProcessedFile{
		FilePath:      target,
		FileName:      "photo.jpg",
		MediaType:     "P",
		Status:        "PENDING",
		OrigSize:      int64(len(origContent)),
		SyncStatus:    "PENDING",
		ProcessedTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Simulate a corrupt zero-byte output file
	tmpDst := target + ".tmp_opt.jpg"
	_ = os.WriteFile(tmpDst, []byte(""), 0644) // 0-byte file

	// Calling processFile on invalid ffmpeg command simulation
	// Because ffmpeg will fail on raw dummy bytes, processFile must mark ERROR
	// and NEVER overwrite target with 0 bytes!
	processFile(p)

	// Verify original file is 100% intact
	readBack, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("Target was deleted: %v", err)
	}
	if len(readBack) != len(origContent) {
		t.Fatalf("CRITICAL BUG: Original file was overwritten or truncated! Expected %d bytes, got %d", len(origContent), len(readBack))
	}
	if p.Status != "ERROR" && p.Status != "SKIPPED" {
		t.Errorf("Expected status ERROR or SKIPPED, got %s", p.Status)
	}
}

func TestSettingsAPIEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	initC := getDefaultConfig()
	initC.Server.Host = "0.0.0.0"
	initC.Server.Port = 5555
	initC.Server.AuthEnabled = false
	initC.Storage.ImmichUploadDir = "/mnt/hdd1/immich/upload/upload"
	setCfg(initC)
	_ = saveConfigFile(initC)

	store = newStore(filepath.Join(tmpDir, "state.json"))

	// Test GET /api/settings
	reqGet := httptest.NewRequest("GET", "/api/settings", nil)
	wGet := httptest.NewRecorder()
	handleRoutes(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected GET /api/settings 200, got %d", wGet.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(wGet.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse settings response: %v", err)
	}

	serverPart, ok := resp["server"].(map[string]interface{})
	if !ok || serverPart["port"] != float64(5555) {
		t.Errorf("Unexpected server settings response: %v", resp["server"])
	}
	if _, hasHash := serverPart["password_hash"]; hasHash {
		t.Errorf("Security risk: password_hash leaked in GET /api/settings")
	}

	// Test POST /api/settings
	newPort := 7777
	newCRF := 24
	newPass := "adminator2026"
	newUploadDir := "/custom/immich/upload"
	updatePayload := map[string]interface{}{
		"server": map[string]interface{}{
			"port":         newPort,
			"new_password": newPass,
		},
		"storage": map[string]interface{}{
			"immich_upload_dir": newUploadDir,
		},
		"compression": map[string]interface{}{
			"video_crf": newCRF,
		},
	}
	bodyBytes, _ := json.Marshal(updatePayload)
	reqPost := httptest.NewRequest("POST", "/api/settings", bytes.NewReader(bodyBytes))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	handleRoutes(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("Expected POST /api/settings 200, got %d: %s", wPost.Code, wPost.Body.String())
	}

	updated := getCfg()
	if updated.Server.Port != 7777 {
		t.Errorf("Expected port 7777, got %d", updated.Server.Port)
	}
	if updated.Storage.ImmichUploadDir != newUploadDir {
		t.Errorf("Expected ImmichUploadDir %s, got %s", newUploadDir, updated.Storage.ImmichUploadDir)
	}
	if updated.Compression.VideoCRF != 24 {
		t.Errorf("Expected CRF 24, got %d", updated.Compression.VideoCRF)
	}

	expectedHash := sha256.Sum256([]byte(newPass))
	if updated.Server.PasswordHash != hex.EncodeToString(expectedHash[:]) {
		t.Errorf("Password was not properly hashed and updated via /api/settings")
	}

	var postResp map[string]interface{}
	if err := json.Unmarshal(wPost.Body.Bytes(), &postResp); err != nil {
		t.Fatalf("Failed to parse POST response: %v", err)
	}
	if postResp["new_token"] == nil || postResp["new_token"] == "" {
		t.Errorf("Expected new_token in response when password is changed")
	}
}

func TestStatusAPIEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	initC := getDefaultConfig()
	initC.Server.AuthEnabled = false
	setCfg(initC)

	st := newStore(filepath.Join(tmpDir, "state.json"))
	st.Mode = "normal"
	st.SyncMode = "normal"
	st.SyncPaused = true
	st.IsThrottled = false
	store = st

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	handleRoutes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 from /api/status, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal /api/status JSON: %v", err)
	}

	if resp["sync_paused"] != true {
		t.Errorf("Expected sync_paused true, got %v", resp["sync_paused"])
	}
	if resp["is_throttled"] != false {
		t.Errorf("Expected is_throttled false, got %v", resp["is_throttled"])
	}
	if resp["mode"] != "normal" {
		t.Errorf("Expected mode 'normal', got %v", resp["mode"])
	}
	if resp["sys"] == nil {
		t.Errorf("Expected sys metrics in status response")
	}
	if resp["stats"] == nil {
		t.Errorf("Expected stats in status response")
	}
}

func TestModeActionAPIEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	initC := getDefaultConfig()
	initC.Server.AuthEnabled = false
	setCfg(initC)
	_ = saveConfigFile(initC)

	st := newStore(filepath.Join(tmpDir, "state.json"))
	st.Mode = "pause"
	store = st

	// Test mode: resume -> should normalize to "normal"
	payload := map[string]string{"mode": "resume"}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/action/mode", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleRoutes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 from /api/action/mode, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["mode"] != "normal" {
		t.Errorf("Expected resume to normalize to 'normal', got %v", resp["mode"])
	}

	if store.Mode != "normal" {
		t.Errorf("Expected store.Mode 'normal', got %s", store.Mode)
	}
	if getCfg().Compression.Mode != "normal" {
		t.Errorf("Expected cfg.Compression.Mode 'normal', got %s", getCfg().Compression.Mode)
	}

	// Test mode: force
	payloadForce := map[string]string{"mode": "force"}
	bodyForce, _ := json.Marshal(payloadForce)
	reqForce := httptest.NewRequest("POST", "/api/action/mode", bytes.NewReader(bodyForce))
	wForce := httptest.NewRecorder()
	handleRoutes(wForce, reqForce)

	if store.Mode != "force" {
		t.Errorf("Expected store.Mode 'force', got %s", store.Mode)
	}
}

func TestExtractGPSSafeExecution(t *testing.T) {
	// Calling extractGPS on a non-existent file should gracefully return "" without panic or crash
	res := extractGPS("/path/to/non/existent/file.jpg")
	if res != "" {
		t.Errorf("Expected empty string for non-existent file, got %s", res)
	}
}

func TestGPSProtectionZeroLoss(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	target := filepath.Join(tmpDir, "geotagged_photo.jpg")
	origContent := []byte("fake geotagged image content with gps data")
	if err := os.WriteFile(target, origContent, 0644); err != nil {
		t.Fatalf("Failed to write target: %v", err)
	}

	st := newStore(filepath.Join(tmpDir, "state.json"))
	store = st

	p := &ProcessedFile{
		FilePath:      target,
		FileName:      "geotagged_photo.jpg",
		MediaType:     "P",
		Status:        "PENDING",
		OrigSize:      int64(len(origContent)),
		SyncStatus:    "PENDING",
		ProcessedTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// When processFile runs and ffmpeg fails or cannot copy GPS, target must remain intact
	processFile(p)

	readBack, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("Target file was deleted: %v", err)
	}
	if string(readBack) != string(origContent) {
		t.Fatalf("Target was altered or overwritten during failure: expected %s, got %s", string(origContent), string(readBack))
	}
}

func TestVerifyDetailedMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	f1 := filepath.Join(tmpDir, "f1.jpg")
	f2 := filepath.Join(tmpDir, "f2.jpg")
	_ = os.WriteFile(f1, []byte("test1"), 0644)
	_ = os.WriteFile(f2, []byte("test2"), 0644)

	// verifyDetailedMetadata should safely handle files without crashing
	status, safeTags, lostTags := verifyDetailedMetadata(f1, f2)
	if status == "" {
		t.Errorf("Expected non-empty metadata status")
	}
	_ = safeTags
	_ = lostTags
}

func TestExtractMediaDateFallback(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	sampleFile := filepath.Join(tmpDir, "sample_media.jpg")
	_ = os.WriteFile(sampleFile, []byte("fake image data"), 0644)

	// Since exiftool on a plain dummy file won't have EXIF tags, it should fallback to file ModTime
	datePath := extractMediaDate(sampleFile)
	expectedToday := time.Now().Format("2006/01/02")
	if datePath != expectedToday {
		t.Errorf("Expected date path %s, got %s", expectedToday, datePath)
	}

	// Non-existent file should fallback to current date
	nonExistentDate := extractMediaDate("/non/existent/path.mp4")
	if nonExistentDate != expectedToday {
		t.Errorf("Expected date path %s for non-existent file, got %s", expectedToday, nonExistentDate)
	}
}

func TestStoreCloudStatsIntegration(t *testing.T) {
	st := newStore("dummy_state.json")
	st.Files["/tmp/test1.jpg"] = &ProcessedFile{
		FilePath:   "/tmp/test1.jpg",
		FileName:   "test1.jpg",
		Status:     "SUCCESS",
		CompSize:   2000,
		SyncStatus: "UPLOADED",
	}

	stats := st.getStats()
	if stats["sync_uploaded"].(int64) != 1 {
		t.Errorf("Expected 1 sync_uploaded, got %v", stats["sync_uploaded"])
	}

	// Simulasi audit live dari MEGA (rclone size): jika ada 5 file berukuran 10000 bytes di MEGA
	st.CloudFilesCount = 5
	st.CloudTotalBytes = 10000

	statsAudited := st.getStats()
	if statsAudited["sync_uploaded"].(int64) != 5 {
		t.Errorf("Expected 5 audited sync_uploaded, got %v", statsAudited["sync_uploaded"])
	}
	if statsAudited["sync_uploaded_bytes"].(int64) != 10000 {
		t.Errorf("Expected 10000 audited sync_uploaded_bytes, got %v", statsAudited["sync_uploaded_bytes"])
	}
}
