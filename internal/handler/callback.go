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
	"strings"
	"time"

	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/service"
	"jenkinsAgent/internal/store"
)

// getAppDir returns the directory where the executable is located.
func getAppDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

// CallbackHandler handles Jenkins build callbacks (no auth required)
type CallbackHandler struct {
	buildStore       *store.BuildStore
	artifactStore    *store.ArtifactStore
	releaseStore     *store.ReleaseStore
	rcStore          *store.ReleaseComponentStore
	componentStore   *store.ComponentStore
	productStore     *store.ProductStore
	jenkinsService   *service.JenkinsService
	packageService   *service.PackageService
	giteaService     *service.GiteaService
	releaseService   *service.ReleaseService
	onBuildComplete  func(*model.Build) // called after all components complete
}

func NewCallbackHandler(jenkinsService *service.JenkinsService, packageService *service.PackageService, giteaService *service.GiteaService, releaseService *service.ReleaseService) *CallbackHandler {
	return &CallbackHandler{
		buildStore:     store.NewBuildStore(),
		artifactStore:  store.NewArtifactStore(),
		releaseStore:   store.NewReleaseStore(),
		rcStore:        store.NewReleaseComponentStore(),
		componentStore: store.NewComponentStore(),
		productStore:   store.NewProductStore(),
		jenkinsService: jenkinsService,
		packageService: packageService,
		giteaService:   giteaService,
		releaseService: releaseService,
	}
}

// SetOnBuildComplete sets a callback that fires after callback processing completes.
func (h *CallbackHandler) SetOnBuildComplete(fn func(*model.Build)) {
	h.onBuildComplete = fn
}

// UploadArtifact handles POST /api/callback/build/{token}
// Supports two modes:
// 1. JSON callback from Jenkins (Content-Type: application/json) - auto-downloads artifact from downloadUrl
// 2. Multipart file upload (legacy) - direct file upload
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

	// Check content type to determine callback mode
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		h.handleJSONCallback(w, build, r)
		return
	}

	// Legacy multipart file upload mode
	h.handleMultipartUpload(w, build, r)
}

