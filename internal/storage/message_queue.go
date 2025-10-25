package storage

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/redis/go-redis/v9"
    "github.com/sirupsen/logrus"
    "instrument-server/pkg/protocol"
)

type MessageQueue struct {
    client  *redis.Client
    channel string
    log     *logrus.Logger
}

func NewMessageQueue(addr, password, channel string, db int, poolSize int, log *logrus.Logger) (*MessageQueue, error) {
    client := redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,
        PoolSize: poolSize,
    })

    // 测试连接
    ctx := context.Background()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("连接Redis失败: %w", err)
    }

    log.Info("Redis连接成功")

    return &MessageQueue{
        client:  client,
        channel: channel,
        log:     log,
    }, nil
}

// Publish 发布消息到Redis
func (mq *MessageQueue) Publish(ctx context.Context, data *protocol.InstrumentData) error {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("序列化数据失败: %w", err)
    }

    // 发布到Redis Pub/Sub
    if err := mq.client.Publish(ctx, mq.channel, jsonData).Err(); err != nil {
        return fmt.Errorf("发布消息失败: %w", err)
    }

    // 同时保存到Redis List（作为持久化备份）
    listKey := fmt.Sprintf("instrument:%s:data", data.DeviceID)
    if err := mq.client.LPush(ctx, listKey, jsonData).Err(); err != nil {
        mq.log.Warnf("保存到List失败: %v", err)
    }

    // 限制List长度（保留最近1000条）
    mq.client.LTrim(ctx, listKey, 0, 999)

    return nil
}

// PublishBatch 批量发布
func (mq *MessageQueue) PublishBatch(ctx context.Context, dataList []*protocol.InstrumentData) error {
    pipe := mq.client.Pipeline()

    for _, data := range dataList {
        jsonData, err := json.Marshal(data)
        if err != nil {
            mq.log.Errorf("序列化数据失败: %v", err)
            continue
        }

        pipe.Publish(ctx, mq.channel, jsonData)
    }

    _, err := pipe.Exec(ctx)
    return err
}

// Close 关闭连接
func (mq *MessageQueue) Close() error {
    return mq.client.Close()
}

// GetStats 获取统计信息
func (mq *MessageQueue) GetStats(ctx context.Context) map[string]interface{} {
    info := mq.client.Info(ctx, "stats").Val()
    
    return map[string]interface{}{
        "info":       info,
        "pool_stats": mq.client.PoolStats(),
    }
}
