package main

import (
    "flag"
    "fmt"
    "os"
    "github.com/sirupsen/logrus"
    "instrument-server/internal/config"
    "instrument-server/internal/server"
)

var (
    Version   = "1.0.0"
    BuildTime = "unknown"
)

func main() {
    // 命令行参数
    configFile := flag.String("config", "configs/config.yaml", "配置文件路径")
    showVersion := flag.Bool("version", false, "显示版本信息")
    flag.Parse()

    // 显示版本
    if *showVersion {
        fmt.Printf("Instrument Server v%s (Build: %s)\n", Version, BuildTime)
        os.Exit(0)
    }

    // 加载配置
    cfg, err := config.LoadConfig(*configFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
        cfg = config.GetDefaultConfig()
        fmt.Println("使用默认配置")
    }

    // 初始化日志
    log := setupLogger(cfg.Log)
    log.Infof("Instrument Server v%s 启动中...", Version)
    log.Infof("配置文件: %s", *configFile)

    // 创建并启动服务器
    srv, err := server.NewTCPServer(cfg, log)
    if err != nil {
        log.Fatalf("创建服务器失败: %v", err)
    }

    if err := srv.Start(); err != nil {
        log.Fatalf("启动服务器失败: %v", err)
    }
}

func setupLogger(cfg config.LogConfig) *logrus.Logger {
    log := logrus.New()

    // 设置日志级别
    level, err := logrus.ParseLevel(cfg.Level)
    if err != nil {
        level = logrus.InfoLevel
    }
    log.SetLevel(level)

    // 设置日志格式
    if cfg.Format == "json" {
        log.SetFormatter(&logrus.JSONFormatter{
            TimestampFormat: "2006-01-02 15:04:05",
        })
    } else {
        log.SetFormatter(&logrus.TextFormatter{
            FullTimestamp:   true,
            TimestampFormat: "2006-01-02 15:04:05",
        })
    }

    // 设置输出
    if cfg.Output == "file" && cfg.FilePath != "" {
        file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
        if err == nil {
            log.SetOutput(file)
        } else {
            log.Warnf("打开日志文件失败: %v, 使用标准输出", err)
        }
    }

    return log
}
