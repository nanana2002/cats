
CMAS 草案的核心创新在于，它提出了一种“计算指标即服务 (Computing Metrics as a Service)”的方法，用一种更简单、标准化的方式来取代在网络中广播复杂、异构的算力指标（如 CPU 频率、GPU 型号等）。

我们将把整个系统拆解成几个可以独立开发的 Go 微服务/模块，让你能一步一步地构建和理解它。

### 核心概念理解

在开始之前，我们先用一个比喻来理解整个流程：

  * **公共服务平台 (Public Service Platform)**：想象成一个“应用商店(App Store)”。开发者（服务提供者）把他们的应用（服务）发布到这里，并详细说明运行这个应用需要什么样的手机配置（计算要求）。
  * **服务站点 (Service Site)**：想象成一个“手机机主”。他会浏览这个应用商店，根据自己的手机性能（算力资源），决定安装哪些应用，以及每个应用可以同时开几个（服务实例数量）。
  * **服务指标 (CMAS Metrics)**：手机机主不会告诉别人“我的 CPU 是 8 核，内存 16GB”，而是直接告诉大家：“我这里可以同时运行 3 个微信（服务ID: AR1, 数量: 3）和 5 个抖音（服务ID: TP1, 数量: 5），运行一次微信收费 4 个金币，抖音 5 个金币”。这就是 CMAS 的核心，用**可提供的服务数量和成本**来描述自己的能力。
  * **客户端 (Client)**：需要使用应用的用户。
  * **路径选择器 C-PS (CATS Path Selector)**：一个智能调度中心。它知道所有“手机机主”能提供哪些“应用”以及收费情况，也知道网络延迟等信息。当用户请求服务时，它会为用户匹配一个最合适的“手机机主”。

-----

### 实现流程指导

#### 第 0 步：环境准备和项目结构

1.  **安装 Go**: 确保你已经安装了最新版的 Go 语言环境。
2.  **项目初始化**: 创建一个项目文件夹，例如 `cmas-cats-go`，并在其中执行 `go mod init cmas-cats-go` 来初始化 Go 模块。
3.  **技术选型**:
      * **HTTP 服务**: 为了实现各个组件之间的 API 通信，我们可以使用 Go 标准库的 `net/http`，或者更方便的 Web 框架，如 [Gin](https://github.com/gin-gonic/gin)。对于初学者，Gin 更易上手。
      * **数据存储**: 项目初期，我们可以先用 Go 的 `map` 在内存中模拟数据库，后续可以轻松替换为真实的数据库（如 SQLite, PostgreSQL）。

#### 第 1 步：定义核心数据结构 (Data Models)

在写任何逻辑代码之前，先在项目中创建一个 `models` 包，用来定义贯穿整个系统的核心数据结构。这就像是构建房子的蓝图。

```go
// file: models/service.go
package models

// Service 对应草案中“公共服务平台”的服务表（Table 1）
type Service struct {
    ID                 string   `json:"id"`                   // 服务ID, e.g., "AR1"
    Name               string   `json:"name"`                 // 服务名, e.g., "AR/VR"
    Description        string   `json:"description"`          // 服务描述
    ComputingRequirement string   `json:"computing_requirement"` // 计算要求描述
    StorageRequirement string   `json:"storage_requirement"`  // 存储要求
    ComputingTime      string   `json:"computing_time"`       // 单次计算时间
    CodeLocation       string   `json:"code_location"`        // 服务代码位置 (e.g., Git URL)
    // 用于验证的数据样本和预期结果
    ValidationSample string `json:"-"` // 设为私有，不通过API暴露
    ValidationResult string `json:"-"` // 设为私有
}

// ServiceInstanceInfo 对应草案中服务站点的服务模型表（Table 3）
type ServiceInstanceInfo struct {
    ServiceID string `json:"service_id"` // 服务ID
    Gas       int    `json:"gas"`        // 可用的服务实例数量
    Cost      int    `json:"cost"`       // 单次服务的成本
    CSCI_ID   string `json:"csci_id"`    // 服务联系实例ID (可理解为服务的访问地址, e.g., "http://site1.com:8081/ar1")
}

// ClientRequest 客户端向C-PS发起的请求结构
type ClientRequest struct {
    ServiceID     string `json:"service_id"` // 请求的服务ID
    MaxAcceptCost int    `json:"max_accept_cost"` // 能接受的最高成本
    MaxAcceptDelay int   `json:"max_accept_delay"`// 能接受的最大延迟 (ms)
}
```

#### 第 2 步：构建公共服务平台 (Public Service Platform)

这是系统的“应用商店”和认证中心。它是一个独立的 HTTP 服务。

1.  **创建主程序**: `cmd/platform/main.go`。
2.  **数据存储**: 在内存中使用 `map[string]models.Service` 来存储所有服务。
3.  **实现 API 端点 (Endpoints)**:
      * `POST /services`: **服务注册**接口。服务提供者调用此接口，提交 `Service` 结构的所有信息。平台验证后，为其分配一个唯一的 `ID` 并存储。
      * `GET /services`: **服务发现**接口。供客户端或服务站点查询当前所有可用的服务列表。
      * `GET /services/{id}`: 获取单个服务的详细信息，包括其计算要求和代码位置，供服务站点部署时使用。
      * `POST /validate`: **服务部署验证**接口。服务站点部署完服务后调用此接口。平台会向该服务站点发送 `ValidationSample`，并比对返回结果和 `ValidationResult`是否一致。

#### 第 3 步：构建服务站点 (Service Site)

服务站点是算力的实际提供者。它也是一个独立的 HTTP 服务。

1.  **创建主程序**: `cmd/site/main.go`。
2.  **核心逻辑**:
      * **资源管理**: 模拟该站点的总资源。可以简单地用一个变量表示，例如 `totalResourceUnits = 100`。
      * **服务部署**:
        1.  启动时，调用“公共服务平台”的 `GET /services` API 获取可部署的服务列表。
        2.  根据自身资源和策略，决定部署哪些服务。例如，部署一个 "AR1" 服务需要 20 个资源单位，部署一个 "TP1" 服务需要 30 个。如果总资源是 100，它可以部署 2 个 "AR1" 和 2 个 "TP1"。
        3.  部署后，调用平台的 `/validate` 接口进行验证。
      * **维护服务模型表 (Table 3)**: 验证通过后，在内存中维护一个 `map[string]models.ServiceInstanceInfo`。这个表的内容就是它要向外界宣告的自身能力。例如：
          * `{"AR1": {ServiceID: "AR1", Gas: 2, Cost: 4, CSCI_ID: "http://site.com/ar1"}}`
          * `{"TP1": {ServiceID: "TP1", Gas: 2, Cost: 5, CSCI_ID: "http://site.com/tp1"}}`
      * **实现 API 端点**:
          * `GET /metrics`: **指标上报**接口。这个接口是给 C-SMA 调用的，返回上面维护的服务模型表。

#### 第 4 步：实现 C-SMA (CATS Service Metric Agent)

[cite\_start]C-SMA 是一个简单的代理，负责从服务站点收集指标并“广播”出去。在 CATS 框架中，C-SMA 收集和报告服务指标 [cite: 1, 32, 125, 189, 190]。

1.  **创建主程序**: `cmd/c-sma/main.go`。
2.  **核心逻辑**:
      * **轮询**: 定期（例如每 10 秒）调用它所负责的服务站点（可以有多个）的 `/metrics` 接口。
      * **广播**: 获取到指标后，将这些 `ServiceInstanceInfo` 数据广播出去。在我们的模拟实现中，可以将其发送到一个 Go `channel`，让 C-PS 模块从这个 `channel` 中读取。

#### 第 5 步：实现 C-PS (CATS Path Selector)

[cite\_start]C-PS 是决策大脑，是 CATS 框架的核心组件之一，它负责选择路径和服务实例 [cite: 1, 34, 123, 197]。

1.  **创建主程序**: `cmd/c-ps/main.go`。
2.  **核心逻辑**:
      * **信息汇总**:
        1.  从 C-SMA 的 `channel` 中持续接收各个服务站点的算力指标 (`ServiceInstanceInfo`)。
        2.  **模拟 C-NMA**: 模拟网络指标。可以简单地创建一个函数，返回从当前位置到每个服务站点 `CSCI_ID` 的网络延迟（例如，随机生成一个 10-50ms 的值）。
        3.  **获取服务元数据**: 从“公共服务平台”获取所有服务的 `ComputingTime` 等静态信息。
      * **构建决策表**: 将上述所有信息整合成一个大的决策表，结构类似草案 Figure 3 中的描述：`(Service ID, CSCI-ID, Gas, Cost, Computing time, Network delay)`。
      * **实现 API 端点**:
          * `POST /select`: **路径选择**接口。这是给客户端（或 Ingress CATS-Forwarder）调用的。
        <!-- end list -->
        1.  接收客户端的 `ClientRequest`。
        2.  根据请求的 `ServiceID`，在决策表中筛选出所有能提供该服务的实例。
        3.  结合客户端的 `MaxAcceptCost` 和 `MaxAcceptDelay` 要求，以及服务实例的 `Cost`、`Gas`（是否可用）、网络延迟和计算延迟，执行一个简单的选择算法（例如，选择总延迟最低且成本符合要求的实例）。
        4.  返回最合适的 `ServiceInstanceInfo`（主要是 `CSCI_ID`）。

#### 第 6 步：模拟客户端 (Client) 和端到端流程

现在，将所有部分串联起来。

1.  **创建主程序**: `cmd/client/main.go`。
2.  **执行流程**:
    1.  **服务发现**: 客户端首先调用**公共服务平台**的 `GET /services`，查看有什么可用的服务，比如它想用 "AR1" 服务。
    2.  **发起请求**: 客户端构建一个 `ClientRequest`，例如：`{ServiceID: "AR1", MaxAcceptCost: 5, MaxAcceptDelay: 25}`。
    3.  **获取最佳服务地址**: 客户端将这个请求发送给 **C-PS** 的 `POST /select` 接口。
    4.  **接收结果**: C-PS 根据其决策表进行计算，返回一个最佳的服务实例地址，例如 `{..., CSCI_ID: "http://site.com/ar1"}`。
    5.  **访问服务**: 客户端拿到这个 `CSCI_ID` 后，就可以直接向这个地址发起真实的业务请求了（在我们的模拟中，可以简单地向这个地址发送一个 HTTP GET 请求）。

### 总结与下一步

通过以上六个步骤，你就用 Go 实现了一个完整的、符合 CMAS 草案思想的算力网络流量调度系统原型。

  * 你构建了**数据模型**作为基础。
  * 你实现了**公共服务平台**作为服务的注册和发现中心。
  * 你实现了**服务站点**来模拟算力资源的提供和部署。
  * 你通过 **C-SMA** 和 **C-PS** 实现了 CMAS 的核心逻辑：**用简单的服务单元（Gas, Cost）替代复杂的硬件指标**，并结合网络状况进行智能决策。
  * 你通过**客户端**模拟了完整的端到端服务请求流程。

这个原型虽然简化了很多细节（如认证、安全、真实的资源调度等），但它完美地体现了 CMAS 草案的设计哲学和工作流程。从这里开始，你可以逐步深化每个模块，比如将内存数据库换成真实数据库，优化 C-PS 的调度算法，或者使用 gRPC 替代 HTTP API 以提高内部通信效率。



# 详细过程
## 第0步.安装Go语言
安装Go语言环境非常简单，以下是针对不同操作系统的详细步骤：

### 一、Windows系统安装

1. **下载安装包**
   访问Go官方网站下载页面：https://golang.org/dl/
   选择适合Windows的安装包（通常是 `go1.x.y.windows-amd64.msi`）

2. **运行安装程序**
   双击下载的MSI文件，按照向导提示进行安装。
   默认安装路径为 `C:\Go\`，建议保持默认路径以便后续配置。

3. **验证安装**
   按下 `Win + R`，输入 `cmd` 打开命令提示符，执行以下命令：
   ```bash
   go version
   ```
   如果显示类似 `go version go1.21.0 windows/amd64` 的信息，说明安装成功。

4. **配置工作目录（可选）**
   Go推荐使用一个工作目录（如 `C:\go-work`），包含三个子目录：
   - `src`：存放源代码
   - `pkg`：存放编译后的包文件
   - `bin`：存放可执行文件

   可以通过设置环境变量 `GOPATH` 来指定工作目录：
   - 右键"此电脑" → "属性" → "高级系统设置" → "环境变量"
   - 新建系统变量 `GOPATH`，值设为你的工作目录路径


### 二、macOS系统安装

1. **方法一：使用安装包**
   - 从官方网站下载macOS版本的安装包（`go1.x.y.darwin-amd64.pkg`）
   - 双击安装包，按照提示完成安装，默认安装到 `/usr/local/go`

2. **方法二：使用Homebrew（推荐）**
   如果已安装Homebrew，只需在终端执行：
   ```bash
   brew install go
   ```

3. **验证安装**
   打开终端，执行：
   ```bash
   go version
   ```

4. **配置工作目录（可选）**
   在终端中执行：
   ```bash
   mkdir -p ~/go-work/{src,pkg,bin}
   echo 'export GOPATH=~/go-work' >> ~/.bash_profile
   echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bash_profile
   source ~/.bash_profile
   ```


### 三、Linux系统安装

1. **下载压缩包**
   在终端中使用wget或curl下载适合Linux的压缩包：
   ```bash
   wget https://dl.google.com/go/go1.21.0.linux-amd64.tar.gz
   ```

2. **解压到系统目录**
   ```bash
   sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
   ```

3. **配置环境变量**
   编辑 `.bashrc` 或 `.profile` 文件：
   ```bash
   sudo nano ~/.bashrc
   ```
   在文件末尾添加：
   ```bash
   export PATH=$PATH:/usr/local/go/bin
   export GOPATH=$HOME/go-work
   export PATH=$PATH:$GOPATH/bin
   ```
   保存后生效：
   ```bash
   source ~/.bashrc
   ```

4. **验证安装**
   ```bash
   go version
   ```


### 四、验证安装并测试

创建一个简单的Go程序来测试环境是否正常工作：

1. 创建工作目录（如果还没有）：
   ```bash
   mkdir -p $GOPATH/src/hello
   cd $GOPATH/src/hello
   ```

2. 创建文件 `main.go`：
   ```go
   package main

   import "fmt"

   func main() {
       fmt.Println("Hello, Go!")
   }
   ```

3. 运行程序：
   ```bash
   go run main.go
   ```

如果输出 `Hello, Go!`，则说明你的Go语言环境已经安装配置成功。

### 五、升级Go版本

如果需要升级已安装的Go版本，只需下载对应版本的安装包，按照相同的步骤安装即可，新版本会覆盖旧版本。

对于使用包管理器的系统（如macOS的Homebrew），可以直接执行：
```bash
brew upgrade go  # macOS
sudo apt upgrade golang  # Ubuntu/Debian
```


## 第1步.搭建公共服务平台（CMAS Core）
### 一、详细执行流程（共5步）
#### **项目初始化**
 创建一个项目文件夹，例如 `cmas-cats-go`，并在其中执行 `go mod init cmas-cats-go` 来初始化 Go 模块。

#### 0. 定义核心数据结构 (Data Models)

在写任何逻辑代码之前，先在项目中创建一个 `models` 包，用来定义贯穿整个系统的核心数据结构。这就像是构建房子的蓝图。

```go
// file: models/service.go
package models
import "time"  // 新增这一行：导入time包，用于识别time.Time类型
// Service 对应草案中“公共服务平台”的服务表（Table 1）
// 存储服务的元数据、计算/存储要求、代码位置等公开信息
type Service struct {
        ID                 string   `json:"id"`                   // 服务唯一ID，如 "AR1"（草案中Service ID）
        Name               string   `json:"name"`                 // 服务名称，如 "AR/VR"（草案中Service Name）
        Description        string   `json:"description"`          // 服务功能描述（草案中Service Description）
        InputFormat        string   `json:"input_format"`         // 服务输入格式，如 "Motion Capture, Voice Tracking"（草案中Input）
        ComputingRequirement string `json:"computing_requirement"`// 计算资源要求，如 "multi-thread CPUs ≥2.0GHz, GPU > RTX4060"（草案中Computing Requirement）
        StorageRequirement string   `json:"storage_requirement"`  // 存储资源要求，如 "16GB DRAM, 256GB SSD"（草案中Storage Requirement）
        ComputingTime      string   `json:"computing_time"`       // 单次计算延迟，如 "≤1ms"（草案中Computing Time）
        CodeLocation       string   `json:"code_location"`        // 服务代码地址，如 "https://github.com/xxx/ar-service"（草案中Service Running Code）
        SoftwareDependency []string `json:"software_dependency"`  // 软件依赖，如 ["Unity", "Unreal Engine"]（草案中Software Dependency）
        CreatedAt          time.Time `json:"created_at"`           // 新增：服务创建时间（用于记录注册时间）
        // 私有字段：仅用于服务部署验证，不通过API暴露给客户端/服务站点（草案中Service Sample Result Table）
        ValidationSample string `json:"-"` // 服务验证用的输入样本（如AR服务的测试视频流）
        ValidationResult string `json:"-"` // 服务验证的预期输出（如样本的正确渲染结果）
}

// ServiceInstanceInfo 对应草案中“服务站点”的服务模型表（Table 3）
// 存储服务站点已部署的服务实例信息，用于向C-SMA上报
type ServiceInstanceInfo struct {
        ServiceID string `json:"service_id"` // 关联的服务ID，如 "AR1"（对应Service.ID）
        Gas       int    `json:"gas"`        // 可用服务实例数量（草案中Gas），如 3 表示可同时处理3个AR请求
        Cost      int    `json:"cost"`       // 单次服务成本（草案中Cost），如 4 表示每次调用消耗4个“资源单位”
        CSCI_ID   string `json:"csci_id"`    // 服务接触实例地址（草案中CSCI-ID），如 "http://192.168.1.100:8080/ar1"（客户端实际访问的地址）
}

// ClientRequest 客户端向C-PS发起的服务请求结构（草案中Client Service Request）
// 包含客户端的服务需求、成本和延迟限制
type ClientRequest struct {
        ServiceID     string `json:"service_id"`     // 目标服务ID，如 "AR1"
        MaxAcceptCost int    `json:"max_accept_cost"`// 客户端可接受的最高成本，如 5（超过此值的服务实例会被过滤）
        MaxAcceptDelay int   `json:"max_accept_delay"`// 客户端可接受的最大总延迟（毫秒），如 25（计算延迟+网络延迟）
}
```
#### 1. 环境准备：安装 Gin 框架（依赖）
公共服务平台用 Gin 实现 HTTP API，需先安装框架：
```bash
# 1. 进入项目目录（已初始化 go.mod）
cd ~/go-work/src/cmas-cats-go

# 2. 安装 Gin v1.9.1（稳定版本，兼容当前 Go 环境）
go get github.com/gin-gonic/gin@v1.9.1

# 3. 验证依赖：查看 go.mod 是否新增 Gin 依赖（执行后含 "github.com/gin-gonic/gin v1.9.1" 即成功）
cat go.mod
```

#### 2. 创建目录结构：存放公共服务平台代码
```bash
# 1. 创建 cmd/platform 目录（Go 项目规范：cmd 存放各模块主程序）
mkdir -p cmd/platform

# 2. 验证目录：执行后显示 "cmd/  go.mod  models/"，说明目录创建成功
tree
```

#### 3. 编写核心代码：`cmd/platform/main.go`
##### 3.1 创建并编辑文件
```bash
nano cmd/platform/main.go
```
##### 3.2 粘贴代码（完整可运行版）
```go
package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"cmas-cats-go/models" // 引用第一步创建的 models 包
)

