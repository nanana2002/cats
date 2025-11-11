# CMAS-CATS-Go

CMAS-CATS-Go 是一个基于 Go 语言开发的算力网络实践项目，实现了 CMAS（Computing Metrics as a Service）草案中提出的"计算指标即服务"方法。该项目构建了一个完整的算力网络流量调度系统原型，支持服务站点的部署、监控和管理。

## 项目概述

本项目旨在标准化和简化算力指标的广播与管理，通过模块化设计实现了一个包含多个独立微服务的算力网络系统。系统支持实时监控、服务部署和资源管理功能。

## 项目架构

```
.
├── api.js                # API 示例文件
├── go.mod                # Go 模块依赖管理
├── go.sum                # Go 模块依赖版本锁定
├── go技术方案.md         # Go 技术方案文档
├── index.html            # Web 界面主页面
├── main.go               # 主站点服务入口（site-1）
├── readme.md             # 项目说明文档
├── site.db               # SQLite 数据库文件
├── start-remote.sh       # 远程启动脚本
├── start.sh              # 本地启动脚本
├── stop.sh               # 停止脚本
├── sync-code.sh          # 代码同步脚本
├── upload_handler.go     # 文件上传处理模块
├── cmd/                  # 各模块的命令行入口
│   ├── c-ps/            # C-PS 模块（路径选择器）
│   ├── c-sma/           # C-SMA 模块（服务指标代理）
│   ├── platform/        # 平台模块（公共服务平台）
│   ├── site/            # 站点模块（服务站点）
│   └── site2/           # 第二站点模块
├── config/               # 配置模块
│   └── config.go         # 全局配置文件
├── db/                   # 数据库目录
├── logs/                 # 日志目录
├── models/               # 数据模型定义
│   └── service.go        # 服务和实例信息模型
├── scripts/              # 脚本目录
├── templates/            # 模板目录
├── tests/                # 测试目录
├── uploads/              # 上传目录
└── webui                 # WebUI 二进制文件
```

## 核心模块

### 1. 公共服务平台（Platform）
- 负责服务注册和元数据管理
- 提供服务的注册、查询和验证接口
- 部署在配置指定的平台地址

### 2. 服务站点（Site）
- 模拟算力提供者，管理服务实例的部署和运行
- 包含两个站点：site1 和 site2，配置在不同地址和端口
- 支持多服务类型（AR/VR、交通流量监测、人脸识别、语音转文字等）
- 提供部署、指标暴露、健康检查等功能
- 包含上传和执行脚本功能

### 3. 服务指标代理（C-SMA）
- 定期从各服务站点收集指标数据
- 聚合并去重来自多个站点的实例信息
- 提供统一的监控数据接口
- 每10秒拉取一次各站点的指标数据

### 4. 路径选择器（C-PS）
- 根据客户端请求选择最优服务实例
- 基于成本、延迟等指标进行路径选择

### 5. Web 界面（WebUI）
- 提供图形化界面进行部署和监控
- 支持代码包和启动脚本的上传
- 包含实时监控面板，显示 C-SMA 收集的数据
- 提供资源选择、部署和停止功能

## 配置系统

项目使用 `config/config.go` 进行统一配置管理：

```go
package config

var LOCAL_LISTEN_IP = "127.0.0.1"

var Cfg = struct {
    Platform struct{ IP string; Port int; URL string }
    Site1    struct{ IP string; Port int; URL string }
    Site2    struct{ IP string; Port int; URL string }
    SMA      struct{ IP string; Port int; URL string }
    PS       struct{ IP string; Port int; URL string }
    MonitoredSites []string
    Resource struct{ Site1Total, Site2Total int }
}{
    Platform: struct{ IP string; Port int; URL string }{IP: "192.168.67.185", Port: 8080, URL: "http://192.168.67.185:8080"},
    Site1:    struct{ IP string; Port int; URL string }{IP: "192.168.235.48", Port: 8081, URL: "http://192.168.235.48:8081"},
    Site2:    struct{ IP string; Port int; URL string }{IP: "192.168.67.159", Port: 8085, URL: "http://192.168.67.159:8085"},
    SMA:      struct{ IP string; Port int; URL string }{IP: "192.168.67.185", Port: 8083, URL: "http://192.168.67.185:8083"},
    PS:       struct{ IP string; Port int; URL string }{IP: "192.168.67.185", Port: 8084, URL: "http://192.168.67.185:8084"},
    MonitoredSites: []string{
        "http://192.168.235.48:8081",
        "http://192.168.67.159:8085",
    },
    Resource: struct{ Site1Total, Site2Total int }{Site1Total: 400, Site2Total: 500},
}

func GetAllSiteURLs() []string {
    return Cfg.MonitoredSites
}
```

