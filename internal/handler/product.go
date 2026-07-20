package handler

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/service"
	"jenkinsAgent/internal/utils"
)

type ProductHandler struct {
	tmpl           map[string]*template.Template
	productService *service.ProductService
	buildService   *service.BuildService
	releaseService *service.ReleaseService
	configService  *service.ConfigService
}

func NewProductHandler(tmpl map[string]*template.Template, ps *service.ProductService, bs *service.BuildService, rs *service.ReleaseService, cs *service.ConfigService) *ProductHandler {
	return &ProductHandler{
		tmpl:           tmpl,
		productService: ps,
		buildService:   bs,
		releaseService: rs,
		configService:  cs,
	}
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("q")
	products, err := h.productService.ListProducts(keyword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"Title":    "项目列表",
		"Username": middleware.GetUsername(r),
		"Products": products,
		"Keyword":  keyword,
	}
	_ = h.tmpl["product_list"].ExecuteTemplate(w, "layout", data)
}

func (h *ProductHandler) CreateForm(w http.ResponseWriter, r *http.Request) {
	configTree, _ := h.configService.GetTree()
	testEnvs, _ := h.configService.ListTestEnvs()
	data := map[string]interface{}{
		"Title":          "新建项目",
		"Username":       middleware.GetUsername(r),
		"Product":        nil,
		"ConfigTree":     configTree,
		"SelectedComps":  map[uint]*model.Component{},
		"TestEnvs":       testEnvs,
		"SelectedEnvIDs": map[uint]bool{},
	}
	_ = h.tmpl["product_form"].ExecuteTemplate(w, "layout", data)
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	version := r.FormValue("current_version")
	if err := utils.ValidateVersion(version); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Test environment
	testEnvEnabled := r.FormValue("test_env_enabled") == "true"

	// Validation: at least one component
	compIDs := r.Form["comp_ids"]
	if len(compIDs) == 0 {
		http.Error(w, "请至少选择一个组件", http.StatusBadRequest)
		return
	}
	// Validation: each component must have Git URL
	for _, idStr := range compIDs {
		gitURL := r.FormValue("comp_git_url_" + idStr)
		if strings.TrimSpace(gitURL) == "" {
			compID, _ := strconv.Atoi(idStr)
			configItem, _ := h.configService.GetByID(uint(compID))
			name := idStr
			if configItem != nil {
				name = configItem.Name
			}
			http.Error(w, "组件「"+name+"」必须填写Git仓库地址", http.StatusBadRequest)
			return
		}
	}
	// Validation: if test env enabled, at least one test env selected
	if testEnvEnabled && len(r.Form["test_env_ids"]) == 0 {
		http.Error(w, "启用测试环境时，请至少选择一个测试环境", http.StatusBadRequest)
		return
	}

	product, err := h.productService.CreateProductWithEnv(
		r.FormValue("name"),
		r.FormValue("description"),
		version,
		testEnvEnabled,
	)
	if err != nil {
		log.Printf("[product] create error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save test env associations
	if testEnvEnabled {
		var envIDs []uint
		for _, idStr := range r.Form["test_env_ids"] {
			if id, err := strconv.Atoi(idStr); err == nil {
				envIDs = append(envIDs, uint(id))
			}
		}
		if err := h.productService.SetProductTestEnvs(product.ID, envIDs); err != nil {
			log.Printf("[product] save test envs error: %v", err)
		}
	}

	// Create selected components
	for _, idStr := range compIDs {
		gitURL := r.FormValue("comp_git_url_" + idStr)
		branchFilter := r.FormValue("comp_branch_filter_" + idStr)

		// Look up config item to get name/code
		compID, _ := strconv.Atoi(idStr)
		configItem, err := h.configService.GetByID(uint(compID))
		if err != nil {
			log.Printf("[product] config item %d not found: %v", compID, err)
			continue
		}

		_, err = h.productService.AddComponent(
			product.ID,
			configItem.Name,
			model.ComponentTypeBackend,
			gitURL,
			"",
			"0.0.0.0",
		)
		if err != nil {
			log.Printf("[product] add component %s error: %v", configItem.Name, err)
			continue
		}
		// Update branch filter (AddComponent doesn't support it)
		comps, _ := h.productService.ListComponents(product.ID)
		for i := range comps {
			if comps[i].Name == configItem.Name {
				_ = h.productService.UpdateComponentBranchFilter(comps[i].ID, branchFilter)
				break
			}
		}
	}

	log.Printf("[product] created product=%s with %d components", product.Name, len(compIDs))
	http.Redirect(w, r, "/products", http.StatusSeeOther)
}

func (h *ProductHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	product, err := h.productService.GetProduct(uint(id))
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	configTree, _ := h.configService.GetTree()
	components, _ := h.productService.ListComponents(uint(id))

	// Build selected components map: configItemID -> component
	selectedComps := make(map[uint]*model.Component)
	for i := range components {
		// Match component name to config item code or name
		for _, item := range configTree {
			if item.Name == components[i].Name || item.Code == components[i].Name {
				selectedComps[item.ID] = &components[i]
				break
			}
		}
	}

	// Load available test envs and selected env IDs
	testEnvs, _ := h.configService.ListTestEnvs()
	envIDs, _ := h.productService.ListProductTestEnvIDs(uint(id))
	selectedEnvIDs := make(map[uint]bool)
	for _, eid := range envIDs {
		selectedEnvIDs[eid] = true
	}

	data := map[string]interface{}{
		"Title":          "编辑项目",
		"Username":       middleware.GetUsername(r),
		"Product":        product,
		"ConfigTree":     configTree,
		"SelectedComps":  selectedComps,
		"TestEnvs":       testEnvs,
		"SelectedEnvIDs": selectedEnvIDs,
	}
	_ = h.tmpl["product_form"].ExecuteTemplate(w, "layout", data)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	version := r.FormValue("current_version")
	if err := utils.ValidateVersion(version); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Test environment
	testEnvEnabled := r.FormValue("test_env_enabled") == "true"

	// Validation: at least one component
	compIDs := r.Form["comp_ids"]
	if len(compIDs) == 0 {
		http.Error(w, "请至少选择一个组件", http.StatusBadRequest)
		return
	}
	// Validation: each component must have Git URL
	for _, idStr := range compIDs {
		gitURL := r.FormValue("comp_git_url_" + idStr)
		if strings.TrimSpace(gitURL) == "" {
			compID, _ := strconv.Atoi(idStr)
			configItem, _ := h.configService.GetByID(uint(compID))
			name := idStr
			if configItem != nil {
				name = configItem.Name
			}
			http.Error(w, "组件「"+name+"」必须填写Git仓库地址", http.StatusBadRequest)
			return
		}
	}
	// Validation: if test env enabled, at least one test env selected
	if testEnvEnabled && len(r.Form["test_env_ids"]) == 0 {
		http.Error(w, "启用测试环境时，请至少选择一个测试环境", http.StatusBadRequest)
		return
	}

	err := h.productService.UpdateProductWithEnv(
		uint(id),
		r.FormValue("name"),
		r.FormValue("description"),
		version,
		testEnvEnabled,
	)
	if err != nil {
		log.Printf("[product] update error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save test env associations
	var envIDs []uint
	if testEnvEnabled {
		for _, idStr := range r.Form["test_env_ids"] {
			if eid, err := strconv.Atoi(idStr); err == nil {
				envIDs = append(envIDs, uint(eid))
			}
		}
	}
	if err := h.productService.SetProductTestEnvs(uint(id), envIDs); err != nil {
		log.Printf("[product] save test envs error: %v", err)
	}

	// Sync components: get current components, compare with form selection
	existingComps, _ := h.productService.ListComponents(uint(id))
	existingMap := make(map[string]*model.Component) // name -> component
	for i := range existingComps {
		existingMap[existingComps[i].Name] = &existingComps[i]
	}

	// Process selected components from form
	selectedNames := make(map[string]bool)
	for _, idStr := range compIDs {
		gitURL := r.FormValue("comp_git_url_" + idStr)
		branchFilter := r.FormValue("comp_branch_filter_" + idStr)

		compID, _ := strconv.Atoi(idStr)
		configItem, err := h.configService.GetByID(uint(compID))
		if err != nil {
			log.Printf("[product] config item %d not found: %v", compID, err)
			continue
		}
		selectedNames[configItem.Name] = true

		if existing, ok := existingMap[configItem.Name]; ok {
			// Update existing component
			existing.GitURL = gitURL
			_ = h.productService.UpdateComponent(existing.ID, existing.Name, existing.Type, gitURL, existing.JenkinsJobName, existing.CurrentVersion)
			_ = h.productService.UpdateComponentBranchFilter(existing.ID, branchFilter)
		} else {
			// Add new component
			comp, err := h.productService.AddComponent(
				uint(id),
				configItem.Name,
				model.ComponentTypeBackend,
				gitURL,
				"",
				"0.0.0.0",
			)
			if err != nil {
				log.Printf("[product] add component %s error: %v", configItem.Name, err)
				continue
			}
			_ = h.productService.UpdateComponentBranchFilter(comp.ID, branchFilter)
		}
	}

	// Delete components that are no longer selected
	for name, comp := range existingMap {
		if !selectedNames[name] {
			_ = h.productService.DeleteComponent(comp.ID)
		}
	}

	http.Redirect(w, r, "/products", http.StatusSeeOther)
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if err := h.productService.DeleteProduct(uint(id)); err != nil {
		log.Printf("[product] delete error: %v", err)
	}
	http.Redirect(w, r, "/products", http.StatusSeeOther)
}

func (h *ProductHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	product, err := h.productService.GetProduct(uint(id))
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	components, _ := h.productService.ListComponents(uint(id))
	builds, _ := h.buildService.ListByProductID(uint(id))
	releases, _ := h.releaseService.ListReleases(uint(id))

	// Load associated test envs
	var linkedTestEnvs []model.TestEnv
	envIDs, _ := h.productService.ListProductTestEnvIDs(uint(id))
	if len(envIDs) > 0 {
		allEnvs, _ := h.configService.ListTestEnvs()
		envMap := make(map[uint]model.TestEnv)
		for _, e := range allEnvs {
			envMap[e.ID] = e
		}
		for _, eid := range envIDs {
			if e, ok := envMap[eid]; ok {
				linkedTestEnvs = append(linkedTestEnvs, e)
			}
		}
	}

	data := map[string]interface{}{
		"Title":          product.Name,
		"Username":       middleware.GetUsername(r),
		"Product":        product,
		"Components":     components,
		"Builds":         builds,
		"Releases":       releases,
		"LinkedTestEnvs": linkedTestEnvs,
	}
	_ = h.tmpl["product_detail"].ExecuteTemplate(w, "layout", data)
}

func (h *ProductHandler) AddComponent(w http.ResponseWriter, r *http.Request) {
	productID, _ := strconv.Atoi(r.PathValue("id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	compVersion := r.FormValue("current_version")
	if err := utils.ValidateVersion(compVersion); err != nil {
		http.Error(w, "组件"+err.Error(), http.StatusBadRequest)
		return
	}
	_, err := h.productService.AddComponent(
		uint(productID),
		r.FormValue("name"),
		model.ComponentType(r.FormValue("type")),
		r.FormValue("git_url"),
		r.FormValue("jenkins_job_name"),
		compVersion,
	)
	if err != nil {
		log.Printf("[component] add error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/products/"+r.PathValue("id"), http.StatusSeeOther)
}

func (h *ProductHandler) DeleteComponent(w http.ResponseWriter, r *http.Request) {
	compID, _ := strconv.Atoi(r.PathValue("componentId"))
	projectID := r.PathValue("id")
	if err := h.productService.DeleteComponent(uint(compID)); err != nil {
		log.Printf("[component] delete error: %v", err)
	}
	http.Redirect(w, r, "/products/"+projectID, http.StatusSeeOther)
}
