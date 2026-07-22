package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/service"
)

type ConfigHandler struct {
	tmpl           map[string]*template.Template
	configService  *service.ConfigService
	jenkinsService *service.JenkinsService
	productService *service.ProductService
}

func NewConfigHandler(tmpl map[string]*template.Template, cs *service.ConfigService, js *service.JenkinsService, ps *service.ProductService) *ConfigHandler {
	return &ConfigHandler{tmpl: tmpl, configService: cs, jenkinsService: js, productService: ps}
}

func (h *ConfigHandler) Page(w http.ResponseWriter, r *http.Request) {
	tree, _ := h.configService.GetTree()
	sysCategories := h.configService.GetSysConfigCategories()
	testEnvs, _ := h.configService.ListTestEnvs()

	data := map[string]interface{}{
		"Title":         "配置管理",
		"Username":      middleware.GetUsername(r),
		"Tree":          tree,
		"SysCategories": sysCategories,
		"TestEnvs":      testEnvs,
	}
	_ = h.tmpl["config"].ExecuteTemplate(w, "layout", data)
}

// TreeJSON returns the config tree as JSON (used by build trigger form).
func (h *ConfigHandler) TreeJSON(w http.ResponseWriter, r *http.Request) {
	tree, err := h.configService.GetTree()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Filter by product's components if product_id is specified
	if pidStr := r.URL.Query().Get("product_id"); pidStr != "" {
		pid, _ := strconv.Atoi(pidStr)
		if pid > 0 && h.productService != nil {
			comps, _ := h.productService.ListComponents(uint(pid))
			compMap := make(map[string]model.Component)
			for _, c := range comps {
				compMap[c.Name] = c
			}
			// Build enriched response with git_url and branch_filter
			type enrichedItem struct {
				model.ConfigItem
				GitURL       string `json:"git_url"`
				BranchFilter string `json:"branch_filter"`
			}
			var enriched []enrichedItem
			for _, item := range tree {
				if comp, ok := compMap[item.Name]; ok {
					enriched = append(enriched, enrichedItem{
						ConfigItem:   item,
						GitURL:       comp.GitURL,
						BranchFilter: comp.BranchFilter,
					})
				}
			}
			_ = json.NewEncoder(w).Encode(enriched)
			return
		}
	}

	_ = json.NewEncoder(w).Encode(tree)
}

