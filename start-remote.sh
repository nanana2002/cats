#!/bin/bash
# start-remote.sh
# 从 WSL 启动远程 site1/2/3

# ===== 配置区 =====
SITE1_USER="daiyina"
SITE1_HOST="192.168.235.48"
SITE1_DIR="/home/daiyina/go-work/src/cmas-cats-go"

SITE2_HOST="localhost"  # WSL 自身 → site2 在 Windows 主机
SITE2_DIR="D:\\go-work\\src\\cmas-cats-go"  # Windows 路径

SITE3_HOST="172.22.118.77"
SITE3_USER="pcsys"       # 替换为 site3 的 Windows 用户名
SITE3_PASS="YourPassword" # ⚠️ 临时用；建议用证书或 CredSSP
SITE3_DIR="D:\\go-work\\src\\cmas-cats-go"

# 日志目录
LOG_DIR="./logs"
mkdir -p "$LOG_DIR"

echo "🚀 开始远程启动服务站点..."

# === 启动 site1 (Ubuntu) ===
echo "📌 启动 site1 @ $SITE1_HOST:8081 ..."
ssh "$SITE1_USER@$SITE1_HOST" "
  cd '$SITE1_DIR' && \
  nohup go run cmd/site/main.go > '$LOG_DIR/site1.log' 2>&1 &
  echo \"✅ site1 启动命令已提交，PID: \$!\"
"

# === 启动 site2 (本地 Windows) ===
echo "📌 启动 site2 @ $SITE2_HOST:8082 ..."
powershell.exe -Command "
  Start-Process -FilePath 'go' -ArgumentList 'run','cmd/site2/main.go' -WorkingDirectory '$SITE2_DIR' -NoNewWindow -RedirectStandardOutput '$LOG_DIR/site2.log' -RedirectStandardError '$LOG_DIR/site2.err'
  Write-Host '✅ site2 启动命令已提交（后台）'
"

# === 启动 site3 (远程 Windows) ===
echo "📌 启动 site3 @ $SITE3_HOST:8085 ..."
# 使用 PowerShell Remoting（需 WinRM 开启）
powershell.exe -Command "
  \$securePass = ConvertTo-SecureString '$SITE3_PASS' -AsPlainText -Force
  \$cred = New-Object System.Management.Automation.PSCredential('$SITE3_USER', \$securePass)
  Invoke-Command -ComputerName '$SITE3_HOST' -Credential \$cred -ScriptBlock {
    param(\$dir)
    cd \$dir
    Start-Process -FilePath 'go' -ArgumentList 'run','cmd/site3/main.go' -WorkingDirectory \$dir -NoNewWindow -RedirectStandardOutput 'D:\\logs\\site3.log' -RedirectStandardError 'D:\\logs\\site3.err'
  } -ArgumentList '$SITE3_DIR'
  Write-Host '✅ site3 启动命令已远程提交'
"

echo "✅ 远程启动流程完成！"
echo "🔍 检查日志："
echo "   site1: $LOG_DIR/site1.log"
echo "   site2: $LOG_DIR/site2.log"
echo "   site3: $LOG_DIR/site3.log (本地) / D:\\logs\\site3.log (远程)"
echo
echo "🧪 测试连接："
echo "   curl http://$SITE1_HOST:8081/metrics"
echo "   curl http://$SITE2_HOST:8082/metrics  # 注意：Windows 防火墙需放行"
echo "   curl http://$SITE3_HOST:8085/metrics"