## 快速开始

### 环境要求

- Go 1.21.0 或更高版本
- SQLite 数据库
- SSH 访问权限（用于远程站点部署）
- 确保网络可达性：站点1（192.168.235.48）、站点2（192.168.67.159）

### 部署方式

#### 方式一：一键启动（推荐）
```bash
./start.sh
```

该脚本会：
1. 生成并更新配置文件
2. 通过 SSH 启动远程站点服务
3. 启动本地平台、C-SMA、C-PS 和 WebUI 服务

#### 方式二：手动启动
分别启动各模块：

```bash
# 启动公共服务平台
go run cmd/platform/main.go

# 启动服务站点
go run cmd/site/main.go
go run cmd/site2/main.go

# 启动 C-SMA（服务指标代理）
go run cmd/c-sma/main.go

# 启动 C-PS（路径选择器）
go run cmd/c-ps/main.go

# 启动 WebUI
./webui
```

### 访问服务

- Web 界面：`http://localhost:9091`
- 平台 API：`http://<platform-ip>:<platform-port>`
- 站点1 API：`http://<site1-ip>:<site1-port>`
- 站点2 API：`http://<site2-ip>:<site2-port>`
- C-SMA：`http://<sma-ip>:<sma-port>`
- C-PS：`http://<ps-ip>:<ps-port>`

## Web 界面功能

### 部署功能
- **服务类型选择**：支持预设服务类型部署（site1）
- **代码上传**：支持 ZIP 格式代码包上传（site2）
- **脚本上传**：支持 SH 格式启动脚本上传（site2）
- **资源选择**：选择目标服务站点进行部署
- **部署执行**：一键部署到选定站点
- **停止服务**：停止运行中的服务

### 实时监控
- **自动刷新**：每10秒自动从 C-SMA 获取最新数据
- **数据展示**：显示来自所有站点的服务实例详细信息
- **状态信息**：显示上次更新时间、监控站点数、服务总数和实例总数
- **实例详情**：展示实例地址、数量、成本和延迟等信息

## API 接口

### 平台 API
- `POST /api/v1/services`：注册服务
- `GET /api/v1/services`：查询所有服务
- `GET /api/v1/services/{id}`：查询单个服务

### 站点 API
- `POST /deploy`：部署服务实例（支持预设服务类型）
- `GET /metrics`：获取实例指标
- `GET /health`：健康检查
- `GET /resource-status`：资源占用状态
- `POST /upload`：上传文件（仅site2）
- `POST /execute`：执行脚本

### WebUI API
- `GET /api/resources`：获取可用资源
- `POST /api/deploy`：部署代码
- `POST /api/stop`：停止服务
- `GET /api/status/:siteId`：获取站点状态

### C-SMA API
- `GET /current-metrics`：获取聚合指标数据
- `GET /sync`：同步数据到 C-PS
- `GET /health`：健康检查

## 数据模型

### Service（服务模型）
```go
type Service struct {
    ID                 string    // 服务唯一ID
    Name               string    // 服务名称
    Description        string    // 服务功能描述
    InputFormat        string    // 服务输入格式
    ComputingRequirement string  // 计算资源要求
    StorageRequirement string    // 存储资源要求
    ComputingTime      string    // 单次计算延迟
    CodeLocation       string    // 服务代码地址
    SoftwareDependency []string  // 软件依赖
    CreatedAt          time.Time // 创建时间
    ValidationSample   string    // 服务验证用的输入样本
    ValidationResult   string    // 服务验证的预期输出
}
```

### ServiceInstanceInfo（服务实例信息模型）
```go
type ServiceInstanceInfo struct {
    ServiceID string // 关联的服务ID
    Gas       int    // 可用服务实例数量
    Cost      int    // 单次服务成本
    CSCI_ID   string // 服务接触实例地址
    Delay     int    // 延迟（ms）
}
```

### ClientRequest（客户端请求模型）
```go
type ClientRequest struct {
    ServiceID        string // 目标服务ID
    MaxAcceptCost    int    // 客户端可接受的最高成本
    MaxAcceptDelay   int    // 客户端可接受的最大总延迟（毫秒）
}
```

## 数据库设计

项目使用 SQLite 数据库存储服务实例信息：

**deployed_services 表**：
- `id`：实例唯一标识
- `service_id`：服务 ID
- `gas`：实例数量
- `cost`：部署成本
- `csci_id`：实例访问地址
- `created_at`：创建时间
- `delay`：延迟指标（ms）
- `resource_per_inst`：单个实例资源占用
- `total_resource_used`：该部署总资源占用