// 全局内存存储：模拟数据库（key=Service.ID，value=Service）
var (
	serviceStore      = make(map[string]models.Service)
	serviceIDCounter  = 100 // 生成 Service ID 的计数器（避免与草案 AR1/TP1 冲突）
)

func main() {
	// 1. 初始化 Gin 引擎（默认包含日志、Panic 恢复中间件）
	r := gin.Default()

	// 2. 注册 API 接口（统一前缀 /api/v1，便于版本管理）
	api := r.Group("/api/v1")
	{
		api.POST("/services", registerServiceHandler)  // 服务注册（服务提供者用）
		api.GET("/services", listServicesHandler)     // 服务列表查询（客户端/服务站点用）
		api.GET("/services/:id", getServiceHandler)   // 服务详情查询（服务站点部署用）
	}

	// 3. 启动服务（监听 8080 端口，端口冲突可改为 8081）
	if err := r.Run(":8080"); err != nil {
		panic("公共服务平台启动失败：" + err.Error())
	}
}

// 服务注册接口：处理服务提供者的注册请求
func registerServiceHandler(c *gin.Context) {
	// 1. 解析客户端提交的服务信息（JSON 格式）
	var req models.Service
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	// 2. 校验必填字段（避免服务信息不完整）
	if req.Name == "" || req.Description == "" || req.ComputingRequirement == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "必填字段缺失：name/description/computing_requirement 不能为空",
		})
		return
	}

	// 3. 生成唯一 Service ID（如 AR100、TP101，贴合草案格式）
	prefix := getServicePrefix(req.Name)
	serviceID := prefix + strconv.Itoa(serviceIDCounter)
	serviceIDCounter++

	// 4. 补充服务信息并存储
	req.ID = serviceID
	req.CreatedAt = time.Now()
	serviceStore[serviceID] = req

	// 5. 返回注册结果
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "服务注册成功",
		"service_id": serviceID,
		"service":    req,
	})
}

// 服务列表查询接口：返回所有已注册服务
func listServicesHandler(c *gin.Context) {
	// 转换 map 为切片（便于 JSON 序列化）
	var services []models.Service
	for _, s := range serviceStore {
		services = append(services, s)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"count":    len(services),
		"services": services,
	})
}

// 服务详情查询接口：通过 Service ID 获取单个服务信息
func getServiceHandler(c *gin.Context) {
	// 从 URL 路径获取 Service ID（如 /api/v1/services/AR100）
	serviceID := c.Param("id")

	// 查询服务是否存在
	service, exists := serviceStore[serviceID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "服务不存在：service_id=" + serviceID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"service": service,
	})
}

