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
 	Delay     int    `json:"delay"` // 新增：延迟（ms）
}

// ClientRequest 客户端向C-PS发起的服务请求结构（草案中Client Service Request）
// 包含客户端的服务需求、成本和延迟限制
type ClientRequest struct {
	ServiceID     string `json:"service_id"`     // 目标服务ID，如 "AR1"
	MaxAcceptCost int    `json:"max_accept_cost"`// 客户端可接受的最高成本，如 5（超过此值的服务实例会被过滤）
	MaxAcceptDelay int   `json:"max_accept_delay"`// 客户端可接受的最大总延迟（毫秒），如 25（计算延迟+网络延迟）
}