// handleJSONCallback processes JSON callback from Jenkins and auto-downloads artifact
func (h *CallbackHandler) handleJSONCallback(w http.ResponseWriter, build *model.Build, r *http.Request) {
	var req struct {
		Status       string `json:"status"`
		Repo         string `json:"repo"`
		Branch       string `json:"branch"`
		BuildNumber  int    `json:"buildNumber"`
		ArtifactName string `json:"artifactName"`
		DownloadURL  string `json:"downloadUrl"`
		ErrorMessage string `json:"errorMessage"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[callback] JSON callback for build#%d: status=%s artifact=%s", build.ID, req.Status, req.ArtifactName)

	// If build failed, update status and handle release
	if req.Status == "FAILURE" {
		build.Status = model.BuildStatusFailed
		now := time.Now()
		build.FinishedAt = &now
		_ = h.buildStore.Update(build)
		log.Printf("[callback] build#%d marked as failed: %s", build.ID, req.ErrorMessage)

		// Mark release as failed if all builds are done
		if build.ReleaseID > 0 && h.packageService != nil {
			allDone, allSuccess := h.packageService.CheckReleaseComplete(build.ReleaseID)
			if allDone && !allSuccess {
				if h.releaseService != nil {
					_ = h.releaseService.UpdateReleaseStatus(build.ReleaseID, model.ReleaseStatusFailed)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"build_id": build.ID,
			"status":   "failed",
			"message":  req.ErrorMessage,
		})
		return
	}

	// Download artifact from Jenkins if downloadUrl is provided
	if req.DownloadURL != "" && h.jenkinsService != nil {
		appDir := getAppDir()
		artifactDir := filepath.Join(appDir, "data", "artifacts", fmt.Sprintf("build_%d", build.ID))
		if err := os.MkdirAll(artifactDir, 0755); err != nil {
			log.Printf("[callback] failed to create artifact dir: %v", err)
		} else {
			body, filename, err := h.jenkinsService.DownloadFile(req.DownloadURL)
			if err != nil {
				log.Printf("[callback] failed to download artifact from %s: %v", req.DownloadURL, err)
			} else {
				defer body.Close()
				if filename == "" {
					filename = req.ArtifactName
				}
				if filename == "" {
					filename = filepath.Base(req.DownloadURL)
				}
				savePath := filepath.Join(artifactDir, filename)
				dst, err := os.Create(savePath)
				if err != nil {
					log.Printf("[callback] failed to create artifact file: %v", err)
				} else {
					written, err := io.Copy(dst, body)
					dst.Close()
					if err != nil {
						log.Printf("[callback] failed to save artifact: %v", err)
					} else {
						artifact := &model.Artifact{
							BuildID:       build.ID,
							ComponentName: req.Branch,
							FileName:      filename,
							FilePath:      savePath,
							FileSize:      written,
							UploadedBy:    "jenkins",
						}
						if err := h.artifactStore.Create(artifact); err != nil {
							log.Printf("[callback] failed to create artifact record: %v", err)
						} else {
							log.Printf("[callback] artifact downloaded: build#%d file=%s size=%d", build.ID, filename, written)
						}
					}
				}
			}
		}
	}

	// Update build status to success
	build.Status = model.BuildStatusSuccess
	now := time.Now()
	build.FinishedAt = &now
	_ = h.buildStore.Update(build)

	// Update ReleaseComponent with git commit from Gitea
	if build.ReleaseID > 0 && h.giteaService != nil {
		h.updateGitCommit(build, req.Branch)
	}

	// Check if all builds for this release are complete
	if build.ReleaseID > 0 && h.packageService != nil {
		allDone, allSuccess := h.packageService.CheckReleaseComplete(build.ReleaseID)
		if allDone {
			log.Printf("[callback] all builds for release#%d complete, success=%v", build.ReleaseID, allSuccess)
			if allSuccess {
				// Package artifacts for this release
				release, err := h.releaseStore.GetByID(build.ReleaseID)
				if err == nil && release != nil {
					// Build component type map for packaging
					productCode := "unknown"
					compTypes := make(map[string]model.ComponentType)
					if p, perr := h.productStore.GetByID(release.ProductID); perr == nil && p != nil {
						productCode = p.Code
						if productCode == "" {
							productCode = "unknown"
						}
						if comps, cerr := h.componentStore.ListByProductID(release.ProductID); cerr == nil {
							for _, c := range comps {
								compTypes[c.Name] = c.Type
							}
						}
					}
					if zipPath, err := h.packageService.PackageRelease(release, build.BuildEnv, productCode, compTypes); err != nil {
						log.Printf("[callback] package error: %v", err)
					} else {
						log.Printf("[callback] packaged release#%d (%s): %s", release.ID, build.BuildEnv, zipPath)
					}
					// Generate manifest with git commits
					if h.releaseService != nil {
						_ = h.releaseService.UpdateReleaseStatus(release.ID, model.ReleaseStatusReleased)
					}
				}
			}
			// Trigger post-build scripts (Playwright) after ALL components complete
			if h.onBuildComplete != nil && build.RunScriptsAfterBuild {
				b := *build
				go h.onBuildComplete(&b)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"build_id": build.ID,
		"status":   "success",
	})
}

// handleMultipartUpload handles legacy multipart file upload
func (h *CallbackHandler) handleMultipartUpload(w http.ResponseWriter, build *model.Build, r *http.Request) {
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
	appDir := getAppDir()
	artifactDir := filepath.Join(appDir, "data", "artifacts", fmt.Sprintf("build_%d", build.ID))
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

// updateGitCommit fetches the latest git commit from Gitea and updates the ReleaseComponent.
func (h *CallbackHandler) updateGitCommit(build *model.Build, branch string) {
	// Parse parameters to get component info
	var params struct {
		ComponentName string `json:"COMPONENT_NAME"`
		RepoURL       string `json:"REPO_URL"`
		Branch        string `json:"BRANCH"`
	}
	if build.ParametersJSON != "" {
		_ = json.Unmarshal([]byte(build.ParametersJSON), &params)
	}

	// Use branch from callback or params
	if branch == "" {
		branch = params.Branch
	}
	if branch == "" {
		branch = "main"
	}

	repoURL := params.RepoURL
	if repoURL == "" {
		return
	}

	sha, msg, err := h.giteaService.GetLatestCommit(repoURL, branch)
	if err != nil {
		log.Printf("[callback] get git commit error: %v", err)
		return
	}

	// Find and update the matching ReleaseComponent
	rcs, err := h.rcStore.ListByReleaseID(build.ReleaseID)
	if err != nil {
		return
	}
	for _, rc := range rcs {
		if rc.ComponentName == params.ComponentName || rc.GitBranch == branch {
			rc.GitCommit = sha
			rc.GitCommitMessage = msg
			rc.BuildStatus = "success"
			_ = h.rcStore.Update(&rc)
			log.Printf("[callback] updated git commit for component %s: %s %s", rc.ComponentName, sha, msg)
			break
		}
	}
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
