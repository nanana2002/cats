#!/bin/bash

# 去除代理设置
unset http_proxy
unset https_proxy
unset all_proxy

# 设置环境变量 - 使用实际IP地址
export PLATFORM_IP="172.28.125.175"  # WSL IP
export PLATFORM_PORT=8080
export SITE1_IP="192.168.235.48"      # site1 服务器
export SITE1_PORT=8081
export SITE2_IP="172.25.21.118"      # site2 wsl宿主机
export SITE2_PORT=8082
export SITE3_IP="172.22.118.77"      # site3 windows
export SITE3_PORT=8085
export SMA_IP="172.28.125.175"       # WSL IP
export SMA_PORT=8083
export PS_IP="172.28.125.175"        # WSL IP
export PS_PORT=8084

# 创建必要的模板目录
mkdir -p templates/platform templates/ps templates/sma

# 动态生成 config.go 文件
cat > config/config.go <<'EOL'
package config

import (
	"reflect"
	"sort"
	"strings"
)

var Cfg = struct {
	Platform struct {
		IP   string
		Port int
		URL  string
	}
	Site1 struct {
		IP   string
		Port int
		URL  string
	}
	Site2 struct {
		IP   string
		Port int
		URL  string
	}
	Site3 struct {
		IP   string
		Port int
		URL  string
	}
	SMA struct {
		IP   string
		Port int
		URL  string
	}
	PS struct {
		IP   string
		Port int
		URL  string
	}
	MonitoredSites []string
	Resource       struct {
		Site1Total int
		Site2Total int
	}
}{
	Platform: struct {
		IP   string
		Port int
		URL  string
	}{
		IP:   "PLATFORM_IP_PLACEHOLDER",
		Port: PLATFORM_PORT_PLACEHOLDER,
		URL:  "http://PLATFORM_IP_PLACEHOLDER:PLATFORM_PORT_PLACEHOLDER",
	},
	Site1: struct {
		IP   string
		Port int
		URL  string
	}{
		IP:   "SITE1_IP_PLACEHOLDER",
		Port: SITE1_PORT_PLACEHOLDER,
		URL:  "http://SITE1_IP_PLACEHOLDER:SITE1_PORT_PLACEHOLDER",
	},
	Site2: struct {
		IP   string
		Port int
		URL  string
	}{
		IP:   "SITE2_IP_PLACEHOLDER",
		Port: SITE2_PORT_PLACEHOLDER,
		URL:  "http://SITE2_IP_PLACEHOLDER:SITE2_PORT_PLACEHOLDER",
	},
	Site3: struct {
		IP   string
		Port int
		URL  string
	}{
		IP:   "SITE3_IP_PLACEHOLDER",
		Port: SITE3_PORT_PLACEHOLDER,
		URL:  "http://SITE3_IP_PLACEHOLDER:SITE3_PORT_PLACEHOLDER",
	},
	SMA: struct {
		IP   string
		Port int
		URL  string
	}{
		IP:   "SMA_IP_PLACEHOLDER",
		Port: SMA_PORT_PLACEHOLDER,
		URL:  "http://SMA_IP_PLACEHOLDER:SMA_PORT_PLACEHOLDER",
	},
	PS: struct {
		IP   string
		Port int
		URL  string
	}{
		IP:   "PS_IP_PLACEHOLDER",
		Port: PS_PORT_PLACEHOLDER,
		URL:  "http://PS_IP_PLACEHOLDER:PS_PORT_PLACEHOLDER",
	},
	MonitoredSites: []string{
		"http://SITE1_IP_PLACEHOLDER:SITE1_PORT_PLACEHOLDER",
		"http://SITE2_IP_PLACEHOLDER:SITE2_PORT_PLACEHOLDER",
		"http://SITE3_IP_PLACEHOLDER:SITE3_PORT_PLACEHOLDER",
	},
	Resource: struct {
		Site1Total int
		Site2Total int
	}{
		Site1Total: 400,
		Site2Total: 500,
	},
}

func GetAllSiteURLs() []string {
	cfgVal := reflect.ValueOf(Cfg)
	cfgType := reflect.TypeOf(Cfg)

	var urls []string
	for i := 0; i < cfgVal.NumField(); i++ {
		field := cfgType.Field(i)
		fieldName := field.Name

		if strings.HasPrefix(fieldName, "Site") {
			siteVal := cfgVal.Field(i)
			if !siteVal.IsValid() || siteVal.Kind() != reflect.Struct {
				continue
			}

			urlField := siteVal.FieldByName("URL")
			if !urlField.IsValid() || urlField.Kind() != reflect.String {
				continue
			}

			urlStr := urlField.String()
			if urlStr == "" {
				continue
			}

			base := strings.TrimRight(urlStr, "/")
			metricsURL := base + "/metrics"
			urls = append(urls, metricsURL)
		}
	}

	seen := make(map[string]bool)
	var result []string
	for _, u := range urls {
		if !seen[u] {
			seen[u] = true
			result = append(result, u)
		}
	}
	sort.Strings(result)
	return result
}
EOL

