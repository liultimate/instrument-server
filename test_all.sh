#!/bin/bash

echo "================================"
echo "完整测试流程"
echo "================================"

# 1. 等待服务器启动
echo "[1] 等待服务器启动..."
sleep 2

# 2. 健康检查
echo "[2] 健康检查..."
curl -s http://localhost:9090/health
echo ""

# 3. 发送测试数据
echo "[3] 发送测试数据..."
go run test/client/main.go -host localhost:8888 -count 5 -interval 500ms

# 4. 检查Redis数据
echo "[4] 检查Redis数据..."
redis-cli LLEN "instrument:127.0.0.1:*:data" 2>/dev/null || echo "Redis中暂无数据"

# 5. 查看Metrics
echo "[5] 查看监控指标..."
curl -s http://localhost:9090/metrics | grep instrument_data_processed_total

echo ""
echo "测试完成！"
