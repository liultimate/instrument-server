package main

import (
    "context"
    "encoding/binary"
    "flag"
    "fmt"
    "math/rand"
    "net"
    "os"
    "os/signal"
    "sync"
    "sync/atomic"
    "syscall"
    "time"

    "github.com/sirupsen/logrus"
)

// 统计指标
type Stats struct {
    TotalSent       int64
    TotalFailed     int64
    TotalConnected  int64
    ConnectFailed   int64
    ActiveDevices   int64
    TotalBytes      int64
    Reconnects      int64
}

// 优化的设备模拟器 - 支持连接复用和自动重连
type OptimizedDevice struct {
    ID           int
    ServerAddr   string
    SendInterval time.Duration
    Stats        *Stats
    Log          *logrus.Logger
    ctx          context.Context
    cancel       context.CancelFunc
    conn         net.Conn
    connMutex    sync.Mutex
}

func NewOptimizedDevice(id int, serverAddr string, interval time.Duration, stats *Stats, log *logrus.Logger) *OptimizedDevice {
    ctx, cancel := context.WithCancel(context.Background())
    return &OptimizedDevice{
        ID:           id,
        ServerAddr:   serverAddr,
        SendInterval: interval,
        Stats:        stats,
        Log:          log,
        ctx:          ctx,
        cancel:       cancel,
    }
}

// connect 建立连接
func (d *OptimizedDevice) connect() error {
    d.connMutex.Lock()
    defer d.connMutex.Unlock()

    if d.conn != nil {
        d.conn.Close()
        d.conn = nil
    }

    conn, err := net.DialTimeout("tcp", d.ServerAddr, 10*time.Second)
    if err != nil {
        return err
    }

    d.conn = conn
    return nil
}

// send 发送数据
func (d *OptimizedDevice) send(packet []byte) error {
    d.connMutex.Lock()
    conn := d.conn
    d.connMutex.Unlock()

    if conn == nil {
        return fmt.Errorf("未连接")
    }

    conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
    n, err := conn.Write(packet)
    if err != nil {
        return err
    }

    atomic.AddInt64(&d.Stats.TotalBytes, int64(n))
    return nil
}

// Run 运行设备模拟器
func (d *OptimizedDevice) Run(wg *sync.WaitGroup) {
    defer wg.Done()
    defer d.Stop()

    // 初始连接（带重试）
    var err error
    for retry := 0; retry < 3; retry++ {
        err = d.connect()
        if err == nil {
            break
        }
        d.Log.Warnf("设备 %d 连接失败(重试 %d/3): %v", d.ID, retry+1, err)
        time.Sleep(time.Duration(retry+1) * time.Second)
    }

    if err != nil {
        d.Log.Errorf("设备 %d 连接失败: %v", d.ID, err)
        atomic.AddInt64(&d.Stats.ConnectFailed, 1)
        return
    }

    atomic.AddInt64(&d.Stats.TotalConnected, 1)
    atomic.AddInt64(&d.Stats.ActiveDevices, 1)
    defer atomic.AddInt64(&d.Stats.ActiveDevices, -1)

    d.Log.Debugf("设备 %d 已连接", d.ID)

    ticker := time.NewTicker(d.SendInterval)
    defer ticker.Stop()

    consecutiveErrors := 0
    maxErrors := 3

    for {
        select {
        case <-d.ctx.Done():
            return

        case <-ticker.C:
            packet := d.generatePacket()
            err := d.send(packet)

            if err != nil {
                consecutiveErrors++
                atomic.AddInt64(&d.Stats.TotalFailed, 1)
                d.Log.Warnf("设备 %d 发送失败: %v", d.ID, err)

                // 尝试重连
                if consecutiveErrors >= maxErrors {
                    d.Log.Warnf("设备 %d 尝试重连...", d.ID)
                    if reconnErr := d.connect(); reconnErr != nil {
                        d.Log.Errorf("设备 %d 重连失败: %v", d.ID, reconnErr)
                        return
                    }
                    atomic.AddInt64(&d.Stats.Reconnects, 1)
                    consecutiveErrors = 0
                    d.Log.Infof("设备 %d 重连成功", d.ID)
                }
                continue
            }

            consecutiveErrors = 0
            atomic.AddInt64(&d.Stats.TotalSent, 1)
        }
    }
}