# 替换占位符为实际环境变量值
sed -i "s|PLATFORM_IP_PLACEHOLDER|$PLATFORM_IP|g" config/config.go
sed -i "s|PLATFORM_PORT_PLACEHOLDER|$PLATFORM_PORT|g" config/config.go
sed -i "s|SITE1_IP_PLACEHOLDER|$SITE1_IP|g" config/config.go
sed -i "s|SITE1_PORT_PLACEHOLDER|$SITE1_PORT|g" config/config.go
sed -i "s|SITE2_IP_PLACEHOLDER|$SITE2_IP|g" config/config.go
sed -i "s|SITE2_PORT_PLACEHOLDER|$SITE2_PORT|g" config/config.go
sed -i "s|SITE3_IP_PLACEHOLDER|$SITE3_IP|g" config/config.go
sed -i "s|SITE3_PORT_PLACEHOLDER|$SITE3_PORT|g" config/config.go
sed -i "s|SMA_IP_PLACEHOLDER|$SMA_IP|g" config/config.go
sed -i "s|SMA_PORT_PLACEHOLDER|$SMA_PORT|g" config/config.go
sed -i "s|PS_IP_PLACEHOLDER|$PS_IP|g" config/config.go
sed -i "s|PS_PORT_PLACEHOLDER|$PS_PORT|g" config/config.go

# 清理旧的日志文件
rm -f platform.log site1.log site2.log sma.log ps.log

# 启动各服务
echo "正在启动公共服务平台..."
nohup go run cmd/platform/main.go > platform.log 2>&1 &
PLATFORM_PID=$!
echo "公共服务平台已启动 (PID: $PLATFORM_PID)"
echo "访问地址: http://$PLATFORM_IP:$PLATFORM_PORT"

echo "正在启动服务站点1..."
nohup go run cmd/site/main.go > site1.log 2>&1 &
SITE1_PID=$!
echo "服务站点1已启动 (PID: $SITE1_PID)"
echo "访问地址: http://$SITE1_IP:$SITE1_PORT"

echo "正在启动服务站点2..."
nohup go run cmd/site2/main.go > site2.log 2>&1 &
SITE2_PID=$!
echo "服务站点2已启动 (PID: $SITE2_PID)"
echo "访问地址: http://$SITE2_IP:$SITE2_PORT"

echo "正在启动服务站点3..."
nohup go run cmd/site3/main.go > site3.log 2>&1 &
SITE3_PID=$!
echo "服务站点3已启动 (PID: $SITE3_PID)"
echo "访问地址: http://$SITE3_IP:$SITE3_PORT"

echo "正在启动C-SMA..."
nohup go run cmd/c-sma/main.go > sma.log 2>&1 &
SMA_PID=$!
echo "C-SMA已启动 (PID: $SMA_PID)"
echo "访问地址: http://$SMA_IP:$SMA_PORT"

echo "正在启动C-PS..."
nohup go run cmd/c-ps/main.go > ps.log 2>&1 &
PS_PID=$!
echo "C-PS已启动 (PID: $PS_PID)"
echo "访问地址: http://$PS_IP:$PS_PORT"

echo "所有服务已启动，可以通过以下地址访问："
echo "- 公共服务平台: http://$PLATFORM_IP:$PLATFORM_PORT"
echo "- 服务站点1: http://$SITE1_IP:$SITE1_PORT"
echo "- 服务站点2: http://$SITE2_IP:$SITE2_PORT"
echo "- 服务站点3: http://$SITE3_IP:$SITE3_PORT"
echo "- C-SMA: http://$SMA_IP:$SMA_PORT"
echo "- C-PS: http://$PS_IP:$PS_PORT"

# 动态生成 stop.sh
cat > stop.sh <<EOL
#!/bin/bash

echo "正在停止所有服务..."
kill $PLATFORM_PID $SITE1_PID $SITE2_PID $SITE3_PID $SMA_PID $PS_PID 2>/dev/null

# 检查端口是否释放
ports=($PLATFORM_PORT $SITE1_PORT $SITE2_PORT $SITE3_PORT $SMA_PORT $PS_PORT)
for port in "\${ports[@]}"; do
    if lsof -i :\$port > /dev/null; then
        echo "警告: 端口 \$port 未释放，尝试强制清理..."
        fuser -k \$port/tcp
        echo "端口 \$port 已强制释放"
    else
        echo "端口 \$port 已成功释放"
    fi

done

echo "所有服务已停止"
EOL
chmod +x stop.sh

echo "如需停止所有服务，请运行 ./stop.sh"