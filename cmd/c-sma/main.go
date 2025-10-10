package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url" // 新增：用于解析URL提取站点Host（修复去重关键）
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"cmas-cats-go/models"
)

// 核心配置（支持多站点，可按需调整）
const (
	ListenPort     = ":8083"                          // C-SMA监听端口
	PollInterval   = 10 * time.Second                 // 拉取站点metrics间隔
	// 服务站点列表：逗号分隔，格式必须为 "http://站点地址/metrics"
	ServiceSiteList = "http://localhost:8082/metrics,http://localhost:8085/metrics"
)

// 全局状态：聚合后的metrics（key=ServiceID，value=所有站点的实例）
var (
	aggregatedMetrics = make(map[string][]models.ServiceInstanceInfo)
	metricsMutex      sync.RWMutex                                   // 读写锁保证并发安全
)

func main() {
	// 启动标识日志
	fmt.Println("=====================================")
	fmt.Println("        C-SMA 度量收集服务启动中...        ")
	fmt.Println("=====================================")

	// 1. 解析并验证服务站点列表
	sites := parseSiteList(ServiceSiteList)
	printSiteConfig(sites)

	// 2. 初始化Gin引擎
	r := gin.Default()

	// 3. 注册核心接口
	r.GET("/sync", syncToCPSHandler)          // 供C-PS拉取聚合数据
	r.GET("/current-metrics", getMetricsHandler) // 调试：查看当前聚合数据
	r.GET("/health", healthCheckHandler)      // 健康检查

	// 4. 启动多站点拉取任务（后台协程，不阻塞服务）
	go startMultiSitePolling(sites)

	// 5. 启动C-SMA服务
	fmt.Printf("\n✅ C-SMA 启动成功！监听端口：%s\n", ListenPort)
	if err := r.Run(ListenPort); err != nil {
		panic("❌ C-SMA 启动失败：" + err.Error())
	}
}

// ------------------------------
// 核心1：多站点metrics拉取（带并发控制）
// ------------------------------

// startMultiSitePolling：定时拉取所有站点的metrics
func startMultiSitePolling(sites []string) {
	if len(sites) == 0 {
		fmt.Println("⚠️ 无有效服务站点，停止拉取任务")
		return
	}

	// 定时触发器：每PollInterval执行一次
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Printf("\n[%s] 📥 开始拉取 %d 个服务站点的metrics...\n",
			time.Now().Format("2006-01-02 15:04:05"), len(sites))

		var wg sync.WaitGroup // 等待所有站点拉取完成
		for _, siteURL := range sites {
			wg.Add(1)
			// 并发拉取（提升多站点场景效率）
			go func(url string) {
				defer wg.Done()
				// 拉取单个站点的metrics
				siteMetrics, siteID, err := fetchSingleSiteMetrics(url)
				if err != nil {
					fmt.Printf("❌ 拉取站点 [%s] 失败：%v\n", url, err)
					return
				}
				// 聚合数据（先删旧实例，再加新实例，避免重复）
				metricsMutex.Lock()
				aggregateSiteMetrics(url, siteMetrics)
				metricsMutex.Unlock()

				fmt.Printf("✅ 拉取站点 [%s] 成功：%d 个实例（站点ID：%s）\n",
					url, len(siteMetrics), siteID)
			}(siteURL)
		}

		wg.Wait() // 等待所有站点处理完成
		// 打印聚合结果摘要
		metricsMutex.RLock()
		serviceCount := len(aggregatedMetrics)
		totalInstances := countTotalInstances()
		metricsMutex.RUnlock()
		fmt.Printf("[%s] 📊 所有站点拉取完成 | 聚合服务数：%d | 总实例数：%d\n",
			time.Now().Format("2006-01-02 15:04:05"), serviceCount, totalInstances)
	}
}

