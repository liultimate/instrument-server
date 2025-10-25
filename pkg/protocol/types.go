package protocol

import "time"

// InstrumentData 仪器数据结构
type InstrumentData struct {
    DeviceID    string    `json:"device_id"`
    Timestamp   time.Time `json:"timestamp"`
    CommandID   uint16    `json:"command_id"`
    DataType    uint8     `json:"data_type"`
    Value       float64   `json:"value"`
    RawData     []byte    `json:"raw_data,omitempty"`
    Status      uint8     `json:"status"`
    Error       string    `json:"error,omitempty"`
}

// ParseResult 解析结果
type ParseResult struct {
    Success bool
    Data    *InstrumentData
    Error   error
}

// 协议常量
const (
    // 数据类型
    DataTypeTemperature = 0x01
    DataTypePressure    = 0x02
    DataTypeFlow        = 0x03
    DataTypeVoltage     = 0x04
    
    // 状态码
    StatusNormal  = 0x00
    StatusWarning = 0x01
    StatusError   = 0x02
    
    // 协议头
    ProtocolHeaderSize = 8
    ProtocolMagic      = 0xAA55
)