// JenkinsJobs returns the list of Jenkins job names as JSON.
func (h *ConfigHandler) JenkinsJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.jenkinsService.ListJobs()
	if err != nil {
		log.Printf("[jenkins] list jobs error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error(), "jobs": []string{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"jobs": jobs})
}

func (h *ConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	parentID, _ := strconv.Atoi(r.FormValue("parent_id"))
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	compType := r.FormValue("type")
	_, err := h.configService.Create(
		r.FormValue("name"),
		r.FormValue("code"),
		r.FormValue("description"),
		uint(parentID),
		sortOrder,
		compType,
	)
	if err != nil {
		log.Printf("[config] create error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	anchor := "components"
	if parentID > 0 {
		anchor = "components"
	}
	http.Redirect(w, r, "/config#"+anchor, http.StatusSeeOther)
}

func (h *ConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	compType := r.FormValue("type")
	err := h.configService.Update(
		uint(id),
		r.FormValue("name"),
		r.FormValue("code"),
		r.FormValue("description"),
		sortOrder,
		compType,
	)
	if err != nil {
		log.Printf("[config] update error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/config#components", http.StatusSeeOther)
}

func (h *ConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if err := h.configService.Delete(uint(id)); err != nil {
		log.Printf("[config] delete error: %v", err)
	}
	http.Redirect(w, r, "/config#components", http.StatusSeeOther)
}

// SaveSysConfig saves system configuration for a given category.
func (h *ConfigHandler) SaveSysConfig(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	pairs := make(map[string]string)
	for key, values := range r.Form {
		if key == "" || len(values) == 0 {
			continue
		}
		pairs[key] = values[0]
	}

	// Strip trailing slash from URL values
	for key, val := range pairs {
		if key == "url" {
			pairs[key] = strings.TrimRight(val, "/")
		}
	}

	if err := h.configService.SaveSysConfig(category, pairs); err != nil {
		log.Printf("[config] save sys config error: %v", err)
		http.Error(w, "保存失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[config] saved sys config category=%s", category)
	http.Redirect(w, r, "/config?saved="+category+"#sys_"+category, http.StatusSeeOther)
}

// TestAPI tests connectivity for a given system config category (jenkins or gitea).
func (h *ConfigHandler) TestAPI(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	w.Header().Set("Content-Type", "application/json")

	var testErr error
	switch category {
	case "jenkins":
		testErr = h.jenkinsService.TestConnection()
	case "gitea":
		testErr = h.testGiteaConnection()
	default:
		http.Error(w, "unknown category", http.StatusBadRequest)
		return
	}

	// Persist test result to DB
	status := "ok"
	if testErr != nil {
		status = "err"
	}
	_ = h.configService.SaveTestResult(category, status)

	resp := map[string]interface{}{"ok": testErr == nil}
	if testErr != nil {
		resp["error"] = testErr.Error()
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// testGiteaConnection tests Gitea API connectivity using stored config.
func (h *ConfigHandler) testGiteaConnection() error {
	giteaURL, _ := h.configService.GetSysConfigValue("gitea", "url")
	giteaToken, _ := h.configService.GetSysConfigValue("gitea", "token")
	if giteaURL == "" {
		return fmt.Errorf("Gitea 地址未配置")
	}
	giteaURL = strings.TrimRight(giteaURL, "/")

	apiURL := giteaURL + "/api/v1/user"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+giteaToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("认证失败 (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("异常状态码: %d", resp.StatusCode)
	}
	return nil
}

// GiteaRepos proxies Gitea repo search API for the product form dropdown.
func (h *ConfigHandler) GiteaRepos(w http.ResponseWriter, r *http.Request) {
	giteaURL, _ := h.configService.GetSysConfigValue("gitea", "url")
	giteaToken, _ := h.configService.GetSysConfigValue("gitea", "token")
	if giteaURL == "" {
		http.Error(w, "Gitea 未配置", http.StatusBadRequest)
		return
	}
	giteaURL = strings.TrimRight(giteaURL, "/")

	q := r.URL.Query().Get("q")
	apiURL := giteaURL + "/api/v1/repos/search?limit=50&sort=updated"
	if q != "" {
		apiURL += "&q=" + url.QueryEscape(q)
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "token "+giteaToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "连接 Gitea 失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	var result struct {
		Data []struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
			CloneURL string `json:"clone_url"`
			SSHURL   string `json:"ssh_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, "解析响应失败", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result.Data)
}

// GiteaBranches fetches branches from a Gitea repo given a git clone URL.
func (h *ConfigHandler) GiteaBranches(w http.ResponseWriter, r *http.Request) {
	giteaURL, _ := h.configService.GetSysConfigValue("gitea", "url")
	giteaToken, _ := h.configService.GetSysConfigValue("gitea", "token")
	if giteaURL == "" {
		http.Error(w, "Gitea 未配置", http.StatusBadRequest)
		return
	}
	giteaURL = strings.TrimRight(giteaURL, "/")

	gitURL := r.URL.Query().Get("git_url")
	if gitURL == "" {
		http.Error(w, "缺少 git_url 参数", http.StatusBadRequest)
		return
	}

	// Extract owner/repo from git URL (e.g. http://gitea/owner/repo.git -> owner/repo)
	ownerRepo := extractOwnerRepo(gitURL, giteaURL)
	if ownerRepo == "" {
		http.Error(w, "无法解析仓库地址", http.StatusBadRequest)
		return
	}

	apiURL := giteaURL + "/api/v1/repos/" + ownerRepo + "/branches?limit=50"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "token "+giteaToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "连接 Gitea 失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	var branches []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		http.Error(w, "解析响应失败", http.StatusBadGateway)
		return
	}

	names := make([]string, 0, len(branches))
	for _, b := range branches {
		names = append(names, b.Name)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(names)
}

func extractOwnerRepo(gitURL, giteaURL string) string {
	// Remove .git suffix
	u := strings.TrimSuffix(gitURL, ".git")
	giteaURL = strings.TrimRight(giteaURL, "/")
	// Try to strip gitea base URL
	if strings.HasPrefix(u, giteaURL) {
		return strings.TrimPrefix(u, giteaURL+"/")
	}
	// Try to extract from http(s)://host/owner/repo
	for _, prefix := range []string{"https://", "http://"} {
		if idx := strings.Index(u, prefix); idx == 0 {
			path := strings.TrimPrefix(u, prefix)
			if slash := strings.Index(path, "/"); slash > 0 {
				return path[slash+1:]
			}
		}
	}
	return ""
}

// TestEnv CRUD

func (h *ConfigHandler) CreateTestEnv(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	_, err := h.configService.CreateTestEnv(
		r.FormValue("name"),
		r.FormValue("url"),
	)
	if err != nil {
		log.Printf("[config] create test env error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/config#testenv", http.StatusSeeOther)
}

func (h *ConfigHandler) UpdateTestEnv(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	err := h.configService.UpdateTestEnv(
		uint(id),
		r.FormValue("name"),
		r.FormValue("url"),
	)
	if err != nil {
		log.Printf("[config] update test env error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/config#testenv", http.StatusSeeOther)
}

func (h *ConfigHandler) DeleteTestEnv(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if err := h.configService.DeleteTestEnv(uint(id)); err != nil {
		log.Printf("[config] delete test env error: %v", err)
	}
	http.Redirect(w, r, "/config#testenv", http.StatusSeeOther)
}