// fetchSingleSiteMetrics：拉取单个站点的metrics（含延迟字段）
func fetchSingleSiteMetrics(siteURL string) ([]models.ServiceInstanceInfo, string, error) {
	// 发送HTTP GET请求到站点的/metrics接口
	resp, err := http.Get(siteURL)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP请求失败：%w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("状态码错误：%d（期望200）", resp.StatusCode)
	}

	// 解析站点返回的JSON数据（包含站点ID和实例列表）
	var siteResp struct {
		Success bool                     `json:"success"`
		SiteID  string                   `json:"site_id"`          // 服务站点的唯一标识（如site-1）
		Metrics []models.ServiceInstanceInfo `json:"metrics"`         // 实例列表（含Delay字段）
		Message string                   `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&siteResp); err != nil {
		return nil, "", fmt.Errorf("JSON解析失败：%w", err)
	}

	// 检查站点业务状态
	if !siteResp.Success {
		return nil, "", fmt.Errorf("站点业务错误：%s", siteResp.Message)
	}

	return siteResp.Metrics, siteResp.SiteID, nil
}

// ------------------------------
// 核心2：实例聚合（修复去重逻辑）
// ------------------------------

// aggregateSiteMetrics：聚合单个站点的metrics（先删旧实例，再加新实例）
func aggregateSiteMetrics(siteURL string, newMetrics []models.ServiceInstanceInfo) {
	// 关键修复：解析站点URL，提取Host（如"localhost:8082"）作为去重标识
	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		fmt.Printf("⚠️ 解析站点URL [%s] 失败：%v，跳过旧实例删除\n", siteURL, err)
		return
	}
	siteKey := parsedURL.Host // 正确的站点标识（不含http://，与CSCI_ID格式匹配）

	// 1. 删除该站点的所有旧实例（避免重复聚合）
	for serviceID, oldInstances := range aggregatedMetrics {
		var retainedInstances []models.ServiceInstanceInfo
		for _, inst := range oldInstances {
			// 通过CSCI_ID是否包含siteKey，判断是否为当前站点的实例
			if !strings.Contains(inst.CSCI_ID, siteKey) {
				retainedInstances = append(retainedInstances, inst)
			}
		}
		aggregatedMetrics[serviceID] = retainedInstances
	}

	// 2. 追加该站点的新实例（此时无重复）
	for _, newInst := range newMetrics {
		serviceID := newInst.ServiceID
		aggregatedMetrics[serviceID] = append(aggregatedMetrics[serviceID], newInst)
	}
}

// ------------------------------
// 辅助函数
// ------------------------------

// parseSiteList：解析服务站点列表（逗号分隔→切片）
func parseSiteList(listStr string) []string {
	var validSites []string
	for _, site := range strings.Split(listStr, ",") {
		trimmedSite := strings.TrimSpace(site)
		// 验证：非空且以"/metrics"结尾，同时能解析为合法URL
		if trimmedSite != "" && strings.HasSuffix(trimmedSite, "/metrics") {
			if _, err := url.Parse(trimmedSite); err == nil {
				validSites = append(validSites, trimmedSite)
			} else {
				fmt.Printf("⚠️ 无效站点URL：%s（解析失败，跳过）\n", trimmedSite)
			}
		}
	}
	return validSites
}

// printSiteConfig：打印站点配置（启动时展示）
func printSiteConfig(sites []string) {
	if len(sites) == 0 {
		fmt.Println("⚠️ 未配置任何有效服务站点！")
		return
	}
	fmt.Printf("✅ 已配置 %d 个服务站点，拉取间隔：%v\n", len(sites), PollInterval)
	for i, site := range sites {
		// 解析站点Host，便于展示
		parsedURL, _ := url.Parse(site)
		fmt.Printf("   %d. %s（Host：%s）\n", i+1, site, parsedURL.Host)
	}
}

// countTotalInstances：统计所有实例总数
func countTotalInstances() int {
	total := 0
	for _, instances := range aggregatedMetrics {
		total += len(instances)
	}
	return total
}

// ------------------------------
// API接口实现（供C-PS和调试）
// ------------------------------

// syncToCPSHandler：向C-PS提供聚合后的metrics（含延迟统计）
func syncToCPSHandler(c *gin.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	// 构造C-PS需要的格式：按ServiceID分组，包含实例和统计信息
	var syncData []struct {
		ServiceID string                     `json:"service_id"`
		Instances []models.ServiceInstanceInfo `json:"instances"` // 所有实例（含Delay）
		TotalGas  int                        `json:"total_gas"`  // 该服务总实例数
		MinDelay  int                        `json:"min_delay"`  // 最小延迟（辅助C-PS决策）
		MaxDelay  int                        `json:"max_delay"`  // 最大延迟
	}

	for serviceID, instances := range aggregatedMetrics {
		// 计算该服务的统计信息
		totalGas := 0
		minDelay := 1 << 30 // 初始化为极大值
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
		// 处理无实例的极端情况
		if minDelay == 1<<30 {
			minDelay = 0
		}

		syncData = append(syncData, struct {
			ServiceID string                     `json:"service_id"`
			Instances []models.ServiceInstanceInfo `json:"instances"`
			TotalGas  int                        `json:"total_gas"`
			MinDelay  int                        `json:"min_delay"`
			MaxDelay  int                        `json:"max_delay"`
		}{
			ServiceID: serviceID,
			Instances: instances,
			TotalGas:  totalGas,
			MinDelay:  minDelay,
			MaxDelay:  maxDelay,
		})
	}

	// 返回同步结果（带时间戳和站点数）
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"sync_time":   time.Now().Format("2006-01-02 15:04:05"),
		"service_num": len(syncData),
		"site_num":    len(parseSiteList(ServiceSiteList)),
		"data":        syncData,
	})
}

// getMetricsHandler：调试接口，查看完整聚合数据
func getMetricsHandler(c *gin.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"last_update_time": time.Now().Format("2006-01-02 15:04:05"),
		"monitored_sites":  len(parseSiteList(ServiceSiteList)),
		"service_count":    len(aggregatedMetrics),
		"total_instances":  countTotalInstances(),
		"aggregated_data":  aggregatedMetrics, // 完整实例数据（含Delay）
	})
}

// healthCheckHandler：健康检查接口（供监控系统）
func healthCheckHandler(c *gin.Context) {
	sites := parseSiteList(ServiceSiteList)
	status := "healthy"
	if len(sites) == 0 {
		status = "degraded" // 无站点配置，标记为降级
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"status":          status,
		"service":         "c-sma",
		"time":            time.Now().Format("2006-01-02 15:04:05"),
		"monitored_sites": len(sites),
	})
}
