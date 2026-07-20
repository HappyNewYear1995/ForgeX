package handler

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/service"
	"jenkinsAgent/internal/utils"
)

type ReleaseHandler struct {
	tmpl           map[string]*template.Template
	releaseService *service.ReleaseService
	productService *service.ProductService
}

func NewReleaseHandler(tmpl map[string]*template.Template, rs *service.ReleaseService, ps *service.ProductService) *ReleaseHandler {
	return &ReleaseHandler{
		tmpl:           tmpl,
		releaseService: rs,
		productService: ps,
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

// ReleasesJSON returns releases for a product as JSON (used by expandable list)
func (h *ReleaseHandler) ReleasesJSON(w http.ResponseWriter, r *http.Request) {
	productID, _ := strconv.Atoi(r.PathValue("id"))
	releases, err := h.releaseService.ListReleases(uint(productID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(releases)
}
