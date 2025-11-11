package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Resource 表示可用的服务器资源
type Resource struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Price           float64 `json:"price"`
	AvailableStorage string `json:"availableStorage"`
	Latency         string `json:"latency"`
	Location        string `json:"location"`
	CPU             string `json:"cpu"`
	Memory          string `json:"memory"`
}

// DeployRequest 部署请求结构
type DeployRequest struct {
	SiteID string `json:"site_id" binding:"required"`
}

// DeployResponse 部署响应结构
type DeployResponse struct {
	Success    bool     `json:"success"`
	Message    string   `json:"message"`
	Site       Resource `json:"site"`
	Files      []string `json:"files"`
	StartScript string  `json:"startScript"`
}

// StopRequest 停止请求结构
type StopRequest struct {
	SiteID string `json:"site_id" binding:"required"`
}

// StatusResponse 状态响应结构
type StatusResponse struct {
	Success bool                  `json:"success"`
	SiteID  string                `json:"site_id"`
	Status  string                `json:"status"`
	ServicesCount int             `json:"services_count"`
	Metrics []ServiceInstanceInfo `json:"metrics"`
	Time    string                `json:"time"`
}

// 模拟的可用资源
var availableResources = []Resource{
	{
		ID:              "site1",
		Name:            "服务器节点 1",
		URL:             "http://localhost:8082",
		Price:           0.5,
		AvailableStorage: "100GB",
		Latency:         "20ms",
		Location:        "北京",
		CPU:             "8核 3.0GHz",
		Memory:          "16GB",
	},
	{
		ID:              "site2",
		Name:            "服务器节点 2",
		URL:             "http://localhost:8085",
		Price:           0.8,
		AvailableStorage: "250GB",
		Latency:         "35ms",
		Location:        "上海",
		CPU:             "12核 2.8GHz",
		Memory:          "32GB",
	},
	{
		ID:              "site3",
		Name:            "服务器节点 3",
		URL:             "http://localhost:8086",
		Price:           1.2,
		AvailableStorage: "500GB",
		Latency:         "15ms",
		Location:        "深圳",
		CPU:             "16核 3.2GHz",
		Memory:          "64GB",
	},
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

	// 静态文件服务，提供前端页面和静态资源
	// 为特定的静态文件类型提供服务，而不是使用通配符
	r.StaticFile("/", "./index.html")  // 主页
	r.StaticFile("/index.html", "./index.html")  // 明确指定index.html
	r.Static("/static", "./static")  // 静态资源目录
	r.Static("/assets", "./assets")  // 资源目录

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
	c.JSON(http.StatusOK, availableResources)
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

	// 获取上传的文件
	codeFiles := c.Request.MultipartForm.File["codeFiles"]
	startScriptHeader := c.Request.MultipartForm.File["startScript"]

	// 确保uploads目录存在
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	var uploadedFiles []string

	// 保存代码文件
	for _, fileHeader := range codeFiles {
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "打开文件失败",
			})
			return
		}
		defer file.Close()

		filename := filepath.Join(uploadDir, fileHeader.Filename)
		dst, err := os.Create(filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "保存文件失败",
			})
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "复制文件失败",
			})
			return
		}

		uploadedFiles = append(uploadedFiles, fileHeader.Filename)
	}

	// 保存启动脚本
	var startScriptName string
	if len(startScriptHeader) > 0 {
		startScriptFileHeader := startScriptHeader[0]
		file, err := startScriptFileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "打开启动脚本失败",
			})
			return
		}
		defer file.Close()

		filename := filepath.Join(uploadDir, startScriptFileHeader.Filename)
		dst, err := os.Create(filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "保存启动脚本失败",
			})
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "复制启动脚本失败",
			})
			return
		}

		startScriptName = startScriptFileHeader.Filename
	}

	// 创建ZIP文件以便传输到目标服务器
	zipBuffer, err := createZipFile(uploadedFiles, startScriptName, uploadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "创建ZIP文件失败: " + err.Error(),
		})
		return
	}

	// 尝试将代码传输到目标服务器
	err = transferCodeToServer(targetSite.URL, zipBuffer.Bytes(), startScriptName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "传输代码到服务器失败: " + err.Error(),
		})
		return
	}

	// 模拟在目标服务器上执行start.sh
	err = executeStartScriptOnServer(targetSite.URL, startScriptName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "执行启动脚本失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DeployResponse{
		Success:    true,
		Message:    fmt.Sprintf("代码已成功部署到 %s", targetSite.Name),
		Site:       *targetSite,
		Files:      uploadedFiles,
		StartScript: startScriptName,
	})
}

// createZipFile 创建包含所有上传文件的ZIP文件
func createZipFile(codeFiles []string, startScriptName, uploadDir string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	defer zipWriter.Close()

	// 添加代码文件
	for _, filename := range codeFiles {
		filePath := filepath.Join(uploadDir, filename)
		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		f, err := zipWriter.Create(filename)
		if err != nil {
			return nil, err
		}
		_, err = f.Write(fileBytes)
		if err != nil {
			return nil, err
		}
	}

	// 添加启动脚本（如果存在）
	if startScriptName != "" {
		filePath := filepath.Join(uploadDir, startScriptName)
		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		f, err := zipWriter.Create(startScriptName)
		if err != nil {
			return nil, err
		}
		_, err = f.Write(fileBytes)
		if err != nil {
			return nil, err
		}
	}

	return &buf, nil
}

// transferCodeToServer 将代码传输到目标服务器
func transferCodeToServer(serverURL string, zipData []byte, startScriptName string) error {
	// 创建一个临时的多部分表单
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// 添加ZIP数据
	part, err := writer.CreateFormFile("codeZip", "code.zip")
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

	// 发送POST请求到目标服务器
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", serverURL+"/upload-code", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("传输代码失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// executeStartScriptOnServer 在目标服务器上执行start.sh
func executeStartScriptOnServer(serverURL string, startScriptName string) error {
	// 发送执行命令到目标服务器
	client := &http.Client{Timeout: 30 * time.Second}
	
	// 创建执行请求体
	execReq := map[string]string{
		"script": startScriptName,
	}
	
	reqBody, err := json.Marshal(execReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", serverURL+"/execute", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("执行启动脚本失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

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
	err := sendStopRequestToServer(targetSite.URL)
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

// ServiceInstanceInfo 服务实例信息，与models/service.go保持一致
type ServiceInstanceInfo struct {
	ServiceID string `json:"service_id"`
	Gas       int    `json:"gas"`
	Cost      int    `json:"cost"`
	CSCI_ID   string `json:"csci_id"`
	Delay     int    `json:"delay"`
}