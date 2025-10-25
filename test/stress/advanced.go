package main

import (
    "context"
    "encoding/binary"
    "flag"
    "fmt"
    "math/rand"
    "net"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "sync/atomic"
    "syscall"
    "time"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/sirupsen/logrus"
)

// Prometheus指标
var (
    devicesActive = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "stress_test_devices_active",
        Help: "当前活跃设备数",
    })

    devicesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "stress_test_devices_total",
        Help: "总设备数",
    })

    packetsSent = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "stress_test_packets_sent_total",
        Help: "总发送包数",
    })

    packetsFailed = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "stress_test_packets_failed_total",
        Help: "总失败包数",
    })

    bytesSent = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "stress_test_bytes_sent_total",
        Help: "总发送字节数",
    })

    sendDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "stress_test_send_duration_seconds",
        Help:    "发送耗时分布",
        Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
    })

    connectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "stress_test_connections_total",
        Help: "总连接数",
    })

    connectionsFailed = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "stress_test_connections_failed_total",
        Help: "连接失败数",
    })

    reconnectsTotal = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "stress_test_reconnects_total",
        Help: "总重连次数",
    })

    qpsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "stress_test_qps",
        Help: "当前QPS",
    })

    bandwidthGauge = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "stress_test_bandwidth_bytes",
        Help: "当前带宽(字节/秒)",
    })

    packetTypeCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "stress_test_packets_by_type_total",
            Help: "按数据类型统计的包数",
        },
        []string{"type"},
    )

    packetStatusCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "stress_test_packets_by_status_total",
            Help: "按状态统计的包数",
        },
        []string{"status"},
    )
)

func init() {
    prometheus.MustRegister(devicesActive)
    prometheus.MustRegister(devicesTotal)
    prometheus.MustRegister(packetsSent)
    prometheus.MustRegister(packetsFailed)
    prometheus.MustRegister(bytesSent)
    prometheus.MustRegister(sendDuration)
    prometheus.MustRegister(connectionsTotal)
    prometheus.MustRegister(connectionsFailed)
    prometheus.MustRegister(reconnectsTotal)
    prometheus.MustRegister(qpsGauge)
    prometheus.MustRegister(bandwidthGauge)
    prometheus.MustRegister(packetTypeCounter)
    prometheus.MustRegister(packetStatusCounter)
}

// 统计指标
type Stats struct {
    TotalSent      int64
    TotalFailed    int64
    TotalConnected int64
    ConnectFailed  int64
    ActiveDevices  int64
    TotalBytes     int64
    Reconnects     int64
}

// 高级设备模拟器
type AdvancedDevice struct {
    ID             int
    ServerAddr     string
    SendInterval   time.Duration
    Stats          *Stats
    Log            *logrus.Logger
    ctx            context.Context
    cancel         context.CancelFunc
    conn           net.Conn
    connMutex      sync.Mutex
    lastSendTime   time.Time
    packetCount    int64
    reconnectCount int64
}

func NewAdvancedDevice(id int, serverAddr string, interval time.Duration, stats *Stats, log *logrus.Logger) *AdvancedDevice {
    ctx, cancel := context.WithCancel(context.Background())
    return &AdvancedDevice{
        ID:           id,
        ServerAddr:   serverAddr,
        SendInterval: interval,
        Stats:        stats,
        Log:          log,
        ctx:          ctx,
        cancel:       cancel,
        lastSendTime: time.Now(),
    }
}

// connect 建立连接
func (d *AdvancedDevice) connect() error {
    d.connMutex.Lock()
    defer d.connMutex.Unlock()

    if d.conn != nil {
        d.conn.Close()
        d.conn = nil
    }

    dialer := net.Dialer{
        Timeout:   10 * time.Second,
        KeepAlive: 30 * time.Second,
    }

    conn, err := dialer.DialContext(d.ctx, "tcp", d.ServerAddr)
    if err != nil {
        return err
    }

    // 设置TCP keepalive
    if tcpConn, ok := conn.(*net.TCPConn); ok {
        tcpConn.SetKeepAlive(true)
        tcpConn.SetKeepAlivePeriod(30 * time.Second)
        tcpConn.SetNoDelay(true)
    }

    d.conn = conn
    return nil
}

