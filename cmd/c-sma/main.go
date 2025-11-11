// cmd/c-sma/main.go

package main

import (
	"cmas-cats-go/config"
	"cmas-cats-go/models"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time" // ❗ 增加 CORS 导入 ❗

	"github.com/gin-contrib/cors" // ❗ 增加 CORS 导入 ❗
	"github.com/gin-gonic/gin"
)

const (
	PollInterval = 10 * time.Second
)

var (
	aggregatedMetrics = make(map[string][]models.ServiceInstanceInfo)
	metricsMutex      sync.RWMutex
)

func main() {
	fmt.Println("=====================================")
	fmt.Println("        C-SMA 度量收集服务启动中...        ")
	fmt.Println("=====================================")

	// ✅ 关键修改：使用 config.GetAllSiteURLs() 动态获取所有站点
	sites := config.GetAllSiteURLs()
	printSiteConfig(sites)

	if len(sites) == 0 {
		fmt.Println("⚠️  未发现任何 SiteN.URL 配置！请检查 config.Cfg 中 Site1/2/3...")
	}

	r := gin.Default() // Gin 引擎实例名为 r
	// ❗ 增加 CORS 配置：允许所有来源 (All Origins) 访问 ❗
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // 允许所有来源 (如果知道前端地址，可以写死，但 * 最方便)
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	// API 路由
	r.GET("/sync", syncToCPSHandler)
	r.GET("/current-metrics", getMetricsHandler)
	r.GET("/health", healthCheckHandler)

	// Web 页面
	r.LoadHTMLGlob("./templates/sma/*.html")
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "C-SMA 度量收集服务",
		})
	})
	r.GET("/dashboard", func(c *gin.Context) {
		metricsMutex.RLock()
		defer metricsMutex.RUnlock()
		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"title":   "服务度量数据",
			"metrics": aggregatedMetrics,
			"sites":   sites,
		})
	})

	// 启动拉取任务
	go startMultiSitePolling(sites)

	// C-SMA 模块启动配置
	// 实际监听地址必须使用 config.LOCAL_LISTEN_IP ("0.0.0.0")
	listenAddr := config.LOCAL_LISTEN_IP + ":" + strconv.Itoa(config.Cfg.SMA.Port)
	// 外部展示地址
	externalListenAddr := fmt.Sprintf("http://%s:%d", config.Cfg.SMA.IP, config.Cfg.SMA.Port)

	// 启动服务前打印信息
	fmt.Printf("\n✅ C-SMA 启动成功！\n")
	fmt.Printf("📌 监听地址：%s\n", externalListenAddr)
	fmt.Printf("📌 监控站点数：%d（动态发现）\n", len(sites))
	fmt.Printf("📌 拉取间隔：%v\n", PollInterval)

	fmt.Println("📌 站点列表：")
	for _, site := range sites {
		if u, err := url.Parse(site); err == nil {
			fmt.Printf("   - %s (Host: %s)\n", site, u.Host)
		} else {
			fmt.Printf("   - %s\n", site)
		}
	}

	// ❗ 修复：使用 r 启动服务，并处理可能的错误 ❗
	if err := r.Run(listenAddr); err != nil {
		panic("❌ C-SMA 启动失败：" + err.Error())
	}

	// --------------------------------------------------------------------------------
	// 移除：以下代码是原始代码中的冗余或错误，已被上面的代码替代。
	// listenAddr := config.LOCAL_LISTEN_IP + ":" + strconv.Itoa(config.Cfg.SMA.Port)
	// router.Run(listenAddr) // 错误：router 未定义
	// fmt.Printf("\n✅ C-SMA 启动成功！...")
	// if err := r.Run(listenAddr); err != nil { panic(...) } // 错误：重复调用 Run
	// --------------------------------------------------------------------------------
}

// ------------------------------
// 核心：多站点拉取 (保持不变)
// ------------------------------

func startMultiSitePolling(sites []string) {
	if len(sites) == 0 {
		fmt.Println("⚠️ 无有效服务站点，停止拉取任务")
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
				siteMetrics, siteID, err := fetchSingleSiteMetrics(url + "/metrics")
				if err != nil {
					fmt.Printf("❌ 拉取站点 [%s] 失败：%v\n", url, err)
					return
				}
				metricsMutex.Lock()
				aggregateSiteMetrics(url, siteMetrics)
				metricsMutex.Unlock()

				fmt.Printf("✅ 拉取站点 [%s] 成功：%d 个实例（站点ID：%s）\n",
					url, len(siteMetrics), siteID)
			}(siteURL)
		}

		wg.Wait()

		metricsMutex.RLock()
		serviceCount := len(aggregatedMetrics)
		totalInstances := countTotalInstances()
		metricsMutex.RUnlock()
		fmt.Printf("[%s] 📊 所有站点拉取完成 | 聚合服务数：%d | 总实例数：%d\n",
			time.Now().Format("2006-01-02 15:04:05"), serviceCount, totalInstances)
	}
}

