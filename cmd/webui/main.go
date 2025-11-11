package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Resource 表示可用的服务器资源
type Resource struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	URL              string  `json:"url"`
	Price            float64 `json:"price"`
	AvailableStorage string  `json:"availableStorage"`
	Latency          string  `json:"latency"`
	Location         string  `json:"location"`
	CPU              string  `json:"cpu"`
	Memory           string  `json:"memory"`
}

// DeployRequest 部署请求结构
type DeployRequest struct {
	SiteID string `json:"site_id" binding:"required"`
}

// DeployResponse 部署响应结构
type DeployResponse struct {
	Success     bool     `json:"success"`
	Message     string   `json:"message"`
	Site        Resource `json:"site"`
	Files       []string `json:"files"`
	StartScript string   `json:"startScript"`
}

// StopRequest 停止请求结构
type StopRequest struct {
	SiteID string `json:"site_id" binding:"required"`
}

// StatusResponse 状态响应结构
type StatusResponse struct {
	Success       bool                  `json:"success"`
	SiteID        string                `json:"site_id"`
	Status        string                `json:"status"`
	ServicesCount int                   `json:"services_count"`
	Metrics       []ServiceInstanceInfo `json:"metrics"`
	Time          string                `json:"time"`
}

// ServiceInstanceInfo 服务实例信息，与models/service.go保持一致
type ServiceInstanceInfo struct {
	ServiceID string `json:"service_id"`
	Gas       int    `json:"gas"`
	Cost      int    `json:"cost"`
	CSCI_ID   string `json:"csci_id"`
	Delay     int    `json:"delay"`
}

// 从C-SMA服务获取真实的资源信息
func getRealResourcesFromCSMA() ([]Resource, error) {
	// 直接从配置获取真实站点地址，而不是从CSCI_ID解析
	site1RealURL := "http://192.168.235.48:8081"
	site2RealURL := "http://192.168.67.159:8085"
	
	resources := []Resource{}
	
	// 获取Site1的实时资源信息
	site1Info := getRealResourceInfoFromSite(site1RealURL)
	if site1Info != nil {
		site1Info.ID = "site1"
		site1Info.Name = "服务器节点 Site-1 (Linux)"
		resources = append(resources, *site1Info)
	}
	
	// 获取Site2的实时资源信息
	site2Info := getRealResourceInfoFromSite(site2RealURL)
	if site2Info != nil {
		site2Info.ID = "site2"
		site2Info.Name = "服务器节点 Site-2 (Mac)"
		resources = append(resources, *site2Info)
	}
	
	return resources, nil
}

// getRealResourceInfoFromSite 从实际站点获取实时资源信息
func getRealResourceInfoFromSite(siteURL string) *Resource {
	resource := &Resource{
		ID:               extractSiteIDFromURL(siteURL),
		Name:             fmt.Sprintf("服务器节点 %s", extractSiteIDFromURL(siteURL)),
		URL:              siteURL,
		Price:            0.5,
		AvailableStorage: "未知",
		Latency:          "未知",
		Location:         getLocationFromURL(siteURL),
		CPU:              "未知",
		Memory:           "未知",
	}

	// 创建不使用代理的客户端
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: nil, // 禁用代理
		},
		Timeout: 5 * time.Second,
	}

	// 尝试从站点的resource-status接口获取信息
	resourceStatusURL := siteURL + "/resource-status"
	resp, err := client.Get(resourceStatusURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		
		var statusData struct {
			Success bool `json:"success"`
			SiteID  string `json:"site_id"`
			Resource struct {
				Total      string `json:"total"`
				Used       string `json:"used"`
				Remaining  string `json:"remaining"`
				UsageRate  string `json:"usage_rate"`
			} `json:"resource"`
			CostConversion string `json:"cost_conversion"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&statusData); err == nil && statusData.Success {
			// 更新资源信息
			resource.AvailableStorage = statusData.Resource.Remaining + " 单位"
			resource.CPU = fmt.Sprintf("使用率 %s", statusData.Resource.UsageRate)
			resource.Memory = fmt.Sprintf("已用 %s / 总 %s 单位", statusData.Resource.Used, statusData.Resource.Total)
		}
	}

	// 尝试从站点的health接口获取延迟信息
	healthURL := siteURL + "/health"
	resp, err = client.Get(healthURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		
		var healthData struct {
			Success bool `json:"success"`
			Status  string `json:"status"`
			SiteID  string `json:"site_id"`
			Time    string `json:"time"`
			ResourceStatus struct {
				Status    string `json:"status"`
				Used      string `json:"used"`
				UsageRate string `json:"usage_rate"`
			} `json:"resource_status"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&healthData); err == nil && healthData.Success {
			// 更新延迟信息
			resource.Latency = "正常"
			if healthData.ResourceStatus.Status != "healthy" {
				resource.Latency = "警告: " + healthData.ResourceStatus.Status
			}
		}
	}

	return resource
}