// Stop 停止设备
func (d *OptimizedDevice) Stop() {
    d.cancel()
    d.connMutex.Lock()
    if d.conn != nil {
        d.conn.Close()
        d.conn = nil
    }
    d.connMutex.Unlock()
}

// generatePacket 生成测试数据包
func (d *OptimizedDevice) generatePacket() []byte {
    packet := make([]byte, 12)

    binary.BigEndian.PutUint16(packet[0:2], 0xAA55)
    binary.BigEndian.PutUint16(packet[2:4], uint16(d.ID%65535))

    dataType := uint8(rand.Intn(3) + 1)
    packet[4] = dataType

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

    packet[6] = 0x00
    packet[7] = 0x00

    var value float32
    switch dataType {
    case 1:
        value = -20 + rand.Float32()*70
    case 2:
        value = rand.Float32() * 10
    case 3:
        value = rand.Float32() * 1000
    }

    intValue := uint32(value * 100)
    binary.BigEndian.PutUint32(packet[8:12], intValue)

    return packet
}

// StressTest 压力测试管理器
type StressTest struct {
    ServerAddr   string
    NumDevices   int
    SendInterval time.Duration
    Duration     time.Duration
    BatchSize    int // 分批启动大小
    BatchDelay   time.Duration
    Stats        *Stats
    Devices      []*OptimizedDevice
    Log          *logrus.Logger
}

func NewStressTest(serverAddr string, numDevices int, sendInterval, duration time.Duration, batchSize int, batchDelay time.Duration) *StressTest {
    log := logrus.New()
    log.SetLevel(logrus.InfoLevel)
    log.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
    })

    return &StressTest{
        ServerAddr:   serverAddr,
        NumDevices:   numDevices,
        SendInterval: sendInterval,
        Duration:     duration,
        BatchSize:    batchSize,
        BatchDelay:   batchDelay,
        Stats:        &Stats{},
        Devices:      make([]*OptimizedDevice, 0, numDevices),
        Log:          log,
    }
}

// Run 运行压力测试
func (st *StressTest) Run() {
    st.Log.Infof("========================================")
    st.Log.Infof("压力测试开始")
    st.Log.Infof("========================================")
    st.Log.Infof("服务器地址: %s", st.ServerAddr)
    st.Log.Infof("设备数量:   %d", st.NumDevices)
    st.Log.Infof("发送间隔:   %v", st.SendInterval)
    st.Log.Infof("测试时长:   %v", st.Duration)
    st.Log.Infof("预计 QPS:   %d", st.NumDevices/int(st.SendInterval.Seconds()))
    st.Log.Infof("分批大小:   %d", st.BatchSize)
    st.Log.Infof("分批延迟:   %v", st.BatchDelay)
    st.Log.Infof("========================================")

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
    for i := 0; i < st.NumDevices; i++ {
        device := NewOptimizedDevice(i+1, st.ServerAddr, st.SendInterval, st.Stats, st.Log)
        st.Devices = append(st.Devices, device)

        wg.Add(1)
        go device.Run(&wg)

        // 分批控制
        if (i+1)%st.BatchSize == 0 {
            st.Log.Infof("已启动 %d/%d 设备 (%.1f%%)...", 
                i+1, st.NumDevices, float64(i+1)/float64(st.NumDevices)*100)
            time.Sleep(st.BatchDelay)
        }
    }

    st.Log.Infof("所有设备启动完成，用时: %v", time.Since(startTime))
    st.Log.Infof("等待设备稳定...")
    time.Sleep(5 * time.Second)

    // 等待测试时长或信号
    if st.Duration > 0 {
        select {
        case <-time.After(st.Duration):
            st.Log.Infof("测试时长到达，准备停止...")
        case sig := <-sigChan:
            st.Log.Infof("收到信号 %v，准备停止...", sig)
        }
    } else {
        sig := <-sigChan
        st.Log.Infof("收到信号 %v，准备停止...", sig)
    }

    // 停止所有设备
    st.Stop()

    // 等待所有设备停止
    wg.Wait()

    // 停止监控
    close(stopMonitor)

    // 打印最终统计
    st.printFinalStats()
}