// send 发送数据
func (d *AdvancedDevice) send(packet []byte, dataType, status uint8) error {
    d.connMutex.Lock()
    conn := d.conn
    d.connMutex.Unlock()

    if conn == nil {
        return fmt.Errorf("未连接")
    }

    start := time.Now()

    conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
    n, err := conn.Write(packet)

    duration := time.Since(start)
    sendDuration.Observe(duration.Seconds())

    if err != nil {
        return err
    }

    atomic.AddInt64(&d.Stats.TotalBytes, int64(n))
    bytesSent.Add(float64(n))

    // 更新Prometheus指标
    packetTypeCounter.WithLabelValues(getDataTypeName(dataType)).Inc()
    packetStatusCounter.WithLabelValues(getStatusName(status)).Inc()

    d.lastSendTime = time.Now()
    d.packetCount++

    return nil
}

// Run 运行设备模拟器
// Run 运行设备模拟器
func (d *AdvancedDevice) Run(wg *sync.WaitGroup) {
    defer wg.Done()
    defer d.Stop()

    // 初始连接（带指数退避重试）
    var err error
    backoff := time.Second
    maxBackoff := 30 * time.Second

    for retry := 0; retry < 5; retry++ {
        err = d.connect()
        if err == nil {
            break
        }

        d.Log.Warnf("设备 %d 连接失败(重试 %d/5): %v", d.ID, retry+1, err)

        select {
        case <-d.ctx.Done():
            return
        case <-time.After(backoff):
            backoff *= 2
            if backoff > maxBackoff {
                backoff = maxBackoff
            }
        }
    }

    if err != nil {
        d.Log.Errorf("设备 %d 连接失败，已放弃: %v", d.ID, err)
        atomic.AddInt64(&d.Stats.ConnectFailed, 1)
        connectionsFailed.Inc()
        return
    }

    atomic.AddInt64(&d.Stats.TotalConnected, 1)
    atomic.AddInt64(&d.Stats.ActiveDevices, 1)
    connectionsTotal.Inc()
    devicesActive.Inc()

    defer func() {
        atomic.AddInt64(&d.Stats.ActiveDevices, -1)
        devicesActive.Dec()
    }()

    d.Log.Debugf("设备 %d 已连接", d.ID)

    ticker := time.NewTicker(d.SendInterval)
    defer ticker.Stop()

    // 健康检查定时器
    healthTicker := time.NewTicker(30 * time.Second)
    defer healthTicker.Stop()

    consecutiveErrors := 0
    maxErrors := 3

    for {
        select {
        case <-d.ctx.Done():
            d.Log.Debugf("设备 %d 收到停止信号", d.ID)
            return

        case <-healthTicker.C:
            // 健康检查：如果长时间没有发送，尝试发送心跳
            if time.Since(d.lastSendTime) > 60*time.Second {
                d.Log.Debugf("设备 %d 执行健康检查", d.ID)
            }

        case <-ticker.C:
            packet, dataType, status := d.generatePacket()
            err := d.send(packet, dataType, status)

            if err != nil {
                consecutiveErrors++
                atomic.AddInt64(&d.Stats.TotalFailed, 1)
                packetsFailed.Inc()
                d.Log.Warnf("设备 %d 发送失败(连续 %d 次): %v", d.ID, consecutiveErrors, err)

                // 达到错误阈值，尝试重连
                if consecutiveErrors >= maxErrors {
                    d.Log.Warnf("设备 %d 尝试重连...", d.ID)

                    // ✅ 修复：在循环外定义 reconnErr
                    reconnBackoff := time.Second
                    var reconnErr error
                    
                    for retry := 0; retry < 3; retry++ {
                        reconnErr = d.connect()
                        if reconnErr == nil {
                            atomic.AddInt64(&d.Stats.Reconnects, 1)
                            reconnectsTotal.Inc()
                            d.reconnectCount++
                            consecutiveErrors = 0
                            d.Log.Infof("设备 %d 重连成功(第 %d 次重连)", d.ID, d.reconnectCount)
                            break
                        }

                        d.Log.Warnf("设备 %d 重连失败(重试 %d/3): %v", d.ID, retry+1, reconnErr)
                        
                        select {
                        case <-d.ctx.Done():
                            return
                        case <-time.After(reconnBackoff):
                            reconnBackoff *= 2
                        }
                    }

                    // 重连失败，退出
                    if reconnErr != nil {
                        d.Log.Errorf("设备 %d 重连失败，退出", d.ID)
                        return
                    }
                }
                continue
            }

            // 发送成功
            consecutiveErrors = 0
            atomic.AddInt64(&d.Stats.TotalSent, 1)
            packetsSent.Inc()
        }
    }
}


