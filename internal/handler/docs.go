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
		{
			Title: "Jenkinsfile 示例",
			Date:  "2026-07-20",
			Content: template.HTML(`<h3>概述</h3>
<p>根据项目的 Jenkins 任务绑定模式，Jenkinsfile 分为两种：</p>
<ul>
<li><strong>统一任务模式</strong>：整个项目共用一个 Jenkins Pipeline，参数为各组件的 <code>{组件code}_REPO_URL</code>、<code>{组件code}_BRANCH</code>、<code>{组件code}_CALLBACK_URL</code>、<code>{组件code}_COMPONENT</code>、<code>{组件code}_MODULE_LIST</code></li>
<li><strong>独立任务模式</strong>：每个组件独立绑定 Jenkins Pipeline，参数为标准 <code>REPO_URL</code>、<code>BRANCH</code>、<code>CALLBACK_URL</code>、<code>COMPONENT</code>、<code>MODULE_LIST</code></li>
</ul>

<h3>统一任务模式 Jenkinsfile 示例</h3>
<p>适用于整个项目绑定一个 Jenkins 任务的场景。系统会为每个参与的组件传递 <code>{组件code}_REPO_URL</code>、<code>{组件code}_BRANCH</code>、<code>{组件code}_CALLBACK_URL</code>、<code>{组件code}_COMPONENT</code>、<code>{组件code}_MODULE_LIST</code> 参数。</p>
<pre><code>pipeline {
    agent any

    parameters {
        // 组件 acis-svr 的参数
        string(name: 'acis-svr_REPO_URL', defaultValue: '', description: 'acis-svr 仓库地址')
        string(name: 'acis-svr_BRANCH', defaultValue: 'main', description: 'acis-svr 分支')
        string(name: 'acis-svr_CALLBACK_URL', defaultValue: '', description: 'acis-svr 回调地址')
        string(name: 'acis-svr_COMPONENT', defaultValue: '', description: 'acis-svr 组件编码')
        string(name: 'acis-svr_MODULE_LIST', defaultValue: '', description: 'acis-svr 模块列表，逗号分隔')

        // 组件 acis-web 的参数
        string(name: 'acis-web_REPO_URL', defaultValue: '', description: 'acis-web 仓库地址')
        string(name: 'acis-web_BRANCH', defaultValue: 'main', description: 'acis-web 分支')
        string(name: 'acis-web_CALLBACK_URL', defaultValue: '', description: 'acis-web 回调地址')
        string(name: 'acis-web_COMPONENT', defaultValue: '', description: 'acis-web 组件编码')
        string(name: 'acis-web_MODULE_LIST', defaultValue: '', description: 'acis-web 模块列表，逗号分隔')

        // 公共参数
        string(name: 'CALLBACK_TOKEN', defaultValue: '', description: '回调Token')
    }

    environment {
        VERSION_GIT_CREDENTIALS_ID = 'git-credentials-id'
        GIT_CREDENTIALS_ID         = 'git-credentials-id'
        NEXUS_REPO                 = 'nexus-repo-url'
    }

    tools {
        maven 'apache-maven-3.9.6'
        jdk   'jdk8'
    }

    stages {
        stage('构建 acis-svr') {
            when { expression { params['acis-svr_REPO_URL'] != '' } }
            steps {
                script {
                    def repoUrl  = params['acis-svr_REPO_URL']
                    def branch   = params['acis-svr_BRANCH']
                    def callback = params['acis-svr_CALLBACK_URL']

                    checkout([$class: 'GitSCM',
                        branches: [[name: branch]],
                        userRemoteConfigs: [[url: repoUrl, credentialsId: env.GIT_CREDENTIALS_ID]]
                    ])

                    sh 'mvn clean package -DskipTests'

                    // 回调上传制品
                    sh """
                        curl -X POST \\n                            -H 'Content-Type: application/json' \\n                            -d '{"status":"SUCCESS","downloadUrl":"'${env.BUILD_URL}'artifact/target/app.jar","artifactName":"app.jar","branch":"'${branch}'"}' \\n                            ${callback}
                    """
                }
            }
        }

        stage('构建 acis-web') {
            when { expression { params['acis-web_REPO_URL'] != '' } }
            steps {
                script {
                    def repoUrl  = params['acis-web_REPO_URL']
                    def branch   = params['acis-web_BRANCH']
                    def callback = params['acis-web_CALLBACK_URL']

                    checkout([$class: 'GitSCM',
                        branches: [[name: branch]],
                        userRemoteConfigs: [[url: repoUrl, credentialsId: env.GIT_CREDENTIALS_ID]]
                    ])

                    tool name: 'node_18_16_1', type: 'jenkins.plugins.nodejs.tools.NodeJSInstallation'
                    sh 'npm install &amp;&amp; npm run build'

                    // 回调上传制品
                    sh """
                        curl -X POST \\n                            -H 'Content-Type: application/json' \\n                            -d '{"status":"SUCCESS","downloadUrl":"'${env.BUILD_URL}'artifact/dist/app.zip","artifactName":"app.zip","branch":"'${branch}'"}' \\n                            ${callback}
                    """
                }
            }
        }
    }

    post {
        failure {
            echo '构建失败，回调通知系统'
            // 回调失败状态
            script {
                def failedCallback = params['acis-svr_CALLBACK_URL'] ?: params['acis-web_CALLBACK_URL']
                if (failedCallback) {
                    sh """
                        curl -X POST \\n                            -H 'Content-Type: application/json' \\n                            -d '{"status":"FAILURE","errorMessage":"Jenkins 构建失败"}' \\n                            ${failedCallback}
                    """
                }
            }
        }
    }
}</code></pre>

<h3>独立任务模式 Jenkinsfile 示例</h3>
<p>适用于每个组件独立绑定 Jenkins 任务的场景。系统为每个组件传递标准的 <code>REPO_URL</code>、<code>BRANCH</code>、<code>CALLBACK_URL</code>、<code>COMPONENT</code>、<code>MODULE_LIST</code> 参数。</p>
<pre><code>pipeline {
    agent any

    parameters {
        string(name: 'REPO_URL',       defaultValue: '', description: '组件仓库地址')
        string(name: 'BRANCH',         defaultValue: 'main', description: '构建分支')
        string(name: 'CALLBACK_URL',   defaultValue: '', description: '回调地址')
        string(name: 'CALLBACK_TOKEN', defaultValue: '', description: '回调Token')
        string(name: 'COMPONENT',      defaultValue: '', description: '组件编码')
        string(name: 'MODULE_LIST',    defaultValue: '', description: '模块列表，逗号分隔')
    }

    environment {
        VERSION_GIT_CREDENTIALS_ID = 'git-credentials-id'
        GIT_CREDENTIALS_ID         = 'git-credentials-id'
        NEXUS_REPO                 = 'nexus-repo-url'
    }

    tools {
        maven 'apache-maven-3.9.6'
        jdk   'jdk8'
    }

    stages {
        stage('拉取代码') {
            steps {
                checkout([$class: 'GitSCM',
                    branches: [[name: params.BRANCH]],
                    userRemoteConfigs: [[url: params.REPO_URL, credentialsId: env.GIT_CREDENTIALS_ID]]
                ])
            }
        }

        stage('构建') {
            steps {
                sh 'mvn clean package -DskipTests'
            }
        }

        stage('回调制品') {
            steps {
                script {
                    def artifactPath = 'target/app.jar'
                    def artifactName = 'app.jar'
                    def downloadUrl  = "${env.BUILD_URL}artifact/${artifactPath}"

                    sh """
                        curl -X POST \\n                            -H 'Content-Type: application/json' \\n                            -d '{
                                "status": "SUCCESS",
                                "downloadUrl": "${downloadUrl}",
                                "artifactName": "${artifactName}",
                                "branch": "${params.BRANCH}",
                                "buildNumber": ${env.BUILD_NUMBER}
                            }' \\n                            ${params.CALLBACK_URL}
                    """
                }
            }
        }
    }

    post {
        failure {
            echo '构建失败，回调通知系统'
            script {
                if (params.CALLBACK_URL) {
                    sh """
                        curl -X POST \\n                            -H 'Content-Type: application/json' \\n                            -d '{
                                "status": "FAILURE",
                                "errorMessage": "Jenkins 构建失败",
                                "buildNumber": ${env.BUILD_NUMBER}
                            }' \\n                            ${params.CALLBACK_URL}
                    """
                }
            }
        }
    }
}</code></pre>

<h3>回调接口说明</h3>
<p>Jenkins 构建完成后需通过 HTTP POST 回调系统接口，地址格式为：</p>
<pre><code>POST {CALLBACK_URL}</code></pre>
<p>请求体（JSON）：</p>
<table class="table">
<tr><th>字段</th><th>类型</th><th>说明</th></tr>
<tr><td><code>status</code></td><td>string</td><td><code>SUCCESS</code> 或 <code>FAILURE</code></td></tr>
<tr><td><code>downloadUrl</code></td><td>string</td><td>制品下载地址（成功时必填）</td></tr>
<tr><td><code>artifactName</code></td><td>string</td><td>制品文件名</td></tr>
<tr><td><code>branch</code></td><td>string</td><td>构建分支</td></tr>
<tr><td><code>buildNumber</code></td><td>int</td><td>Jenkins 构建号</td></tr>
<tr><td><code>errorMessage</code></td><td>string</td><td>错误信息（失败时填写）</td></tr>
</table>
<p>系统收到成功回调后会自动下载制品，待所有组件回调完成后打包为最终升级包/整包，并触发 Playwright 测试脚本。</p>`),
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
