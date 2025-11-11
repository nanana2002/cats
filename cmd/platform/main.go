package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"strconv"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3" // SQLite驱动
	"cmas-cats-go/config"
	"cmas-cats-go/models"
)

// 全局数据库连接
var db *sql.DB

func main() {
	// 启动标识日志（确保main函数执行）
	fmt.Println("=====================================")
	fmt.Println("            公共服务平台启动中...            ")
	fmt.Println("=====================================")

	// 1. 初始化数据库（带详细日志）
	if err := initDB(); err != nil {
		fmt.Printf("❌ 初始化失败，程序退出：%v\n", err)
		return // 初始化失败则退出
	}
	defer db.Close() // 程序退出时关闭数据库连接

	// 2. 初始化Gin引擎（默认开启调试日志）
	r := gin.Default() // ❗ 引擎实例名为 r ❗

	// 3. 注册API路由
	r.POST("/api/v1/services", registerServiceHandler)          // 注册服务
	r.GET("/api/v1/services", getServicesHandler)                  // 获取所有服务
	r.GET("/api/v1/services/:id", getServiceByIDHandler)       // 获取单个服务详情

	// 4. 添加简单的Web界面
	r.LoadHTMLGlob("./templates/platform/*.html")
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "公共服务平台",
		})
	})
	r.GET("/dashboard", func(c *gin.Context) {
		services := []models.Service{}
		rows, err := db.Query("SELECT id, name, description FROM services")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var svc models.Service
				// 假设 models.Service 只有这三个字段的数据库兼容性
				rows.Scan(&svc.ID, &svc.Name, &svc.Description)
				services = append(services, svc)
			}
		}
		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"title": "服务管理面板",
			"services": services,
		})
	})
    
	// 5. 启动服务配置
	// 实际监听地址必须使用 config.LOCAL_LISTEN_IP ("0.0.0.0")
	listenAddr := config.LOCAL_LISTEN_IP + ":" + strconv.Itoa(config.Cfg.Platform.Port)
    
	// 外部展示地址
	externalListenAddr := fmt.Sprintf("http://%s:%d", config.Cfg.Platform.IP, config.Cfg.Platform.Port)
    
	// 启动服务前打印信息
	fmt.Printf("\n✅ 公共服务平台启动成功！\n")
	fmt.Printf("📌 监听地址：%s\n", externalListenAddr)
	fmt.Printf("📌 可用接口：\n")
	fmt.Printf("    - POST    /api/v1/services          注册服务\n")
	fmt.Printf("    - GET      /api/v1/services          获取所有服务\n")
	fmt.Printf("    - GET      /api/v1/services/:id    获取单个服务详情\n")

	// ❗ 修复：使用 r 实例启动HTTP服务（带错误处理） ❗
	if err := r.Run(listenAddr); err != nil {
		// router.Run(listenAddr) // 原始错误代码 1：router 未定义
		// 原始错误代码 2：在第一个 if 之后还有重复的 r.Run(listenAddr)
		fmt.Printf("❌ 服务启动失败：%v\n", err)
	}
}

// initDB：初始化SQLite数据库（带详细错误日志） (保持不变)
func initDB() error {
	var err error

	// 1. 打开数据库文件（不存在则自动创建）
	db, err = sql.Open("sqlite3", "./db/platform.db")
	if err != nil {
		return fmt.Errorf("数据库连接失败：%w", err)
	}

	// 2. 验证数据库连接（sql.Open不会立即连接，需手动Ping）
	err = db.Ping()
	if err != nil {
		return fmt.Errorf("数据库连接验证失败：%w", err)
	}

	// 3. 创建服务表（与models.Service结构对应）
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS services (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		input_format TEXT,
		computing_requirement TEXT NOT NULL,
		storage_requirement TEXT,
		computing_time TEXT,
		code_location TEXT,
		software_dependency TEXT, -- 存储JSON数组字符串
		created_at DATETIME NOT NULL, -- 对应time.Time类型
		validation_sample TEXT,
		validation_result TEXT
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("创建services表失败：%w", err)
	}

	fmt.Println("✅ 数据库初始化成功（SQLite）")
	return nil
}

