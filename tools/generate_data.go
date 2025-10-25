package main

import (
    "encoding/binary"
    "encoding/hex"
    "flag"
    "fmt"
    "math/rand"
    "time"
)

func main() {
    commandID := flag.Uint("cmd", 1, "命令ID")
    dataType := flag.Uint("type", 1, "数据类型 (1=温度, 2=压力, 3=流量)")
    status := flag.Uint("status", 0, "状态 (0=正常, 1=警告, 2=错误)")
    value := flag.Float64("value", 25.36, "测量值")
    random := flag.Bool("random", false, "生成随机数据")
    count := flag.Int("count", 1, "生成数量")
    flag.Parse()

    rand.Seed(time.Now().UnixNano())

    for i := 0; i < *count; i++ {
        var packet []byte
        
        if *random {
            packet = generateRandomPacket()
        } else {
            packet = generatePacket(
                uint16(*commandID),
                uint8(*dataType),
                uint8(*status),
                float32(*value),
            )
        }

        fmt.Printf("数据包 %d:\n", i+1)
        fmt.Printf("  十六进制: %s\n", hex.EncodeToString(packet))
        fmt.Printf("  字节数组: % x\n", packet)
        fmt.Printf("  C格式:    {%s}\n", toCArray(packet))
        fmt.Printf("  Go格式:   []byte{%s}\n", toGoArray(packet))
        parseAndDisplay(packet)
        fmt.Println()
    }
}

// generatePacket 生成协议数据包
func generatePacket(commandID uint16, dataType, status uint8, value float32) []byte {
    packet := make([]byte, 12)
    
    // 协议头
    binary.BigEndian.PutUint16(packet[0:2], 0xAA55)
    
    // 命令ID
    binary.BigEndian.PutUint16(packet[2:4], commandID)
    
    // 数据类型
    packet[4] = dataType
    
    // 状态
    packet[5] = status
    
    // 保留字段
    packet[6] = 0x00
    packet[7] = 0x00
    
    // 值（转换为整数）
    intValue := uint32(value * 100)
    binary.BigEndian.PutUint32(packet[8:12], intValue)
    
    return packet
}

// generateRandomPacket 生成随机数据包
func generateRandomPacket() []byte {
    commandID := uint16(rand.Intn(1000))
    dataType := uint8(rand.Intn(3) + 1) // 1-3
    status := uint8(rand.Intn(3))       // 0-2
    
    var value float32
    switch dataType {
    case 1: // 温度 -20~50℃
        value = -20 + rand.Float32()*70
    case 2: // 压力 0~10MPa
        value = rand.Float32() * 10
    case 3: // 流量 0~1000L/min
        value = rand.Float32() * 1000
    }
    
    return generatePacket(commandID, dataType, status, value)
}

// parseAndDisplay 解析并显示数据包内容
func parseAndDisplay(packet []byte) {
    if len(packet) != 12 {
        fmt.Println("  错误: 数据包长度不正确")
        return
    }
    
    header := binary.BigEndian.Uint16(packet[0:2])
    commandID := binary.BigEndian.Uint16(packet[2:4])
    dataType := packet[4]
    status := packet[5]
    value := float32(binary.BigEndian.Uint32(packet[8:12])) / 100.0
    
    fmt.Printf("  解析结果:\n")
    fmt.Printf("    协议头:   0x%04X %s\n", header, checkHeader(header))
    fmt.Printf("    命令ID:   %d\n", commandID)
    fmt.Printf("    数据类型: %d (%s)\n", dataType, getDataTypeName(dataType))
    fmt.Printf("    状态:     %d (%s)\n", status, getStatusName(status))
    fmt.Printf("    测量值:   %.2f %s\n", value, getUnit(dataType))
}

func checkHeader(header uint16) string {
    if header == 0xAA55 {
        return "✓"
    }
    return "✗ 错误"
}

func getDataTypeName(t uint8) string {
    switch t {
    case 1:
        return "温度"
    case 2:
        return "压力"
    case 3:
        return "流量"
    default:
        return "未知"
    }
}

func getStatusName(s uint8) string {
    switch s {
    case 0:
        return "正常"
    case 1:
        return "警告"
    case 2:
        return "错误"
    default:
        return "未知"
    }
}

func getUnit(t uint8) string {
    switch t {
    case 1:
        return "℃"
    case 2:
        return "MPa"
    case 3:
        return "L/min"
    default:
        return ""
    }
}

func toCArray(data []byte) string {
    result := ""
    for i, b := range data {
        if i > 0 {
            result += ", "
        }
        result += fmt.Sprintf("0x%02X", b)
    }
    return result
}

func toGoArray(data []byte) string {
    result := ""
    for i, b := range data {
        if i > 0 {
            result += ", "
        }
        result += fmt.Sprintf("0x%02X", b)
    }
    return result
}
