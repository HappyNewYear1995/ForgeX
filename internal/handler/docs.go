package handler

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
)

// DocItem represents a document entry
type DocItem struct {
	Title   string
	Date    string
	Content template.HTML
}

// DocsHandler handles the documentation page
type DocsHandler struct {
	tmpl     map[string]*template.Template
	faqStore *store.FAQStore
}

func NewDocsHandler(tmpl map[string]*template.Template) *DocsHandler {
	return &DocsHandler{
		tmpl:     tmpl,
		faqStore: store.NewFAQStore(),
	}
}

func (h *DocsHandler) Page(w http.ResponseWriter, r *http.Request) {
	specDocs := []DocItem{
		{
			Title: "版本号命名规范",
			Date:  "2026-07-17",
			Content: template.HTML(`<h3>版本号格式</h3>
<p>版本号采用 <code>AA.BB.CC.DD</code> 四段式格式：</p>
<ul>
<li><strong>AA</strong>：大版本号 — 产品重大架构变更时递增</li>
<li><strong>BB</strong>：ECR版本号 — 工程变更请求版本</li>
<li><strong>CC</strong>：TCN版本号 — 技术变更通知版本</li>
<li><strong>DD</strong>：小版本号 — 每次构建自动递增</li>
</ul>
<p>默认版本号为 <code>0.0.0.0</code>，构建时若不指定版本，系统将自动对第四位（DD）加1。</p>
<h3>组件版本</h3>
<p>每个组件也有独立的版本号，同样采用四段式格式。每次构建参与时，组件版本自动递增。</p>`),
		},
		{
			Title: "Git 仓库与分支管理规范",
			Date:  "2026-07-17",
			Content: template.HTML(`<h3>Git 仓库配置</h3>
<p>每个组件需配置对应的 Git 仓库地址，用于构建时拉取代码。</p>
<ul>
<li>仓库地址格式：<code>http://gitea-host/owner/repo.git</code></li>
<li>分支过滤支持通配符，如 <code>release/*</code>，多个用逗号分隔</li>
</ul>
</ul>`),
		},
	}

	guideDocs := []DocItem{
		{
			Title: "新建项目操作指南",
			Date:  "2026-07-17",
			Content: template.HTML(`<h3>操作步骤</h3>
<ol>
<li>进入「项目」页面，点击「新建项目」</li>
<li>填写项目名称、描述、初始版本号</li>
<li>勾选关联组件，每个组件须填写 Git 仓库地址</li>
<li>如需测试环境支持，勾选「支持测试环境」并选择关联的测试环境</li>
<li>点击「保存」完成创建</li>
</ol>
<h3>注意事项</h3>
<ul>
<li>至少选择一个组件</li>
<li>每个组件必须填写 Git 仓库地址</li>
<li>启用测试环境时至少选择一个测试环境</li>
</ul>`),
		},
		{
			Title: "触发构建操作指南",
			Date:  "2026-07-17",
			Content: template.HTML(`<h3>操作步骤</h3>
<ol>
<li>在项目列表中，点击对应项目的「构建」按钮</li>
<li>选择构建类型：升级包（默认）或整包</li>
<li>若不勾选「指定版本」，系统将自动递增版本号（DD+1）</li>
<li>勾选需要构建的组件及模块</li>
<li>勾选组件后自动加载该组件 Git 仓库的分支列表，选择目标分支</li>
<li>如需自动同步测试环境，保持「自动同步测试环境」勾选</li>
<li>点击「触发构建」启动 Jenkins 构建流水线</li>
</ol>
<h3>构建完成后</h3>
<p>构建成功后，Jenkins 通过回调 API 自动上传制品到平台。可在构建详情中查看和下载制品。</p>`),
		},
		{
			Title: "配置管理操作指南",
			Date:  "2026-07-17",
			Content: template.HTML(`<h3>组件管理</h3>
<p>在「配置管理」页面可以管理组件和模块的树形结构：</p>
<ul>
<li>点击「新建组件」添加新组件</li>
<li>在组件下方添加模块</li>
<li>点击「编辑」按钮进行行内编辑</li>
</ul>
<h3>系统配置</h3>
<p>在系统配置区域可设置：</p>
<ul>
<li><strong>Jenkins</strong>：地址、用户名、Token</li>
<li><strong>Gitea</strong>：地址、Token（用于仓库和分支查询）</li>
</ul>`),
		},
		{
			Title: "测试环境管理",
			Date:  "2026-07-17",
			Content: template.HTML(`<h3>测试环境配置</h3>
<ol>
<li>在「配置管理」页面的测试环境区域，点击「新建测试环境」</li>
<li>填写环境名称和 URL 地址</li>
<li>在项目编辑页面勾选需要关联的测试环境</li>
</ol>
<h3>脚本录制与执行</h3>
<ol>
<li>在测试环境列表中，点击脚本列的单元格打开脚本弹窗</li>
<li>点击「录制」按钮，通过 Playwright 录制升级操作脚本</li>
<li>录制完成后点击「保存」保存脚本</li>
<li>构建完成后可手动点击「执行」运行脚本，或勾选「自动同步测试环境」自动执行</li>
</ol>`),
		},
	}

	data := map[string]interface{}{
		"Title":     "文档中心",
		"Username":  middleware.GetUsername(r),
		"SpecDocs":  specDocs,
		"GuideDocs": guideDocs,
	}

	// Load FAQs from database
	faqs, _ := h.faqStore.ListAll()
	data["FAQs"] = faqs

	_ = h.tmpl["docs"].ExecuteTemplate(w, "layout", data)
}

// CreateFAQ handles POST /docs/faq/new
func (h *DocsHandler) CreateFAQ(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	question := r.FormValue("question")
	answer := r.FormValue("answer")
	if question == "" {
		http.Error(w, "问题不能为空", http.StatusBadRequest)
		return
	}

	faq := &model.FAQ{
		Question:  question,
		Answer:    answer,
		CreatedBy: middleware.GetUsername(r),
	}
	if err := h.faqStore.Create(faq); err != nil {
		log.Printf("[docs] create FAQ error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/docs", http.StatusSeeOther)
}

// DeleteFAQ handles POST /docs/faq/{id}/delete
func (h *DocsHandler) DeleteFAQ(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if id == 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.faqStore.Delete(uint(id)); err != nil {
		log.Printf("[docs] delete FAQ error: %v", err)
	}
	http.Redirect(w, r, "/docs", http.StatusSeeOther)
}

// ListFAQsJSON returns FAQs as JSON for AJAX
func (h *DocsHandler) ListFAQsJSON(w http.ResponseWriter, r *http.Request) {
	faqs, err := h.faqStore.ListAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(faqs)
}
