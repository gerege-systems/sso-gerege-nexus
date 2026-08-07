# Gerege Nexus

**一体化数字运营平台**

**Gerege Nexus** 是一个开源的模块化平台，用于连接公共与私营机构的服务、业务运营、
系统与数据。平台以**蒙古语为默认语言**，并直接对接蒙古国国家数字基础设施
（DAN、E-ID、XYP / ХУР）。

*Nexus* 意为连接点：机构、服务、工作流、系统、用户与数据在此交汇。平台本身不限定
行业——真正决定一次部署形态的，是运行其上的模块。

各模块编译进同一个 Go 二进制文件，由基于 PostgreSQL 的应用商店决定每个租户
启用哪些应用——既保留模块边界，又不引入微服务的网络开销与运维复杂度。

<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <a href="README_AR.md"><img src="assets/icons/flag-ar.png" width="18" height="18" alt=""> العربية</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-zh.png" width="18" height="18" alt=""> <b>中文</b>
  &nbsp;·&nbsp;
  <a href="README_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
  &nbsp;·&nbsp;
  <a href="README_FR.md"><img src="assets/icons/flag-fr.png" width="18" height="18" alt=""> Français</a>
  &nbsp;·&nbsp;
  <a href="README_RU.md"><img src="assets/icons/flag-ru.png" width="18" height="18" alt=""> Русский</a>
  &nbsp;·&nbsp;
  <a href="README_ES.md"><img src="assets/icons/flag-es.png" width="18" height="18" alt=""> Español</a>
