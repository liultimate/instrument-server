package handler

import (
    "context"
    "fmt"
    "net"
    "time"
    "github.com/sirupsen/logrus"
    "instrument-server/internal/monitor"
    "instrument-server/internal/parser"
    "instrument-server/internal/storage"
)

type ConnectionHandler struct {
    conn        net.Conn
    deviceID    string
    parser      *parser.Parser
    storage     *storage.MessageQueue
    log         *logrus.Logger
    bufferSize  int
    readTimeout time.Duration
}

func NewConnectionHandler(
    conn net.Conn,
    parser *parser.Parser,
    storage *storage.MessageQueue,
    log *logrus.Logger,
    bufferSize int,
    readTimeout time.Duration,
) *ConnectionHandler {
    deviceID := conn.RemoteAddr().String()

    return &ConnectionHandler{
        conn:        conn,
        deviceID:    deviceID,
        parser:      parser,
        storage:     storage,
        log:         log,
        bufferSize:  bufferSize,
        readTimeout: readTimeout,
    }
}

// Handle 处理连接
func (h *ConnectionHandler) Handle() {
    defer func() {
        h.conn.Close()
        monitor.ActiveConnections.Dec()
        h.log.Infof("连接关闭: %s", h.deviceID)
    }()

    monitor.ActiveConnections.Inc()
    monitor.TotalConnections.Inc()
    h.log.Infof("新连接: %s", h.deviceID)

    buffer := make([]byte, h.bufferSize)
    ctx := context.Background()

    for {
        // 设置读取超时
        h.conn.SetReadDeadline(time.Now().Add(h.readTimeout))

        n, err := h.conn.Read(buffer)
        if err != nil {
            if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                h.log.Debugf("读取超时: %s", h.deviceID)
                continue
            }
            h.log.Debugf("连接断开: %s, 错误: %v", h.deviceID, err)
            return
        }

        if n == 0 {
            continue
        }

        // 记录接收字节数
        monitor.BytesReceived.Add(float64(n))

        // 处理数据
        h.processData(ctx, buffer[:n])
    }
}

// processData 处理接收到的数据
func (h *ConnectionHandler) processData(ctx context.Context, data []byte) {
    startTime := time.Now()

    // 解析数据
    result := h.parser.Parse(h.deviceID, data)

    if !result.Success {
        monitor.DataErrors.Inc()
        h.log.Warnf("解析失败 [%s]: %v, 数据: % x", h.deviceID, result.Error, data)
        return
    }

    // 记录数据类型
    dataType := fmt.Sprintf("%d", result.Data.DataType)
    monitor.DataReceived.WithLabelValues(h.deviceID, dataType).Inc()

    // 发送到消息队列
    if err := h.storage.Publish(ctx, result.Data); err != nil {
        monitor.DataErrors.Inc()
        h.log.Errorf("发布消息失败 [%s]: %v", h.deviceID, err)
        return
    }

    monitor.DataProcessed.Inc()

    // 记录处理时间
    duration := time.Since(startTime).Seconds()
    monitor.ProcessingDuration.Observe(duration)

    // 日志输出 - 修复了这里的换行问题
    h.log.Debugf("数据处理成功 [%s]: 命令ID=%d, 数据类型=%d, 值=%.2f, 耗时=%.3fms",
        h.deviceID, 
        result.Data.CommandID, 
        result.Data.DataType, 
        result.Data.Value,
        duration*1000,
    )
}

// SendResponse 发送响应（可选功能）
func (h *ConnectionHandler) SendResponse(data []byte) error {
    h.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
    
    n, err := h.conn.Write(data)
    if err != nil {
        return fmt.Errorf("发送响应失败: %w", err)
    }

    h.log.Debugf("发送响应 [%s]: %d 字节", h.deviceID, n)
    return nil
}
