package main

import (
    "encoding/binary"
    "flag"
    "fmt"
    "log"
    "net"
    // "time"
)

func main() {
    host := flag.String("host", "localhost:8888", "服务器地址")
    count := flag.Int("count", 10, "发送数据包数量")
    // interval := flag.Duration("interval", time.Second, "发送间隔")
    flag.Parse()

    conn, err := net.Dial("tcp", *host)
    if err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer conn.Close()

    fmt.Printf("已连接到: %s\n", *host)

    for i := 0; i < *count; i++ {
        // 构造测试数据包
        data := makeTestPacket(uint16(i), 0x01, float32(20.5+float32(i)))
        
        // 发送数据
        n, err := conn.Write(data)
        if err != nil {
            log.Printf("发送失败: %v", err)
            break
        }

        fmt.Printf("[%d] 发送 %d 字节: % x\n", i+1, n, data)
        
        // time.Sleep(*interval)
    }

    fmt.Println("发送完成")
}

// makeTestPacket 构造测试数据包
func makeTestPacket(commandID uint16, dataType uint8, value float32) []byte {
    packet := make([]byte, 12)
    
    // 协议头 (0xAA55)
    binary.BigEndian.PutUint16(packet[0:2], 0xAA55)
    
    // 命令ID
    binary.BigEndian.PutUint16(packet[2:4], commandID)
    
    // 数据类型
    packet[4] = dataType
    
    // 状态
    packet[5] = 0x00
    
    // 保留字节
    packet[6] = 0x00
    packet[7] = 0x00
    
    // 值 (转换为整数 * 100)
    intValue := uint32(value * 100)
    binary.BigEndian.PutUint32(packet[8:12], intValue)
    
    return packet
}
