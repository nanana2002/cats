#!/bin/bash
set -e
# ==================== 配置 (保持不变) ====================
export LOCAL_LISTEN_IP="127.0.0.1" # ❗ 更改:本地服务监听 127.0.0.1 ❗
export PLATFORM_IP="192.168.67.185" # 外部访问 IP
export PLATFORM_PORT=8080

export SITE1_IP="192.168.235.48"  
export SITE1_PORT=8081

export SITE2_IP="192.168.67.159"  
export SITE2_PORT=8085 

export SMA_IP="192.168.67.185"
export SMA_PORT=8083

export PS_IP="192.168.67.185"
export PS_PORT=8084

# SSH 账户
export SITE1_USER="daiyina"
export SITE1_PASS="hUJ9!s8B"
export SITE2_USER="daiyina" 
export SITE2_PASS="ilovecnbluez" 

# 统一项目根目录（绝对路径）
PROJECT_PATH="$HOME/go-work/src/cmas-cats-go"
LOG_DIR="$PROJECT_PATH/logs"

# 颜色辅助
GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'
ok() { echo -e "${GREEN}✓${NC} $1"; }
err() { echo -e "${RED}✗${NC} $1"; }
# 保持 -t 选项
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=30 -o ServerAliveInterval=3 -o Compression=no -o LogLevel=ERROR -t" 
REMOTE_CHECK_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=5"


# ==================== 环境检查 ====================
command -v sshpass >/dev/null 2>&1 || { err "请安装 sshpass"; exit 1; }
command -v go >/dev/null 2>&1 || { err "请安装 Go 1.21+"; exit 1; }
mkdir -p "$LOG_DIR"

for site in 1 2; do
  u="SITE${site}_USER"; h="SITE${site}_IP"; p="SITE${site}_PASS"
  # 基础 SSH 检查
  if ! sshpass -p "${!p}" ssh $REMOTE_CHECK_OPTS "${!u}@${!h}" "echo test" >/dev/null 2>&1; then
    err "Site$site SSH 连接失败:${!u}@${!h}"; exit 1
  fi
  ok "Site$site SSH 正常"
done

# ==================== 生成 config.go (保持不变) ====================
mkdir -p config templates/platform templates/ps templates/sma
cat > config/config.go <<EOF
package config

// LOCAL_LISTEN_IP 供本地服务 (Platform, C-SMA, C-PS) 实际监听使用 (应为 0.0.0.0 或 127.0.0.1)
var LOCAL_LISTEN_IP = "$LOCAL_LISTEN_IP"

var Cfg = struct {
    Platform struct{ IP string; Port int; URL string }
    Site1    struct{ IP string; Port int; URL string }
    Site2    struct{ IP string; Port int; URL string }
    SMA      struct{ IP string; Port int; URL string }
    PS       struct{ IP string; Port int; URL string }
    MonitoredSites []string
    Resource struct{ Site1Total, Site2Total int }
}{
    Platform: struct{ IP string; Port int; URL string }{IP: "$PLATFORM_IP", Port: $PLATFORM_PORT, URL: "http://$PLATFORM_IP:$PLATFORM_PORT"},
    Site1:    struct{ IP string; Port int; URL string }{IP: "$SITE1_IP", Port: $SITE1_PORT, URL: "http://$SITE1_IP:$SITE1_PORT"},
    Site2:    struct{ IP string; Port int; URL string }{IP: "$SITE2_IP", Port: $SITE2_PORT, URL: "http://$SITE2_IP:$SITE2_PORT"},
    SMA:      struct{ IP string; Port int; URL string }{IP: "$SMA_IP", Port: $SMA_PORT, URL: "http://$SMA_IP:$SMA_PORT"},
    PS:       struct{ IP string; Port int; URL string }{IP: "$PS_IP", Port: $PS_PORT, URL: "http://$PS_IP:$PS_PORT"},
    MonitoredSites: []string{
        "http://$SITE1_IP:$SITE1_PORT",
        "http://$SITE2_IP:$SITE2_PORT",
    },
    Resource: struct{ Site1Total, Site2Total int }{Site1Total: 400, Site2Total: 500},
}

// GetAllSiteURLs 用于 C-SMA 模块，它需要一个函数来获取所有监控站点的URL
func GetAllSiteURLs() []string {
    return Cfg.MonitoredSites
}
EOF
ok "配置文件已生成"


# ==================== 启动远程服务 (修复 go 命令找不到) ====================
for site in 1 2; do
  u="SITE${site}_USER"; h="SITE${site}_IP"; p="SITE${site}_PASS"; port="SITE${site}_PORT"
  
  if [ $site -eq 1 ]; then
    cmd_name="site"; log_name="site1" # cmd/site/main.go -> logs/site1.log
  else # site 2 (Mac)
    cmd_name="site2"; log_name="site2" # cmd/site2/main.go -> logs/site2.log
  fi
  
  REMOTE_BASE_DIR="\$HOME/go-work/src/cmas-cats-go" 
  
  # ❗ 最终修复远程 Go 启动问题:使用 bash -lc 启动登录 Shell，确保 $PATH 正确加载
  CMD="mkdir -p logs && bash -lc 'cd \"${REMOTE_BASE_DIR}\" && nohup go run cmd/${cmd_name}/main.go > logs/${log_name}.log 2>&1 &' && sleep 3 && echo SERVICE_STARTED"

  echo ""
  ok "启动 Site$site ($h)..."
  # 使用 -t 选项和 SSH 远程执行 CMD
  sshpass -p "${!p}" ssh $SSH_OPTS "${!u}@${!h}" "${CMD}"

  # 简单健康检查
  sleep 2
  if curl -s --connect-timeout 3 "http://${!h}:${!port}/metrics" >/dev/null; then
    ok "Site$site 已就绪"
  else
    err "Site$site 启动失败，查看日志:ssh ${!u}@${!h} 'tail -20 ${REMOTE_BASE_DIR}/logs/${log_name}.log'"
  fi
done

# ==================== 启动本地 WSL 服务 (保持不变) ====================
ok "启动本地 Platform / C-SMA / C-PS ..."
for svc in platform c-sma c-ps; do
  # 确保变量名正确映射
  if [ "$svc" == "c-sma" ]; then
    port_var="SMA_PORT"
  elif [ "$svc" == "c-ps" ]; then
    port_var="PS_PORT"
  else
    port_var="PLATFORM_PORT"
  fi
  
  # 远程启动命令:依赖 Go 代码使用 config.LOCAL_LISTEN_IP (127.0.0.1) 
  nohup go run cmd/${svc}/main.go > "$LOG_DIR/${svc}.log" 2>&1 &
  sleep 2
  
  # 健康检查使用 localhost
  if curl -s --connect-timeout 3 "http://localhost:${!port_var}/metrics" >/dev/null; then 
    ok "${svc^^} 启动成功"
  else
    err "${svc^^} 启动失败，日志:tail -20 $LOG_DIR/${svc}.log"
  fi
done

go run web_server.go 

# ==================== 访问提示 (保持不变) ====================
cat <<EOF
==================== 系统已启动 ====================
平台:     http://$PLATFORM_IP:$PLATFORM_PORT
Site1:    http://$SITE1_IP:$SITE1_PORT  (Linux)
Site2:    http://$SITE2_IP:$SITE2_PORT  (Mac)
C-SMA:    http://$SMA_IP:$SMA_PORT
C-PS:     http://$PS_IP:$PS_PORT
日志:     $LOG_DIR
停止:     ./stop.sh
==================================================
EOF