// ... (fetchSingleSiteMetrics, aggregateSiteMetrics, printSiteConfig, countTotalInstances 保持不变)

func fetchSingleSiteMetrics(siteURL string) ([]models.ServiceInstanceInfo, string, error) {
	resp, err := http.Get(siteURL)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("状态码错误：%d", resp.StatusCode)
	}

	var siteResp struct {
		Success bool                         `json:"success"`
		SiteID  string                       `json:"site_id"`
		Metrics []models.ServiceInstanceInfo `json:"metrics"`
		Message string                       `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&siteResp); err != nil {
		return nil, "", fmt.Errorf("JSON解析失败：%w", err)
	}

	if !siteResp.Success {
		return nil, "", fmt.Errorf("站点业务错误：%s", siteResp.Message)
	}

	return siteResp.Metrics, siteResp.SiteID, nil
}

// ------------------------------
// 聚合逻辑（去重）
// ------------------------------

func aggregateSiteMetrics(siteURL string, newMetrics []models.ServiceInstanceInfo) {
	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		fmt.Printf("⚠️ 解析站点URL [%s] 失败：%v\n", siteURL, err)
		return
	}
	siteKey := parsedURL.Host // e.g., "192.168.235.48:8081"

	// 删除旧实例
	for serviceID, oldInstances := range aggregatedMetrics {
		var retained []models.ServiceInstanceInfo
		for _, inst := range oldInstances {
			if !strings.Contains(inst.CSCI_ID, siteKey) {
				retained = append(retained, inst)
			}
		}
		aggregatedMetrics[serviceID] = retained
	}

	// 添加新实例
	for _, inst := range newMetrics {
		aggregatedMetrics[inst.ServiceID] = append(aggregatedMetrics[inst.ServiceID], inst)
	}
}

// ------------------------------
// 辅助函数
// ------------------------------

func printSiteConfig(sites []string) {
	if len(sites) == 0 {
		fmt.Println("⚠️ 未配置任何有效服务站点！")
		return
	}
	fmt.Printf("✅ 已配置 %d 个服务站点，拉取间隔：%v\n", len(sites), PollInterval)
	for i, site := range sites {
		parsedURL, _ := url.Parse(site)
		host := parsedURL.Host
		if host == "" {
			host = site
		}
		fmt.Printf("   %d. %s（Host：%s）\n", i+1, site, host)
	}
}

func countTotalInstances() int {
	total := 0
	for _, instances := range aggregatedMetrics {
		total += len(instances)
	}
	return total
}

// ------------------------------
// API Handlers (保持不变)
// ------------------------------

func syncToCPSHandler(c *gin.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	var syncData []struct {
		ServiceID string                       `json:"service_id"`
		Instances []models.ServiceInstanceInfo `json:"instances"`
		TotalGas  int                          `json:"total_gas"`
		MinDelay  int                          `json:"min_delay"`
		MaxDelay  int                          `json:"max_delay"`
	}

	for serviceID, instances := range aggregatedMetrics {
		totalGas := 0
		minDelay := 1 << 30
		maxDelay := 0
		for _, inst := range instances {
			totalGas += inst.Gas
			if inst.Delay < minDelay {
				minDelay = inst.Delay
			}
			if inst.Delay > maxDelay {
				maxDelay = inst.Delay
			}
		}
		if minDelay == 1<<30 {
			minDelay = 0
		}

		syncData = append(syncData, struct {
			ServiceID string                       `json:"service_id"`
			Instances []models.ServiceInstanceInfo `json:"instances"`
			TotalGas  int                          `json:"total_gas"`
			MinDelay  int                          `json:"min_delay"`
			MaxDelay  int                          `json:"max_delay"`
		}{
			ServiceID: serviceID,
			Instances: instances,
			TotalGas:  totalGas,
			MinDelay:  minDelay,
			MaxDelay:  maxDelay,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"sync_time":   time.Now().Format("2006-01-02 15:04:05"),
		"service_num": len(syncData),
		"site_num":    len(config.GetAllSiteURLs()), // ✅ 动态获取
		"data":        syncData,
	})
}

func getMetricsHandler(c *gin.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	sites := config.GetAllSiteURLs() // ✅ 动态获取
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"last_update_time": time.Now().Format("2006-01-02 15:04:05"),
		"monitored_sites":  len(sites),
		"service_count":    len(aggregatedMetrics),
		"total_instances":  countTotalInstances(),
		"aggregated_data":  aggregatedMetrics,
	})
}

func healthCheckHandler(c *gin.Context) {
	sites := config.GetAllSiteURLs() // ✅ 动态获取
	status := "healthy"
	if len(sites) == 0 {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"status":          status,
		"service":         "c-sma",
		"time":            time.Now().Format("2006-01-02 15:04:05"),
		"monitored_sites": len(sites),
	})
}