// 工具函数：根据服务名生成 ID 前缀（如 "AR/VR" → "AR"）
func getServicePrefix(name string) string {
	name = strings.TrimSpace(name)
	switch {
	case strings.Contains(name, "AR") || strings.Contains(name, "VR"):
		return "AR"
	case strings.Contains(name, "Transport") || strings.Contains(name, "交通"):
		return "TP"
	case strings.Contains(name, "Live") || strings.Contains(name, "直播"):
		return "LB"
	default:
		if len(name) >= 2 {
			return strings.ToUpper(name[:2])
		}
		return "SV" // 兜底前缀
	}
}
```

###### 3.3 保存退出
按 `Ctrl + O` → 按 `Enter` 确认保存 → 按 `Ctrl + X` 退出 `nano`。



#### 4. 启动并验证公共服务平台
#### 4.1 启动服务
```bash
# 在项目目录执行，启动公共服务平台
go run cmd/platform/main.go
```

#### 4.2 验证启动成功
终端输出以下内容即正常（重点看“路由注册”和“监听”日志）：
```
[GIN-debug] POST   /api/v1/services          --> main.registerServiceHandler (3 handlers)
[GIN-debug] GET    /api/v1/services          --> main.listServicesHandler (3 handlers)
[GIN-debug] GET    /api/v1/services/:id      --> main.getServiceHandler (3 handlers)
[GIN-debug] Listening and serving HTTP on :8080
```

#### 4.3 验证接口连通性（关键！）
**新开一个终端**，执行以下命令（先取消代理，避免请求被拦截）：
```bash
# 1. 取消 HTTP 代理（若系统配置了代理，必须执行）
unset http_proxy
unset https_proxy

# 2. 测试服务列表查询接口（无服务时返回空列表，状态码 200）
curl http://localhost:8080/api/v1/services -i
```

#### 4.4 验证服务注册（核心功能验证）
```bash
# 发送 POST 请求，注册 AR/VR 服务
curl -X POST http://localhost:8080/api/v1/services \
-H "Content-Type: application/json" \
-d '{"name":"AR/VR","description":"接收传感器输入生成AR场景","input_format":"Motion Capture","computing_requirement":"CPU≥2.0GHz, GPU>RTX4060","storage_requirement":"16GB DRAM","computing_time":"≤1ms","code_location":"https://github.com/xxx/ar","software_dependency":["Unity"],"validation_sample":"test.mp4","validation_result":"result.json"}'

# 再次查询服务列表，确认注册成功（返回 count=1，包含 AR100 服务）
curl http://localhost:8080/api/v1/services
```


### 二、常见问题及解决方案
| 问题现象 | 可能原因 | 解决方案 |
|----------|----------|----------|
| 启动报错：`req.CreatedAt undefined` | `models/Service` 结构体未定义 `CreatedAt` 字段 | 编辑 `models/service.go`，在 `SoftwareDependency` 后补充 `CreatedAt time.Time json:"created_at"`，并导入 `time` 包 |
| 启动报错：`undefined: time` | `models/service.go` 使用 `time.Time` 但未导入 `time` 包 | 在 `models/service.go` 顶部添加 `import "time"` |
| `curl` 请求无返回，提示 `503 Service Unavailable` | 系统配置了 HTTP 代理，`curl` 请求被转发到代理服务器 | 1. 临时取消代理：`unset http_proxy; unset https_proxy` <br> 2. 永久取消：编辑 `~/.bashrc`，删除代理配置行，执行 `source ~/.bashrc` |
| `curl` 请求提示 `Connection refused` | 1. 公共服务平台未启动 <br> 2. 8080 端口被占用 <br> 3. 防火墙拦截 | 1. 重新启动公共服务平台 <br> 2. 更换端口：将 `r.Run(":8080")` 改为 `r.Run(":8081")` <br> 3. 开放端口：`ufw allow 8080` |
| 服务注册报错：`请求格式错误` | JSON 参数格式错误（如逗号遗漏、引号不匹配） | 1. 使用单行 JSON 避免格式问题（参考 5.4 中的命令） <br> 2. 用 `jsonlint` 工具校验 JSON 格式 |


## 第2步. 开发服务站点（Service Site）
### 一、详细执行流程（共4步）
#### 1. 创建目录结构：存放服务站点代码
```bash
# 1. 进入项目目录
cd ~/go-work/src/cmas-cats-go

# 2. 创建 cmd/site 目录（服务站点主程序目录）
mkdir -p cmd/site

# 3. 验证目录：执行后含 "cmd/site/"，说明创建成功
tree cmd/
```

#### 2. 编写服务站点主程序：`cmd/site/main.go`
##### 2.1 创建并编辑文件
```bash
nano cmd/site/main.go
```

##### 2.2 粘贴代码（完整可运行版）
```go
package main

import (
        "encoding/json"
        "fmt"
        "net/http"
        "strings"
        "time"

        "github.com/gin-gonic/gin"
        "cmas-cats-go/models"
)

// 服务站点配置（可根据实际修改）
const (
        PublicPlatformURL = "http://localhost:8080/api/v1" // 公共服务平台地址
        SiteListenPort    = ":8082"                        // 服务站点监听端口（避免与 8080 冲突）
        TotalResource     = 100                           // 总资源单位（模拟）
)

// 服务站点状态（已部署服务、已用资源）
var (
        deployedServices = make(map[string]models.ServiceInstanceInfo)
        usedResource     = 0
)

func main() {
        // 1. 初始化 Gin 引擎
        r := gin.Default()

        // 2. 注册 API 接口
        r.POST("/deploy", deployServiceHandler)  // 部署服务接口
        r.GET("/metrics", getMetricsHandler)     // 向 C-SMA 上报 metrics 接口

        // 3. 启动服务站点
        fmt.Printf("服务站点启动成功，监听：%s\n", SiteListenPort)
        if err := r.Run(SiteListenPort); err != nil {
                panic("服务站点启动失败：" + err.Error())
        }
}

// 部署服务接口：向公共服务平台申请服务并模拟部署
func deployServiceHandler(c *gin.Context) {
        // 1. 解析请求参数（ServiceID 和实例数量）
        var req struct {
                ServiceID string `json:"service_id" binding:"required"`
                Gas       int    `json:"gas" binding:"min=1"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误：" + err.Error()})
                return
        }

        // 2. 向公共服务平台获取服务详情
        _, err := getServiceFromPlatform(req.ServiceID)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "申请服务失败：" + err.Error()})
                return
        }

        // 3. 模拟资源检查（1个实例需20单位资源）
        resourcePerInst := 20
        totalNeed := resourcePerInst * req.Gas
        if usedResource+totalNeed > TotalResource {
                c.JSON(http.StatusForbidden, gin.H{
                        "success": false,
                        "message": fmt.Sprintf("资源不足：已用%d，需%d，总%d", usedResource, totalNeed, TotalResource),
                })
                return
        }

        // 4. 模拟部署：占用资源并记录服务实例
        usedResource += totalNeed
        csciID := fmt.Sprintf("http://localhost:8082/%s", strings.ToLower(req.ServiceID))
        deployedServices[req.ServiceID] = models.ServiceInstanceInfo{
                ServiceID: req.ServiceID,
                Gas:       req.Gas,
                Cost:      4,  // 模拟成本
                CSCI_ID:   csciID,
        }

        // 5. 返回部署结果
        c.JSON(http.StatusOK, gin.H{
                "success": true,
                "message": fmt.Sprintf("部署成功：%d个%s实例", req.Gas, req.ServiceID),
                "info":    deployedServices[req.ServiceID],
        })
}

// 上报 metrics 接口：供 C-SMA 拉取已部署服务信息
func getMetricsHandler(c *gin.Context) {
        // 转换为切片，便于 JSON 序列化
        var metrics []models.ServiceInstanceInfo
        for _, info := range deployedServices {
                metrics = append(metrics, info)
        }

        c.JSON(http.StatusOK, gin.H{
                "success":   true,
                "metrics":   metrics,
                "timestamp": time.Now().Unix(),
        })
}

// 工具函数：向公共服务平台获取服务详情
func getServiceFromPlatform(serviceID string) (models.Service, error) {
        reqURL := fmt.Sprintf("%s/services/%s", PublicPlatformURL, serviceID)
        resp, err := http.Get(reqURL)
        if err != nil {
                return models.Service{}, fmt.Errorf("请求平台失败：%w", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
                return models.Service{}, fmt.Errorf("平台返回错误：状态码%d", resp.StatusCode)
        }

        var result struct {
                Success bool          `json:"success"`
                Service models.Service `json:"service"`
                Message string        `json:"message"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
                return models.Service{}, fmt.Errorf("解析响应失败：%w", err)
        }

        if !result.Success {
                return models.Service{}, fmt.Errorf("服务不存在：%s", result.Message)
        }

        return result.Service, nil
}
```

##### 2.3 保存退出（`Ctrl + O` → `Enter` → `Ctrl + X`）

#### 3. 启动服务站点
```bash
# 在项目目录执行，启动服务站点
go run cmd/site/main.go
```

##### 3.1 验证启动成功
终端输出以下内容即正常：
```
root@danana:~/go-work/src/cmas-cats-go# go run cmd/site/main.go
[GIN-debug] [WARNING] Creating an Engine instance with the Logger and Recovery middleware already attached.

