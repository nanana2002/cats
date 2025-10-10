package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"cmas-cats-go/models"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3" // SQLite驱动
)

// 全局状态：资源管理（需加锁保证并发安全）
var (
	db            *sql.DB
	usedResource  int          // 已使用资源单位（动态更新）
	resourceMutex sync.RWMutex // 资源操作锁（避免并发修改冲突）
)

// 服务站点核心配置（新增资源与成本相关配置）
const (
	ListenPort      = ":8082"      // 服务站点监听端口
	DBFile          = "./site1.db" // 数据库文件路径
	SiteID          = "site-1"     // 站点唯一标识
	TotalResource   = 400          // 站点总资源单位（可根据硬件配置调整）
	ResourcePerAR   = 40           // 每个AR服务实例占用资源单位
	ResourcePerTP   = 10           // 每个交通服务实例占用资源单位
	ResourcePerCost = 40           // 每30单位资源对应1个成本单位（核心：成本换算系数）
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
	r.GET("/resource-status", getResourceStatus) // 新增：查看资源占用状态

	// 5. 启动服务
	printStartInfo()
	if err := r.Run(ListenPort); err != nil {
		fmt.Printf("❌ 服务启动失败：%v\n", err)
	}
}

// ------------------------------
// 核心1：数据库初始化与资源加载
// ------------------------------

// initDB：初始化SQLite数据库（表结构不变，新增资源相关字段兼容）
func initDB() error {
	var err error

	// 1. 打开数据库
	db, err = sql.Open("sqlite3", DBFile)
	if err != nil {
		return fmt.Errorf("数据库连接失败：%w", err)
	}

	// 2. 验证连接
	if err := db.Ping(); err != nil {
		return fmt.Errorf("数据库验证失败：%w", err)
	}

	// 3. 创建部署实例表（保留原有结构，确保资源计算字段兼容）
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS deployed_services (
		id TEXT PRIMARY KEY,
		service_id TEXT NOT NULL,
		gas INT NOT NULL,
		cost INT NOT NULL,
		csci_id TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		delay INT NOT NULL,
		resource_per_inst INT NOT NULL, -- 新增：单个实例占用资源单位（用于重启后加载）
		total_resource_used INT NOT NULL -- 新增：该部署占用的总资源（用于重启后加载）
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("创建部署表失败：%w", err)
	}

	fmt.Println("✅ 数据库初始化成功（SQLite）")
	return nil
}

// loadUsedResource：从数据库加载历史部署的资源占用（避免重启后资源统计清零）
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
// 核心2：部署接口（资源占比成本计算）
// ------------------------------

