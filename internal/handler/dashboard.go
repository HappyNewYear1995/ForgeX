package handler

import (
	"html/template"
	"net/http"

	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/service"
)

type DashboardHandler struct {
	tmpl           map[string]*template.Template
	productService *service.ProductService
	buildService   *service.BuildService
	releaseService *service.ReleaseService
}

func NewDashboardHandler(tmpl map[string]*template.Template, ps *service.ProductService, bs *service.BuildService, rs *service.ReleaseService) *DashboardHandler {
	return &DashboardHandler{
		tmpl:           tmpl,
		productService: ps,
		buildService:   bs,
		releaseService: rs,
	}
}

func (h *DashboardHandler) Index(w http.ResponseWriter, r *http.Request) {
	productCount, _ := h.productService.CountProducts()
	componentCount, _ := h.productService.CountComponents()
	releaseCount, _ := h.releaseService.CountReleases()
	buildStats, _ := h.buildService.Stats()
	recentBuilds, _ := h.buildService.ListRecent(10)

	data := map[string]interface{}{
		"Title":          "仪表盘",
		"Username":       middleware.GetUsername(r),
		"ProductCount":   productCount,
		"ComponentCount": componentCount,
		"ReleaseCount":   releaseCount,
		"BuildStats":     buildStats,
		"RecentBuilds":   recentBuilds,
	}
	_ = h.tmpl["dashboard"].ExecuteTemplate(w, "layout", data)
}
