package monitor

import (
    "fmt"  // 添加这个导入
    "net/http"
    "runtime"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/sirupsen/logrus"
)

var (
    // 连接指标
    ActiveConnections = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "instrument_active_connections",
        Help: "当前活跃连接数",
    })

    TotalConnections = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "instrument_total_connections",
        Help: "总连接数",
    })

    // 数据指标
    DataReceived = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "instrument_data_received_total",
            Help: "接收的数据包总数",
        },
        []string{"device_id", "data_type"},
    )

    BytesReceived = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "instrument_bytes_received_total",
        Help: "接收的字节总数",
    })

    // 处理指标
    DataProcessed = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "instrument_data_processed_total",
        Help: "处理成功的数据包数",
    })

    DataErrors = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "instrument_data_errors_total",
        Help: "数据处理错误数",
    })

    // 延迟指标
    ProcessingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "instrument_processing_duration_seconds",
        Help:    "数据处理耗时",
        Buckets: prometheus.DefBuckets,
    })

    // Goroutine指标
    GoroutineCount = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "instrument_goroutines",
        Help: "当前Goroutine数量",
    })

    // 内存指标
    MemoryUsage = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "instrument_memory_usage_bytes",
        Help: "内存使用量",
    })
)

type Monitor struct {
    log *logrus.Logger
}

func NewMonitor(log *logrus.Logger) *Monitor {
    // 注册指标
    prometheus.MustRegister(
        ActiveConnections,
        TotalConnections,
        DataReceived,
        BytesReceived,
        DataProcessed,
        DataErrors,
        ProcessingDuration,
        GoroutineCount,
        MemoryUsage,
    )

    return &Monitor{log: log}
}

// StartMetricsServer 启动Metrics HTTP服务器
func (m *Monitor) StartMetricsServer(port int) {
    http.Handle("/metrics", promhttp.Handler())
    
    // 健康检查端点
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    addr := fmt.Sprintf(":%d", port)
    m.log.Infof("Metrics服务器启动: %s", addr)
    
    go func() {
        if err := http.ListenAndServe(addr, nil); err != nil {
            m.log.Errorf("Metrics服务器错误: %v", err)
        }
    }()
}

// StartRuntimeMonitor 启动运行时监控
func (m *Monitor) StartRuntimeMonitor() {
    ticker := time.NewTicker(10 * time.Second)
    
    go func() {
        for range ticker.C {
            // 更新Goroutine数量
            GoroutineCount.Set(float64(runtime.NumGoroutine()))

            // 更新内存使用
            var memStats runtime.MemStats
            runtime.ReadMemStats(&memStats)
            MemoryUsage.Set(float64(memStats.Alloc))

            m.log.Debugf("Goroutines: %d, 内存: %.2f MB",
                runtime.NumGoroutine(),
                float64(memStats.Alloc)/1024/1024,
            )
        }
    }()
}
