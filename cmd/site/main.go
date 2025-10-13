package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"cmas-cats-go/config"
	"cmas-cats-go/models"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3" // SQLite驱动
)

// 全局状态：资源管理（需加锁保证并发安全）
var (
	db                *sql.DB
	usedResource      int                               // 已使用资源单位（动态更新）
	resourceMutex     sync.RWMutex                      // 资源操作锁（避免并发修改冲突）
	serviceStore      = make(map[string]models.Service) // 缓存已查询的服务信息，减少重复请求
	serviceStoreMutex sync.RWMutex                      // 服务信息缓存锁
)

// 服务站点核心配置（资源与成本相关）
const (
	DBFile          = "./db/site1.db" // 数据库文件路径
	SiteID          = "site-1"        // 站点唯一标识
	TotalResource   = 400             // 站点总资源单位（可根据硬件调整）
	ResourcePerCost = 40              // 每40单位资源对应1个成本单位（成本换算系数）
)

func main() {
	// 启动标识日志
	fmt.Println("=====================================")
	fmt.Println("          服务站点（site-1）启动中...          ")
	fmt.Println("=====================================")
	fmt.Printf("📌 站点总资源：%d 单位 | 成本换算：每%d单位资源=1成本\n",
		TotalResource, ResourcePerCost)

	// 1. 初始化数据库（启动时加载已用资源）
	if err := initDB(); err != nil {
		fmt.Printf("❌ 初始化失败，程序退出：%v\n", err)
		return
	}
	defer db.Close()

	// 2. 初始化已用资源（从数据库加载历史部署的实例资源占用）
	if err := loadUsedResource(); err != nil {
		fmt.Printf("⚠️ 加载历史资源占用失败：%v（将从0开始计算）\n", err)
	}

	// 3. 初始化Gin引擎
	r := gin.Default()

	// 4. 注册API接口
	r.POST("/deploy", deployServiceHandler)      // 部署服务实例（核心：资源+成本计算）
	r.GET("/metrics", getMetricsHandler)         // 暴露实例metrics（供C-SMA拉取）
	r.GET("/health", healthCheckHandler)         // 健康检查接口
	r.GET("/resource-status", getResourceStatus) // 查看资源占用状态

	// 5. 启动服务
	listenAddr := "0.0.0.0:8081"
	publicPlatformURL := fmt.Sprintf("%s/api/v1/services/", config.Cfg.Platform.URL)

	printStartInfo()
	fmt.Printf("📌 监听地址：http://%s\n", listenAddr)
	fmt.Printf("📌 平台地址：%s\n", publicPlatformURL)

	if err := r.Run(listenAddr); err != nil {
		fmt.Printf("服务启动失败：%v\n", err)
	}
}

// ------------------------------
// 核心1：数据库初始化与资源加载
// ------------------------------

// initDB：初始化SQLite数据库（含资源相关字段）
func initDB() error {
	var err error

	// 1. 打开数据库
	db, err = sql.Open("sqlite3", DBFile)
	if err != nil {
		return fmt.Errorf("数据库连接失败：%w", err)
	}

	// 2. 验证连接
	err = db.Ping()
	if err != nil {
		return fmt.Errorf("数据库验证失败：%w", err)
	}

	// 3. 创建部署实例表（含资源相关字段，支持重启后加载）
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS deployed_services (
		id TEXT PRIMARY KEY,
		service_id TEXT NOT NULL,
		gas INT NOT NULL,
		cost INT NOT NULL,
		csci_id TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		delay INT NOT NULL,
		resource_per_inst INT NOT NULL, -- 单个实例资源占用（用于重启加载）
		total_resource_used INT NOT NULL -- 该部署总资源占用（用于重启加载）
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("创建部署表失败：%w", err)
	}

	fmt.Println("✅ 数据库初始化成功（SQLite）")
	return nil
}