[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
 - using env:   export GIN_MODE=release
 - using code:  gin.SetMode(gin.ReleaseMode)

[GIN-debug] POST   /deploy                   --> main.deployServiceHandler (3 handlers)
[GIN-debug] GET    /metrics                  --> main.getMetricsHandler (3 handlers)
服务站点启动成功，监听：:8082
[GIN-debug] [WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.
Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.
[GIN-debug] Listening and serving HTTP on :8082

```

#### 4. 验证服务站点功能
##### 4.1 部署服务（核心验证）
**新开终端**，执行以下命令，部署之前注册的 `AR100` 服务：
```bash
# 向服务站点发送部署请求（部署 2 个 AR100 实例）
curl -X POST http://localhost:8082/deploy \
-H "Content-Type: application/json" \
-d '{"service_id":"AR100","gas":2}'
```

##### 4.2 验证部署成功
```bash
# 1. 查看部署结果（返回“部署成功”，包含实例信息）
# 2. 查看 metrics 接口，确认已部署服务能被 C-SMA 拉取
curl http://localhost:8082/metrics
```

##### 4.3 预期结果
```json
root@danana:~/go-work/src/cmas-cats-go# curl http://localhost:8082/metrics
{"metrics":[{"service_id":"AR100","gas":2,"cost":4,"csci_id":"http://localhost:8082/ar100"}],"success":true,"timestamp":1760089372}
```


### 二、常见问题及解决方案
| 问题现象 | 可能原因 | 解决方案 |
|----------|----------|----------|
| 启动报错：`cannot find package "cmas-cats-go/models"` | `go.mod` 中的模块名与包引用路径不匹配 | 1. 查看 `go.mod` 第一行：`module cmas-cats-go`（必须与引用路径一致） <br> 2. 若模块名不同，修改 `models` 包引用为 `module名/models` |
| 部署服务报错：`申请服务失败：请求平台失败` | 1. 公共服务平台未启动 <br> 2. 公共服务平台地址配置错误 | 1. 启动公共服务平台（`go run cmd/platform/main.go`） <br> 2. 检查 `PublicPlatformURL` 是否为 `http://localhost:8080/api/v1`（与公共服务平台端口一致） |
| 部署服务报错：`资源不足` | 服务站点总资源（100单位）不足以部署请求的实例数量 | 1. 减少 `gas` 值（如改为 `gas=1`） <br> 2. 修改 `TotalResource` 为更大值（如 200） |
| `curl` 访问服务站点提示 `Connection refused` | 1. 服务站点未启动 <br> 2. 8082 端口被占用 | 1. 重新启动服务站点 <br> 2. 更换端口：将 `SiteListenPort` 改为 `:8083` |




## 第3步：实现 C-SMA（CATS Service Metric Agent）

### 一、模块定位与核心目标
根据 `zhang` 草案和 CATS 框架规范，C-SMA 的核心职责是 **“定期从服务站点拉取算力度量（metrics），并将聚合后的度量数据同步给 C-PS”**，是连接“服务站点”与“C-PS”的关键桥梁。本步骤将实现一个轻量级 C-SMA，包含以下核心功能：
1. 配置可监控的服务站点列表（支持多个服务站点）；
2. 定期（如每 10 秒）拉取服务站点的 `/metrics` 接口；
3. 聚合多站点的度量数据（按 `ServiceID` 分类）；
4. 提供 `GET /sync` 接口供 C-PS 拉取聚合后的度量数据（模拟同步逻辑）。


### 二、详细执行流程（共5步）
#### 1. 环境准备：确认依赖与目录结构
##### 1.1 依赖检查
C-SMA 需复用之前的 `models` 包（`ServiceInstanceInfo` 结构体），且无需新增第三方依赖（用 Go 标准库 `net/http` 拉取 metrics，Gin 提供接口），已安装的 Gin 框架可直接复用。

##### 1.2 创建 C-SMA 目录
在 `cmas-cats-go` 目录下执行命令，创建 C-SMA 的代码目录（遵循 Go 项目 `cmd` 规范）：
```bash
# 进入项目目录
cd ~/go-work/src/cmas-cats-go

# 创建 cmd/c-sma 目录（存放 C-SMA 主程序）
mkdir -p cmd/c-sma

# 验证目录结构（执行后含 "cmd/c-sma/"，说明创建成功）
tree cmd/
```
终端输出如下，目录结构合规：
```
cmd/
├── platform       # 公共服务平台
│   └── main.go
├── site           # 服务站点
│   └── main.go
└── c-sma          # 新增：C-SMA 目录
```


#### 2. 编写 C-SMA 主程序：`cmd/c-sma/main.go`
#### 2.1 创建并编辑文件
```bash
nano cmd/c-sma/main.go
```

#### 2.2 粘贴完整代码（含详细注释）
```go
// file: cmd/c-sma/main.go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"cmas-cats-go/models"
)

// C-SMA 核心配置（多站点场景适配）
const (
	ListenPort     = ":8083"                // C-SMA 监听端口
	PollInterval   = 10 * time.Second       // 拉取间隔（10秒，可根据需求调整）
	// 多服务站点列表：用逗号分隔，支持新增多个站点（示例："站点1,站点2,站点3"）
	ServiceSiteList = "http://localhost:8082/metrics,http://localhost:8084/metrics,http://localhost:8085/metrics"
)

// 全局状态：
// - aggregatedMetrics：聚合后的度量数据（key=ServiceID，value=该Service的所有站点实例）
// - metricsMutex：读写锁，避免多协程并发读写冲突
var (
	aggregatedMetrics = make(map[string][]models.ServiceInstanceInfo)
	metricsMutex      sync.RWMutex
)

func main() {
	// 1. 初始化 Gin 引擎（开启生产模式可注释 debug 日志）
	r := gin.Default()

	// 2. 注册核心接口（供 C-PS 同步和调试用）
	r.GET("/sync", syncToCPSHandler)          // 供 C-PS 拉取聚合后的 metrics
	r.GET("/current-metrics", getMetricsHandler) // 调试：查看当前所有聚合数据

	// 3. 启动“多站点 metrics 拉取”后台任务（单独协程，不阻塞接口）
	go startMultiSitePolling()

	// 4. 启动 C-SMA 服务
	fmt.Printf("✅ C-SMA 启动成功！监听端口：%s | 拉取间隔：%v | 监控站点数：%d\n",
		ListenPort, PollInterval, len(parseSiteList(ServiceSiteList)))
	if err := r.Run(ListenPort); err != nil {
		panic("❌ C-SMA 启动失败：" + err.Error())
	}
}

// ------------------------------
// 核心功能：多站点 metrics 拉取与聚合
// ------------------------------

// startMultiSitePolling：启动多站点定期拉取任务
func startMultiSitePolling() {
	// 解析配置的服务站点列表
	sites := parseSiteList(ServiceSiteList)
	if len(sites) == 0 {
		fmt.Println("⚠️  警告：未配置任何服务站点，C-SMA 无法拉取 metrics")
		return
	}

	// 定时任务：每 PollInterval 拉取一次
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Printf("\n[%s] 📥 开始拉取 %d 个服务站点的 metrics...\n",
			time.Now().Format("2006-01-02 15:04:05"), len(sites))

		var wg sync.WaitGroup // 等待组：确保所有站点拉取完成后再汇总
		for _, siteURL := range sites {
			wg.Add(1)
			// 并发拉取（多站点场景下提升效率）
			go func(url string) {
				defer wg.Done()
				// 拉取单个站点的 metrics
				siteMetrics, err := fetchSingleSiteMetrics(url)
				if err != nil {
					fmt.Printf("❌ 拉取站点 [%s] 失败：%v\n", url, err)
					return
				}
				// 聚合数据（加写锁，避免并发冲突）
				metricsMutex.Lock()
				aggregateMultiSiteMetrics(url, siteMetrics)
				metricsMutex.Unlock()

				fmt.Printf("✅ 拉取站点 [%s] 成功：%d 个服务实例\n", url, len(siteMetrics))
			}(siteURL)
		}

		wg.Wait() // 等待所有站点拉取完成
		fmt.Printf("[%s] 📊 所有站点拉取完成，当前聚合服务数：%d\n",
			time.Now().Format("2006-01-02 15:04:05"), len(aggregatedMetrics))
	}
}

// fetchSingleSiteMetrics：拉取单个服务站点的 metrics
func fetchSingleSiteMetrics(siteURL string) ([]models.ServiceInstanceInfo, error) {
	// 发送 GET 请求到服务站点的 /metrics 接口
	resp, err := http.Get(siteURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败：%w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码（必须为 200 OK）
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码错误：%d（期望 200）", resp.StatusCode)
	}

	// 解析站点返回的 JSON 数据（匹配服务站点 /metrics 接口格式）
	var siteResp struct {
		Success bool                     `json:"success"`
		Metrics []models.ServiceInstanceInfo `json:"metrics"`
		Message string                   `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&siteResp); err != nil {
		return nil, fmt.Errorf("JSON解析失败：%w", err)
	}

	// 检查站点返回的结果是否成功
	if !siteResp.Success {
		return nil, fmt.Errorf("站点返回错误：%s", siteResp.Message)
	}

	return siteResp.Metrics, nil
}

// aggregateMultiSiteMetrics：多站点 metrics 聚合（覆盖旧数据，避免重复）
// 核心逻辑：先删除该站点的旧实例，再追加新实例
func aggregateMultiSiteMetrics(siteURL string, newMetrics []models.ServiceInstanceInfo) {
	// 提取站点标识（如 "http://localhost:8082/metrics" → "localhost:8082"）
	siteID := strings.TrimSuffix(siteURL, "/metrics")

	// 1. 先删除该站点的所有旧实例（避免重复）
	for serviceID, oldInstances := range aggregatedMetrics {
		var retainedInstances []models.ServiceInstanceInfo
		for _, inst := range oldInstances {
			// 保留“非当前站点”的实例（通过 CSCI-ID 包含站点标识判断）
			if !strings.Contains(inst.CSCI_ID, siteID) {
				retainedInstances = append(retainedInstances, inst)
			}
		}
		aggregatedMetrics[serviceID] = retainedInstances
	}

	// 2. 追加该站点的新实例
	for _, newInst := range newMetrics {
		serviceID := newInst.ServiceID
		aggregatedMetrics[serviceID] = append(aggregatedMetrics[serviceID], newInst)
	}
}

// ------------------------------
// 辅助函数：站点列表解析
// ------------------------------

// parseSiteList：解析服务站点列表（逗号分隔 → 切片）
func parseSiteList(listStr string) []string {
	var validSites []string
	// 按逗号分割，去除空格和空字符串
	for _, site := range strings.Split(listStr, ",") {
		trimmedSite := strings.TrimSpace(site)
		if trimmedSite != "" && strings.HasSuffix(trimmedSite, "/metrics") {
			validSites = append(validSites, trimmedSite)
		}
	}
	return validSites
}

// ------------------------------
// API 接口实现（供 C-PS 和调试）
// ------------------------------

// syncToCPSHandler：供 C-PS 拉取聚合后的 metrics（多站点数据已整合）
func syncToCPSHandler(c *gin.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	// 转换为 C-PS 易解析的格式（按 ServiceID 分组）
	var syncData []struct {
		ServiceID string                     `json:"service_id"`
		Instances []models.ServiceInstanceInfo `json:"instances"` // 该服务的所有站点实例
		TotalGas  int                        `json:"total_gas"`  // 新增：该服务的总实例数（方便 C-PS 决策）
	}

	for serviceID, instances := range aggregatedMetrics {
		// 计算该服务的总实例数（gas 累加）
		totalGas := 0
		for _, inst := range instances {
			totalGas += inst.Gas
		}

		syncData = append(syncData, struct {
			ServiceID string                     `json:"service_id"`
			Instances []models.ServiceInstanceInfo `json:"instances"`
			TotalGas  int                        `json:"total_gas"`
		}{
			ServiceID: serviceID,
			Instances: instances,
			TotalGas:  totalGas,
		})
	}

	// 返回同步数据（带时间戳，供 C-PS 判断数据新鲜度）
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"sync_time":  time.Now().Format("2006-01-02 15:04:05"),
		"service_num": len(syncData), // 聚合的服务总数
		"data":       syncData,
	})
}

