# CMAS-CATS-Go

CMAS-CATS-Go 是一个基于 Go 语言开发的算力网实践项目，旨在模拟服务站点的部署、监控和管理。项目采用模块化设计，包含多个独立的微服务模块。

## 项目背景

CMAS（Computing Metrics as a Service）草案提出了一种“计算指标即服务”的方法，用于标准化和简化算力指标的广播与管理。CMAS-CATS-Go 项目实现了这一思想，构建了一个完整的算力网络流量调度系统原型。

## 项目结构

```
.
├── cmd/                # 各模块的命令行入口
│   ├── c-ps/          # C-PS 模块（路径选择器）
│   ├── c-sma/         # C-SMA 模块（服务指标代理）
│   ├── platform/      # 平台模块（公共服务平台）
│   ├── site/          # 站点模块（服务站点）
│   └── site2/         # 第二站点模块
├── models/             # 数据模型定义
│   └── service.go     # 服务实例信息模型
├── main.go             # 主站点服务入口
├── platform.db         # 平台数据库
├── site1.db            # 站点1数据库
├── site2.db            # 站点2数据库
├── go.mod              # Go 模块依赖管理
├── go.sum              # Go 模块依赖版本锁定
└── readme.md           # 项目说明文档
```

## 功能模块

1. **公共服务平台（Platform）**
   - 提供服务注册、验证和查询功能。
   - 接口示例：
     - `POST /services`：注册服务。
     - `GET /services`：查询所有服务。

2. **服务站点（Site）**
   - 模拟算力提供者，支持服务实例的部署和管理。
   - 接口示例：
     - `POST /deploy`：部署服务实例。
     - `GET /metrics`：暴露服务实例的指标。

3. **服务指标代理（C-SMA）**
   - 定期从服务站点收集指标并广播。

4. **路径选择器（C-PS）**
   - 根据用户请求选择最优服务实例。

## 快速开始

### 环境要求

- Go 1.21.0 或更高版本
- SQLite 数据库

### 安装依赖

```bash
go mod tidy
```

### 启动服务

1. 启动公共服务平台：

```bash
go run cmd/platform/main.go
```

2. 启动服务站点：

```bash
go run cmd/site/main.go
go run cmd/site2/main.go
```

3. 启动其他模块（如 C-SMA、C-PS）：

```bash
go run cmd/c-sma/main.go
go run cmd/c-ps/main.go
```

### 示例请求

#### 注册服务

```bash
curl -X POST http://localhost:8080/api/v1/services \
-H "Content-Type: application/json" \
-d '{
  "name": "AR/VR",
  "description": "接收传感器输入生成AR场景",
  "input_format": "Motion Capture",
  "computing_requirement": "CPU≥2.0GHz, GPU>RTX4060",
  "storage_requirement": "16GB DRAM",
  "computing_time": "≤1ms",
  "code_location": "https://github.com/xxx/ar",
  "software_dependency": ["Unity"],
  "validation_sample": "test.mp4",
  "validation_result": "result.json"
}'
```

```bash
# 注册交通监测服务（TP100）
curl -X POST http://localhost:8080/api/v1/services \
-H "Content-Type: application/json" \
-d '{
  "name": "交通流量监测",
  "description": "实时分析道路摄像头数据，统计车流量",
  "input_format": "视频流（H.264）",
  "computing_requirement": "CPU≥1.8GHz, 支持OpenCV",
  "storage_requirement": "8GB DRAM",
  "computing_time": "≤50ms",
  "code_location": "https://github.com/xxx/traffic-monitor",
  "software_dependency": ["OpenCV", "FFmpeg"],
  "validation_sample": "traffic_sample.mp4",
  "validation_result": "traffic_analysis.json"
}'
```

```bash
# 注册人脸识别服务（FR300））
curl -X POST http://localhost:8080/api/v1/services \
-H "Content-Type: application/json" \
-d '{
  "name": "人脸识别",
  "description": "从图像中检测并识别人脸身份",
  "input_format": "图像（JPG/PNG）",
  "computing_requirement": "CPU≥2.5GHz, GPU≥RTX3050",
  "storage_requirement": "32GB DRAM",
  "computing_time": "≤200ms",
  "code_location": "https://github.com/xxx/face-recognition",
  "software_dependency": ["TensorFlow", "OpenCV"],
  "validation_sample": "face_samples.zip",
  "validation_result": "recognition_rate.json"
}'
```

```bash
# 注册语音转文字服务（ASR400）
curl -X POST http://localhost:8080/api/v1/services \
-H "Content-Type: application/json" \
-d '{
  "name": "语音转文字",
  "description": "将音频中的语音转换为文本",
  "input_format": "音频（WAV/MP3）",
  "computing_requirement": "CPU≥2.0GHz, 8核",
  "storage_requirement": "16GB DRAM",
  "computing_time": "≤1s（每30秒音频）",
  "code_location": "https://github.com/xxx/speech-to-text",
  "software_dependency": ["PyTorch", "FFmpeg"],
  "validation_sample": "speech_samples.wav",
  "validation_result": "transcription_accuracy.json"
}'
```
##### 查询所有已注册服务
```bash 
curl http://localhost:8080/api/v1/services
```
##### 查询单个服务详情（替换{service_id}为注册返回的ID，如AR1760101784963）
```bash 
curl http://localhost:8080/api/v1/services/{service_id}
```
#### 部署服务实例

```bash
curl -X POST http://localhost:8082/deploy \
-H "Content-Type: application/json" \
-d '{"service_id": "example-service", "gas": 5}'
```

#### 查询 Metrics

```bash
curl -X GET http://localhost:8082/metrics
```

## 数据库设计

项目使用 SQLite 数据库，包含以下表：

- **deployed_services**：存储部署的服务实例信息。

字段：
- `id`：实例唯一标识
- `service_id`：服务 ID
- `gas`：实例数量
- `cost`：部署成本
- `csci_id`：实例访问地址
- `created_at`：创建时间
- `delay`：延迟指标（ms）

## 测试方法

### 单元测试

1. 确保测试文件已编写并位于项目目录中。
2. 使用以下命令运行所有测试：

```bash
go test ./...
```

3. 查看测试结果，确保所有测试用例通过。

### 接口测试

#### 测试部署服务实例接口

```bash
curl -X POST http://localhost:8082/deploy \
-H "Content-Type: application/json" \
-d '{"service_id": "test-service", "gas": 3}'
```

预期返回：
```json
{
  "success": true,
  "message": "服务实例部署成功：test-service（3个）",
  "info": {
    "service_id": "test-service",
    "gas": 3,
    "cost": 6,
    "csci_id": "http://localhost:8082/test-service-site-1-<timestamp>",
    "delay": 13
  }
}
```

#### 测试 Metrics 接口

```bash
curl -X GET http://localhost:8082/metrics
```

预期返回：
```json
{
  "success": true,
  "site_id": "site-1",
  "count": <实例数量>,
  "metrics": [
    {
      "service_id": "test-service",
      "gas": 3,
      "cost": 6,
      "csci_id": "http://localhost:8082/test-service-site-1-<timestamp>",
      "delay": 13
    }
  ],
  "time": "<当前时间>"
}
```

## 依赖库

项目使用以下主要依赖：

- [Gin](https://github.com/gin-gonic/gin)：高性能 Web 框架
- [SQLite3](https://github.com/mattn/go-sqlite3)：SQLite 数据库驱动

完整依赖请参考 `go.mod` 文件。

## 贡献

欢迎提交 Issue 和 Pull Request 来改进项目！

## 许可证

本项目采用 [MIT 许可证](LICENSE)。