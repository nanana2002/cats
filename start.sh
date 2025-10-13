#!/bin/bash

# 设置环境变量 - 使用实际IP地址
export PLATFORM_IP="172.28.125.175"  # WSL IP
export PLATFORM_PORT=8080
export SITE1_IP="172.22.118.77"      # Windows主机IP
export SITE1_PORT=8081
export SITE2_IP="172.25.21.118"      # WSL所在Windows的IP
export SITE2_PORT=8082
export SMA_IP="172.28.125.175"       # WSL IP
export SMA_PORT=8083
export PS_IP="172.28.125.175"        # WSL IP
export PS_PORT=8084

# 创建必要的模板目录
mkdir -p templates/platform templates/ps templates/sma

# 动态生成 config.go 文件
cat <<EOL > config/config.go
package config

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
        IP:   "$PLATFORM_IP",
        Port: $PLATFORM_PORT,
        URL:  "http://$PLATFORM_IP:$PLATFORM_PORT",
    },
    Site1: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "$SITE1_IP",
        Port: $SITE1_PORT,
        URL:  "http://$SITE1_IP:$SITE1_PORT",
    },
    Site2: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "$SITE2_IP",
        Port: $SITE2_PORT,
        URL:  "http://$SITE2_IP:$SITE2_PORT",
    },
    SMA: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "$SMA_IP",
        Port: $SMA_PORT,
        URL:  "http://$SMA_IP:$SMA_PORT",
    },
    PS: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "$PS_IP",
        Port: $PS_PORT,
        URL:  "http://$PS_IP:$PS_PORT",
    },
    MonitoredSites: []string{
        "http://$SITE1_IP:$SITE1_PORT",
        "http://$SITE2_IP:$SITE2_PORT",
    },
    Resource: struct {
        Site1Total int
        Site2Total int
    }{
        Site1Total: 400,
        Site2Total: 500,
    },
}
EOL

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
echo "- C-SMA: http://$SMA_IP:$SMA_PORT"
echo "- C-PS: http://$PS_IP:$PS_PORT"

# 动态生成 stop.sh
cat > stop.sh <<EOL
#!/bin/bash

echo "正在停止所有服务..."
kill $PLATFORM_PID $SITE1_PID $SITE2_PID $SMA_PID $PS_PID 2>/dev/null

# 检查端口是否释放
ports=($PLATFORM_PORT $SITE1_PORT $SITE2_PORT $SMA_PORT $PS_PORT)
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