// loadUsedResource：从数据库加载历史资源占用（避免重启后统计清零）
func loadUsedResource() error {
	// 查询所有已部署实例的总资源占用
	row := db.QueryRow(`SELECT SUM(total_resource_used) FROM deployed_services`)
	var totalUsed int
	err := row.Scan(&totalUsed)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// 更新已用资源（加写锁）
	resourceMutex.Lock()
	usedResource = totalUsed
	resourceMutex.Unlock()

	fmt.Printf("✅ 加载历史资源完成：已用%d / 总%d 单位\n", usedResource, TotalResource)
	return nil
}

// ------------------------------
// 核心2：部署接口（支持多服务类型）
// ------------------------------

// deployServiceHandler：处理服务部署请求（按资源占比计算成本）
func deployServiceHandler(c *gin.Context) {
	var req struct {
		ServiceID string `json:"service_id" binding:"required"` // 目标服务ID（动态生成的ID，如AR1760108514766）
		Gas       int    `json:"gas" binding:"min=1"`           // 部署实例数量（至少1个）
	}

	// 1. 解析请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	// 2. 获取服务类型（按服务ID查询公共服务平台，获取服务名）
	serviceName, err := getServiceNameByID(req.ServiceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "获取服务信息失败：" + err.Error(),
		})
		return
	}

	// 3. 确定单个实例的资源占用（按服务名区分）
	resourcePerInst, err := getResourcePerInstance(serviceName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 4. 计算本次部署的总资源需求
	totalResourceNeed := resourcePerInst * req.Gas

	// 5. 检查资源是否充足（加读写锁：读已用资源，避免并发冲突）
	resourceMutex.RLock()
	remainingResource := TotalResource - usedResource
	resourceMutex.RUnlock()

	if totalResourceNeed > remainingResource {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": fmt.Sprintf("资源不足！当前已用%d/总%d单位，本次需%d单位，剩余%d单位",
				usedResource, TotalResource, totalResourceNeed, remainingResource),
			"resource_status": map[string]int{
				"used":      usedResource,
				"total":     TotalResource,
				"remaining": remainingResource,
				"need":      totalResourceNeed,
			},
		})
		return
	}

	// 6. （核心）按资源占比计算成本：成本 = 总占用资源 / 资源成本系数（向上取整）
	cost := calculateCostByResource(totalResourceNeed)

	// 7. 生成实例基础信息
	instanceID := fmt.Sprintf("%s-%s-%d", req.ServiceID, SiteID, time.Now().UnixNano()/1e6)
	listenAddr := fmt.Sprintf("%s:%d", config.Cfg.Site1.IP, config.Cfg.Site1.Port)
	csciID := fmt.Sprintf("http://%s/%s", listenAddr, instanceID)
	delay := 10 + (req.Gas % 10) // 模拟延迟（10-20ms，与实例数量正相关）
	createdAt := time.Now()

	// 8. 占用资源（加写锁：更新已用资源）
	resourceMutex.Lock()
	usedResource += totalResourceNeed
	resourceMutex.Unlock()

	// 9. 存入数据库（含资源相关字段，便于重启后加载）
	_, err = db.Exec(`
		INSERT INTO deployed_services (
			id, service_id, gas, cost, csci_id, created_at, delay,
			resource_per_inst, total_resource_used
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		instanceID, req.ServiceID, req.Gas, cost, csciID, createdAt, delay,
		resourcePerInst, totalResourceNeed) // 存储资源相关信息

	if err != nil {
		// 数据库插入失败，回滚资源占用
		resourceMutex.Lock()
		usedResource -= totalResourceNeed
		resourceMutex.Unlock()

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "部署失败（数据库错误）：" + err.Error(),
		})
		return
	}

	// 10. 返回成功响应（包含资源和成本明细）
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("服务实例部署成功：%s（服务名：%s，%d个实例）", req.ServiceID, serviceName, req.Gas),
		"info": models.ServiceInstanceInfo{
			ServiceID: req.ServiceID,
			Gas:       req.Gas,
			Cost:      cost, // 按资源计算的成本
			CSCI_ID:   csciID,
			Delay:     delay,
		},
		"resource_detail": map[string]int{
			"single_inst_resource": resourcePerInst,              // 单个实例资源占用
			"total_resource_used":  totalResourceNeed,            // 本次占用总资源
			"current_used":         usedResource,                 // 部署后总已用资源
			"remaining_resource":   TotalResource - usedResource, // 剩余资源
		},
	})
	fmt.Printf("[%s] 部署成功：ID=%s, 服务名=%s, 实例数=%d, 成本=%d（占用资源%d单位）\n",
		time.Now().Format("15:04:05"), instanceID, serviceName, req.Gas, cost, totalResourceNeed)
}

// ------------------------------
// 核心3：服务信息查询与资源计算工具函数
// ------------------------------

// getServiceNameByID：按服务ID查询公共服务平台，获取服务名（含缓存）
func getServiceNameByID(serviceID string) (string, error) {
	// 1. 先查缓存，避免重复请求公共服务平台
	serviceStoreMutex.RLock()
	cachedService, exists := serviceStore[serviceID]
	serviceStoreMutex.RUnlock()
	if exists {
		return cachedService.Name, nil
	}

	// 2. 缓存未命中，调用公共服务平台接口查询
	publicPlatformURL := fmt.Sprintf("%s/api/v1/services/", config.Cfg.Platform.URL)
	reqURL := publicPlatformURL + serviceID
	resp, err := http.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("调用公共服务平台失败：%w", err)
	}
	defer resp.Body.Close()

	// 3. 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("公共服务平台返回错误状态码：%d（服务ID：%s）", resp.StatusCode, serviceID)
	}

	// 4. 解析响应，提取服务名
	var result struct {
		Success bool           `json:"success"`
		Service models.Service `json:"service"`
		Message string         `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析服务信息失败：%w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("公共服务平台查询失败：%s（服务ID：%s）", result.Message, serviceID)
	}

	// 5. 存入缓存，后续复用
	serviceStoreMutex.Lock()
	serviceStore[serviceID] = result.Service
	serviceStoreMutex.Unlock()

	return result.Service.Name, nil
}

