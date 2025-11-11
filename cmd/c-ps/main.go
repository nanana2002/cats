package main

import (
	"cmas-cats-go/config"
	"cmas-cats-go/models"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/cors" // ❗ 增加 CORS 导入 ❗
	"github.com/gin-gonic/gin"    // ❗ 增加 CORS 导入 ❗
)

// 核心配置常量
const (
	MaxSyncRetry  = 3
	RetryInterval = 2 * time.Second
	CacheExpire   = 5 * time.Minute
)

// 全局状态管理
var (
	cachedMetrics = make(map[string][]models.ServiceInstanceInfo) // 缓存服务实例数据
	lastSyncTime  time.Time
	mutex         sync.RWMutex
)

// 合法API Key列表（生产环境建议存储在数据库）
var validAPIKeys = map[string]bool{
	"client-001": true,
	"client-002": true,
	"client-003": true,
}

func main() {
	// 启动标识
	fmt.Println("=====================================")
	fmt.Println("        C-PS 路径选择服务启动中...        ")
	fmt.Println("=====================================")

	// 初始化Gin引擎
	r := gin.Default() // 引擎实例名为 r
	// ❗ 增加 CORS 配置：允许所有来源 (All Origins) 访问 ❗
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // 允许所有来源 (如果知道前端地址，可以写死，但 * 最方便)
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	// 注册路由
	r.POST("/request-service", authMiddleware(), handleClientRequest) // 客户端请求（需认证）
	r.GET("/refresh-metrics", refreshMetricsCache)                    // 手动刷新缓存
	r.GET("/cached-metrics", getCachedMetrics)                        // 查看缓存数据

	// 添加Web界面
	r.LoadHTMLGlob("./templates/ps/*.html")
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "C-PS 路径选择服务",
		})
	})
	r.GET("/dashboard", func(c *gin.Context) {
		mutex.RLock()
		defer mutex.RUnlock()

		// 准备展示数据
		services := make(map[string][]models.ServiceInstanceInfo)
		for k, v := range cachedMetrics {
			services[k] = v
		}

		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"title":    "服务实例数据",
			"services": services,
			"lastSync": lastSyncTime.Format("2006-01-02 15:04:05"),
		})
	})

	// 从配置获取C-SMA同步地址
	CSMASyncURL := fmt.Sprintf("http://%s:%d/sync", config.Cfg.SMA.IP, config.Cfg.SMA.Port)

	// 预加载C-SMA数据
	if err := syncMetricsFromCSMA(); err != nil {
		fmt.Printf("⚠️ 预加载C-SMA数据失败：%v（将在首次请求时重试）\n", err)
	} else {
		fmt.Printf("✅ 预加载成功！当前缓存 %d 个服务的实例数据\n", len(cachedMetrics))
	}

	// C-PS 模块启动配置
	// 实际监听地址必须使用 config.LOCAL_LISTEN_IP ("0.0.0.0")
	listenAddr := config.LOCAL_LISTEN_IP + ":" + strconv.Itoa(config.Cfg.PS.Port)

	// 启动成功后，信息输出应该使用外部 IP
	externalListenAddr := fmt.Sprintf("http://%s:%d", config.Cfg.PS.IP, config.Cfg.PS.Port)

	// 启动服务前打印信息
	fmt.Printf("\n✅ C-PS 启动成功！\n")
	fmt.Printf("📌 监听地址：%s\n", externalListenAddr)
	fmt.Printf("📌 C-SMA 同步地址：%s\n", CSMASyncURL)
	fmt.Printf("📌 缓存过期时间：%v\n", CacheExpire)

	// 启动服务 (使用 r 实例和 listenAddr)
	if err := r.Run(listenAddr); err != nil {
		fmt.Printf("❌ C-PS 启动失败：%v\n", err)
	}

	// --------------------------------------------------------------------------------
	// 移除：原始代码中有两行错误的代码，导致编译失败和重复启动：
	// router.Run(listenAddr) // 错误：router 未定义
	// fmt.Printf("📌 监听地址：http://%s\n", listenAddr) // 错误：使用内部 IP 0.0.0.0
	// --------------------------------------------------------------------------------
}

// ------------------------------
// 以下是所有的辅助函数 (保持不变)
// ------------------------------

// API Key认证中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "缺少必要的X-API-Key请求头",
			})
			c.Abort()
			return
		}

		if !validAPIKeys[apiKey] {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无效的API Key，拒绝访问",
			})
			c.Abort()
			return
		}

		c.Next() // 认证通过，继续处理请求
	}
}

