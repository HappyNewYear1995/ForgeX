package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
)

// CallbackHandler handles Jenkins build callbacks (no auth required)
type CallbackHandler struct {
	buildStore    *store.BuildStore
	artifactStore *store.ArtifactStore
}

func NewCallbackHandler() *CallbackHandler {
	return &CallbackHandler{
		buildStore:    store.NewBuildStore(),
		artifactStore: store.NewArtifactStore(),
	}
}

// UploadArtifact handles POST /api/callback/build/{token}
// Jenkins calls this to upload build artifacts after a successful build.
//
// Multipart form fields:
//   - file: the artifact file (required)
//   - component: component name (optional)
//
// Multiple files can be uploaded by calling the endpoint multiple times.
func (h *CallbackHandler) UploadArtifact(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	// Find build by callback token
	build, err := h.buildStore.GetByCallbackToken(token)
	if err != nil {
		http.Error(w, "invalid callback token", http.StatusUnauthorized)
		return
	}

	// Parse multipart form (max 500MB)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer func(file io.ReadCloser) {
		_ = file.Close()
	}(file)

	componentName := r.FormValue("component")

	// Create artifact storage directory
	artifactDir := filepath.Join("data", "artifacts", fmt.Sprintf("build_%d", build.ID))
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		http.Error(w, "failed to create directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate unique filename to avoid conflicts
	timestamp := time.Now().Format("20060102_150405")
	safeName := filepath.Base(header.Filename)
	saveName := fmt.Sprintf("%s_%s", timestamp, safeName)
	savePath := filepath.Join(artifactDir, saveName)

	// Save file to disk
	dst, err := os.Create(savePath)
	if err != nil {
		http.Error(w, "failed to create file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func(dst *os.File) {
		_ = dst.Close()
	}(dst)

	written, err := io.Copy(dst, file)
	if err != nil {
		http.Error(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create artifact record
	artifact := &model.Artifact{
		BuildID:       build.ID,
		ComponentName: componentName,
		FileName:      header.Filename,
		FilePath:      savePath,
		FileSize:      written,
		ContentType:   header.Header.Get("Content-Type"),
		UploadedBy:    "jenkins",
	}
	if err := h.artifactStore.Create(artifact); err != nil {
		http.Error(w, "failed to create artifact record: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[callback] artifact uploaded: build#%d file=%s size=%d component=%s",
		build.ID, header.Filename, written, componentName)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"build_id":   build.ID,
		"artifact_id": artifact.ID,
		"file_name":  header.Filename,
		"file_size":  written,
	})
}

// BuildStatus handles POST /api/callback/build/{token}/status
// Jenkins calls this to report build completion status.
func (h *CallbackHandler) BuildStatus(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	build, err := h.buildStore.GetByCallbackToken(token)
	if err != nil {
		http.Error(w, "invalid callback token", http.StatusUnauthorized)
		return
	}

	var req struct {
		Status  string `json:"status"`  // success, failed
		Message string `json:"message"` // optional message
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	switch req.Status {
	case "success":
		build.Status = model.BuildStatusSuccess
	case "failed":
		build.Status = model.BuildStatusFailed
	default:
		http.Error(w, "status must be 'success' or 'failed'", http.StatusBadRequest)
		return
	}

	now := time.Now()
	build.FinishedAt = &now
	_ = h.buildStore.Update(build)

	log.Printf("[callback] build#%d status updated: %s - %s", build.ID, req.Status, req.Message)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"build_id": build.ID,
		"status":   build.Status,
	})
}

// ListArtifacts handles GET /api/callback/build/{token}/artifacts
// Returns list of uploaded artifacts for the build.
func (h *CallbackHandler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	build, err := h.buildStore.GetByCallbackToken(token)
	if err != nil {
		http.Error(w, "invalid callback token", http.StatusUnauthorized)
		return
	}

	artifacts, err := h.artifactStore.ListByBuildID(build.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(artifacts)
}

// DownloadArtifact handles GET /api/callback/build/{token}/artifacts/{artifactId}
func (h *CallbackHandler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	artifactIDStr := r.PathValue("artifactId")
	artifactID, _ := strconv.Atoi(artifactIDStr)

	if token == "" || artifactID == 0 {
		http.Error(w, "invalid parameters", http.StatusBadRequest)
		return
	}

	build, err := h.buildStore.GetByCallbackToken(token)
	if err != nil {
		http.Error(w, "invalid callback token", http.StatusUnauthorized)
		return
	}

	artifact, err := h.artifactStore.GetByID(uint(artifactID))
	if err != nil {
		http.Error(w, "artifact not found", http.StatusNotFound)
		return
	}

	if artifact.BuildID != build.ID {
		http.Error(w, "artifact does not belong to this build", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifact.FileName))
	if artifact.ContentType != "" {
		w.Header().Set("Content-Type", artifact.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	http.ServeFile(w, r, artifact.FilePath)
}