// Stop 停止设备
func (d *AdvancedDevice) Stop() {
    d.cancel()
    d.connMutex.Lock()
    if d.conn != nil {
        d.conn.Close()
        d.conn = nil
    }
    d.connMutex.Unlock()
}

// generatePacket 生成测试数据包
func (d *AdvancedDevice) generatePacket() ([]byte, uint8, uint8) {
    packet := make([]byte, 12)

    // 协议头
    binary.BigEndian.PutUint16(packet[0:2], 0xAA55)

    // 命令ID（使用设备ID）
    binary.BigEndian.PutUint16(packet[2:4], uint16(d.ID%65535))

    // 随机数据类型 (1=温度, 2=压力, 3=流量)
    dataType := uint8(rand.Intn(3) + 1)
    packet[4] = dataType

    // 随机状态 (0=正常, 1=警告, 2=错误)
    // 90%正常, 8%警告, 2%错误
    r := rand.Intn(100)
    var status uint8
    if r < 90 {
        status = 0
    } else if r < 98 {
        status = 1
    } else {
        status = 2
    }
    packet[5] = status

    // 保留字段
    packet[6] = 0x00
    packet[7] = 0x00

    // 生成随机值
    var value float32
    switch dataType {
    case 1: // 温度 -20~50℃
        value = -20 + rand.Float32()*70
    case 2: // 压力 0~10MPa
        value = rand.Float32() * 10
    case 3: // 流量 0~1000L/min
        value = rand.Float32() * 1000
    }

    intValue := uint32(value * 100)
    binary.BigEndian.PutUint32(packet[8:12], intValue)

    return packet, dataType, status
}

// 辅助函数
func getDataTypeName(t uint8) string {
    switch t {
    case 1:
        return "temperature"
    case 2:
        return "pressure"
    case 3:
        return "flow"
    default:
        return "unknown"
    }
}

func getStatusName(s uint8) string {
    switch s {
    case 0:
        return "normal"
    case 1:
        return "warning"
    case 2:
        return "error"
    default:
        return "unknown"
    }
}

// AdvancedStressTest 高级压力测试管理器
type AdvancedStressTest struct {
    ServerAddr    string
    NumDevices    int
    SendInterval  time.Duration
    Duration      time.Duration
    BatchSize     int
    BatchDelay    time.Duration
    MetricsPort   int
    Stats         *Stats
    Devices       []*AdvancedDevice
    Log           *logrus.Logger
    metricsServer *http.Server
}

func NewAdvancedStressTest(serverAddr string, numDevices int, sendInterval, duration time.Duration, 
    batchSize int, batchDelay time.Duration, metricsPort int) *AdvancedStressTest {
    
    log := logrus.New()
    log.SetLevel(logrus.InfoLevel)
    log.SetFormatter(&logrus.TextFormatter{
        FullTimestamp:   true,
        TimestampFormat: "2006-01-02 15:04:05",
    })

    return &AdvancedStressTest{
        ServerAddr:   serverAddr,
        NumDevices:   numDevices,
        SendInterval: sendInterval,
        Duration:     duration,
        BatchSize:    batchSize,
        BatchDelay:   batchDelay,
        MetricsPort:  metricsPort,
        Stats:        &Stats{},
        Devices:      make([]*AdvancedDevice, 0, numDevices),
        Log:          log,
    }
}