// deployServiceHandler：处理服务部署请求（按资源占比计算成本）
func deployServiceHandler(c *gin.Context) {
	var req struct {
		ServiceID string `json:"service_id" binding:"required"` // 目标服务ID（如AR100、TP100）
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

	// 2. 确定单个实例的资源占用（按服务类型区分）
	resourcePerInst, err := getResourcePerInstance(req.ServiceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 3. 计算本次部署的总资源需求
	totalResourceNeed := resourcePerInst * req.Gas

	// 4. 检查资源是否充足（加读写锁：读已用资源，避免并发冲突）
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

	// 5. （核心）按资源占比计算成本：成本 = 总占用资源 / 资源成本系数（向上取整）
	cost := calculateCostByResource(totalResourceNeed)

	// 6. 生成实例基础信息
	instanceID := fmt.Sprintf("%s-%s-%d", req.ServiceID, SiteID, time.Now().UnixNano()/1e6)
	csciID := fmt.Sprintf("http://localhost%s/%s", ListenPort, instanceID)
	delay := 10 + (req.Gas % 10) // 模拟延迟（10-20ms，与实例数量正相关）
	createdAt := time.Now()

	// 7. 占用资源（加写锁：更新已用资源）
	resourceMutex.Lock()
	usedResource += totalResourceNeed
	resourceMutex.Unlock()

	// 8. 存入数据库（新增资源相关字段，便于重启后加载）
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

	// 9. 返回成功响应（包含资源和成本明细）
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("服务实例部署成功：%s（%d个）", req.ServiceID, req.Gas),
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
	fmt.Printf("[%s] 部署成功：ID=%s, 服务=%s, 实例数=%d, 成本=%d（占用资源%d单位）\n",
		time.Now().Format("15:04:05"), instanceID, req.ServiceID, req.Gas, cost, totalResourceNeed)
}

// ------------------------------
// 核心3：资源与成本计算工具函数
// ------------------------------

// getResourcePerInstance：根据服务类型获取单个实例的资源占用
func getResourcePerInstance(serviceID string) (int, error) {
	// 按服务类型定义资源占用（可扩展更多服务类型）
	switch {
	case serviceID == "AR100" || serviceID == "AR200": // AR类服务：资源占用高
		return ResourcePerAR, nil
	case serviceID == "TP100" || serviceID == "TP200": // 交通类服务：资源占用中
		return ResourcePerTP, nil
	default:
		return 0, fmt.Errorf("不支持的服务类型：%s（请先定义该服务的资源占用）", serviceID)
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

// getResourceStatus：新增接口，查看当前资源占用状态
func getResourceStatus(c *gin.Context) {
	resourceMutex.RLock()
	defer resourceMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"site_id": SiteID,
		"resource": map[string]string{ // 值类型为string
			"total":      fmt.Sprintf("%d", TotalResource),              // 整数转字符串
			"used":       fmt.Sprintf("%d", usedResource),               // 整数转字符串
			"remaining":  fmt.Sprintf("%d", TotalResource-usedResource), // 整数转字符串
			"usage_rate": fmt.Sprintf("%.1f%%", float64(usedResource)/float64(TotalResource)*100),
		},
		"cost_conversion": fmt.Sprintf("每%d单位资源 = 1成本单位", ResourcePerCost),
	})
}

// getMetricsHandler：暴露实例metrics（不变，保留成本和延迟字段）
func getMetricsHandler(c *gin.Context) {
	rows, err := db.Query(`
		SELECT service_id, gas, cost, csci_id, delay
		FROM deployed_services
		ORDER BY created_at DESC`)
	if err != nil {
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
			fmt.Printf("⚠️ 解析metrics失败：%v\n", err)
			continue
		}
		metrics = append(metrics, m)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"site_id": SiteID,
		"count":   len(metrics),
		"metrics": metrics,
		"time":    time.Now().Format(time.RFC3339),
	})
}

// healthCheckHandler：健康检查接口（新增资源状态检查）
func healthCheckHandler(c *gin.Context) {
	// 检查数据库连接和资源状态
	resourceMutex.RLock()
	resourceStatus := "healthy"
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
			"usage_rate": fmt.Sprintf("%.1f%%", float64(usedResource)/float64(TotalResource)*100),
		},
	})
}

// printStartInfo：打印启动信息（格式化输出）
func printStartInfo() {
	resourceMutex.RLock()
	defer resourceMutex.RUnlock()

	fmt.Printf("\n✅ 服务站点（site-1）启动成功！\n")
	fmt.Printf("📌 站点ID：%s\n", SiteID)
	fmt.Printf("📌 监听地址：http://localhost%s\n", ListenPort)
	fmt.Printf("📌 当前资源：已用%d / 总%d 单位（使用率%.1f%%）\n",
		usedResource, TotalResource, float64(usedResource)/float64(TotalResource)*100)
	fmt.Printf("📌 可用接口：\n")
	fmt.Printf("   - POST   /deploy              部署服务实例\n")
	fmt.Printf("   - GET    /metrics             查看实例metrics\n")
	fmt.Printf("   - GET    /health              健康检查\n")
	fmt.Printf("   - GET    /resource-status     查看资源占用\n")
}