// 从C-SMA同步数据
func syncMetricsFromCSMA() error {
	// 发送请求到C-SMA
	csmaSyncURL := fmt.Sprintf("%s/sync", config.Cfg.SMA.URL)
	resp, err := http.Get(csmaSyncURL)
	if err != nil {
		return fmt.Errorf("请求C-SMA失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("C-SMA返回非200状态码：%d", resp.StatusCode)
	}

	// 解析响应数据
	var csmaResp struct {
		Success    bool `json:"success"`
		ServiceNum int  `json:"service_num"`
		Data       []struct {
			ServiceID string                       `json:"service_id"`
			Instances []models.ServiceInstanceInfo `json:"instances"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&csmaResp); err != nil {
		return fmt.Errorf("解析C-SMA响应失败：%w", err)
	}

	if !csmaResp.Success {
		return fmt.Errorf("C-SMA业务错误：%s", csmaResp.Message)
	}

	// 更新缓存
	mutex.Lock()
	defer mutex.Unlock()
	cachedMetrics = make(map[string][]models.ServiceInstanceInfo) // 清空旧数据

	for _, item := range csmaResp.Data {
		cachedMetrics[item.ServiceID] = item.Instances
	}
	lastSyncTime = time.Now()

	fmt.Printf("[%s] 同步C-SMA成功：%d个服务，共%d个实例\n",
		lastSyncTime.Format("15:04:05"),
		len(cachedMetrics),
		countTotalInstances())

	return nil
}

// 客户端请求处理
func handleClientRequest(c *gin.Context) {
	// 解析请求参数
	var req models.ClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	// 参数验证
	if req.ServiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "service_id不能为空",
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

	// 检查并刷新缓存
	if needRefreshCache(req.ServiceID) {
		fmt.Printf("缓存过期或无%s实例数据，尝试刷新...\n", req.ServiceID)
		var syncErr error
		for i := 0; i < MaxSyncRetry; i++ {
			if err := syncMetricsFromCSMA(); err != nil {
				syncErr = err
				fmt.Printf("刷新重试(%d/%d)失败：%v\n", i+1, MaxSyncRetry, err)
				time.Sleep(RetryInterval)
			} else {
				syncErr = nil
				break
			}
		}
		if syncErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "无法获取服务实例数据：" + syncErr.Error(),
			})
			return
		}
	}

	// 获取可用实例
	mutex.RLock()
	targetInstances := cachedMetrics[req.ServiceID]
	mutex.RUnlock()

	// 筛选符合条件的实例（成本+延迟）
	qualified := filterInstances(targetInstances, req.MaxAcceptCost, req.MaxAcceptDelay)
	if len(qualified) == 0 {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": fmt.Sprintf("无符合条件的%s实例：所有实例成本或延迟超出限制", req.ServiceID),
		})
		return
	}

	// 选择最优实例
	bestInst := selectBestInstance(qualified)

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "路径选择成功",
		"result": map[string]interface{}{
			"service_id":    bestInst.ServiceID,
			"csci_id":       bestInst.CSCI_ID,
			"cost":          bestInst.Cost,
			"delay":         bestInst.Delay,
			"available_gas": bestInst.Gas,
			"decision_time": time.Now().Format("2006-01-02 15:04:05"),
		},
	})
}

// 检查是否需要刷新缓存
func needRefreshCache(serviceID string) bool {
	mutex.RLock()
	defer mutex.RUnlock()

	// 缓存过期 或 目标服务无实例数据
	return time.Since(lastSyncTime) > CacheExpire || len(cachedMetrics[serviceID]) == 0
}

// 筛选符合条件的实例
func filterInstances(instances []models.ServiceInstanceInfo, maxCost, maxDelay int) []models.ServiceInstanceInfo {
	var qualified []models.ServiceInstanceInfo
	for _, inst := range instances {
		if inst.Cost <= maxCost && inst.Delay <= maxDelay {
			qualified = append(qualified, inst)
		}
	}
	return qualified
}

// 选择最优实例（成本优先，延迟为辅）
func selectBestInstance(instances []models.ServiceInstanceInfo) models.ServiceInstanceInfo {
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Cost != instances[j].Cost {
			return instances[i].Cost < instances[j].Cost
		}
		return instances[i].Delay < instances[j].Delay
	})
	return instances[0]
}

// 统计实例总数
func countTotalInstances() int {
	total := 0
	for _, instances := range cachedMetrics {
		total += len(instances)
	}
	return total
}

// 手动刷新缓存接口
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

// 查看缓存数据接口
func getCachedMetrics(c *gin.Context) {
	mutex.RLock()
	defer mutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"last_sync_time":  lastSyncTime.Format("2006-01-02 15:04:05"),
		"cache_expire":    CacheExpire.String(),
		"service_count":   len(cachedMetrics),
		"total_instances": countTotalInstances(),
		"cached_data":     cachedMetrics,
	})
}