// Stop 停止所有设备
func (st *StressTest) Stop() {
    st.Log.Infof("正在停止所有设备...")
    for i, device := range st.Devices {
        device.Stop()
        if (i+1)%1000 == 0 {
            st.Log.Infof("已停止 %d/%d 设备...", i+1, len(st.Devices))
        }
    }
}

// monitorStats 监控统计信息
func (st *StressTest) monitorStats(stopChan chan struct{}) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    lastSent := int64(0)
    lastBytes := int64(0)
    lastTime := time.Now()

    for {
        select {
        case <-stopChan:
            return
        case <-ticker.C:
            now := time.Now()
            duration := now.Sub(lastTime).Seconds()

            currentSent := atomic.LoadInt64(&st.Stats.TotalSent)
            currentBytes := atomic.LoadInt64(&st.Stats.TotalBytes)
            activeDev := atomic.LoadInt64(&st.Stats.ActiveDevices)
            totalConn := atomic.LoadInt64(&st.Stats.TotalConnected)
            connectFailed := atomic.LoadInt64(&st.Stats.ConnectFailed)
            totalFailed := atomic.LoadInt64(&st.Stats.TotalFailed)
            reconnects := atomic.LoadInt64(&st.Stats.Reconnects)

            qps := float64(currentSent-lastSent) / duration
            bps := float64(currentBytes-lastBytes) / duration / 1024

            st.Log.Infof("活跃: %d | 总连接: %d | 连接失败: %d | 重连: %d | 已发送: %d | 发送失败: %d | QPS: %.0f | 带宽: %.2f KB/s",
                activeDev, totalConn, connectFailed, reconnects, currentSent, totalFailed, qps, bps)

            lastSent = currentSent
            lastBytes = currentBytes
            lastTime = now
        }
    }
}

// printFinalStats 打印最终统计
func (st *StressTest) printFinalStats() {
    st.Log.Infof("========================================")
    st.Log.Infof("压力测试完成")
    st.Log.Infof("========================================")

    totalConn := atomic.LoadInt64(&st.Stats.TotalConnected)
    connectFailed := atomic.LoadInt64(&st.Stats.ConnectFailed)
    totalSent := atomic.LoadInt64(&st.Stats.TotalSent)
    totalFailed := atomic.LoadInt64(&st.Stats.TotalFailed)
    totalBytes := atomic.LoadInt64(&st.Stats.TotalBytes)
    reconnects := atomic.LoadInt64(&st.Stats.Reconnects)

    st.Log.Infof("目标设备数: %d", st.NumDevices)
    st.Log.Infof("成功连接:   %d", totalConn)
    st.Log.Infof("连接失败:   %d", connectFailed)
    st.Log.Infof("重连次数:   %d", reconnects)
    st.Log.Infof("总发送数:   %d", totalSent)
    st.Log.Infof("发送失败:   %d", totalFailed)
    st.Log.Infof("总字节数:   %.2f MB", float64(totalBytes)/1024/1024)

    if totalConn > 0 {
        connSuccessRate := float64(totalConn) / float64(totalConn+connectFailed) * 100
        st.Log.Infof("连接成功率: %.2f%%", connSuccessRate)
    }

    if totalSent+totalFailed > 0 {
        sendSuccessRate := float64(totalSent) / float64(totalSent+totalFailed) * 100
        st.Log.Infof("发送成功率: %.2f%%", sendSuccessRate)
    }

    st.Log.Infof("========================================")
}

func main() {
    serverAddr := flag.String("server", "localhost:8888", "服务器地址")
    numDevices := flag.Int("devices", 10000, "设备数量")
    sendInterval := flag.Duration("interval", 1*time.Second, "发送间隔")
    duration := flag.Duration("duration", 60*time.Second, "测试时长(0表示手动停止)")
    batchSize := flag.Int("batch", 50, "分批启动大小")
    batchDelay := flag.Duration("delay", 100*time.Millisecond, "分批延迟")
    debug := flag.Bool("debug", false, "调试模式")
    flag.Parse()

    rand.Seed(time.Now().UnixNano())

    st := NewStressTest(*serverAddr, *numDevices, *sendInterval, *duration, *batchSize, *batchDelay)

    if *debug {
        st.Log.SetLevel(logrus.DebugLevel)
    }

    st.Run()
}