// extractSiteIDFromURL 从URL中提取站点ID
func extractSiteIDFromURL(urlStr string) string {
	// 从URL中提取站点ID，对于localhost的情况，使用端口来区分
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "unknown"
	}
	
	host := parsed.Host
	if host == "" {
		host = parsed.Path // 回退到路径
	}
	
	// 对于localhost，包含端口来区分不同站点
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") {
		return host // 例如: localhost:8082, localhost:8085
	}
	
	// 对于其他情况，去掉端口号之后的部分
	for i, ch := range host {
		if ch == ':' || ch == '/' || ch == '?' || ch == '#' {
			return host[:i]
		}
	}
	
	return host
}

// getLocationFromURL 从URL获取位置信息
func getLocationFromURL(urlStr string) string {
	// 从URL提取主机和端口信息
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "未知位置"
	}

	host := parsed.Host
	if host == "" {
		host = parsed.Path
	}
	
	// 根据端口返回对应的站点信息
	if strings.Contains(host, "localhost:8082") || strings.Contains(host, "127.0.0.1:8082") {
		return "Site-1 (Linux)"
	} else if strings.Contains(host, "localhost:8085") || strings.Contains(host, "127.0.0.1:8085") {
		return "Site-2 (Mac)"
	} else if strings.Contains(host, "localhost") || strings.HasPrefix(host, "127.") {
		return "本地服务器"
	} else if strings.Contains(host, "192.168") {
		return "内网服务器"
	} else if strings.Contains(host, "10.") || strings.Contains(host, "172.") {
		return "内网服务器"
	}

	// 如果是具体的IP地址，可以根据IP段推测位置
	// 或者可以使用IP地理位置服务来获取真实位置信息
	return "服务器(" + host + ")"
}

func main() {
	r := gin.Default()

	// 设置多部分形式上传的最大内存为 8 MB
	r.MaxMultipartMemory = 8 << 20

	// API路由组 - 首先定义API路由，避免与静态文件路由冲突
	api := r.Group("/api")
	{
		api.GET("/resources", getResources)
		api.POST("/deploy", deployCode)
		api.POST("/stop", stopCode)
		api.GET("/status/:siteId", getStatus)
	}

	// 静态文件服务，提供前端页面和静态资源为特定的静态文件类型提供服务，而不是使用通配符
	r.StaticFile("/", "./index.html")           // 主页
	r.StaticFile("/index.html", "./index.html") // 明确指定index.html
	r.Static("/static", "./static")             // 静态资源目录
	r.Static("/assets", "./assets")             // 资源目录

	// 如果请求不是API也不是静态文件，则返回index.html（用于SPA）
	r.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path != "/" &&
			!strings.HasPrefix(c.Request.URL.Path, "/api") &&
			!strings.HasPrefix(c.Request.URL.Path, "/static") &&
			!strings.HasPrefix(c.Request.URL.Path, "/assets") {
			c.File("./index.html")
		}
	})

	// 启动服务器
	fmt.Println("🚀 Web界面服务器启动在 :9091")
	fmt.Println("🌐 访问 http://localhost:9091 查看界面")
	r.Run(":9091")
}

// getResources 获取可用资源
func getResources(c *gin.Context) {
	resources, err := getRealResourcesFromCSMA()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取资源信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resources)
}