// startMetricsServer 启动Prometheus指标服务器
func (st *AdvancedStressTest) startMetricsServer() {
    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.Handler())
    
    // 健康检查端点
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "OK\n")
        fmt.Fprintf(w, "Active Devices: %d\n", atomic.LoadInt64(&st.Stats.ActiveDevices))
        fmt.Fprintf(w, "Total Sent: %d\n", atomic.LoadInt64(&st.Stats.TotalSent))
    })

    // 统计信息端点
    mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, `{
            "active_devices": %d,
            "total_connected": %d,
            "connect_failed": %d,
            "total_sent": %d,
            "total_failed": %d,
            "total_bytes": %d,
            "reconnects": %d
        }`,
            atomic.LoadInt64(&st.Stats.ActiveDevices),
            atomic.LoadInt64(&st.Stats.TotalConnected),
            atomic.LoadInt64(&st.Stats.ConnectFailed),
            atomic.LoadInt64(&st.Stats.TotalSent),
            atomic.LoadInt64(&st.Stats.TotalFailed),
            atomic.LoadInt64(&st.Stats.TotalBytes),
            atomic.LoadInt64(&st.Stats.Reconnects),
        )
    })

    st.metricsServer = &http.Server{
        Addr:    fmt.Sprintf(":%d", st.MetricsPort),
        Handler: mux,
    }

    go func() {
        st.Log.Infof("Prometheus指标服务器启动在 http://localhost:%d/metrics", st.MetricsPort)
        st.Log.Infof("健康检查端点: http://localhost:%d/health", st.MetricsPort)
        st.Log.Infof("统计信息端点: http://localhost:%d/stats", st.MetricsPort)
        
        if err := st.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            st.Log.Errorf("指标服务器错误: %v", err)
        }
    }()
}

// Run 运行压力测试
func (st *AdvancedStressTest) Run() {
    st.Log.Infof("========================================")
    st.Log.Infof("🚀 高级压力测试开始")
    st.Log.Infof("========================================")
    st.Log.Infof("服务器地址:   %s", st.ServerAddr)
    st.Log.Infof("设备数量:     %d", st.NumDevices)
    st.Log.Infof("发送间隔:     %v", st.SendInterval)
    st.Log.Infof("测试时长:     %v", st.Duration)
    st.Log.Infof("预计 QPS:     %d", st.NumDevices/int(st.SendInterval.Seconds()))
    st.Log.Infof("分批大小:     %d", st.BatchSize)
    st.Log.Infof("分批延迟:     %v", st.BatchDelay)
    st.Log.Infof("指标端口:     %d", st.MetricsPort)
    st.Log.Infof("========================================")

    // 启动指标服务器
    st.startMetricsServer()

    // 设置总设备数
    devicesTotal.Set(float64(st.NumDevices))

    // 启动统计监控
    stopMonitor := make(chan struct{})
    go st.monitorStats(stopMonitor)

    // 信号处理
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // 创建设备
    var wg sync.WaitGroup
    startTime := time.Now()

    // 分批启动设备
    st.Log.Infof("开始启动设备...")
    for i := 0; i < st.NumDevices; i++ {
		device := NewAdvancedDevice(i+1, st.ServerAddr, st.SendInterval, st.Stats, st.Log)
        st.Devices = append(st.Devices, device)

        wg.Add(1)
        go device.Run(&wg)

        // 分批控制
        if (i+1)%st.BatchSize == 0 {
            progress := float64(i+1) / float64(st.NumDevices) * 100
            st.Log.Infof("已启动 %d/%d 设备 (%.1f%%)...", i+1, st.NumDevices, progress)
            time.Sleep(st.BatchDelay)
        }
    }

    st.Log.Infof("✅ 所有设备启动完成，用时: %v", time.Since(startTime))
    st.Log.Infof("等待设备稳定连接...")
    time.Sleep(5 * time.Second)

    st.Log.Infof("========================================")
    st.Log.Infof("测试运行中...")
    st.Log.Infof("按 Ctrl+C 手动停止测试")
    st.Log.Infof("========================================")

    // 等待测试时长或信号
    if st.Duration > 0 {
        select {
        case <-time.After(st.Duration):
            st.Log.Infof("⏰ 测试时长到达，准备停止...")
        case sig := <-sigChan:
            st.Log.Infof("📡 收到信号 %v，准备停止...", sig)
        }
    } else {
        sig := <-sigChan
        st.Log.Infof("📡 收到信号 %v，准备停止...", sig)
    }

    // 停止所有设备
    st.Stop()

    // 等待所有设备停止
    st.Log.Infof("等待所有设备完全停止...")
    wg.Wait()

    // 停止监控
    close(stopMonitor)

    // 停止指标服务器
    if st.metricsServer != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        st.metricsServer.Shutdown(ctx)
    }

    // 打印最终统计
    st.printFinalStats()
}