// getMetricsHandler：调试接口，查看当前所有聚合的 metrics 数据
func getMetricsHandler(c *gin.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"last_update_time": time.Now().Format("2006-01-02 15:04:05"),
		"monitored_sites":  len(parseSiteList(ServiceSiteList)), // 当前监控的站点数
		"aggregated_data":  aggregatedMetrics,                  // 完整聚合数据
	})
}
```

#### 3. 启动 C-SMA 并验证
##### 3.1 启动前的前置条件
确保以下服务已启动（C-SMA 依赖它们）：
1. **公共服务平台**：已启动（监听 8080，虽 C-SMA 不直接调用，但服务站点需依赖它）；
2. **服务站点**：已启动（监听 8082，且已部署 `AR100` 服务，`/metrics` 接口可访问）。

##### 3.2 启动 C-SMA
在 `cmas-cats-go` 目录下执行命令：
```bash
go run cmd/c-sma/main.go
```

##### 3.3 验证 C-SMA 启动成功
终端输出以下内容，说明 C-SMA 启动正常，开始定期拉取服务站点 metrics：
```
[GIN-debug] [WARNING] Creating an Engine instance with the Logger and Recovery middleware already attached.
[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
...
C-SMA 启动成功！监听端口：:8083，拉取间隔：10s
[GIN-debug] Listening and serving HTTP on :8083

[2025-10-10 18:00:00] 开始拉取 1 个服务站点的 metrics...
拉取站点 http://localhost:8082/metrics 成功：1 个服务实例
[2025-10-10 18:00:10] 所有站点拉取完成，当前聚合后 metrics：1 个 Service
```


#### 4. 验证 C-SMA 核心功能（关键步骤）
##### 4.1 验证“定期拉取”功能
观察 C-SMA 的终端日志，每 10 秒会自动拉取一次服务站点的 `/metrics` 接口，输出类似：
```
[2025-10-10 18:00:10] 开始拉取 1 个服务站点的 metrics...
拉取站点 http://localhost:8082/metrics 成功：1 个服务实例
[2025-10-10 18:00:20] 所有站点拉取完成，当前聚合后 metrics：1 个 Service
```
说明“定期拉取”功能正常。

##### 4.2 验证“聚合 metrics”功能
**新开一个终端**，执行以下命令，调用 C-SMA 的 `/current-metrics` 接口，查看聚合后的度量数据：
```bash
curl http://localhost:8083/current-metrics
```
正常结果（包含 `AR100` 服务的 2 个实例）：
```json
{"aggregated_metrics":{"AR100":[{"service_id":"AR100","gas":2,"cost":4,"csci_id":"http://localhost:8082/ar100"},{"service_id":"AR100","gas":2,"cost":4,"csci_id":"http://localhost:8082/ar100"},{"service_id":"AR100","gas":2,"cost":4,"csci_id":"http://localhost:8082/ar100"},{"service_id":"AR100","gas":2,"cost":4,"csci_id":"http://localhost:8082/ar100"},{"service_id":"AR100","gas":2,"cost":4,"csci_id":"http://localhost:8082/ar100"},{"service_id":"AR100","gas":2,"cost":4,"csci_id":"http://localhost:8082/ar100"}]},"success":true,"update_time":"2025-10-10 19:24:29"}

```

##### 4.3 验证“同步给 C-PS”功能
执行以下命令，调用 C-SMA 的 `/sync` 接口（模拟 C-PS 拉取数据）：
```bash
curl http://localhost:8083/sync
```
正常结果（按 `ServiceID` 聚合，供 C-PS 做流量调度决策）：
```json
{
  "success": true,
  "sync_time": "2025-10-10 18:00:30",
  "service_count": 1,
  "data": [
    {
      "service_id": "AR100",
      "instances": [
        {
          "service_id": "AR100",
          "gas": 2,
          "cost": 4,
          "csci_id": "http://localhost:8082/ar100"
        }
      ]
    }
  ]
}
```


### 三、常见问题及解决方案
| 问题现象 | 可能原因 | 解决方案 |
|----------|----------|----------|
| 启动报错：`undefined: strings` | `main.go` 未导入 `strings` 包 | 在 `import` 块中添加 `import "strings"` |
| C-SMA 提示“拉取站点失败：状态码错误 404” | 服务站点未启动，或 `/metrics` 接口路径错误 | 1. 启动服务站点（`go run cmd/site/main.go`） <br> 2. 检查 `ServiceSiteList` 配置是否为 `http://localhost:8082/metrics` |
| C-SMA 拉取成功但 `/sync` 接口无数据 | 1. 服务站点 `/metrics` 返回的 `metrics` 字段为空 <br> 2. 聚合逻辑错误 | 1. 先调用 `curl http://localhost:8082/metrics` 确认服务站点有数据 <br> 2. 检查 `aggregateMetrics` 函数是否正确将 `siteMetrics` 追加到 `aggregatedMetrics` |
| 并发拉取时日志乱序 | 多协程同时打印日志，导致输出混乱 | （可选）添加日志锁，确保日志打印串行化（在 `fmt.Printf` 前后加 `sync.Mutex`） |


### 四、C-SMA 成功标志总结
当出现以下情况时，说明 C-SMA 模块完全实现目标：
1. 启动后自动定期（每 10 秒）拉取服务站点的 `/metrics` 接口，无报错；
2. `/current-metrics` 接口能返回聚合后的 `AR100` 实例数据；
3. `/sync` 接口能按 `ServiceID` 分类返回数据，格式符合 C-PS 预期。

至此，你已完成 CMAS 系统“公共服务平台→服务站点→C-SMA”的三级架构开发，下一步可推进 **C-PS（CATS Path Selector）** 模块，实现“接收客户端请求→结合 C-SMA 数据→选择最优服务实例”的核心决策逻辑。

## 第4步：实现 C-PS (CATS Path Selector)
### 一、模块定位与核心目标
根据 `zhang` 草案，C-PS（CATS 路径选择器）是整个 CMAS 系统的“决策大脑”，核心职责是：
1. **接收客户端服务请求**：获取客户端的目标服务 ID、可接受的最大成本（`MaxAcceptCost`）、最大延迟（`MaxAcceptDelay`）；
2. **拉取 C-SMA 聚合的 metrics**：获取所有服务站点的算力分布（实例数量 `gas`、成本 `cost`、服务地址 `CSCI-ID`）；
3. **执行路径选择算法**：基于“成本优先+实例充足”原则，筛选出符合客户端限制的最优服务实例；
4. **返回决策结果**：向客户端返回最优服务实例的 `CSCI-ID`，指引客户端直接访问服务站点。

本步骤将实现一个完整的 C-PS，包含上述所有核心功能，确保客户端能通过 C-PS 找到“性价比最高”的服务实例。

### 二、详细执行流程（共5步）
#### 1. 环境准备：确认依赖与目录结构
##### 1.1 依赖检查
C-PS 需复用以下已有资源，无需新增第三方依赖：
- `models` 包：复用 `ClientRequest`（客户端请求结构）、`ServiceInstanceInfo`（服务实例结构）；
- Gin 框架：用于提供 HTTP 接口，接收客户端请求；
- C-SMA 接口：依赖 C-SMA 的 `/sync` 接口拉取 metrics（需确保 C-SMA 已启动）。

##### 1.2 创建 C-PS 目录
在 `cmas-cats-go` 目录下执行命令，创建 C-PS 的代码目录（遵循 Go 项目 `cmd` 规范）：
```bash
# 进入项目目录
cd ~/go-work/src/cmas-cats-go

# 创建 cmd/c-ps 目录（存放 C-PS 主程序）
mkdir -p cmd/c-ps

# 验证目录结构（执行后含 "cmd/c-ps/"，说明创建成功）
tree cmd/
```
终端输出如下，目录结构合规：
```
cmd/
├── platform       # 公共服务平台
├── site           # 服务站点
├── c-sma          # C-SMA 度量收集
└── c-ps           # 新增：C-PS 路径选择器
```


#### 2. 编写 C-PS 主程序：`cmd/c-ps/main.go`
##### 2.1 创建并编辑文件
```bash
nano cmd/c-ps/main.go
```

##### 2.2 粘贴完整代码（含详细注释）
```go
// file: cmd/c-ps/main.go
package main

import (
        "encoding/json"
        "fmt"
        "net/http"
        "sort"
        "sync"
        "time"

        "github.com/gin-gonic/gin"
        "cmas-cats-go/models"
)

// C-PS 核心配置（无语法错误，常量名无空格）
const (
        ListenPort     = ":8084"                           // C-PS 监听端口（避免与其他服务冲突）
        CSMASyncURL    = "http://localhost:8083/sync"      // C-SMA 的 /sync 接口地址
        MaxSyncRetry   = 3                                 // 拉取 C-SMA 失败时的重试次数
        RetryInterval  = 2 * time.Second                   // 重试间隔
        CacheExpire    = 5 * time.Minute                   //  metrics 缓存过期时间
)

// 全局缓存：存储 C-SMA 拉取的 metrics，减少重复请求
var (
        cachedMetrics = make(map[string][]models.ServiceInstanceInfo) // key=ServiceID
        lastSyncTime  time.Time                                      // 上次同步时间
        mutex         sync.RWMutex                                   // 读写锁，避免并发冲突
)

func main() {
        // 1. 初始化 Gin 引擎
        r := gin.Default()

        // 2. 注册核心接口
        r.POST("/request-service", handleClientRequest) // 处理客户端服务请求（核心接口）
        r.GET("/refresh-metrics", refreshMetricsCache)   // 手动刷新 metrics 缓存（调试）
        r.GET("/cached-metrics", getCachedMetrics)       // 查看当前缓存的 metrics（调试）

        // 3. 启动前预加载 C-SMA metrics（避免首次请求为空）
        if err := syncMetricsFromCSMA(); err != nil {
                fmt.Printf("⚠️  预加载 C-SMA metrics 失败：%v（后续请求会自动重试）\n", err)
        } else {
                fmt.Printf("✅ 预加载成功！当前缓存 %d 个服务的 metrics\n", len(cachedMetrics))
        }

        // 4. 启动 C-PS 服务
        fmt.Printf("\n✅ C-PS 启动成功！监听端口：%s | C-SMA 地址：%s\n", ListenPort, CSMASyncURL)
        if err := r.Run(ListenPort); err != nil {
                panic("❌ C-PS 启动失败：" + err.Error())
        }
}

// ------------------------------
// 核心1：从 C-SMA 拉取并更新 metrics 缓存
// ------------------------------

// syncMetricsFromCSMA：向 C-SMA 发起请求，拉取 metrics 并更新缓存
func syncMetricsFromCSMA() error {
        // 发送 GET 请求到 C-SMA
        resp, err := http.Get(CSMASyncURL)
        if err != nil {
                return fmt.Errorf("请求 C-SMA 失败：%w", err)
        }
        defer resp.Body.Close()

        // 检查响应状态码
        if resp.StatusCode != http.StatusOK {
                return fmt.Errorf("C-SMA 返回错误状态码：%d（期望 200）", resp.StatusCode)
        }

        // 解析 C-SMA 返回的 JSON 数据
        var csmaResp struct {
                Success    bool                     `json:"success"`
                ServiceNum int                      `json:"service_num"`
                Data       []struct {
                        ServiceID string                     `json:"service_id"`
                        Instances []models.ServiceInstanceInfo `json:"instances"`
                } `json:"data"`
                Message string `json:"message"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&csmaResp); err != nil {
                return fmt.Errorf("解析 C-SMA 响应失败：%w", err)
        }

        // 检查 C-SMA 返回结果是否成功
        if !csmaResp.Success {
                return fmt.Errorf("C-SMA 业务失败：%s", csmaResp.Message)
        }

        // 更新缓存（加写锁，避免并发冲突）
        mutex.Lock()
        defer mutex.Unlock()
        // 清空旧缓存，避免残留无效数据
        cachedMetrics = make(map[string][]models.ServiceInstanceInfo)
        for _, item := range csmaResp.Data {
                cachedMetrics[item.ServiceID] = item.Instances
        }
        lastSyncTime = time.Now()

        // 打印同步日志
        fmt.Printf("[%s] 📥 同步 C-SMA 成功：%d 个服务，共 %d 个实例\n",
                lastSyncTime.Format("2006-01-02 15:04:05"),
                len(cachedMetrics),
                countTotalInstances())

        return nil
}

// refreshMetricsCache：手动刷新缓存（对外提供 HTTP 接口）
func refreshMetricsCache(c *gin.Context) {
        if err := syncMetricsFromCSMA(); err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "success": false,
                        "message": "刷新缓存失败：" + err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "success":       true,
                "message":       "缓存刷新成功",
                "service_count": len(cachedMetrics),
                "last_sync":     lastSyncTime.Format("2006-01-02 15:04:05"),
        })
}

// ------------------------------
// 核心2：处理客户端请求，选择最优服务实例
// ------------------------------