// getResourcePerInstance：根据服务名分配单个实例的资源占用
func getResourcePerInstance(serviceName string) (int, error) {
	// 按服务名定义资源占用（覆盖所有已注册服务类型）
	switch serviceName {
	case "AR/VR": // AR/VR服务（如AR1760106899671）
		return 60, nil
	case "交通流量监测": // 交通流量监测服务（如AR1760108487856）
		return 15, nil // 交通服务资源占用较低，15单位/实例
	case "人脸识别": // 人脸识别服务（如AR1760108501919）
		return 50, nil // 人脸识别计算密集，50单位/实例
	case "语音转文字": // 语音转文字服务（如AR1760108514766）
		return 30, nil // 语音处理中等资源消耗，30单位/实例
	default:
		return 0, fmt.Errorf("不支持的服务类型：%s（请先在getResourcePerInstance函数中定义资源占用）", serviceName)
	}
}

// calculateCostByResource：按资源占比计算成本（向上取整，避免零成本）
func calculateCostByResource(totalResource int) int {
	if totalResource <= 0 {
		return 1 // 最低成本1，避免免费服务
	}
	// 核心公式：成本 = 总资源 / 资源成本系数（向上取整）
	cost := totalResource / ResourcePerCost
	if totalResource%ResourcePerCost != 0 {
		cost += 1
	}
	return cost
}

// ------------------------------
// 辅助接口：状态查询与日志打印
// ------------------------------

