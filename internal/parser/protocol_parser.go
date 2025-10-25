package parser

import (
    "encoding/binary"
    "fmt"
    "time"

    "instrument-server/pkg/protocol"
)

type Parser struct{}

func NewParser() *Parser {
    return &Parser{}
}

// Parse 解析二进制数据
func (p *Parser) Parse(deviceID string, data []byte) *protocol.ParseResult {
    result := &protocol.ParseResult{
        Success: false,
    }

    // 检查数据长度
    if len(data) < protocol.ProtocolHeaderSize {
        result.Error = fmt.Errorf("数据长度不足: %d bytes", len(data))
        return result
    }

    // 解析协议头
    magic := binary.BigEndian.Uint16(data[0:2])
    if magic != protocol.ProtocolMagic {
        result.Error = fmt.Errorf("协议头错误: 0x%04X", magic)
        return result
    }

    // 解析字段
    commandID := binary.BigEndian.Uint16(data[2:4])
    dataType := data[4]
    status := data[5]
    
    // 解析数值（假设后续4字节是float32）
    var value float64
    if len(data) >= 12 {
        bits := binary.BigEndian.Uint32(data[8:12])
        value = float64(bits) / 100.0 // 示例：除以100作为实际值
    }

    // 构建仪器数据
    instrumentData := &protocol.InstrumentData{
        DeviceID:  deviceID,
        Timestamp: time.Now(),
        CommandID: commandID,
        DataType:  dataType,
        Value:     value,
        RawData:   data,
        Status:    status,
    }

    result.Success = true
    result.Data = instrumentData
    return result
}

// ParseBatch 批量解析数据
func (p *Parser) ParseBatch(deviceID string, data []byte) []*protocol.ParseResult {
    var results []*protocol.ParseResult
    
    // 假设每个数据包12字节
    packetSize := 12
    for i := 0; i < len(data); i += packetSize {
        end := i + packetSize
        if end > len(data) {
            end = len(data)
        }
        
        result := p.Parse(deviceID, data[i:end])
        results = append(results, result)
    }
    
    return results
}

// ValidateChecksum 验证校验和（示例实现）
func (p *Parser) ValidateChecksum(data []byte) bool {
    if len(data) < 2 {
        return false
    }
    
    // 简单的累加校验
    var sum uint8
    for i := 0; i < len(data)-1; i++ {
        sum += data[i]
    }
    
    return sum == data[len(data)-1]
}
