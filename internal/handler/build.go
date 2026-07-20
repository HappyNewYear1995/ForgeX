package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/service"
	"jenkinsAgent/internal/store"
	"jenkinsAgent/internal/utils"
)

type BuildHandler struct {
	tmpl           map[string]*template.Template
	buildService   *service.BuildService
	releaseService *service.ReleaseService
	productService *service.ProductService
}

func NewBuildHandler(tmpl map[string]*template.Template, bs *service.BuildService, rs *service.ReleaseService, ps *service.ProductService) *BuildHandler {
	return &BuildHandler{
		tmpl:           tmpl,
		buildService:   bs,
		releaseService: rs,
		productService: ps,
	}
}

func (h *BuildHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	productID, _ := strconv.Atoi(r.PathValue("id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	buildType := r.FormValue("build_type")
	if buildType == "" {
		buildType = "upgrade"
	}
	productVersion := r.FormValue("product_version")
	isFormal := r.FormValue("is_formal") == "true"
	releaseNotes := r.FormValue("release_notes")
	componentMods := r.FormValue("component_mods")
	autoSyncTest := r.FormValue("auto_sync_test") == "true"
	releaseID, _ := strconv.Atoi(r.FormValue("release_id"))

	// Validate version format if specified (empty is OK, will auto-increment)
	if productVersion != "" {
		if err := utils.ValidateVersion(productVersion); err != nil {
			http.Error(w, "项目"+err.Error(), http.StatusBadRequest)
			return
		}
	}

	params := service.BuildParams{
		ProductID:      uint(productID),
		ReleaseID:      uint(releaseID),
		ProductVersion: productVersion,
		BuildType:      buildType,
		IsFormal:       isFormal,
		ReleaseNotes:   releaseNotes,
		ComponentMods:  componentMods,
		AutoSyncTest:   autoSyncTest,
		TriggeredBy:    middleware.GetUsername(r),
		RequestHost:    r.Host,
	}

	_, err := h.buildService.Trigger(params)
	if err != nil {
		log.Printf("[build] trigger error: %v", err)
		http.Error(w, "触发构建失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[build] triggered product=%d version=%s by=%s", productID, productVersion, params.TriggeredBy)
	http.Redirect(w, r, "/products", http.StatusSeeOther)
}

func (h *BuildHandler) List(w http.ResponseWriter, r *http.Request) {
	productID, _ := strconv.Atoi(r.PathValue("id"))
	builds, err := h.buildService.ListByProductID(uint(productID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"Title":    "构建历史",
		"Username": middleware.GetUsername(r),
		"Builds":   builds,
	}
	_ = h.tmpl["build_list"].ExecuteTemplate(w, "layout", data)
}

func (h *BuildHandler) Log(w http.ResponseWriter, r *http.Request) {
	buildID, _ := strconv.Atoi(r.PathValue("buildId"))
	logText, err := h.buildService.GetBuildLog(uint(buildID))
	if err != nil {
		log.Printf("[build] get log error: %v", err)
		http.Error(w, "获取日志失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(logText))
}

// DownloadArtifact serves an artifact file for authenticated users
func (h *BuildHandler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	artifactID, _ := strconv.Atoi(r.PathValue("artifactId"))
	if artifactID == 0 {
		http.Error(w, "invalid artifact id", http.StatusBadRequest)
		return
	}

	artifactStore := store.NewArtifactStore()
	artifact, err := artifactStore.GetByID(uint(artifactID))
	if err != nil {
		http.Error(w, "artifact not found", http.StatusNotFound)
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

// Artifacts returns artifact list as JSON for a build
func (h *BuildHandler) Artifacts(w http.ResponseWriter, r *http.Request) {
	buildID, _ := strconv.Atoi(r.PathValue("buildId"))
	if buildID == 0 {
		http.Error(w, "invalid build id", http.StatusBadRequest)
		return
	}

	artifacts, err := h.buildService.GetArtifacts(uint(buildID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(artifacts)
}