// Stop 停止所有设备
func (st *AdvancedStressTest) Stop() {
    st.Log.Infof("正在停止所有设备...")
    startTime := time.Now()

    // 使用goroutine批量停止，加快速度
    stopBatchSize := 100
    for i := 0; i < len(st.Devices); i += stopBatchSize {
        end := i + stopBatchSize
        if end > len(st.Devices) {
            end = len(st.Devices)
        }

        for j := i; j < end; j++ {
            go st.Devices[j].Stop()
        }

        if (i + stopBatchSize) < len(st.Devices) {
            st.Log.Infof("已停止 %d/%d 设备...", end, len(st.Devices))
        }
    }

    st.Log.Infof("✅ 所有设备停止信号已发送，用时: %v", time.Since(startTime))
}

// monitorStats 监控统计信息
func (st *AdvancedStressTest) monitorStats(stopChan chan struct{}) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    lastSent := int64(0)
    lastBytes := int64(0)
    lastTime := time.Now()
    reportCount := 0

    for {
        select {
        case <-stopChan:
            return
        case <-ticker.C:
            reportCount++
            now := time.Now()
            duration := now.Sub(lastTime).Seconds()

            currentSent := atomic.LoadInt64(&st.Stats.TotalSent)
            currentBytes := atomic.LoadInt64(&st.Stats.TotalBytes)
            activeDev := atomic.LoadInt64(&st.Stats.ActiveDevices)
            totalConn := atomic.LoadInt64(&st.Stats.TotalConnected)
            connectFailed := atomic.LoadInt64(&st.Stats.ConnectFailed)
            totalFailed := atomic.LoadInt64(&st.Stats.TotalFailed)
            reconnects := atomic.LoadInt64(&st.Stats.Reconnects)

            // 计算速率
            sentDelta := currentSent - lastSent
            bytesDelta := currentBytes - lastBytes
            qps := float64(sentDelta) / duration
            bps := float64(bytesDelta) / duration / 1024 // KB/s

            // 更新Prometheus指标
            qpsGauge.Set(qps)
            bandwidthGauge.Set(float64(bytesDelta) / duration)

            // 计算连接成功率
            totalAttempts := totalConn + connectFailed
            var connSuccessRate float64
            if totalAttempts > 0 {
                connSuccessRate = float64(totalConn) / float64(totalAttempts) * 100
            }

            // 计算发送成功率
            totalSends := currentSent + totalFailed
            var sendSuccessRate float64
            if totalSends > 0 {
                sendSuccessRate = float64(currentSent) / float64(totalSends) * 100
            }

            // 详细日志
            if reportCount%6 == 0 { // 每30秒显示详细信息
                st.Log.Infof("========================================")
                st.Log.Infof("📊 运行时统计 [%s]", now.Format("15:04:05"))
                st.Log.Infof("----------------------------------------")
                st.Log.Infof("活跃设备:     %d / %d (%.1f%%)", activeDev, st.NumDevices, 
                    float64(activeDev)/float64(st.NumDevices)*100)
                st.Log.Infof("连接统计:     成功 %d | 失败 %d | 成功率 %.2f%%", 
                    totalConn, connectFailed, connSuccessRate)
                st.Log.Infof("重连次数:     %d", reconnects)
                st.Log.Infof("----------------------------------------")
                st.Log.Infof("发送统计:     成功 %d | 失败 %d | 成功率 %.2f%%", 
                    currentSent, totalFailed, sendSuccessRate)
                st.Log.Infof("数据传输:     %.2f MB", float64(currentBytes)/1024/1024)
                st.Log.Infof("----------------------------------------")
                st.Log.Infof("实时性能:     QPS: %.0f | 带宽: %.2f KB/s", qps, bps)
                st.Log.Infof("平均性能:     QPS: %.0f | 带宽: %.2f KB/s", 
                    float64(currentSent)/time.Since(lastTime).Seconds(),
                    float64(currentBytes)/time.Since(lastTime).Seconds()/1024)
                st.Log.Infof("========================================")
            } else {
                // 简洁日志
                st.Log.Infof("📈 活跃: %d | 已发: %d | 失败: %d | 重连: %d | QPS: %.0f | 带宽: %.2f KB/s",
                    activeDev, currentSent, totalFailed, reconnects, qps, bps)
            }

            lastSent = currentSent
            lastBytes = currentBytes
            lastTime = now
        }
    }
}