## 服务类型支持

项目支持以下预设服务类型及其资源分配：
- **AR100**: 基础计算服务，单实例占用 50 资源单位
- **AR1760108487856**: 交通流量监测，单实例占用 15 资源单位
- **AR1760108501919**: 人脸识别，单实例占用 50 资源单位
- **AR1760108514766**: 语音转文字，单实例占用 30 资源单位

## 资源管理

- **成本计算**：基于资源占用按比例计算
- **资源限制**：每个站点有总资源限制（site1: 400, site2: 500）
- **资源监控**：实时监控资源使用情况，防止超限部署

## WebUI 实时监控功能

index.html 页面包含实时监控功能：

1. **自动数据获取**：每10秒从 C-SMA 的 `/current-metrics` 端点获取数据
2. **数据展示**：以卡片形式展示各个服务实例的详细信息
3. **状态更新**：显示上次更新时间、监控站点数、服务总数和实例总数
4. **错误处理**：当无法连接到 C-SMA 时显示错误信息
5. **中文支持**：正确显示中文字符

## 文件上传处理

`upload_handler.go` 提供了安全的文件上传功能：
- 支持 ZIP 格式文件上传
- 文件解压到指定目录
- 防止路径遍历攻击
- 启动脚本执行功能

## 部署与配置

### 环境变量配置

在 `start.sh` 中定义了以下环境变量：
- `LOCAL_LISTEN_IP`: 本地服务监听地址
- `PLATFORM_IP/PORT`: 平台服务地址和端口
- `SITE1_IP/PORT`: 站点1地址和端口
- `SITE2_IP/PORT`: 站点2地址和端口
- `SMA_IP/PORT`: C-SMA服务地址和端口
- `PS_IP/PORT`: C-PS服务地址和端口
- SSH 账户信息：用于远程服务启动

### SSH 账户配置

`start.sh` 脚本中包含以下 SSH 配置：
- `SITE1_USER/SITE1_PASS`: 站点1的用户名和密码
- `SITE2_USER/SITE2_PASS`: 站点2的用户名和密码

## 故障排除

### 常见问题

1. **实例数持续上升**
   如果发现 C-SMA 检测的实例数持续上升：
   ```
   # 停止所有服务
   ./stop.sh
   
   # 删除数据库文件
   rm -f db/site1.db db/site2.db
   
   # 重新启动服务
   ./start.sh
   ```

2. **Web 界面无法加载资源**
   - 检查 C-SMA 服务是否正常运行
   - 验证网络连接是否正常
   - 确认配置文件中的地址端口是否正确

3. **SSH 连接失败**
   - 确认 SSH 服务在远程站点上运行
   - 验证用户名和密码是否正确
   - 检查防火墙设置是否允许 SSH 连接

### 日志文件

- 平台日志：`logs/platform.log`
- C-SMA 日志：`logs/c-sma.log`
- C-PS 日志：`logs/c-ps.log`
- 站点日志：`logs/site1.log`, `logs/site2.log`
- WebUI 日志：`logs/webui.log`

## 依赖库

主要依赖：
- [Gin Framework](https://github.com/gin-gonic/gin)：Web 框架
- [SQLite3](https://github.com/mattn/go-sqlite3)：SQLite 驱动
- [Gin-CORS](https://github.com/gin-contrib/cors)：跨域支持

完整依赖请参考 `go.mod` 文件。

## 开发指南

### 添加新服务类型

要添加新服务类型，请在 `main.go` 中的 `getPresetServiceInfo` 函数中添加对应配置：

```go
presetServices["NEW_SERVICE_ID"] = struct {
    name            string
    resourcePerInst int
    description     string
}{
    name:            "服务名称",
    resourcePerInst: 资源占用单位,
    description:     "服务描述",
}
```

### 扩展站点功能

1. 复制 `cmd/site/` 目录创建新的站点模块
2. 修改端口、数据库路径和站点ID
3. 更新 `config/config.go` 和 `start.sh` 中的配置

## 项目特点

1. **模块化设计**：各组件独立部署，便于扩展和维护
2. **实时监控**：通过C-SMA实现集中监控
3. **灵活部署**：支持预设服务和自定义代码部署
4. **资源管理**：动态分配和监控资源使用情况
5. **Web 界面**：提供友好的用户交互界面

## 许可证

本项目采用 [MIT 许可证](LICENSE)。

## 贡献

欢迎提交 Issue 和 Pull Request 来改进项目！

## 联系方式

如需技术支持或合作，请联系项目维护者。