// handleClientRequest：接收客户端请求，执行路径选择
func handleClientRequest(c *gin.Context) {
        // 1. 解析客户端请求参数
        var req models.ClientRequest
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{
                        "success": false,
                        "message": "请求格式错误：" + err.Error(),
                })
                return
        }

        // 2. 校验必填参数
        if req.ServiceID == "" {
                c.JSON(http.StatusBadRequest, gin.H{
                        "success": false,
                        "message": "缺少必填参数：service_id（目标服务ID）",
                })
                return
        }
        if req.MaxAcceptCost <= 0 {
                c.JSON(http.StatusBadRequest, gin.H{
                        "success": false,
                        "message": "MaxAcceptCost 必须大于 0（请设置客户端可接受的最大成本）",
                })
                return
        }

        // 3. 检查缓存：若过期或无目标服务，刷新缓存（最多重试 MaxSyncRetry 次）
        if needRefreshCache(req.ServiceID) {
                fmt.Printf("⚠️  缓存过期/无 %s 实例，开始刷新 C-SMA...\n", req.ServiceID)
                var syncErr error
                for i := 0; i < MaxSyncRetry; i++ {
                        if err := syncMetricsFromCSMA(); err != nil {
                                syncErr = err
                                fmt.Printf("🔄 重试拉取（%d/%d）：%v\n", i+1, MaxSyncRetry, err)
                                time.Sleep(RetryInterval)
                        } else {
                                syncErr = nil
                                break
                        }
                }
                if syncErr != nil {
                        c.JSON(http.StatusInternalServerError, gin.H{
                                "success": false,
                                "message": "获取服务实例失败：" + syncErr.Error(),
                        })
                        return
                }
        }

        // 4. 从缓存获取目标服务的实例（加读锁）
        mutex.RLock()
        targetInstances := cachedMetrics[req.ServiceID]
        mutex.RUnlock()

        // 5. 筛选符合客户端成本限制的实例
        qualified := filterInstances(targetInstances, req.MaxAcceptCost)
        if len(qualified) == 0 {
                c.JSON(http.StatusForbidden, gin.H{
                        "success": false,
                        "message": fmt.Sprintf("无符合条件的实例：%s 服务所有实例成本均超过 %d",
                                req.ServiceID, req.MaxAcceptCost),
                })
                return
        }

        // 6. 选择最优实例（成本最低 → 实例最多）
        bestInst := selectBestInstance(qualified)

        // 7. 返回结果给客户端
        c.JSON(http.StatusOK, gin.H{
                "success": true,
                "message": "路径选择成功",
                "result": map[string]interface{}{
                        "service_id":   req.ServiceID,
                        "csci_id":      bestInst.CSCI_ID,   // 客户端直接访问的地址
                        "cost":         bestInst.Cost,      // 实例成本（≤ MaxAcceptCost）
                        "available_gas": bestInst.Gas,      // 实例剩余数量
                        "decision_time": time.Now().Format("2006-01-02 15:04:05"),
                },
        })
}

// needRefreshCache：判断是否需要刷新缓存
func needRefreshCache(serviceID string) bool {
        mutex.RLock()
        defer mutex.RUnlock()
        // 缓存过期 或 无目标服务的实例 → 需要刷新
        return time.Since(lastSyncTime) > CacheExpire || len(cachedMetrics[serviceID]) == 0
}

// filterInstances：筛选成本 ≤ 客户端最大可接受成本的实例
func filterInstances(instances []models.ServiceInstanceInfo, maxCost int) []models.ServiceInstanceInfo {
        var qualified []models.ServiceInstanceInfo
        for _, inst := range instances {
                if inst.Cost <= maxCost {
                        qualified = append(qualified, inst)
                }
        }
        return qualified
}

// selectBestInstance：选择最优实例（排序规则：成本升序 → 实例数降序）
func selectBestInstance(instances []models.ServiceInstanceInfo) models.ServiceInstanceInfo {
        sort.Slice(instances, func(i, j int) bool {
                // 先按成本低的排前面
                if instances[i].Cost != instances[j].Cost {
                        return instances[i].Cost < instances[j].Cost
                }
                // 成本相同，实例数多的排前面（可用性更高）
                return instances[i].Gas > instances[j].Gas
        })
        return instances[0]
}

// ------------------------------
// 辅助函数：统计实例总数
// ------------------------------

// countTotalInstances：统计缓存中所有实例的总数
func countTotalInstances() int {
        total := 0
        for _, instances := range cachedMetrics {
                total += len(instances)
        }
        return total
}

// getCachedMetrics：查看当前缓存的 metrics（调试用）
func getCachedMetrics(c *gin.Context) {
        mutex.RLock()
        defer mutex.RUnlock()

        c.JSON(http.StatusOK, gin.H{
                "success":        true,
                "last_sync_time": lastSyncTime.Format("2006-01-02 15:04:05"),
                "cache_expire":   CacheExpire.String(),
                "service_count":  len(cachedMetrics),
                "total_instances": countTotalInstances(),
                "cached_data":    cachedMetrics,
        })
}
```

##### 2.3 保存退出
按 `Ctrl + O` → 按 `Enter` 保存文件 → 按 `Ctrl + X` 退出 `nano`。


#### 3. 启动 C-PS 并验证前置条件
##### 3.1 启动前的必备服务
确保以下服务已启动（C-PS 依赖它们）：
1. **公共服务平台**：`go run cmd/platform/main.go`（监听 8080）；
2. **服务站点**：`go run cmd/site/main.go`（监听 8082，已部署 `AR100` 实例）；
3. **C-SMA**：`go run cmd/c-sma/main.go`（监听 8083，能正常拉取服务站点 metrics）。

##### 3.2 启动 C-PS
在 `cmas-cats-go` 目录下执行命令：
```bash
go run cmd/c-ps/main.go
```

##### 3.3 验证 C-PS 启动成功
终端输出以下内容，说明 C-PS 启动正常，且已预加载 C-SMA 的 metrics：
```
✅ 启动时预加载 C-SMA metrics 成功，服务数：1

✅ C-PS 启动成功！监听端口：:8084 | C-SMA 地址：http://localhost:8083/sync
[GIN-debug] Listening and serving HTTP on :8084
```


#### 4. 验证 C-PS 核心功能（关键步骤）
##### 4.1 模拟客户端请求：获取最优服务实例
**新开一个终端**，执行以下命令，模拟客户端向 C-PS 请求 `AR100` 服务（`MaxAcceptCost=5`，即接受成本≤5的实例）：
```bash
# 取消代理（避免请求被拦截）
unset http_proxy
unset https_proxy

# 发送 POST 请求到 C-PS 的 /request-service 接口
curl -X POST http://localhost:8084/request-service \
-H "Content-Type: application/json" \
-d '{
  "service_id": "AR100",
  "max_accept_cost": 5,
  "max_accept_delay": 25
}'
```

##### 4.2 验证决策结果（正常情况）
返回以下内容，说明 C-PS 成功选择最优实例（`CSCI-ID` 为服务站点的 `ar100` 地址，成本 `4` ≤ 客户端的 `5`）：
```json
{
  "message": "路径选择成功，返回最优服务实例",
  "result": {
    "csci_id": "http://localhost:8082/ar100",
    "cost": 4,
    "decision_time": "2025-10-10 20:30:00",
    "gas": 2,
    "service_id": "AR100"
  },
  "success": true
}
```

##### 4.3 验证筛选逻辑（异常情况）
模拟客户端设置过低的 `MaxAcceptCost`（如 `3`，低于实例成本 `4`），验证 C-PS 会拒绝请求：
```bash
curl -X POST http://localhost:8084/request-service \
-H "Content-Type: application/json" \
-d '{
  "service_id": "AR100",
  "max_accept_cost": 3,
  "max_accept_delay": 25
}'
```
返回以下内容，说明筛选逻辑正常：
```json
{
  "message": "无符合条件的服务实例：AR100 服务的所有实例成本均超过客户端可接受的最大成本（3）",
  "success": false
}
```

##### 4.4 验证缓存功能（调试用）
执行以下命令，查看 C-PS 缓存的 metrics：
```bash
curl http://localhost:8084/cached-metrics
```
返回内容包含 `AR100` 实例数据和缓存时间，说明缓存功能正常。


### 三、常见问题及解决方案
| 问题现象 | 可能原因 | 解决方案 |
|----------|----------|----------|
| 启动报错：`undefined: sync` | 未导入 `sync` 包（缓存用读写锁） | 在 `import` 块中添加 `import "sync"` |
| 启动报错：`CSMA SyncURL undefined` | 常量名存在空格笔误（`CSMA SyncURL` → `CSMASyncURL`） | 修正常量名，确保所有调用处一致 |
| 客户端请求报错：`拉取服务实例数据失败` | C-SMA 未启动，或 `CSMASyncURL` 配置错误 | 1. 启动 C-SMA（`go run cmd/c-sma/main.go`） <br> 2. 确认 `CSMASyncURL` 为 `http://localhost:8083/sync` |
| 客户端请求返回“无符合条件的实例” | 1. 服务站点未部署 `AR100` 实例 <br> 2. 客户端 `MaxAcceptCost` 低于所有实例成本 | 1. 重新部署服务站点实例（`curl -X POST http://localhost:8082/deploy ...`） <br> 2. 提高客户端 `MaxAcceptCost`（如改为 `5`） |


## 
以下是实现 **数据持久化（SQLite）**、**延迟感知** 和 **多客户端API Key认证** 的详细修改方案，分模块逐步实施，确保与现有系统兼容：


# 一、数据持久化（SQLite）
## 1. 准备工作
安装SQLite驱动：
```bash
cd ~/go-work/src/cmas-cats-go
go get github.com/mattn/go-sqlite3  # SQLite Go驱动
```


## 2. 公共服务平台（`cmd/platform/main.go`）
### 2.1 修改存储逻辑（替换内存map为SQLite）
```go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3" // SQLite驱动
	"cmas-cats-go/models"
)

var db *sql.DB // 全局数据库连接

func main() {
	// 初始化SQLite
	initDB()
	defer db.Close()

	// 其他初始化逻辑...
}

// 初始化数据库
func initDB() {
	var err error
	// 打开数据库（不存在则创建）
	db, err = sql.Open("sqlite3", "./platform.db")
	if err != nil {
		panic("数据库连接失败: " + err.Error())
	}

	// 创建服务表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS services (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		input_format TEXT,
		computing_requirement TEXT,
		storage_requirement TEXT,
		computing_time TEXT,
		code_location TEXT,
		software_dependency TEXT, -- 存储JSON数组
		created_at DATETIME
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		panic("创建表失败: " + err.Error())
	}
}

// 修改服务注册接口
func registerServiceHandler(c *gin.Context) {
	var service models.Service
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 生成ID
	service.ID = "AR" + fmt.Sprintf("%d", time.Now().Unix()%1000)
	service.CreatedAt = time.Now().Format(time.RFC3339)

	// 序列化依赖列表为JSON
	deps, _ := json.Marshal(service.SoftwareDependency)

	// 存入数据库
	_, err := db.Exec(`
		INSERT INTO services (id, name, description, input_format, computing_requirement,
		storage_requirement, computing_time, code_location, software_dependency, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		service.ID, service.Name, service.Description, service.InputFormat,
		service.ComputingRequirement, service.StorageRequirement, service.ComputingTime,
		service.CodeLocation, string(deps), service.CreatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "存储失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "服务注册成功",
		"service_id": service.ID,
	})
}

// 修改获取服务列表接口
func getServicesHandler(c *gin.Context) {
	rows, err := db.Query("SELECT * FROM services")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var services []models.Service
	for rows.Next() {
		var s models.Service
		var depsStr string
		err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.InputFormat,
			&s.ComputingRequirement, &s.StorageRequirement, &s.ComputingTime,
			&s.CodeLocation, &depsStr, &s.CreatedAt)
		if err != nil {
			continue
		}
		// 解析JSON依赖
		json.Unmarshal([]byte(depsStr), &s.SoftwareDependency)
		services = append(services, s)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"count":    len(services),
		"services": services,
	})
}
```


## 3. 服务站点（`cmd/site/main.go`）
### 3.1 修改部署实例存储为SQLite
```go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"cmas-cats-go/models"
)

var db *sql.DB // 全局数据库连接