</p>

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](../LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8.svg)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-15.1-black.svg)](https://nextjs.org)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](../CONTRIBUTING.md)

---

## 目录

- [作者](#作者)
- [核心能力](#核心能力)
- [业务应用](#业务应用)
- [仓库结构](#仓库结构)
- [快速开始](#快速开始)
- [配置项](#配置项)
- [API 概览](#api-概览)
- [测试与质量门禁](#测试与质量门禁)
- [安全](#安全)
- [文档索引](#文档索引)

---

## 作者

| 贡献者 | 职责 |
| --- | --- |
| **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems)) | 架构与平台核心 |
| **Gemini AI** | 代码生成与文档 |
| **Claude AI** | 代码分析与安全审计 |

---

## 核心能力

### 1. 高性能模块化单体架构

- **编译期 Go 应用模块** —— `contacts`、`products`、`inventory`、`billing`、
  `documents`、`developer_portal` 编译进同一个二进制，进程内直接调用。
- **租户级应用商店** —— 应用权限、菜单与 RBAC 由 PostgreSQL
  （`app_installations`）动态驱动。
- **依赖解析引擎** —— 基于有向无环图（DAG）的递归解析，支持环检测与 semver
  约束校验。
- **目录同步** —— `catalog/apps.json` 是唯一事实来源，`apps` 表在每次启动时据此
  同步。

### 2. 云原生韧性引擎

| 模块 | 用途 |
| --- | --- |
| `resilience/breaker.go` | Google SRE 风格的自适应熔断器 |
| `resilience/loadshedder.go` | 过载时返回 `503` 与 `Retry-After` |
| `resilience/singleflight.go` | 合并重复的并发请求 |
| `resilience/retry.go` | 指数退避重试 |

### 3. 国家数字基础设施集成

- **XYP 国家信息交换系统**（`platform/gerege/xyp.go`）：公民户籍登记
  （`WS100101`）与法人主体核验（`WS100201`）。
- **国家 E-ID 与 DAN**（[`developer.gerege.mn`](https://developer.gerege.mn)、
  [`eidmongolia.mn`](https://eidmongolia.mn)）—— PKI 数字签名、手机 OTP、
  银行 SSO 与人脸生物识别。
- **内置 OAuth2 / OIDC 提供方**
  （`/.well-known/openid-configuration`），为第三方系统签发 client credentials
  令牌。

> **注意。** E-ID、DAN 与 XYP 的 mock 模式仅用于开发环境。当
> `ENVIRONMENT=production` 时会自动关闭，伪造的登记号无法完成认证。

### 4. AI 助手与业务分析

- **AI 助手**（`platform/ai/copilot.go`）—— 连接租户实时数据的意图分类对话。
- **库存需求预测**（`platform/ai/inventory_forecaster.go`）—— 基于历史出入库
  给出安全库存与再订货点建议。

---

## 业务应用

| # | 应用 | ID | 路由 | 说明 |
| --- | --- | --- | --- | --- |
| 1 | Contacts | `io.example.contacts` | `/contacts` | 客户与供应商目录，支持 XYP 自动填充 |
| 2 | Products | `io.example.products` | `/products` | 商品目录、定价与租户级 SKU |
| 3 | Inventory | `io.example.inventory` | `/inventory` | 仓库、库存与出入库流水 |
| 4 | Public Billing & e-Barimt | `io.example.billing` | `/billing` | 开票、10% 增值税与 e-Barimt 税务凭证 |
| 5 | Digital Documents & E-Sign | `io.example.documents` | `/documents` | 文档流转、签名与审批 |
| 6 | Developer Portal & OAuth2 SSO | `io.example.developer_portal` | `/developer/apps` | OAuth2 客户端注册 |

只有当应用在该租户下安装并启用后路由才会开放，否则网关返回 `403 Forbidden`。

---

## 仓库结构

```
backend/
  cmd/api/            HTTP API 服务（含 demo seeder）
  cmd/migrate/        Goose 迁移执行器
  db/migrations/      SQL 迁移脚本
  internal/
    module.go         Go Module 契约
    apps/             业务模块
    platform/         平台核心服务
frontend/             Next.js 15（App Router）Web 客户端
catalog/              应用商店目录与 manifest
deploy/               生产 Dockerfile 与 Nginx 配置
docs/                 文档与翻译
```

---

## 快速开始

### 环境要求

- Go 1.25+
- Node.js 20+
- PostgreSQL 16+（或 Docker Compose）

### 1. Docker Compose

```bash
docker compose up -d
```

迁移由独立的一次性 `migrate` 服务执行，完成后 API 才会启动。

### 2. 手动运行

**后端：**

```bash
cd backend
go mod download
DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable" \
  go run ./cmd/migrate up
go run ./cmd/api
```

**前端：**

```bash
cd frontend
npm ci
npm run dev
```

打开 [http://localhost:3000](http://localhost:3000)。

### 演示账号

| 字段 | 值 |
| --- | --- |
| 邮箱 | `admin@example.com` |
| 密码 | `Password123!` |
| 租户 | `Demo Corporation`（`slug: demo`） |

演示账号仅在非生产环境创建；生产环境下必须显式设置 `SEED_DEMO_DATA=true`。

---

## 配置项

完整列表见 [`.env.example`](../.env.example)。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DATABASE_URL` | localhost | PostgreSQL 连接串 |
| `PORT` | `8080` | API 监听端口 |
| `ENVIRONMENT` | `development` | `production` 启用安全加固默认值 |
| `APP_CATALOG_PATH` | `catalog/apps.json` | 应用商店目录路径 |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | CORS 白名单 |
| `TRUST_PROXY_HEADERS` | `false` | 是否信任 `X-Forwarded-For` |
| `SEED_DEMO_DATA` | 非生产环境默认开启 | 创建演示账号 |
| `SSO_DEFAULT_CLIENT_SECRET` | — | 生产环境必填 |
| `EID_MOCK_MODE` / `DAN_MOCK_MODE` / `XYP_MOCK_MODE` | 非生产环境默认开启 | 国家系统 mock 模式 |

---

## API 概览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health`、`/ready` | 存活与就绪探针 |
| `GET` | `/metrics` | Prometheus 指标 |
| `POST` | `/api/v1/auth/login` | 邮箱密码登录 |
| `POST` | `/api/v1/auth/eid/login` | 国家 E-ID 登录 |
| `POST` | `/api/v1/auth/dan/login` | DAN 网关登录 |
| `POST` | `/api/v1/auth/logout` | 注销会话 |
| `GET` | `/api/v1/menus` | 租户已启用应用的菜单 |
| `GET` | `/api/v1/store/apps` | 应用商店列表 |
| `POST` | `/api/v1/store/apps/{slug}/install` | 安装应用（管理员） |
| `POST` | `/oauth2/token` | OAuth2 client credentials 令牌 |

会话令牌通过 HttpOnly Cookie 或 `Authorization: Bearer <token>` 传递。

---

## 测试与质量门禁

```bash
# 后端单元测试（开启竞态检测）
cd backend && go test -race ./...

# 静态分析
cd backend && go vet ./... && golangci-lint run

# 漏洞扫描
cd backend && govulncheck ./...

# 前端构建
cd frontend && npm run build
```

CI 在每次 push 与 pull request 上运行 lint、测试、前端构建、Docker 镜像构建、
govulncheck 与 gosec。

---

## 安全

- 会话令牌为 256 位随机值，数据库中仅保存其 SHA-256 摘要。
- 密码使用 bcrypt 哈希，登录接口按 IP 限流。
- 安装、启用、停用应用以及注册集成需要租户管理员权限。
- OAuth2 客户端认证采用常量时间比较。

漏洞报告流程见 [`SECURITY.md`](../SECURITY.md)。

---

## 文档索引

| 文档 | 说明 |
| --- | --- |
| [文档中心](README.md) | 全部文档与翻译索引 |
| [架构规范](ARCHITECTURE_SPECIFICATION.md) | 平台分层与设计决策 |
| [模块开发指南](MODULE_AUTHORING_GUIDE.md) | 如何开发新的应用模块 |
| [贡献指南](../CONTRIBUTING.md) | 贡献流程 |
| [安全策略](../SECURITY.md) | 漏洞报告 |
| [行为准则](../CODE_OF_CONDUCT.md) | 社区规范 |
| [变更日志](../CHANGELOG.md) | 版本历史 |

---

## 致谢与灵感来源

1. **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
   （作者 **[@snykk](https://github.com/snykk)**）—— Go REST API 基础架构。
2. **[Odoo](https://github.com/odoo/odoo)** —— 模块化应用商店与依赖模型。
3. **[go-zero](https://github.com/zeromicro/go-zero)** —— 云原生韧性引擎。

---

## 许可证

Copyright (c) 2026 **Gerege Systems Development Team, Gemini AI &
Claude AI**。基于 Apache 2.0 许可证发布，详见 [`LICENSE`](../LICENSE)。

国旗图标来自 [Flaticon](https://www.flaticon.com/)
（[署名](assets/icons/ATTRIBUTION.md)）。