// deployCode 部署代码到指定站点
func deployCode(c *gin.Context) {
	// 解析表单数据
	siteID := c.PostForm("site_id")
	if siteID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "必须指定站点ID",
		})
		return
	}

	// 从C-SMA获取最新的资源信息
	availableResources, err := getRealResourcesFromCSMA()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取资源信息失败: " + err.Error(),
		})
		return
	}

	// 查找目标站点
	var targetSite *Resource
	for i := range availableResources {
		if availableResources[i].ID == siteID {
			targetSite = &availableResources[i]
			break
		}
	}

	if targetSite == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "未找到指定的站点",
		})
		return
	}

	// 根据目标站点的类型，采用不同的部署策略
	switch targetSite.ID {
	case "site1":
		// Site1: 使用/deploy接口，不支持文件上传
		// 获取服务类型（从启动脚本文件名推断或使用默认值）
		serviceType := c.PostForm("startScript")
		if serviceType == "" {
			serviceType = "AR100" // 默认服务类型
		}
		
		// 直接调用部署接口
		err = deployToSite1(targetSite.URL, serviceType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "部署到site1失败: " + err.Error(),
			})
			return
		}
		
		c.JSON(http.StatusOK, DeployResponse{
			Success:     true,
			Message:     fmt.Sprintf("服务已部署到 %s", targetSite.Name),
			Site:        *targetSite,
			Files:       []string{}, // site1不支持文件上传
			StartScript: serviceType,
		})
		
	case "site2":
		// Site2: 使用/upload接口，支持文件上传
		// 获取上传的文件
		file, err := c.FormFile("codeFiles")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "获取代码文件失败: " + err.Error(),
			})
			return
		}

		// 确保uploads目录存在
		uploadDir := "./uploads"
		if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
			os.MkdirAll(uploadDir, 0755)
		}

		// 保存上传的ZIP文件
		zipFilename := filepath.Join(uploadDir, file.Filename)
		if err := c.SaveUploadedFile(file, zipFilename); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "保存ZIP文件失败: " + err.Error(),
			})
			return
		}

		// 读取ZIP文件内容
		zipData, err := os.ReadFile(zipFilename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "读取ZIP文件失败: " + err.Error(),
			})
			return
		}

		// 读取startScript参数
		startScriptName := c.PostForm("startScript")

		// 上传到site2
		err = uploadToSite2(targetSite.URL, zipData, startScriptName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "上传到site2失败: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, DeployResponse{
			Success:     true,
			Message:     fmt.Sprintf("代码已成功上传到 %s", targetSite.Name),
			Site:        *targetSite,
			Files:       []string{file.Filename},
			StartScript: startScriptName,
		})
		
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "不支持的站点类型: " + targetSite.ID,
		})
	}
}

