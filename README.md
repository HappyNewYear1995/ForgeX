# ForgeX

A lightweight release management platform built with Go, integrating Jenkins build triggering, Gitea branch management, Playwright automation testing, and artifact management.

**ForgeX** — *Forge your releases with confidence.*

## Features

- **项目管理** — 创建/编辑/删除项目，关联组件和测试环境
- **配置管理** — 组件/模块树形结构管理，行内编辑，系统配置（Jenkins/Gitea）
- **构建触发** — 选择组件与分支触发 Jenkins 构建，支持整包/升级包类型
- **自动版本管理** — 版本号 `AA.BB.CC.DD` 四段式，构建时自动递增第四位
- **Release 管理** — 构建自动创建 Release 记录，关联组件版本
- **测试环境管理** — 配置测试环境，录制/执行 Playwright 自动化脚本
- **自动同步** — 构建完成后自动在测试环境执行升级脚本
- **Jenkins 回调** — 构建完成后 Jenkins 通过回调 API 上传制品
- **文档中心** — 内置规范文档和操作文档

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
│   │   ├── build.go           # 构建触发
│   │   ├── release.go         # Release 查看
│   │   ├── config.go          # 配置管理、Gitea API
│   │   ├── callback.go        # Jenkins 回调 API
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
│   │   ├── build_service.go   # 构建逻辑 + 版本自动递增
│   │   ├── release_service.go # Release 创建 + 组件版本递增
│   │   ├── config_service.go
│   │   ├── jenkins_service.go # Jenkins API 调用 + 构建状态轮询
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
    └── artifacts/             # Jenkins 回调上传的制品
```

## 快速开始

### 前置要求

- Go 1.26+
- Node.js + Playwright（用于自动化测试功能）

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
go build -o releaseManager.exe .
./releaseManager.exe
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

### 构建流程

1. 在项目列表点击「构建」按钮
2. 选择构建类型（升级包/整包）、勾选组件和模块
3. 组件自动从 Gitea 加载可用分支（支持 `branch_filter` 过滤）
4. 可选择指定版本号，不指定则自动递增（DD+1）
5. 触发 Jenkins Pipeline 构建
6. 构建完成后 Jenkins 通过回调 API 上传制品
7. 如启用「自动同步测试环境」，自动执行 Playwright 脚本

### 版本号规则

格式：`AA.BB.CC.DD`
- **AA** — 大版本号（架构变更）
- **BB** — ECR 版本号
- **CC** — TCN 版本号
- **DD** — 小版本号（构建自动递增）

### Jenkins 回调 API

构建完成后，Jenkins Pipeline 通过以下回调地址上传制品和上报状态：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/callback/build/{token}` | POST | 上传制品文件（multipart） |
| `/api/callback/build/{token}/status` | POST | 上报构建状态 |
| `/api/callback/build/{token}/artifacts` | GET | 查询制品列表 |
| `/api/callback/build/{token}/artifacts/{id}` | GET | 下载制品 |

回调地址和 Token 在触发构建时自动生成并传递给 Jenkins。

## 配置说明

### 系统配置（通过 Web 界面）

在「配置管理」页面可在线配置：

- **Jenkins** — 地址、用户名、API Token，支持连接测试
- **Gitea** — 地址、API Token，用于查询仓库和分支

### 分支过滤

组件可配置 `branch_filter` 字段，支持通配符匹配：
- `main` — 精确匹配 main 分支
- `release/*` — 匹配所有 release 开头的分支
- 多个规则用逗号分隔：`main,release/*,hotfix/*`

## 开发

```bash
# 编译
go build -o releaseManager.exe .

# 运行
./releaseManager.exe

# 修改模板/静态文件后需重新编译（使用 go:embed）
```

## License

Internal use only.
