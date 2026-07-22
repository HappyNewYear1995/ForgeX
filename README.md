# ForgeX

A lightweight release management platform built with Go, integrating Jenkins build triggering, Gitea branch management, Playwright automation testing, and artifact management.

**ForgeX** — *Forge your releases with confidence.*

## Features

- **项目管理** — 创建/编辑/删除项目，关联组件和测试环境，支持 Jenkins 任务双模式绑定（统一/独立）
- **配置管理** — 组件/模块树形结构管理，行内编辑，系统配置（Jenkins/Gitea）
- **构建触发** — 支持升级包/整包两种类型，双模式参数传递，自动从 Gitea 获取分支和 Git Commit
- **自动版本管理** — 版本号 `AA.BB.CC.DD` 四段式，构建时自动递增第四位
- **Release 管理** — 构建自动创建 Release 记录，关联组件版本，支持版本发布记录删除
- **制品打包** — 多组件构建完成后自动聚合制品，打包为升级包或整包供下载
- **版本包下载** — 版本发布记录提供「更新包」和「整包」下载按钮，未构建类型自动置灰
- **测试环境管理** — 配置测试环境，录制/执行 Playwright 自动化脚本
- **自动同步** — 所有组件回调完成后自动在测试环境执行升级脚本
- **Jenkins 回调** — 支持 JSON 回调自动下载制品，使用内网 IP 确保回调可达
- **Manifest** — 构建时从 Gitea API 获取最新 Git Commit，生成完整版本清单
- **文档中心** — 内置规范文档、操作文档（含 Jenkinsfile 示例）和 FAQ

## 技术栈

| 层次 | 技术 |
|------|------|
| 后端框架 | Go 1.26 + Chi v5 |
| 数据库 | SQLite（通过 GORM） |
| 模板引擎 | Go html/template + `//go:embed` |
| 前端 | 原生 HTML/CSS/JS |
| 自动化测试 | Playwright |
| 认证 | bcrypt + cookie session |

## 项目结构

```
jenkinsAgent/
├── main.go                    # 入口：模板解析、服务初始化、路由注册
├── config/
│   └── config.go              # YAML 配置加载
├── config.yaml                # 运行时配置（不入库）
├── internal/
│   ├── handler/               # HTTP Handler 层
│   │   ├── auth.go            # 登录/登出
│   │   ├── product.go         # 项目 CRUD
│   │   ├── build.go           # 构建触发、日志查看、制品下载、删除
│   │   ├── release.go         # Release 查看、Manifest、下载、删除
│   │   ├── config.go          # 配置管理、Gitea/Jenkins API
│   │   ├── callback.go        # Jenkins 回调 API（JSON + Multipart）
│   │   ├── test_env.go        # 测试环境脚本录制/执行
│   │   ├── docs.go            # 文档中心
│   │   └── dashboard.go       # 仪表盘
│   ├── middleware/
│   │   └── auth.go            # 认证中间件
│   ├── model/                 # GORM 数据模型
│   │   ├── product.go         # 项目
│   │   ├── component.go       # 组件
│   │   ├── build.go           # 构建记录
│   │   ├── release.go         # Release
│   │   ├── release_component.go
│   │   ├── artifact.go        # 制品
│   │   ├── config_item.go     # 配置项（组件/模块树）
│   │   ├── sys_config.go      # 系统配置
│   │   ├── test_env.go        # 测试环境
│   │   ├── test_env_script.go # 测试脚本
│   │   └── user.go            # 用户
│   ├── service/               # 业务逻辑层
│   │   ├── product_service.go
│   │   ├── build_service.go   # 构建逻辑 + 版本自动递增 + 双模式触发
│   │   ├── release_service.go # Release 创建 + 组件版本递增
│   │   ├── config_service.go
│   │   ├── jenkins_service.go # Jenkins API 调用 + 构建状态轮询
│   │   ├── gitea_service.go   # Gitea API（Git Commit 查询）
│   │   ├── package_service.go # 制品聚合打包
│   │   └── playwright_service.go # 脚本录制与执行
│   ├── store/                 # 数据访问层（GORM CRUD）
│   └── utils/
│       └── version.go         # 版本号工具函数
├── static/
│   ├── app.js                 # 前端交互逻辑
│   └── style.css              # 全局样式
├── templates/                 # HTML 模板
│   ├── layout.html            # 公共布局
│   ├── product_list.html
│   ├── product_form.html
│   ├── product_detail.html
│   ├── config.html
│   ├── build_list.html
│   ├── release_list.html
│   ├── release_detail.html
│   ├── docs.html
│   └── login.html
└── data/                      # 运行时数据（不入库）
    ├── platform.db            # SQLite 数据库
    ├── artifacts/             # Jenkins 回调上传的制品
    └── packages/              # 聚合打包的升级包/整包
```

## 快速开始

### 前置要求

- Go 1.26+
- Node.js + Playwright（用于自动化测试功能）

