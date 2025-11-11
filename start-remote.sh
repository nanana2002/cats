#!/bin/bash
set -e

# 配置（✓ 已修正路径拼写）
SITE1_USER="daiyina"
SITE1_HOST="192.168.235.48"
SITE1_PASS="hUJ9!s8B"
SITE1_DIR="/home/daiyina/go-work/src/cmas-cats-go"

SITE2_USER="daiyn"
SITE2_HOST="192.168.67.185"
SITE2_PASS="ilovecnblueZ1."
SITE2_DIR="D:/go-work/src/cmas-cats-go"

SITE3_USER="daiyina"
SITE3_HOST="192.168.67.159"
SITE3_PASS="ilovecnbluez"
SITE3_DIR="/Users/daiyina/go-work/src/cmas-cats-go"  # ✓ 正确拼写

LOG_DIR="./logs"
mkdir -p "$LOG_DIR"

# 安装 sshpass
[ ! -x "$(command -v sshpass)" ] && sudo apt install -y sshpass

echo "🚀 开始远程启动服务站点..."

# site1: Linux（✓ SSH + 后台）
echo "📌 启动 site1 @$SITE1_HOST:8081"
sshpass -p "$SITE1_PASS" ssh -o StrictHostKeyChecking=no "$SITE1_USER@$SITE1_HOST" "
  cd '$SITE1_DIR' && \
  nohup go run cmd/site/main.go > /tmp/site1.log 2>&1 &
  echo '✅ site1 PID:' \$!
" &
SITE1_PID=$!

# site2: Windows（✓ 本地直接启动，不走 WinRM）
echo "📌 启动 site2 @$SITE2_HOST:8082"
cmd.exe /c "start /B D:\\tools\\go\\bin\\go.exe run $SITE2_DIR\\cmd\\site2\\main.go"
sleep 1
echo "✅ site2 已启动（本地后台）" &
SITE2_PID=$!

# site3: Mac（✓ SSH + 后台）
echo "📌 启动 site3 @$SITE3_HOST:8085"
sshpass -p "$SITE3_PASS" ssh -o StrictHostKeyChecking=no "$SITE3_USER@$SITE3_HOST" "
  cd '$SITE3_DIR' && \
  nohup go run cmd/site3/main.go > /tmp/site3.log 2>&1 &
  echo '✅ site3 PID:' \$!
" &
SITE3_PID=$!

sleep 3
echo "✅ 启动提交完成（PID: $SITE1_PID, $SITE2_PID, $SITE3_PID）"

# 检查
for name host port in \
  "site1" "$SITE1_HOST" 8081 \
  "site2" "$SITE2_HOST" 8082 \
  "site3" "$SITE3_HOST" 8085
do
  echo -n "[$name] http://$host:$port/metrics → "
  if timeout 3 curl -s "http://$host:$port/metrics" | grep -q '"success":true'; then
    echo "✅ OK"
  else
    echo "❌ FAIL"
  fi
done