// registerServiceHandler：处理服务注册请求 (保持不变)
func registerServiceHandler(c *gin.Context) {
	var service models.Service

	// 1. 解析请求体
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	// 2. 生成服务ID（确保唯一性）
	service.ID = "AR" + fmt.Sprintf("%d", time.Now().UnixNano()/1e6) // 毫秒级时间戳

	// 3. 设置创建时间（time.Time类型，修复类型错误）
	service.CreatedAt = time.Now()

	// 4. 序列化软件依赖列表（[]string → JSON字符串）
	depsJSON, err := json.Marshal(service.SoftwareDependency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "软件依赖序列化失败：" + err.Error(),
		})
		return
	}

	// 5. 插入数据库
	_, err = db.Exec(`
		INSERT INTO services (
			id, name, description, input_format, computing_requirement,
			storage_requirement, computing_time, code_location, software_dependency,
			created_at, validation_sample, validation_result
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		service.ID, service.Name, service.Description, service.InputFormat,
		service.ComputingRequirement, service.StorageRequirement, service.ComputingTime,
		service.CodeLocation, string(depsJSON), service.CreatedAt, // 正确传入time.Time类型
		service.ValidationSample, service.ValidationResult)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "服务注册失败（数据库错误）：" + err.Error(),
		})
		return
	}

	// 6. 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "服务注册成功",
		"service_id": service.ID,
		"created_at": service.CreatedAt.Format(time.RFC3339), // 响应时转为字符串
	})
	fmt.Printf("[%s] 服务注册成功：ID=%s, 名称=%s\n",
		time.Now().Format("15:04:05"), service.ID, service.Name)
}

// getServicesHandler：获取所有服务列表 (保持不变)
func getServicesHandler(c *gin.Context) {
	// 1. 查询数据库
	rows, err := db.Query(`
		SELECT id, name, description, input_format, computing_requirement,
			   storage_requirement, computing_time, code_location,
			   software_dependency, created_at
		FROM services
		ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询服务列表失败：" + err.Error(),
		})
		return
	}
	defer rows.Close()

	// 2. 解析结果
	var services []models.Service
	for rows.Next() {
		var s models.Service
		var depsJSON string          // 数据库中存储的JSON字符串
		var createdAt time.Time    // 从数据库读取的time.Time类型

		// 扫描字段（注意与表结构顺序一致）
		err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.InputFormat, &s.ComputingRequirement,
			&s.StorageRequirement, &s.ComputingTime, &s.CodeLocation,
			&depsJSON, &createdAt,
		)
		if err != nil {
			fmt.Printf("⚠️ 解析服务数据失败：%v\n", err)
			continue
		}

		// 反序列化软件依赖
		if err := json.Unmarshal([]byte(depsJSON), &s.SoftwareDependency); err != nil {
			fmt.Printf("⚠️ 解析依赖列表失败：%v\n", err)
			s.SoftwareDependency = []string{} // 默认为空列表
		}

		// 赋值时间字段
		s.CreatedAt = createdAt

		services = append(services, s)
	}

	// 3. 返回结果
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"count":      len(services),
		"services": services,
	})
}

// getServiceByIDHandler：根据ID获取单个服务详情 (保持不变)
func getServiceByIDHandler(c *gin.Context) {
	serviceID := c.Param("id")
	if serviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少服务ID参数",
		})
		return
	}

	// 查询数据库
	var s models.Service
	var depsJSON string
	var createdAt time.Time

	err := db.QueryRow(`
		SELECT id, name, description, input_format, computing_requirement,
			   storage_requirement, computing_time, code_location,
			   software_dependency, created_at, validation_sample, validation_result
		FROM services WHERE id = ?`, serviceID).Scan(
		&s.ID, &s.Name, &s.Description, &s.InputFormat, &s.ComputingRequirement,
		&s.StorageRequirement, &s.ComputingTime, &s.CodeLocation,
		&depsJSON, &createdAt, &s.ValidationSample, &s.ValidationResult,
	)

	// 处理查询结果
	switch {
	case err == sql.ErrNoRows:
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "服务不存在（ID：" + serviceID + "）",
		})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询服务失败：" + err.Error(),
		})
		return
	}

	// 反序列化依赖列表
	json.Unmarshal([]byte(depsJSON), &s.SoftwareDependency)
	s.CreatedAt = createdAt

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"service": s,
	})
}