// printFinalStats 打印最终统计
func (st *AdvancedStressTest) printFinalStats() {
    totalConn := atomic.LoadInt64(&st.Stats.TotalConnected)
    connectFailed := atomic.LoadInt64(&st.Stats.ConnectFailed)
    totalSent := atomic.LoadInt64(&st.Stats.TotalSent)
    totalFailed := atomic.LoadInt64(&st.Stats.TotalFailed)
    totalBytes := atomic.LoadInt64(&st.Stats.TotalBytes)
    reconnects := atomic.LoadInt64(&st.Stats.Reconnects)

    st.Log.Infof("")
    st.Log.Infof("========================================")
    st.Log.Infof("🏁 压力测试完成")
    st.Log.Infof("========================================")
    st.Log.Infof("")
    st.Log.Infof("📋 连接统计:")
    st.Log.Infof("  目标设备数:   %d", st.NumDevices)
    st.Log.Infof("  成功连接:     %d", totalConn)
    st.Log.Infof("  连接失败:     %d", connectFailed)
    st.Log.Infof("  重连次数:     %d", reconnects)

    if totalConn+connectFailed > 0 {
        connSuccessRate := float64(totalConn) / float64(totalConn+connectFailed) * 100
        st.Log.Infof("  连接成功率:   %.2f%%", connSuccessRate)
    }

    st.Log.Infof("")
    st.Log.Infof("📦 数据统计:")
    st.Log.Infof("  总发送数:     %d", totalSent)
    st.Log.Infof("  发送失败:     %d", totalFailed)
    st.Log.Infof("  总字节数:     %.2f MB", float64(totalBytes)/1024/1024)

    if totalSent+totalFailed > 0 {
        sendSuccessRate := float64(totalSent) / float64(totalSent+totalFailed) * 100
        st.Log.Infof("  发送成功率:   %.2f%%", sendSuccessRate)
    }

    if totalConn > 0 {
        avgPacketsPerDevice := float64(totalSent) / float64(totalConn)
        avgBytesPerDevice := float64(totalBytes) / float64(totalConn) / 1024 // KB
        st.Log.Infof("  平均每设备:   %.0f 包 / %.2f KB", avgPacketsPerDevice, avgBytesPerDevice)
    }

    st.Log.Infof("")
    st.Log.Infof("⚡ 性能指标:")
    if st.Duration > 0 {
        avgQPS := float64(totalSent) / st.Duration.Seconds()
        avgBandwidth := float64(totalBytes) / st.Duration.Seconds() / 1024 // KB/s
        st.Log.Infof("  平均 QPS:     %.0f", avgQPS)
        st.Log.Infof("  平均带宽:     %.2f KB/s", avgBandwidth)
        st.Log.Infof("  测试时长:     %v", st.Duration)
    }

    st.Log.Infof("")
    st.Log.Infof("========================================")

    // 评分系统
    score := st.calculateScore(totalConn, connectFailed, totalSent, totalFailed, reconnects)
    st.Log.Infof("🎯 综合评分: %s", score)
    st.Log.Infof("========================================")
}