> **ℹ️ Playwright 自动安装**：首次使用脚本录制功能时，系统会自动执行 `npx playwright install chromium` 安装浏览器依赖，无需手动安装。如自动安装失败，请手动执行该命令。

### 安装与运行

1. **克隆项目**

```bash
git clone <repo-url>
cd jenkinsAgent
```

2. **创建配置文件** `config.yaml`

```yaml
server:
  port: 19090
  secret_key: "your-secret-key"

database:
  path: "./data/platform.db"

auth:
  admin_user: "admin"
  admin_password: "admin123"

jenkins:
  url: "http://your-jenkins-host:8080"
  user: "jenkins-user"
  token: "jenkins-api-token"
```

3. **编译运行**

```bash
go build -o forgex.exe .
./forgex.exe
```

4. **访问**

浏览器打开 `http://localhost:19090`，使用 `config.yaml` 中配置的管理员账号登录。

### 安装 Playwright（可选）

如需使用自动化测试功能：

```bash
npm install playwright
npx playwright install chromium
```

## 核心功能说明

### Jenkins 任务绑定模式

项目支持两种 Jenkins 任务绑定模式（在项目编辑页面选择）：

| 模式 | 说明 | 参数传递方式 |
|------|------|------------|
| **统一任务** | 整个项目绑定一个 Jenkins Job | 每个组件传递 `{code}_REPO_URL`、`{code}_BRANCH`、`{code}_CALLBACK_URL` |
| **独立任务** | 每个组件各自绑定 Jenkins Job | 每个组件传递 `REPO_URL`、`BRANCH`、`CALLBACK_URL` |

### 构建流程

1. 在项目列表点击「构建」按钮
2. 选择构建类型：
   - **升级包**：勾选需要构建的组件和分支（仅触发选中的组件任务）
   - **整包**：自动包含所有组件（无需选择，触发所有组件任务）
3. 组件自动从 Gitea 加载可用分支（支持 `branch_filter` 过滤）
4. 可选择指定版本号，不指定则自动递增（DD+1）
5. 触发 Jenkins Pipeline 构建，同时从 Gitea API 获取各组件最新 Git Commit 写入 Manifest
6. 构建完成后 Jenkins 通过 JSON 回调 API 上报状态，系统自动下载制品
7. **所有组件回调完成后**，系统聚合制品并打包为升级包/整包，然后触发 Playwright 脚本
8. 在版本发布记录中下载对应的更新包或整包

### 版本号规则

格式：`AA.BB.CC.DD`
- **AA** — 大版本号（架构变更）
- **BB** — ECR 版本号
- **CC** — TCN 版本号
- **DD** — 小版本号（构建自动递增）

### Jenkins 回调 API

构建完成后，Jenkins Pipeline 通过以下回调地址上报状态和通知制品：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/callback/build/{token}` | POST | JSON 回调（状态+制品下载地址，系统自动下载） |
| `/api/callback/build/{token}` | POST | Multipart 上传（兼容旧模式） |
| `/api/callback/build/{token}/status` | POST | 上报构建状态 |
| `/api/callback/build/{token}/artifacts` | GET | 查询制品列表 |
| `/api/callback/build/{token}/artifacts/{id}` | GET | 下载制品 |

回调地址使用本机内网 IP 自动生成，确保 Jenkins 可访问。回调地址和 Token 在触发构建时自动生成并传递给 Jenkins。

### 制品打包与下载

- **聚合打包**：多组件构建场景下，系统等待所有组件回调成功后，将各组件制品打包为一个 zip 文件
- **下载入口**：版本发布记录提供「更新包」和「整包」两个下载按钮
- **置灰逻辑**：若某个构建类型未触发或未成功，对应按钮自动置灰不可点击
- **打包位置**：`data/packages/release_{id}_{type}.zip`

### Manifest

构建触发时，系统通过 Gitea API 获取各组件分支的最新 Git Commit，写入 ReleaseComponent 记录。所有组件回调完成后，自动生成 Manifest JSON，包含：

- 产品信息（版本、日期、构建环境、描述）
- 各组件信息（名称、版本、分支、Git Commit、制品文件名、构建状态）

## 配置说明

### 系统配置（通过 Web 界面）

在「配置管理」页面可在线配置：

- **Jenkins** — 地址、用户名、API Token，支持连接测试
- **Gitea** — 地址、API Token，用于查询仓库、分支和 Git Commit

### 分支过滤

组件可配置 `branch_filter` 字段，支持通配符匹配：
- `main` — 精确匹配 main 分支
- `release/*` — 匹配所有 release 开头的分支
- 多个规则用逗号分隔：`main,release/*,hotfix/*`

## 开发

```bash
# 编译
go build -o forgex.exe .

# 运行
./forgex.exe

# 修改模板/静态文件后需重新编译（使用 go:embed）
```

## License

Internal use only.