func main() {
	// 初始化SQLite
	initDB()
	defer db.Close()

	// 其他初始化逻辑...
}

// 初始化数据库
func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./site.db")
	if err != nil {
		panic("数据库连接失败: " + err.Error())
	}

	// 创建部署实例表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS deployed_services (
		id TEXT PRIMARY KEY,
		service_id TEXT NOT NULL,
		gas INT NOT NULL,
		cost INT NOT NULL,
		csci_id TEXT NOT NULL,
		created_at DATETIME,
		delay INT  -- 新增：延迟指标（ms）
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		panic("创建表失败: " + err.Error())
	}
}

// 修改部署接口（增加延迟字段）
func deployHandler(c *gin.Context) {
	var req struct {
		ServiceID string `json:"service_id"`
		Gas       int    `json:"gas"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 生成实例ID和CSCI地址
	instanceID := req.ServiceID + "-" + fmt.Sprintf("%d", time.Now().Unix()%1000)
	csciID := fmt.Sprintf("http://localhost:8082/%s", instanceID)
	cost := req.Gas * 2 // 成本计算逻辑
	delay := 10 + req.Gas%5 // 模拟延迟（10-15ms）

	// 存入数据库
	_, err := db.Exec(`
		INSERT INTO deployed_services (id, service_id, gas, cost, csci_id, created_at, delay)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		instanceID, req.ServiceID, req.Gas, cost, csciID,
		time.Now().Format(time.RFC3339), delay)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "部署失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("部署成功：%d个%s实例", req.Gas, req.ServiceID),
		"info": map[string]interface{}{
			"service_id": req.ServiceID,
			"gas":        req.Gas,
			"cost":       cost,
			"csci_id":    csciID,
			"delay":      delay, // 返回延迟
		},
	})
}

// 修改metrics接口（返回延迟）
func metricsHandler(c *gin.Context) {
	rows, err := db.Query("SELECT service_id, gas, cost, csci_id, delay FROM deployed_services")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var metrics []models.ServiceInstanceInfo
	for rows.Next() {
		var m models.ServiceInstanceInfo
		err := rows.Scan(&m.ServiceID, &m.Gas, &m.Cost, &m.CSCI_ID, &m.Delay)
		if err != nil {
			continue
		}
		metrics = append(metrics, m)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"metrics": metrics,
	})
}
```


# 二、延迟感知
## 1. 数据模型修改（`models/instance.go`）
```go
package models

// 服务实例信息（增加延迟字段）
type ServiceInstanceInfo struct {
	ServiceID string `json:"service_id"`
	Gas       int    `json:"gas"`
	Cost      int    `json:"cost"`
	CSCI_ID   string `json:"csci_id"`
	Delay     int    `json:"delay"` // 新增：延迟（ms）
}
```


## 2. C-SMA修改（`cmd/c-sma/main.go`）
确保聚合metrics时包含延迟字段（无需大幅修改，只需确保`ServiceInstanceInfo`结构体正确解析延迟字段）。
```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"cmas-cats-go/models" // 依赖包含Delay字段的ServiceInstanceInfo模型
)

// C-SMA核心配置（多站点支持）
const (
	ListenPort     = ":8083" // C-SMA监听端口
	PollInterval   = 10 * time.Second // 拉取metrics间隔
	// 服务站点列表（可添加多个，用逗号分隔）
	ServiceSiteList = "http://localhost:8082/metrics,http://localhost:8085/metrics"
)

// 全局状态：聚合后的metrics数据（含延迟字段）
var (
	// key: ServiceID, value: 该服务在所有站点的实例（包含Delay字段）
	aggregatedMetrics = make(map[string][]models.ServiceInstanceInfo)
	metricsMutex      sync.RWMutex // 并发安全锁
)

func main() {
	// 初始化Gin引擎
	r := gin.Default()

	// 注册API接口
	r.GET("/sync", syncToCPSHandler)          // 供C-PS拉取聚合数据
	r.GET("/current-metrics", getMetricsHandler) // 调试用：查看当前聚合数据

	// 启动多站点metrics拉取任务（后台协程）
	go startMultiSitePolling()

	// 启动服务
	fmt.Printf("✅ C-SMA启动成功！监听端口：%s | 拉取间隔：%v | 监控站点数：%d\n",
		ListenPort, PollInterval, len(parseSiteList(ServiceSiteList)))
	if err := r.Run(ListenPort); err != nil {
		panic("❌ C-SMA启动失败：" + err.Error())
	}
}

// ------------------------------
// 核心功能：多站点metrics拉取与聚合（含延迟字段处理）
// ------------------------------

// startMultiSitePolling：启动多站点定期拉取任务
func startMultiSitePolling() {
	sites := parseSiteList(ServiceSiteList)
	if len(sites) == 0 {
		fmt.Println("⚠️ 未配置服务站点，无法拉取metrics")
		return
	}

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Printf("\n[%s] 📥 开始拉取 %d 个服务站点的metrics...\n",
			time.Now().Format("2006-01-02 15:04:05"), len(sites))

		var wg sync.WaitGroup
		for _, siteURL := range sites {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				// 拉取单个站点的metrics（包含Delay字段）
				siteMetrics, err := fetchSingleSiteMetrics(url)
				if err != nil {
					fmt.Printf("❌ 拉取站点 [%s] 失败：%v\n", url, err)
					return
				}
				// 聚合数据（覆盖该站点旧数据，保留Delay）
				metricsMutex.Lock()
				aggregateMultiSiteMetrics(url, siteMetrics)
				metricsMutex.Unlock()

				fmt.Printf("✅ 拉取站点 [%s] 成功：%d个实例（含延迟数据）\n", url, len(siteMetrics))
			}(siteURL)
		}

		wg.Wait()
		fmt.Printf("[%s] 📊 所有站点拉取完成，当前聚合服务数：%d\n",
			time.Now().Format("2006-01-02 15:04:05"), len(aggregatedMetrics))
	}
}

