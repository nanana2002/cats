#!/bin/bash

echo "正在停止所有服务..."
kill 32242 32243 32244 32245 32246 2>/dev/null

# 检查端口是否释放
ports=(8080 8081 8082 8083 8084)
for port in "${ports[@]}"; do
    if lsof -i :$port > /dev/null; then
        echo "警告: 端口 $port 未释放，尝试强制清理..."
        fuser -k $port/tcp
        echo "端口 $port 已强制释放"
    else
        echo "端口 $port 已成功释放"
    fi

done

echo "所有服务已停止"
