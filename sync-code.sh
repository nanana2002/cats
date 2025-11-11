#!/bin/bash
set -e
PROJECT_PATH="$HOME/go-work/src/cmas-cats-go"
# 增加 Compression=no 禁用压缩（减少 CPU 占用，避免断连），延长超时到 30 秒
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=30 -o ServerAliveInterval=3 -o Compression=no -o LogLevel=ERROR"

# 本地临时文件
LOCAL_TAR="/tmp/cmas-cats-go-sync.tar.gz"

# 1. 打包项目
echo "=== 开始打包项目 ==="
tar -zcf "$LOCAL_TAR" \
  --exclude='*.git' --exclude='*/.git/*' \
  --exclude='*.log' --exclude='*/logs/*' \
  -C "${PROJECT_PATH}/.." "$(basename "${PROJECT_PATH}")"
echo "=== 打包完成（路径：$LOCAL_TAR，大小：$(du -sh "$LOCAL_TAR" | awk '{print $1}')） ==="

# 同步函数
sync_tar() {
  local user=$1 host=$2 pass=$3 remote=$4 untar_cmd=$5
  echo -e "\n>>> 同步到 ${user}@${host}:${remote}"

  # --- 1. 远程创建临时目录 ---
  local REMOTE_TMP_DIR_SHORT="~/.tmp" # 仅用于 mkdir
  if ! sshpass -p "$pass" ssh $SSH_OPTS "${user}@${host}" "mkdir -p ${REMOTE_TMP_DIR_SHORT}"; then
    echo "  ❌ 无法创建远程临时目录，跳过该主机"
    return 1
  fi

  # --- 2. 文件传输 (SCP) ---
  
  echo "  尝试使用 SCP 协议上传..."
  # Site 1/2 使用空选项（已验证成功，避免不必要的速率限制）
  local scp_options=""
  
  if sshpass -p "$pass" scp $SSH_OPTS $scp_options "$LOCAL_TAR" "${user}@${host}:${REMOTE_TMP_DIR_SHORT}/"; then
    echo "  ✅ 压缩包上传成功"
  else
    echo "  ❌ SCP 上传失败"
    return 1
  fi
  
  # --- 3. 远程解压+清理 ---
  
  # 构造绝对路径用于远程解压
  local REMOTE_HOME=$(sshpass -p "$pass" ssh $SSH_OPTS "${user}@${host}" 'echo $HOME')
  local REMOTE_TAR_ABS_PATH="${REMOTE_HOME}/.tmp/$(basename "$LOCAL_TAR")"
  
  echo "  远程文件路径确定为: ${REMOTE_TAR_ABS_PATH}"
  
  # 远程解压+清理
  sshpass -p "$pass" ssh $SSH_OPTS "${user}@${host}" "bash -c '
    mkdir -p \"${remote}\" &&
    ${untar_cmd} \"${REMOTE_TAR_ABS_PATH}\" -C \"${remote}/..\" &&
    rm -f \"${REMOTE_TAR_ABS_PATH}\"
  '"
  
  [ $? -eq 0 ] && echo "  ✅ 同步完成" || echo "  ❌ 远程解压失败"
}

# 执行同步
# Site 1 (Linux)
sync_tar "daiyina" "192.168.235.48" "hUJ9!s8B" "/home/daiyina/go-work/src/cmas-cats-go" "tar -zxf"
# Site 2 (Mac - 原 Site 3)
sync_tar "daiyina" "192.168.67.159" "ilovecnbluez" "/Users/daiyina/go-work/src/cmas-cats-go" "tar -zxf"

# 清理本地临时文件
rm -f "$LOCAL_TAR"
echo -e "\n=== 所有同步操作结束，本地临时文件已清理 ==="