// fetchSingleSiteMetrics：拉取单个站点的metrics（解析Delay字段）
func fetchSingleSiteMetrics(siteURL string) ([]models.ServiceInstanceInfo, error) {
	resp, err := http.Get(siteURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码错误：%d（期望200）", resp.StatusCode)
	}

	// 解析站点响应（包含Delay字段的ServiceInstanceInfo）
	var siteResp struct {
		Success bool                     `json:"success"`
		Metrics []models.ServiceInstanceInfo `json:"metrics"` // 包含Delay字段
		Message string                   `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&siteResp); err != nil {
		return nil, fmt.Errorf("JSON解析失败（可能缺少Delay字段）：%w", err)
	}

	if !siteResp.Success {
		return nil, fmt.Errorf("站点返回错误：%s", siteResp.Message)
	}

	return siteResp.Metrics, nil
}

// aggregateMultiSiteMetrics：聚合多站点数据（保留Delay字段，去重同站点旧数据）
func aggregateMultiSiteMetrics(siteURL string, newMetrics []models.ServiceInstanceInfo) {
	// 提取站点标识（用于区分不同站点的实例）
	siteID := strings.TrimSuffix(siteURL, "/metrics")

	// 1. 删除该站点的旧实例（避免重复）
	for serviceID, oldInstances := range aggregatedMetrics {
		var retainedInstances []models.ServiceInstanceInfo
		for _, inst := range oldInstances {
			// 保留非当前站点的实例（通过CSCI-ID包含站点标识判断）
			if !strings.Contains(inst.CSCI_ID, siteID) {
				retainedInstances = append(retainedInstances, inst)
			}
		}
		aggregatedMetrics[serviceID] = retainedInstances
	}

	// 2. 追加新实例（包含Delay字段）
	for _, newInst := range newMetrics {
		serviceID := newInst.ServiceID
		// 直接追加，包含所有字段（ID、Gas、Cost、CSCI_ID、Delay）
		aggregatedMetrics[serviceID] = append(aggregatedMetrics[serviceID], newInst)
	}
}

// ------------------------------
// 辅助函数：站点列表解析
// ------------------------------

func parseSiteList(listStr string) []string {
	var validSites []string
	for _, site := range strings.Split(listStr, ",") {
		trimmedSite := strings.TrimSpace(site)
		if trimmedSite != "" && strings.HasSuffix(trimmedSite, "/metrics") {
			validSites = append(validSites, trimmedSite)
		}
	}
	return validSites
}

// ------------------------------
// API接口实现（供C-PS和调试）
// ------------------------------

// syncToCPSHandler：向C-PS提供聚合后的metrics（包含Delay字段）
func syncToCPSHandler(c *gin.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	// 转换为C-PS需要的格式（包含所有实例的Delay字段）
	var syncData []struct {
		ServiceID string                     `json:"service_id"`
		Instances []models.ServiceInstanceInfo `json:"instances"` // 包含Delay
		TotalGas  int                        `json:"total_gas"`
		MinDelay  int                        `json:"min_delay"` // 新增：该服务的最小延迟（辅助C-PS决策）
	}

	for serviceID, instances := range aggregatedMetrics {
		// 计算总实例数和最小延迟
		totalGas := 0
		minDelay := 1 << 30 // 初始化为较大值
		for _, inst := range instances {
			totalGas += inst.Gas
			if inst.Delay < minDelay {
				minDelay = inst.Delay
			}
		}

		syncData = append(syncData, struct {
			ServiceID string                     `json:"service_id"`
			Instances []models.ServiceInstanceInfo `json:"instances"`
			TotalGas  int                        `json:"total_gas"`
			MinDelay  int                        `json:"min_delay"`
		}{
			ServiceID: serviceID,
			Instances: instances, // 包含所有实例的Delay
			TotalGas:  totalGas,
			MinDelay:  minDelay,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"sync_time":   time.Now().Format("2006-01-02 15:04:05"),
		"service_num": len(syncData),
		"data":        syncData,
	})
}

// getMetricsHandler：调试接口（展示包含Delay的完整数据）
func getMetricsHandler(c *gin.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"last_update_time": time.Now().Format("2006-01-02 15:04:05"),
		"monitored_sites":  len(parseSiteList(ServiceSiteList)),
		"aggregated_data":  aggregatedMetrics, // 包含所有实例的Delay字段
	})
}
```

## 3. C-PS修改（`cmd/c-ps/main.go`）
更新路径选择算法，同时考虑成本和延迟：
```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"cmas-cats-go/models"
)

// C-PS核心配置
const (
	ListenPort     = ":8084"                          // C-PS监听端口
	CSMASyncURL    = "http://localhost:8083/sync"     // C-SMA同步接口地址
	MaxSyncRetry   = 3                                // 同步重试次数
	RetryInterval  = 2 * time.Second                  // 重试间隔
	CacheExpire    = 5 * time.Minute                  // 缓存过期时间
)

// 全局状态：缓存C-SMA的metrics数据（含延迟字段）
var (
	cachedMetrics = make(map[string][]models.ServiceInstanceInfo) // key=ServiceID
	lastSyncTime  time.Time                                      // 上次同步时间
	mutex         sync.RWMutex                                   // 读写锁
)

// 合法API Key列表（生产环境建议存储在数据库）
var validAPIKeys = map[string]bool{
	"client-001": true,  // 示例客户端1
	"client-002": true,  // 示例客户端2
	"client-003": true,  // 示例客户端3
}

func main() {
	// 初始化Gin引擎
	r := gin.Default()

	// 注册路由（核心接口添加认证中间件）
	r.POST("/request-service", authMiddleware(), handleClientRequest) // 客户端请求接口（需认证）
	r.GET("/refresh-metrics", refreshMetricsCache)                    // 刷新缓存（调试用）
	r.GET("/cached-metrics", getCachedMetrics)                        // 查看缓存（调试用）

	// 启动前预加载C-SMA数据
	if err := syncMetricsFromCSMA(); err != nil {
		fmt.Printf("⚠️ 预加载C-SMA数据失败：%v（后续会自动重试）\n", err)
	} else {
		fmt.Printf("✅ 预加载成功！缓存了 %d 个服务的数据\n", len(cachedMetrics))
	}

	// 启动服务
	fmt.Printf("\n✅ C-PS启动成功！监听端口：%s | C-SMA地址：%s\n", ListenPort, CSMASyncURL)
	if err := r.Run(ListenPort); err != nil {
		panic("❌ C-PS启动失败：" + err.Error())
	}
}

// ------------------------------
// 核心1：API Key认证中间件
// ------------------------------

// authMiddleware：验证客户端API Key
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取API Key
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "缺少X-API-Key请求头",
			})
			c.Abort() // 终止请求处理
			return
		}

		// 验证API Key有效性
		if !validAPIKeys[apiKey] {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无效的API Key",
			})
			c.Abort()
			return
		}

		// 认证通过，继续处理请求
		c.Next()
	}
}

// ------------------------------
// 核心2：从C-SMA同步metrics（含延迟数据）
// ------------------------------

// syncMetricsFromCSMA：拉取并更新缓存
func syncMetricsFromCSMA() error {
	// 发送请求到C-SMA
	resp, err := http.Get(CSMASyncURL)
	if err != nil {
		return fmt.Errorf("请求C-SMA失败：%w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("C-SMA返回错误状态码：%d", resp.StatusCode)
	}

	// 解析响应数据（包含延迟字段）
	var csmaResp struct {
		Success    bool                     `json:"success"`
		ServiceNum int                      `json:"service_num"`
		Data       []struct {
			ServiceID string                     `json:"service_id"`
			Instances []models.ServiceInstanceInfo `json:"instances"` // 包含Delay字段
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&csmaResp); err != nil {
		return fmt.Errorf("解析C-SMA响应失败：%w", err)
	}

	if !csmaResp.Success {
		return fmt.Errorf("C-SMA业务错误：%s", csmaResp.Message)
	}

	// 更新缓存（加写锁）
	mutex.Lock()
	defer mutex.Unlock()
	cachedMetrics = make(map[string][]models.ServiceInstanceInfo) // 清空旧数据
	for _, item := range csmaResp.Data {
		cachedMetrics[item.ServiceID] = item.Instances // 保存包含延迟的实例数据
	}
	lastSyncTime = time.Now()

	// 打印同步日志
	fmt.Printf("[%s] 📥 同步C-SMA成功：%d个服务，共%d个实例\n",
		lastSyncTime.Format("2006-01-02 15:04:05"),
		len(cachedMetrics),
		countTotalInstances())

	return nil
}

// refreshMetricsCache：手动刷新缓存（调试接口）
func refreshMetricsCache(c *gin.Context) {
	if err := syncMetricsFromCSMA(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "刷新缓存失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "缓存刷新成功",
		"service_count": len(cachedMetrics),
		"last_sync":     lastSyncTime.Format("2006-01-02 15:04:05"),
	})
}

// ------------------------------
// 核心3：处理客户端请求（成本+延迟双筛选）
// ------------------------------

// handleClientRequest：接收客户端请求并返回最优实例
func handleClientRequest(c *gin.Context) {
	// 1. 解析客户端请求
	var req models.ClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	// 2. 验证请求参数
	if req.ServiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少必填参数：service_id",
		})
		return
	}
	if req.MaxAcceptCost <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "max_accept_cost必须大于0",
		})
		return
	}
	if req.MaxAcceptDelay <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "max_accept_delay必须大于0",
		})
		return
	}

	// 3. 检查是否需要刷新缓存
	if needRefreshCache(req.ServiceID) {
		fmt.Printf("⚠️ 缓存过期或无%s实例，开始刷新C-SMA...\n", req.ServiceID)
		var syncErr error
		for i := 0; i < MaxSyncRetry; i++ {
			if err := syncMetricsFromCSMA(); err != nil {
				syncErr = err
				fmt.Printf("🔄 重试拉取（%d/%d）：%v\n", i+1, MaxSyncRetry, err)
				time.Sleep(RetryInterval)
			} else {
				syncErr = nil
				break
			}
		}
		if syncErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "获取服务实例失败：" + syncErr.Error(),
			})
			return
		}
	}

	// 4. 获取目标服务的实例
	mutex.RLock()
	targetInstances := cachedMetrics[req.ServiceID]
	mutex.RUnlock()

	// 5. 筛选符合条件的实例（成本≤最大可接受成本 且 延迟≤最大可接受延迟）
	qualified := filterInstances(targetInstances, req.MaxAcceptCost, req.MaxAcceptDelay)
	if len(qualified) == 0 {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": fmt.Sprintf("无符合条件的实例：%s服务所有实例的成本或延迟超过限制", req.ServiceID),
		})
		return
	}

	// 6. 选择最优实例（成本优先，延迟为辅）
	bestInst := selectBestInstance(qualified)

	// 7. 返回结果
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "路径选择成功",
		"result": map[string]interface{}{
			"service_id":    req.ServiceID,
			"csci_id":       bestInst.CSCI_ID,   // 客户端直接访问的地址
			"cost":          bestInst.Cost,      // 实例成本
			"delay":         bestInst.Delay,     // 实例延迟（ms）
			"available_gas": bestInst.Gas,       // 可用实例数量
			"decision_time": time.Now().Format("2006-01-02 15:04:05"),
		},
	})
}

// needRefreshCache：判断是否需要刷新缓存
func needRefreshCache(serviceID string) bool {
	mutex.RLock()
	defer mutex.RUnlock()
	return time.Since(lastSyncTime) > CacheExpire || len(cachedMetrics[serviceID]) == 0
}

// filterInstances：双重筛选（成本+延迟）
func filterInstances(instances []models.ServiceInstanceInfo, maxCost, maxDelay int) []models.ServiceInstanceInfo {
	var qualified []models.ServiceInstanceInfo
	for _, inst := range instances {
		// 同时满足成本和延迟限制
		if inst.Cost <= maxCost && inst.Delay <= maxDelay {
			qualified = append(qualified, inst)
		}
	}
	return qualified
}

// selectBestInstance：选择最优实例（成本优先，延迟为辅）
func selectBestInstance(instances []models.ServiceInstanceInfo) models.ServiceInstanceInfo {
	// 排序规则：
	// 1. 成本低的优先
	// 2. 成本相同则延迟低的优先
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Cost != instances[j].Cost {
			return instances[i].Cost < instances[j].Cost
		}
		return instances[i].Delay < instances[j].Delay
	})
	return instances[0]
}

// ------------------------------
// 辅助函数与调试接口
// ------------------------------

// countTotalInstances：统计实例总数
func countTotalInstances() int {
	total := 0
	for _, instances := range cachedMetrics {
		total += len(instances)
	}
	return total
}

// getCachedMetrics：查看缓存数据（调试用）
func getCachedMetrics(c *gin.Context) {
	mutex.RLock()
	defer mutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"last_sync_time":  lastSyncTime.Format("2006-01-02 15:04:05"),
		"cache_expire":    CacheExpire.String(),
		"service_count":   len(cachedMetrics),
		"total_instances": countTotalInstances(),
		"cached_data":     cachedMetrics, // 包含所有实例的延迟数据
	})
}
    
```


# 三、多客户端API Key认证
## 1. C-PS添加认证中间件（`cmd/c-ps/main.go`）
```go
// API Key存储（实际生产环境应存在数据库）
var validAPIKeys = map[string]bool{
	"client-123": true,
	"client-456": true,
}

// 认证中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if !validAPIKeys[apiKey] {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "无效的API Key"})
			c.Abort() // 终止请求
			return
		}
		c.Next() // 继续处理请求
	}
}

// 在路由注册时使用中间件
func main() {
	r := gin.Default()
	
	// 对客户端请求接口添加认证
	r.POST("/request-service", authMiddleware(), handleClientRequest)
	
	// 其他路由...
}
```


## 2. 客户端调用方式修改
```bash
# 带API Key的请求示例
curl -X POST http://localhost:8084/request-service \
-H "Content-Type: application/json" \
-H "X-API-Key: client-123" \
-d '{
  "service_id": "AR100",
  "max_accept_cost": 5,
  "max_accept_delay": 20
}'
```

## 终端操作（集成）
```bash
cd ~/go-work/src/cmas-cats-go
go run cmd/platform/main.go
go run cmd/site/main.go
go run cmd/site2/main.go
go run cmd/c-sma/main.go
go run cmd/c-ps/main.go
```

**新开一个终端**，执行以下命令（先取消代理，避免请求被拦截）：
```bash
# 1. 取消 HTTP 代理（若系统配置了代理，必须执行）
unset http_proxy
unset https_proxy

# 2. 测试服务列表查询接口（无服务时返回空列表，状态码 200）
curl http://localhost:8080/api/v1/services -i

# 3.服务注册, 发送 POST 请求，注册 AR/VR 服务
curl -X POST http://localhost:8080/api/v1/services \
-H "Content-Type: application/json" \
-d '{"name":"AR/VR","description":"接收传感器输入生成AR场景","input_format":"Motion Capture","computing_requirement":"CPU≥2.0GHz, GPU>RTX4060","storage_requirement":"16GB DRAM","computing_time":"≤1ms","code_location":"https://github.com/xxx/ar","software_dependency":["Unity"],"validation_sample":"test.mp4","validation_result":"result.json"}'

# 4.再次查询服务列表，确认注册成功（返回 count=1，包含 AR100 服务）
curl http://localhost:8080/api/v1/services

# 5.向服务站点发送部署请求（部署 2 个 AR100 实例）
curl -X POST http://localhost:8082/deploy \
-H "Content-Type: application/json" \
-d '{"service_id":"AR100","gas":2}'

curl -X POST http://localhost:8085/deploy \
-H "Content-Type: application/json" \
-d '{"service_id":"AR100","gas":2}'

# 6. 查看部署结果（返回“部署成功”，包含实例信息）查看 metrics 接口，确认已部署服务能被 C-SMA 拉取
curl http://localhost:8082/metrics
curl http://localhost:8085/metrics

# 7.调用 C-SMA 的 `/current-metrics` 接口，可看到监控的站点数和聚合数据：

curl http://localhost:8083/current-metrics
# 8.验证“同步给 C-PS”功能,调用 C-SMA 的 `/sync` 接口（模拟 C-PS 拉取数据）可看到按服务分组的多站点实例数据：
curl http://localhost:8083/sync

# 9.发送 POST 请求到 C-PS 的 /request-service 接口
# 带API Key的请求示例
curl -X POST http://localhost:8084/request-service \
-H "Content-Type: application/json" \
-H "X-API-Key: client-001" \
-d '{
  "service_id": "AR100",
  "max_accept_cost": 5,
  "max_accept_delay": 20
}'

# 10.模拟客户端设置过低的 `MaxAcceptCost`（如 `3`，低于实例成本 `4`），验证 C-PS 会拒绝请求：
curl -X POST http://localhost:8084/request-service \
-H "Content-Type: application/json" \
-H "X-API-Key: client-002" \
-d '{
  "service_id": "AR100",
  "max_accept_cost": 3,
  "max_accept_delay": 25
}'

# 11.验证缓存功能（调试用）执行以下命令，查看 C-PS 缓存的 metrics：
curl http://localhost:8084/cached-metrics
```
