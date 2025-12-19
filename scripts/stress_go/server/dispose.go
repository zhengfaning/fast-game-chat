package server

import (
	"fmt"
	"log"
	"sync"
	"time"

	"stress_go/client"
	"stress_go/model"
	"stress_go/statistics"
)

// Dispose 处理压测请求
func Dispose(request *model.Request) {
	// 创建统计收集器
	stats := statistics.NewStatistics(request.Concurrency, request.TotalNumber)
	stats.Start()

	// 创建 WaitGroup
	var wg sync.WaitGroup
	wg.Add(int(request.Concurrency))

	// 启动进度显示
	stopProgress := make(chan bool)
	go showProgress(stats, request.GetTotalRequests(), stopProgress)

	// 启动所有用户的压测
	startTime := time.Now()
	for i := uint64(0); i < request.Concurrency; i++ {
		userID := request.StartUserID + int32(i)
		go runUserStressTest(userID, request, stats, &wg)

		// 错开连接，避免连接风暴（可配置间隔）
		if i < request.Concurrency-1 && request.ConnectionInterval > 0 {
			time.Sleep(time.Duration(request.ConnectionInterval) * time.Millisecond)
		}
	}

	// 等待所有用户完成
	wg.Wait()
	stopProgress <- true

	// 停止统计
	stats.Stop()

	// 打印报告
	fmt.Println()
	stats.PrintReport()

	log.Printf("总耗时: %v", time.Since(startTime))
}

// runUserStressTest 运行单个用户的压测
func runUserStressTest(userID int32, request *model.Request, stats *statistics.Statistics, wg *sync.WaitGroup) {
	defer wg.Done()

	// 恢复 panic
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[User %d] Panic recovered: %v", userID, r)
			stats.AddResult(&model.RequestResult{
				UserID:  userID,
				Success: false,
				Error:   fmt.Errorf("panic: %v", r),
			})
		}
	}()

	// 创建客户端 (带重试)
	var chatClient *client.GameChatClient
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		chatClient, err = client.NewGameChatClient(userID, request.URL, request.Debug)
		if err == nil {
			break
		}
		// Connection Refused 是常见的启动时错误，等待一下重试
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}

	if err != nil {
		if request.Debug {
			log.Printf("[User %d] ❌ Failed to create client after retries: %v", userID, err)
		}
		stats.AddResult(&model.RequestResult{
			UserID:  userID,
			Success: false,
			Error:   fmt.Errorf("create client failed: %w", err),
		})
		return
	}
	defer chatClient.Close()

	// 执行多次请求
	for i := uint64(0); i < request.TotalNumber; i++ {
		result := chatClient.RunTest(1) // 每次测试发送1条消息
		stats.AddResult(result)

		if !result.Success && request.Debug {
			log.Printf("[User %d] Request %d failed: %v", userID, i+1, result.Error)
		}

		// 请求间隔
		if i < request.TotalNumber-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	if request.Debug {
		log.Printf("[User %d] ✅ Completed all requests", userID)
	}
}

// showProgress 显示进度
func showProgress(stats *statistics.Statistics, totalRequests uint64, stop chan bool) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 这里可以添加实时进度显示
			// 由于统计数据在另一个 goroutine 中更新，这里只是简单提示
			fmt.Print(".")
		case <-stop:
			return
		}
	}
}
