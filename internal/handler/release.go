package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/service"
	"jenkinsAgent/internal/utils"
)

type ReleaseHandler struct {
	tmpl           map[string]*template.Template
	releaseService *service.ReleaseService
	productService *service.ProductService
	packageService *service.PackageService
	buildService   *service.BuildService
}

func NewReleaseHandler(tmpl map[string]*template.Template, rs *service.ReleaseService, ps *service.ProductService, pkgSvc *service.PackageService, bs *service.BuildService) *ReleaseHandler {
	return &ReleaseHandler{
		tmpl:           tmpl,
		releaseService: rs,
		productService: ps,
		packageService: pkgSvc,
		buildService:   bs,
	}
}

func (h *ReleaseHandler) List(w http.ResponseWriter, r *http.Request) {
	releases, err := h.releaseService.ListAllReleases(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"Title":    "版本发布",
		"Username": middleware.GetUsername(r),
		"Releases": releases,
	}
	_ = h.tmpl["release_list"].ExecuteTemplate(w, "layout", data)
}

func (h *ReleaseHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	release, err := h.releaseService.GetRelease(uint(id))
	if err != nil {
		http.Error(w, "release not found", http.StatusNotFound)
		return
	}

	// Parse manifest JSON for display
	var manifest interface{}
	if release.ManifestJSON != "" {
		_ = json.Unmarshal([]byte(release.ManifestJSON), &manifest)
	}

	data := map[string]interface{}{
		"Title":    release.Version,
		"Username": middleware.GetUsername(r),
		"Release":  release,
		"Manifest": manifest,
	}
	_ = h.tmpl["release_detail"].ExecuteTemplate(w, "layout", data)
}

func (h *ReleaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	productID, _ := strconv.Atoi(r.PathValue("id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	version := r.FormValue("version")
	buildEnv := r.FormValue("build_env")
	description := r.FormValue("description")

	if err := utils.ValidateVersionRequired(version); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Parse component branches from form
	componentBranches := make(map[uint]string)
	if branches := r.Form["component_branch"]; len(branches) > 0 {
		compIDs := r.Form["component_id"]
		for i, cid := range compIDs {
			id, _ := strconv.Atoi(cid)
			if i < len(branches) {
				componentBranches[uint(id)] = branches[i]
			}
		}
	}

	release, err := h.releaseService.CreateRelease(uint(productID), version, buildEnv, description, componentBranches)
	if err != nil {
		log.Printf("[release] create error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[release] created: %s for product %d", release.Version, productID)
	http.Redirect(w, r, "/products/"+r.PathValue("id"), http.StatusSeeOther)
}

// ReleasesJSON returns released (completed) releases for a product as JSON (used by expandable list)
func (h *ReleaseHandler) ReleasesJSON(w http.ResponseWriter, r *http.Request) {
	productID, _ := strconv.Atoi(r.PathValue("id"))
	releases, err := h.releaseService.ListReleases(uint(productID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build release download availability: releaseID -> {"upgrade": true, "full": true}
	buildSvc := h.buildService
	var buildTypes map[uint]map[string]bool
	if buildSvc != nil {
		builds, _ := buildSvc.ListByProductID(uint(productID))
		buildTypes = make(map[uint]map[string]bool)
		for _, b := range builds {
			if b.ReleaseID > 0 && b.Status == model.BuildStatusSuccess {
				if _, ok := buildTypes[b.ReleaseID]; !ok {
					buildTypes[b.ReleaseID] = make(map[string]bool)
				}
				buildTypes[b.ReleaseID][b.BuildEnv] = true
			}
		}
	}

	// Only return released (completed) ones, with build type info
	type releaseItem struct {
		model.Release
		BuildTypes map[string]bool `json:"build_types"`
	}
	var filtered []releaseItem
	for _, rel := range releases {
		if rel.Status == model.ReleaseStatusReleased {
			bt := buildTypes[rel.ID]
			filtered = append(filtered, releaseItem{Release: rel, BuildTypes: bt})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(filtered)
}

// Manifest returns the manifest JSON for a release
func (h *ReleaseHandler) Manifest(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	release, err := h.releaseService.GetRelease(uint(id))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "release not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if release.ManifestJSON != "" {
		var raw json.RawMessage = []byte(release.ManifestJSON)
		_ = json.NewEncoder(w).Encode(raw)
	} else {
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "暂无 Manifest 数据"})
	}
}

// Download serves the packaged zip for a release+buildType
func (h *ReleaseHandler) Download(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	buildType := r.PathValue("buildType")

	if buildType != "upgrade" && buildType != "full" {
		http.Error(w, "invalid build type", http.StatusBadRequest)
		return
	}

	release, err := h.releaseService.GetRelease(uint(id))
	if err != nil {
		http.Error(w, "release not found", http.StatusNotFound)
		return
	}

	// Build component type map: componentName -> componentType
	compTypes := make(map[string]model.ComponentType)
	if release.Product != nil {
		comps, _ := h.productService.ListComponents(release.ProductID)
		for _, c := range comps {
			compTypes[c.Name] = c.Type
		}
	}

	productCode := "unknown"
	if release.Product != nil && release.Product.Code != "" {
		productCode = release.Product.Code
	}

	zipPath, err := h.packageService.PackageRelease(release, buildType, productCode, compTypes)
	if err != nil {
		log.Printf("[release] download error: %v", err)
		http.Error(w, "打包失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("acis_%s_%s_%s.zip", productCode, release.Version, buildType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/zip")
	http.ServeFile(w, r, zipPath)
}

// Delete deletes a release and redirects back to the product detail page
func (h *ReleaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	release, err := h.releaseService.GetRelease(uint(id))
	if err != nil {
		http.Error(w, "release not found", http.StatusNotFound)
		return
	}
	productID := release.ProductID
	if err := h.releaseService.DeleteRelease(uint(id)); err != nil {
		log.Printf("[release] delete error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[release] deleted release#%d version=%s", id, release.Version)
	http.Redirect(w, r, "/products/"+strconv.Itoa(int(productID)), http.StatusSeeOther)
}
