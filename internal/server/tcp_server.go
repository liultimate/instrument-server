package server

import (
    "context"
    "fmt"
    "net"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "github.com/sirupsen/logrus"
    "instrument-server/internal/config"
    "instrument-server/internal/handler"
    "instrument-server/internal/monitor"
    "instrument-server/internal/parser"
    "instrument-server/internal/storage"
)

type TCPServer struct {
    config   *config.Config
    listener net.Listener
    parser   *parser.Parser
    storage  *storage.MessageQueue
    monitor  *monitor.Monitor
    log      *logrus.Logger
    limiter  chan struct{}
    wg       sync.WaitGroup
    shutdown chan struct{}
}

func NewTCPServer(cfg *config.Config, log *logrus.Logger) (*TCPServer, error) {
    // 创建解析器
    parser := parser.NewParser()

    // 创建消息队列
    mq, err := storage.NewMessageQueue(
        cfg.Redis.Addr,
        cfg.Redis.Password,
        cfg.Redis.Channel,
        cfg.Redis.DB,
        cfg.Redis.PoolSize,
        log,
    )
    if err != nil {
        return nil, err
    }

    // 创建监控
    mon := monitor.NewMonitor(log)

    return &TCPServer{
        config:   cfg,
        parser:   parser,
        storage:  mq,
        monitor:  mon,
        log:      log,
        limiter:  make(chan struct{}, cfg.Server.MaxConnections),
        shutdown: make(chan struct{}),
    }, nil
}

func (s *TCPServer) Start() error {
    // 启动监控
    if s.config.Monitor.Enabled {
        s.monitor.StartMetricsServer(s.config.Monitor.MetricsPort)
        s.monitor.StartRuntimeMonitor()
    }

    // 监听TCP端口
    addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
    
    lc := net.ListenConfig{
        KeepAlive: s.config.Server.KeepAlive,
    }

    listener, err := lc.Listen(context.Background(), "tcp", addr)
    if err != nil {
        return fmt.Errorf("监听失败: %w", err)
    }

    s.listener = listener
    s.log.Infof("服务器启动成功: %s (最大连接: %d)", addr, s.config.Server.MaxConnections)

    // 优雅退出处理
    go s.handleShutdown()

    // 接受连接
    for {
        select {
        case <-s.shutdown:
            s.log.Info("停止接受新连接")
            return nil
        default:
        }

        conn, err := listener.Accept()
        if err != nil {
            select {
            case <-s.shutdown:
                return nil
            default:
                s.log.Errorf("接受连接错误: %v", err)
                continue
            }
        }

        // 连接数限制
        select {
        case s.limiter <- struct{}{}:
            s.wg.Add(1)
            go s.handleConnection(conn)
        default:
            s.log.Warn("达到最大连接数，拒绝连接")
            conn.Close()
        }
    }
}

func (s *TCPServer) handleConnection(conn net.Conn) {
    defer func() {
        <-s.limiter
        s.wg.Done()
    }()

    h := handler.NewConnectionHandler(
        conn,
        s.parser,
        s.storage,
        s.log,
        s.config.Server.BufferSize,
        s.config.Server.ReadTimeout,
    )

    h.Handle()
}

func (s *TCPServer) handleShutdown() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    sig := <-sigChan
    s.log.Infof("收到信号: %v, 开始优雅关闭...", sig)

    close(s.shutdown)

    // 停止接受新连接
    if s.listener != nil {
        s.listener.Close()
    }

    // 等待现有连接处理完成（最多30秒）
    done := make(chan struct{})
    go func() {
        s.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        s.log.Info("所有连接已关闭")
    case <-time.After(30 * time.Second):
        s.log.Warn("关闭超时，强制退出")
    }

    // 关闭存储连接
    if err := s.storage.Close(); err != nil {
        s.log.Errorf("关闭存储连接失败: %v", err)
    }

    s.log.Info("服务器已关闭")
    os.Exit(0)
}