// deployToSite1 向site1发送部署请求（使用/deploy接口）
func deployToSite1(serverURL string, serviceType string) error {
	// Site1的/deploy接口需要service_id和gas参数
	// 由于site1不支持自定义上传，我们使用预设的服务类型
	deployData := struct {
		ServiceID string `json:"service_id"`
		Gas       int    `json:"gas"`
	}{
		ServiceID: serviceType, // 使用用户选择的脚本名称作为服务类型
		Gas:       1,           // 默认部署1个实例
	}

	jsonData, err := json.Marshal(deployData)
	if err != nil {
		return fmt.Errorf("序列化部署数据失败: %v", err)
	}

	client := &http.Client{
		Timeout: 60 * time.Second, // 增加到60秒，给Site1更多时间
		Transport: &http.Transport{
			Proxy: nil, // 禁用代理
		},
	}

	deployURL := serverURL + "/deploy"
	resp, err := client.Post(deployURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("部署请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("部署失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	var deployResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Info    struct {
			ServiceID string `json:"service_id"`
			Gas       int    `json:"gas"`
			Cost      int    `json:"cost"`
			CSCI_ID   string `json:"csci_id"`
			Delay     int    `json:"delay"`
		} `json:"info"`
	}

	if err := json.Unmarshal(respBody, &deployResp); err != nil {
		return fmt.Errorf("解析部署响应失败: %v", err)
	}

	if !deployResp.Success {
		return fmt.Errorf("部署失败: %s", deployResp.Message)
	}

	fmt.Printf("✅ Site1部署成功: %s\n", deployResp.Message)
	return nil
}

// uploadToSite2 向site2上传文件（使用/upload接口）
func uploadToSite2(serverURL string, zipData []byte, startScriptName string) error {
	// 创建一个临时的多部分表单
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// 添加ZIP数据
	part, err := writer.CreateFormFile("file", "code.zip")
	if err != nil {
		return err
	}
	_, err = part.Write(zipData)
	if err != nil {
		return err
	}

	// 添加startScript参数
	err = writer.WriteField("startScript", startScriptName)
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	// 发送POST请求到目标服务器的上传端点
	uploadURL := serverURL + "/upload"
	client := &http.Client{Timeout: 60 * time.Second} // 增加超时时间
	req, err := http.NewRequest("POST", uploadURL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查响应
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var uploadResponse struct {
		Success     bool   `json:"success"`
		Message     string `json:"message"`
		ExtractPath string `json:"extractPath"`
		StartScript string `json:"startScript"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &uploadResponse); err != nil {
		return fmt.Errorf("解析上传响应失败: %v", err)
	}

	if !uploadResponse.Success {
		return fmt.Errorf("上传失败: %s", uploadResponse.Error)
	}

	fmt.Printf("✅ Site2文件上传成功: %s\n", uploadResponse.Message)

	// 暂时不执行脚本，因为执行端点可能不存在
	// 如果有startScript，可以后续手动执行
	if startScriptName != "" {
		fmt.Printf("ℹ️ 启动脚本 %s 已上传，可以在路径 %s 下手动执行\n", startScriptName, uploadResponse.ExtractPath)
	}

	return nil
}

// executeStartScriptOnServer 在目标服务器上执行start.sh

// stopCode 停止在指定站点上运行的服务
func stopCode(c *gin.Context) {
	var req StopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求格式错误: " + err.Error(),
		})
		return
	}

	// 从C-SMA获取最新的资源信息
	availableResources, err := getRealResourcesFromCSMA()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取资源信息失败: " + err.Error(),
		})
		return
	}

	// 查找目标站点
	var targetSite *Resource
	for i := range availableResources {
		if availableResources[i].ID == req.SiteID {
			targetSite = &availableResources[i]
			break
		}
	}

	if targetSite == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "未找到指定的站点",
		})
		return
	}

	// 发送停止请求到目标服务器
	err = sendStopRequestToServer(targetSite.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "停止服务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("已在 %s 上停止服务", targetSite.Name),
	})
}

// sendStopRequestToServer 向目标服务器发送停止请求
func sendStopRequestToServer(serverURL string) error {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("POST", serverURL+"/stop", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("停止服务失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// getStatus 获取指定站点的状态
func getStatus(c *gin.Context) {
	siteID := c.Param("siteId")

	// 获取所有可用资源（从C-SMA获取）
	resources, err := getRealResourcesFromCSMA()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取资源信息失败: " + err.Error(),
		})
		return
	}

	// 查找目标站点
	var targetSite *Resource
	for i := range resources {
		if resources[i].ID == siteID {
			targetSite = &resources[i]
			break
		}
	}

	if targetSite == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "未找到指定的站点",
		})
		return
	}

	// 获取目标服务器的指标
	metrics, err := fetchMetricsFromServer(targetSite.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取指标失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, StatusResponse{
		Success:       true,
		SiteID:        targetSite.ID,
		Status:        "running",
		ServicesCount: len(metrics),
		Metrics:       metrics,
		Time:          time.Now().Format(time.RFC3339),
	})
}

// fetchMetricsFromServer 从目标服务器获取指标
func fetchMetricsFromServer(serverURL string) ([]ServiceInstanceInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", serverURL+"/metrics", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取指标失败，状态码: %d", resp.StatusCode)
	}

	var response struct {
		Success bool                  `json:"success"`
		Metrics []ServiceInstanceInfo `json:"metrics"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Metrics, nil
}
