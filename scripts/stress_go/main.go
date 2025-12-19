package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"stress_go/model"
	"stress_go/server"

	"gopkg.in/yaml.v3"
)

type GatewayConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
}

var (
	concurrency  uint64 = 1    // 并发用户数
	totalReqs    uint64 = 1    // 每个用户的请求数
	gatewayURL   string        // Gateway URL
	startUserID  int64  = 2000 // 起始用户ID
	debugMode    bool   = false
	connInterval int    = 2 // 连接间隔（毫秒）
)

func init() {
	flag.Uint64Var(&concurrency, "c", 1, "并发用户数")
	flag.Uint64Var(&totalReqs, "n", 1, "每个用户的请求数")
	flag.StringVar(&gatewayURL, "u", "", "Gateway WebSocket URL (留空则自动从配置读取)")
	flag.Int64Var((*int64)(&startUserID), "s", 2000, "起始用户ID")
	flag.BoolVar(&debugMode, "d", false, "调试模式")
	flag.IntVar(&connInterval, "i", 2, "连接间隔（毫秒），0=无间隔（最猛）")
}

// loadGatewayURL 从配置文件读取 Gateway 地址
func loadGatewayURL() string {
	// 优先级：dist/configs/gateway.yaml > game-gateway/configs/gateway.yaml
	configPaths := []string{
		"dist/configs/gateway.yaml",
		"game-gateway/configs/gateway.yaml",
	}

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var config GatewayConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			continue
		}

		// 构建 WebSocket URL
		host := config.Server.Host
		if host == "0.0.0.0" || host == "" {
			host = "localhost"
		}
		return fmt.Sprintf("ws://%s:%d/ws", host, config.Server.Port)
	}

	// 如果都读取失败，返回默认值
	return "ws://localhost:8080/ws"
}

func main() {
	flag.Parse()

	// 如果未指定 URL，从配置文件读取
	if gatewayURL == "" {
		gatewayURL = loadGatewayURL()
	}

	// 打印配置信息
	printHeader()

	// 验证参数
	if concurrency == 0 || totalReqs == 0 {
		log.Fatal("❌ 并发数和请求数必须大于0")
	}

	// 设置 GOMAXPROCS
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 创建请求配置
	request := &model.Request{
		Concurrency:        concurrency,
		TotalNumber:        totalReqs,
		URL:                gatewayURL,
		StartUserID:        int32(startUserID),
		Debug:              debugMode,
		ConnectionInterval: connInterval,
	}

	// 启动压测
	log.Printf("🚀 开始压测...")
	log.Printf("   并发用户: %d", concurrency)
	log.Printf("   每用户请求数: %d", totalReqs)
	log.Printf("   总请求数: %d", concurrency*totalReqs)
	log.Printf("   Gateway: %s", gatewayURL)
	log.Printf("   用户ID范围: %d - %d", startUserID, startUserID+int64(concurrency)-1)
	log.Printf("   连接间隔: %dms%s", connInterval, func() string {
		if connInterval == 0 {
			return " (🔥 无间隔冲击模式)"
		}
		return ""
	}())
	fmt.Println()

	// 执行压测
	server.Dispose(request)
}

func printHeader() {
	fmt.Println("========================================")
	fmt.Println("  Game Chat Stress Testing Tool")
	fmt.Println("  高性能 WebSocket 压力测试工具")
	fmt.Println("========================================")
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("CPU Cores: %d\n", runtime.NumCPU())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("========================================")
	fmt.Println()
}
