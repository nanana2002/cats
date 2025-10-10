package main

import (
        "database/sql"
        "fmt"
        "net/http"
        "time"

        "github.com/gin-gonic/gin"
        _ "github.com/mattn/go-sqlite3" // SQLite驱动
        "cmas-cats-go/models"
)

// 全局数据库连接
var db *sql.DB

// 服务站点配置
const (
        ListenPort = ":8082"                  // 服务站点监听端口
        DBFile     = "./site.db"              // 数据库文件路径
        SiteID     = "site-1"                 // 站点唯一标识（可修改为不同值部署多站点）
)

func main() {
        // 启动标识日志
        fmt.Println("=====================================")
        fmt.Println("          服务站点启动中...          ")
        fmt.Println("=====================================")

        // 1. 初始化数据库
        if err := initDB(); err != nil {
                fmt.Printf("❌ 初始化失败，程序退出：%v\n", err)
                return
        }
        defer db.Close()

        // 2. 初始化Gin引擎
        r := gin.Default()

        // 3. 注册API接口
        r.POST("/deploy", deployServiceHandler)       // 部署服务实例
        r.GET("/metrics", getMetricsHandler)          // 暴露实例metrics（供C-SMA拉取）
        r.GET("/health", healthCheckHandler)          // 健康检查接口

        // 4. 启动服务
        fmt.Printf("\n✅ 服务站点启动成功！\n")
        fmt.Printf("📌 站点ID：%s\n", SiteID)
        fmt.Printf("📌 监听地址：http://localhost%s\n", ListenPort)
        fmt.Printf("📌 可用接口：\n")
        fmt.Printf("   - POST   /deploy        部署服务实例\n")
        fmt.Printf("   - GET    /metrics       查看实例metrics\n")
        fmt.Printf("   - GET    /health        健康检查\n")

        // 启动HTTP服务
        if err := r.Run(ListenPort); err != nil {
                fmt.Printf("❌ 服务启动失败：%v\n", err)
        }
}

// initDB：初始化SQLite数据库
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

        // 3. 创建部署实例表
        createTableSQL := `
        CREATE TABLE IF NOT EXISTS deployed_services (
                id TEXT PRIMARY KEY,
                service_id TEXT NOT NULL,
                gas INT NOT NULL,
                cost INT NOT NULL,
                csci_id TEXT NOT NULL,
                created_at DATETIME NOT NULL,
                delay INT NOT NULL  -- 延迟指标（ms）
        );`
        _, err = db.Exec(createTableSQL)
        if err != nil {
                return fmt.Errorf("创建部署表失败：%w", err)
        }

        fmt.Println("✅ 数据库初始化成功（SQLite）")
        return nil
}

// deployServiceHandler：处理服务部署请求
func deployServiceHandler(c *gin.Context) {
        var req struct {
                ServiceID string `json:"service_id" binding:"required"` // 目标服务ID（必须）
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

        // 2. 生成实例信息
        instanceID := fmt.Sprintf("%s-%s-%d", req.ServiceID, SiteID, time.Now().UnixNano()/1e6)
        csciID := fmt.Sprintf("http://localhost%s/%s", ListenPort, instanceID)
        cost := req.Gas * 2 // 成本计算逻辑（示例：数量×2）
        delay := 10 + (req.Gas % 10) // 模拟延迟（10-20ms）
        createdAt := time.Now()

        // 3. 存入数据库
        _, err := db.Exec(`
                INSERT INTO deployed_services (
                        id, service_id, gas, cost, csci_id, created_at, delay
                ) VALUES (?, ?, ?, ?, ?, ?, ?)`,
                instanceID, req.ServiceID, req.Gas, cost, csciID, createdAt, delay)

        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "success": false,
                        "message": "部署失败（数据库错误）：" + err.Error(),
                })
                return
        }

        // 4. 返回成功响应
        c.JSON(http.StatusOK, gin.H{
                "success": true,
                "message": fmt.Sprintf("服务实例部署成功：%s（%d个）", req.ServiceID, req.Gas),
                "info": models.ServiceInstanceInfo{
                        ServiceID: req.ServiceID,
                        Gas:       req.Gas,
                        Cost:      cost,
                        CSCI_ID:   csciID,
                        Delay:     delay,
                },
        })
        fmt.Printf("[%s] 部署成功：ID=%s, 服务=%s, 实例数=%d\n",
                time.Now().Format("15:04:05"), instanceID, req.ServiceID, req.Gas)
}

// getMetricsHandler：暴露实例metrics（供C-SMA拉取）
func getMetricsHandler(c *gin.Context) {
        // 1. 查询所有部署的实例
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

        // 2. 解析结果
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

        // 3. 返回metrics数据（供C-SMA聚合）
        c.JSON(http.StatusOK, gin.H{
                "success": true,
                "site_id": SiteID,
                "count":   len(metrics),
                "metrics": metrics,
                "time":    time.Now().Format(time.RFC3339),
        })
}

// healthCheckHandler：健康检查接口
func healthCheckHandler(c *gin.Context) {
        // 简单检查数据库连接
        if err := db.Ping(); err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "success": false,
                        "status":  "unhealthy",
                        "reason":  "数据库连接失败",
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "success": true,
                "status":  "healthy",
                "site_id": SiteID,
                "time":    time.Now().Format(time.RFC3339),
        })
}



