# 🎼 Instrument Server

[![Go Version](https://img.shields.io/badge/Go-1.19+-00ADD8?logo=go)](https://golang.org/) [![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
 [![Build](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com)

---

 **[English](#english)** | **[中文](#chinese)**

<a name="english"></a>
## 📖 English
High-performance TCP data gateway built with Go, providing concurrent connection handling, real-time data collection, protocol parsing and forwarding for industrial equipment and IoT sensors.
<a name="chinese"></a>
## 📖 中文
基于Go语言的高性能TCP数据接入网关，提供高并发连接处理、实时数据采集、协议解析和数据转发，适用于工业设备和IoT传感器场景。

---

## 一、压力测试

### 1. 基本测试（10000设备，每秒1条）

```bash
go run test/stress/main.go -server localhost:8888 -devices 10000 -interval 1s -duration 60s
```

### 2、高频测试（1000设备，每100ms一条）

```bash
go run test/stress/main.go \
    -server localhost:8888 \
    -devices 1000 \
    -interval 100ms \
    -duration 300s
```

### 3. 极限测试（50000设备，每秒1条）

```bash
go run test/stress/main.go \
    -server localhost:8888 \
    -devices 50000 \
    -interval 1s \
    -duration 600s
```

### 4. 调试模式

```bash
go run test/stress/main.go \
    -server localhost:8888 \
    -devices 100 \
    -interval 1s \
    -duration 30s \
    -debug
```

### 5. 编译后运行

```bash
# 编译
go build -o build/stress-test test/stress/main.go

# 运行
./build/stress-test -server localhost:8888 -devices 10000 -interval 1s -duration 120s
```

## 二、高级压力测试脚本

### 1、10000设备，每秒1条，运行5分钟

```bash
go run test/stress/advanced_test.go -server localhost:8888 -devices 10000 -duration 5m -metrics-port 9090
```

### 2、极限测试

```bash
# 50000设备，每秒1条，手动停止
go run test/stress/advanced_test.go \
    -server localhost:8888 \
    -devices 50000 \
    -interval 1s \
    -duration 0 \
    -batch 100 \
    -delay 200ms
```