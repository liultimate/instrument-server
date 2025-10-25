package config

import (
    "fmt"
    "os"
    "time"

    "gopkg.in/yaml.v3"
)

type Config struct {
    Server  ServerConfig  `yaml:"server"`
    Redis   RedisConfig   `yaml:"redis"`
    Log     LogConfig     `yaml:"log"`
    Monitor MonitorConfig `yaml:"monitor"`
}

type ServerConfig struct {
    Host           string        `yaml:"host"`
    Port           int           `yaml:"port"`
    MaxConnections int           `yaml:"max_connections"`
    ReadTimeout    time.Duration `yaml:"read_timeout"`
    WriteTimeout   time.Duration `yaml:"write_timeout"`
    BufferSize     int           `yaml:"buffer_size"`
    KeepAlive      time.Duration `yaml:"keep_alive"`
}

type RedisConfig struct {
    Addr     string `yaml:"addr"`
    Password string `yaml:"password"`
    DB       int    `yaml:"db"`
    PoolSize int    `yaml:"pool_size"`
    Channel  string `yaml:"channel"`
}

type LogConfig struct {
    Level    string `yaml:"level"`
    Format   string `yaml:"format"`
    Output   string `yaml:"output"`
    FilePath string `yaml:"file_path"`
}

type MonitorConfig struct {
    Enabled     bool `yaml:"enabled"`
    MetricsPort int  `yaml:"metrics_port"`
}

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("读取配置文件失败: %w", err)
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("解析配置文件失败: %w", err)
    }

    return &config, nil
}

// GetDefaultConfig 返回默认配置
func GetDefaultConfig() *Config {
    return &Config{
        Server: ServerConfig{
            Host:           "0.0.0.0",
            Port:           8888,
            MaxConnections: 100000,
            ReadTimeout:    30 * time.Second,
            WriteTimeout:   30 * time.Second,
            BufferSize:     4096,
            KeepAlive:      180 * time.Second,
        },
        Redis: RedisConfig{
            Addr:     "localhost:6379",
            Password: "",
            DB:       0,
            PoolSize: 100,
            Channel:  "instrument_data",
        },
        Log: LogConfig{
            Level:  "info",
            Format: "json",
            Output: "stdout",
        },
        Monitor: MonitorConfig{
            Enabled:     true,
            MetricsPort: 9090,
        },
    }
}
