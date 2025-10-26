package main

import (
	"encoding/binary"
	"flag"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// 统计指标
type Stats struct {
	TotalSent      int64 // 总发送数
	TotalFailed    int64 // 总失败数
	TotalConnected int64 // 总连接数
	ActiveDevices  int64 // 活跃设备数
	TotalBytes     int64 // 总字节数
}

// 设备模拟器
type Device struct {
	ID           int
	ServerAddr   string
	SendInterval time.Duration
	Stats        *AStats
	Log          *logrus.Logger
	StopChan     chan struct{}
}

func NewDevice(id int, serverAddr string, interval time.Duration, stats *AStats, log *logrus.Logger) *Device {
	return &Device{
		ID:           id,
		ServerAddr:   serverAddr,
		SendInterval: interval,
		Stats:        stats,
		Log:          log,
		StopChan:     make(chan struct{}),
	}
}

// Run 运行设备模拟器
func (d *Device) Run(wg *sync.WaitGroup) {
	defer wg.Done()

	// 连接服务器
	conn, err := net.DialTimeout("tcp", d.ServerAddr, 5*time.Second)
	if err != nil {
		d.Log.Errorf("设备 %d 连接失败: %v", d.ID, err)
		atomic.AddInt64(&d.Stats.TotalFailed, 1)
		return
	}
	defer conn.Close()

	atomic.AddInt64(&d.Stats.TotalConnected, 1)
	atomic.AddInt64(&d.Stats.ActiveDevices, 1)
	defer atomic.AddInt64(&d.Stats.ActiveDevices, -1)

	d.Log.Debugf("设备 %d 已连接", d.ID)

	ticker := time.NewTicker(d.SendInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.StopChan:
			d.Log.Debugf("设备 %d 停止", d.ID)
			return

		case <-ticker.C:
			// 生成并发送数据
			packet := d.generatePacket()

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			n, err := conn.Write(packet)
			if err != nil {
				d.Log.Errorf("设备 %d 发送失败: %v", d.ID, err)
				atomic.AddInt64(&d.Stats.TotalFailed, 1)
				return
			}

			atomic.AddInt64(&d.Stats.TotalSent, 1)
			atomic.AddInt64(&d.Stats.TotalBytes, int64(n))
		}
	}
}

// Stop 停止设备
func (d *Device) Stop() {
	close(d.StopChan)
}

// generatePacket 生成测试数据包
func (d *Device) generatePacket() []byte {
	packet := make([]byte, 12)

	// 协议头
	binary.BigEndian.PutUint16(packet[0:2], 0xAA55)

	// 命令ID（使用设备ID）
	binary.BigEndian.PutUint16(packet[2:4], uint16(d.ID))

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

	return packet
}

// StressTest 压力测试管理器
type StressTest struct {
	ServerAddr   string
	NumDevices   int
	SendInterval time.Duration
	Duration     time.Duration
	Stats        *AStats
	Devices      []*Device
	Log          *logrus.Logger
}

func NewStressTest(serverAddr string, numDevices int, sendInterval, duration time.Duration) *StressTest {
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
		Stats:        &AStats{},
		Devices:      make([]*Device, 0),
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
	st.Log.Infof("========================================")

	// 启动统计监控
	go st.monitorStats()

	// 创建设备
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < st.NumDevices; i++ {
		device := NewDevice(i+1, st.ServerAddr, st.SendInterval, st.Stats, st.Log)
		st.Devices = append(st.Devices, device)

		wg.Add(1)
		go device.Run(&wg)

		// 分批启动，避免瞬间连接过多
		if (i+1)%100 == 0 {
			time.Sleep(10 * time.Millisecond)
			st.Log.Infof("已启动 %d/%d 设备...", i+1, st.NumDevices)
		}
	}

	st.Log.Infof("所有设备启动完成，用时: %v", time.Since(startTime))

	// 等待测试时长
	if st.Duration > 0 {
		time.Sleep(st.Duration)
		st.Log.Infof("测试时长到达，准备停止...")
		st.Stop()
	}

	// 等待所有设备停止
	wg.Wait()

	// 打印最终统计
	st.printFinalStats()
}

// Stop 停止所有设备
func (st *StressTest) Stop() {
	st.Log.Infof("正在停止所有设备...")
	for _, device := range st.Devices {
		device.Stop()
	}
}

// monitorStats 监控统计信息
func (st *StressTest) monitorStats() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastSent := int64(0)
	lastBytes := int64(0)
	lastTime := time.Now()

	for range ticker.C {
		now := time.Now()
		duration := now.Sub(lastTime).Seconds()

		currentSent := atomic.LoadInt64(&st.Stats.TotalSent)
		currentBytes := atomic.LoadInt64(&st.Stats.TotalBytes)
		activeDev := atomic.LoadInt64(&st.Stats.ActiveDevices)
		totalConn := atomic.LoadInt64(&st.Stats.TotalConnected)
		totalFailed := atomic.LoadInt64(&st.Stats.TotalFailed)

		// 计算速率
		qps := float64(currentSent-lastSent) / duration
		bps := float64(currentBytes-lastBytes) / duration / 1024 // KB/s

		st.Log.Infof("活跃设备: %d | 总连接: %d | 失败: %d | 已发送: %d | QPS: %.0f | 带宽: %.2f KB/s",
			activeDev, totalConn, totalFailed, currentSent, qps, bps)

		lastSent = currentSent
		lastBytes = currentBytes
		lastTime = now
	}
}

// printFinalStats 打印最终统计
func (st *StressTest) printFinalStats() {
	st.Log.Infof("========================================")
	st.Log.Infof("压力测试完成")
	st.Log.Infof("========================================")
	st.Log.Infof("总连接数:   %d", atomic.LoadInt64(&st.Stats.TotalConnected))
	st.Log.Infof("总发送数:   %d", atomic.LoadInt64(&st.Stats.TotalSent))
	st.Log.Infof("总失败数:   %d", atomic.LoadInt64(&st.Stats.TotalFailed))
	st.Log.Infof("总字节数:   %.2f MB", float64(atomic.LoadInt64(&st.Stats.TotalBytes))/1024/1024)

	successRate := float64(atomic.LoadInt64(&st.Stats.TotalSent)) /
		float64(atomic.LoadInt64(&st.Stats.TotalSent)+atomic.LoadInt64(&st.Stats.TotalFailed)) * 100
	st.Log.Infof("成功率:     %.2f%%", successRate)
	st.Log.Infof("========================================")
}

func main() {
	// 命令行参数
	serverAddr := flag.String("server", "localhost:8888", "服务器地址")
	numDevices := flag.Int("devices", 10000, "设备数量")
	sendInterval := flag.Duration("interval", 1*time.Second, "发送间隔")
	duration := flag.Duration("duration", 60*time.Second, "测试时长(0表示无限)")
	debug := flag.Bool("debug", false, "调试模式")
	flag.Parse()

	// 设置随机种子
	rand.Seed(time.Now().UnixNano())

	// 创建压力测试
	st := NewStressTest(*serverAddr, *numDevices, *sendInterval, *duration)

	if *debug {
		st.Log.SetLevel(logrus.DebugLevel)
	}

	// 运行测试
	st.Run()
}
