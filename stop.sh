#!/bin/bash
set -e
# ==================== 停止脚本 (按端口杀进程) ====================

# --- 停止本地服务 (WSL) ---
# 增加 -r 参数（xargs -r：无参数时不执行命令，消除 kill 帮助信息）
echo ">>> 停止本地 8080/8083/8084 端口进程..."
lsof -i:8080 | grep -v "PID" | awk '{print $2}' | xargs -r kill -9
lsof -i:8083 | grep -v "PID" | awk '{print $2}' | xargs -r kill -9
lsof -i:8084 | grep -v "PID" | awk '{print $2}' | xargs -r kill -9
echo ">>> 本地进程处理完成"


# --- 停止远程 site1 (192.168.235.48) ---
REMOTE_USER1="daiyina"
REMOTE_IP1="192.168.235.48"
REMOTE_PASS1="hUJ9!s8B"
TARGET_PORT1=8081

echo ">>> 开始杀远程 ${REMOTE_IP1}:${TARGET_PORT1} 的进程..."
sshpass -p "${REMOTE_PASS1}" ssh -o StrictHostKeyChecking=no "${REMOTE_USER1}@${REMOTE_IP1}" "
  lsof -i:${TARGET_PORT1} | grep -v 'PID' | awk '{print \$2}' | xargs -r kill -9
  echo '远程进程已处理（若存在）'
"
echo ">>> site1操作完成"


# --- 停止远程 site2 (192.168.67.159) ---
REMOTE_USER2="daiyina"
REMOTE_IP2="192.168.67.159"
REMOTE_PASS2="ilovecnbluez"
TARGET_PORT2=8085

echo ">>> 开始杀远程 ${REMOTE_IP2}:${TARGET_PORT2} 的进程..."
sshpass -p "${REMOTE_PASS2}" ssh -o StrictHostKeyChecking=no "${REMOTE_USER2}@${REMOTE_IP2}" "
  lsof -i:${TARGET_PORT2} | grep -v 'PID' | awk '{print \$2}' | xargs -r kill -9
  echo '远程进程已处理（若存在）'
"
echo ">>> site2操作完成"

echo "=== 所有停止操作结束 ==="