// getResourceStatus：查看当前资源占用状态
func getResourceStatus(c *gin.Context) {
	resourceMutex.RLock()
	defer resourceMutex.RUnlock()

	remaining := TotalResource - usedResource
	usageRate := fmt.Sprintf("%.1f%%", float64(usedResource)/float64(TotalResource)*100)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"site_id": SiteID,
		"resource": map[string]string{
			"total":      fmt.Sprintf("%d", TotalResource),
			"used":       fmt.Sprintf("%d", usedResource),
			"remaining":  fmt.Sprintf("%d", remaining),
			"usage_rate": usageRate,
		},
		"cost_conversion": fmt.Sprintf("每%d单位资源 = 1成本单位", ResourcePerCost),
	})
}

// getMetricsHandler：暴露实例metrics（供C-SMA拉取）
func getMetricsHandler(c *gin.Context) {
	fmt.Println("[DEBUG] /metrics endpoint accessed")

	rows, err := db.Query(`
		SELECT service_id, gas, cost, csci_id, delay
		FROM deployed_services
		ORDER BY created_at DESC`)
	if err != nil {
		fmt.Printf("[ERROR] Failed to query metrics: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询metrics失败：" + err.Error(),
		})
		return
	}
	defer rows.Close()

	var metrics []models.ServiceInstanceInfo
	for rows.Next() {
		var m models.ServiceInstanceInfo
		if err := rows.Scan(
			&m.ServiceID, &m.Gas, &m.Cost, &m.CSCI_ID, &m.Delay,
		); err != nil {
			fmt.Printf("[WARNING] Failed to parse metrics row: %v\n", err)
			continue
		}
		metrics = append(metrics, m)
	}

	fmt.Printf("[DEBUG] Metrics retrieved: %d records\n", len(metrics))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"site_id": SiteID,
		"count":   len(metrics),
		"metrics": metrics,
		"time":    time.Now().Format(time.RFC3339),
	})
}

// healthCheckHandler：健康检查接口（含资源状态）
func healthCheckHandler(c *gin.Context) {
	// 检查数据库连接和资源状态
	resourceMutex.RLock()
	resourceStatus := "healthy"
	usageRate := fmt.Sprintf("%.1f%%", float64(usedResource)/float64(TotalResource)*100)
	if usedResource > TotalResource*0.9 { // 资源占用超90%标记为预警
		resourceStatus = "warning (high resource usage)"
	}
	resourceMutex.RUnlock()

	if err := db.Ping(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":         false,
			"status":          "unhealthy",
			"reason":          "数据库连接失败",
			"resource_status": resourceStatus,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  "healthy",
		"site_id": SiteID,
		"time":    time.Now().Format(time.RFC3339),
		"resource_status": map[string]string{
			"status":     resourceStatus,
			"used":       fmt.Sprintf("%d/%d", usedResource, TotalResource),
			"usage_rate": usageRate,
		},
	})
}

// printStartInfo：打印启动信息（格式化输出）
func printStartInfo() {
	resourceMutex.RLock()
	defer resourceMutex.RUnlock()

	usageRate := fmt.Sprintf("%.1f%%", float64(usedResource)/float64(TotalResource)*100)
	fmt.Printf("\n✅ 服务站点（site-1）启动成功！\n")
	fmt.Printf("📌 站点ID：%s\n", SiteID)
	listenAddr := fmt.Sprintf("%s:%d", config.Cfg.Site1.IP, config.Cfg.Site1.Port)
	fmt.Printf("📌 监听地址：http://%s\n", listenAddr)
	fmt.Printf("📌 当前资源：已用%d / 总%d 单位（使用率%s）\n",
		usedResource, TotalResource, usageRate)
	fmt.Printf("📌 可用接口：\n")
	fmt.Printf("   - POST   /deploy              部署服务实例（支持多服务类型）\n")
	fmt.Printf("   - GET    /metrics             查看实例metrics\n")
	fmt.Printf("   - GET    /health              健康检查\n")
	fmt.Printf("   - GET    /resource-status     查看资源占用\n")
}