// calculateScore 计算测试评分
func (st *AdvancedStressTest) calculateScore(totalConn, connectFailed, totalSent, totalFailed, reconnects int64) string {
    totalAttempts := totalConn + connectFailed
    totalSends := totalSent + totalFailed

    var connRate, sendRate float64
    if totalAttempts > 0 {
        connRate = float64(totalConn) / float64(totalAttempts)
    }
    if totalSends > 0 {
        sendRate = float64(totalSent) / float64(totalSends)
    }

    // 计算分数 (0-100)
    score := connRate*50 + sendRate*50

    // 重连惩罚
    if reconnects > 0 {
        penalty := float64(reconnects) / float64(totalConn) * 10
        if penalty > 10 {
            penalty = 10
        }
        score -= penalty
    }

    switch {
    case score >= 95:
        return fmt.Sprintf("%.1f/100 ⭐⭐⭐⭐⭐ (优秀)", score)
    case score >= 85:
        return fmt.Sprintf("%.1f/100 ⭐⭐⭐⭐ (良好)", score)
    case score >= 70:
        return fmt.Sprintf("%.1f/100 ⭐⭐⭐ (中等)", score)
    case score >= 50:
        return fmt.Sprintf("%.1f/100 ⭐⭐ (及格)", score)
    default:
        return fmt.Sprintf("%.1f/100 ⭐ (需改进)", score)
    }
}

func main() {
    // 命令行参数
    serverAddr := flag.String("server", "localhost:8888", "服务器地址 (host:port)")
    numDevices := flag.Int("devices", 10000, "设备数量")
    sendInterval := flag.Duration("interval", 1*time.Second, "发送间隔")
    duration := flag.Duration("duration", 60*time.Second, "测试时长 (0表示手动停止)")
    batchSize := flag.Int("batch", 50, "分批启动大小")
    batchDelay := flag.Duration("delay", 100*time.Millisecond, "分批延迟")
    metricsPort := flag.Int("metrics-port", 9090, "Prometheus指标端口")
    debug := flag.Bool("debug", false, "调试模式")
    
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "高级压力测试工具 - 模拟大量设备并发连接\n\n")
        fmt.Fprintf(os.Stderr, "使用方法:\n")
        fmt.Fprintf(os.Stderr, "  %s [选项]\n\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "选项:\n")
        flag.PrintDefaults()
        fmt.Fprintf(os.Stderr, "\n示例:\n")
        fmt.Fprintf(os.Stderr, "  # 10000设备，每秒1条，运行5分钟\n")
        fmt.Fprintf(os.Stderr, "  %s -server localhost:8888 -devices 10000 -interval 1s -duration 5m\n\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  # 1000设备高频测试，每100ms一条\n")
        fmt.Fprintf(os.Stderr, "  %s -server localhost:8888 -devices 1000 -interval 100ms -duration 1m\n\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  # 手动控制停止\n")
        fmt.Fprintf(os.Stderr, "  %s -server localhost:8888 -devices 5000 -interval 1s -duration 0\n\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "Prometheus指标:\n")
        fmt.Fprintf(os.Stderr, "  访问 http://localhost:9090/metrics 查看详细指标\n")
        fmt.Fprintf(os.Stderr, "  访问 http://localhost:9090/health 查看健康状态\n")
        fmt.Fprintf(os.Stderr, "  访问 http://localhost:9090/stats 查看统计信息\n\n")
    }
    
    flag.Parse()

    // 参数验证
    if *numDevices <= 0 {
        fmt.Fprintf(os.Stderr, "错误: 设备数量必须大于0\n")
        os.Exit(1)
    }

    if *sendInterval <= 0 {
        fmt.Fprintf(os.Stderr, "错误: 发送间隔必须大于0\n")
        os.Exit(1)
    }

    if *batchSize <= 0 {
        *batchSize = 50
    }

    // 设置随机种子
    rand.Seed(time.Now().UnixNano())

    // 创建压力测试
    st := NewAdvancedStressTest(*serverAddr, *numDevices, *sendInterval, *duration, 
        *batchSize, *batchDelay, *metricsPort)

    if *debug {
        st.Log.SetLevel(logrus.DebugLevel)
    }

    // 运行测试
    st.Run()
}

