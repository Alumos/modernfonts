# 腾讯文档字体解析

项目只负责两件事：解析腾讯文档中的字体记录，以及按需解析蓝奏云下载直链。

## Docker 启动

```bash
docker compose up -d --build
```

访问 `http://localhost:5176`。现在前端和 API 在同一个容器、同一个端口中。

首次登录使用 `admin` / `123456`，系统会要求立即修改管理员凭据。管理员可以在后台创建普通用户。

文档链接、腾讯文档凭据和蓝奏云统一密码不会写入代码或镜像。部署后登录管理员账号，在“站点设置”中填写腾讯文档 OpenAPI 凭据和蓝奏云密码，再在“文档源”中添加自己的腾讯文档链接。

私有腾讯文档需要填写开放平台提供的应用 ID、Access Token 和 Open ID。Access Token 会使用本实例的会话密钥加密后保存在 SQLite 中，不会通过管理接口回传。当前版本支持手动更新 Token；如果没有配置 OpenAPI 凭据，则继续使用原有的公开文档解析方式。

## 当前功能

- 腾讯文档私有文档 OpenAPI 接入，以及文档源的新增、启停、手动解析和定时解析
- 字体名称、蓝奏云链接、访问码和发现时间入库
- 字体名称/链接搜索、文档顺序展示和字体筛选
- 登录用户按需解析蓝奏云单文件、文件夹、密码链接和分页内容
- 直链解析结果短时内存缓存与并发解析
- 管理员、普通用户和站点设置
- 解析日志查看、按文档源筛选和清空

字体文件不会保存到服务器。数据库只保存字体元数据；蓝奏云直链按需生成，不写入数据库。

## 数据存储

| 数据 | 容器路径 | 说明 |
| --- | --- | --- |
| SQLite 数据库 | `/app/data/app.db` | 账号、站点设置、文档源、字体元数据和解析日志 |
| 会话密钥 | `/app/data/jwt-secret` | 首次启动自动生成，迁移时需与数据库一起保留 |
| 站点 Logo | `/app/data/uploads/` | 站点设置中的可选图片 |
| 前端静态文件 | `/app/public` | 构建进统一镜像，由 Go 服务托管 |

宿主机 `./data` 会挂载到容器 `/app/data`。升级时不会删除数据库；首次启动新版本会执行一次存储精简迁移，只清理旧版本的非核心表和字段，保留字体记录、账号和站点设置。

SQLite 已启用 WAL、`synchronous=NORMAL`、外键约束和忙等待；字体写入使用批量 upsert，首页目录按文档顺序加载后在浏览器本地筛选，后台管理列表使用分页查询。

## 使用 GitHub Actions 镜像

每次推送到 `main` 分支都会自动构建并发布：

`ghcr.io/alumos/modernfonts:latest`

部署时复制 [`docker-compose.example.yml`](docker-compose.example.yml)，然后执行：

```bash
docker compose -f docker-compose.example.yml up -d
```

数据默认保存在当前目录的 `modernfonts-data/`。更新镜像时执行：

```bash
docker compose -f docker-compose.example.yml pull
docker compose -f docker-compose.example.yml up -d
```

如果 GHCR 镜像设为私有，需要先在面板或服务器执行 `docker login ghcr.io`。

## 权限

| 功能 | 访客 | 普通用户 | 管理员 |
| --- | --- | --- | --- |
| 浏览和搜索字体 | 支持 | 支持 | 支持 |
| 解析下载直链 | 不支持 | 支持 | 支持 |
| 管理文档源、字体和账号 | 不支持 | 不支持 | 支持 |

公开字体接口不会返回蓝奏云地址和访问码；下载解析接口要求登录。

## 本地开发

```bash
cd backend
DATA_DIR=./data go run ./cmd/server
```

```bash
cd frontend
npm ci
npm run dev
```

可通过环境变量调整蓝奏云解析并发和缓存时间：

```bash
LANZOU_WORKERS=8 LANZOU_CACHE_TTL_SECONDS=180
```

代码位置：

- 后端业务：`backend/cmd/server`
- 前端页面：`frontend/src/pages`
- 后台布局：`frontend/src/layouts/AdminShell.tsx`
