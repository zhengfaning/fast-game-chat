package statistics

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"stress_go/model"
)

// Statistics 统计信息收集器
type Statistics struct {
	concurrency  uint64    // 并发数
	totalReqs    uint64    // 总请求数
	startTime    time.Time // 开始时间
	endTime      time.Time // 结束时间
	successCount uint64    // 成功数
	failureCount uint64    // 失败数
	totalSent    uint64    // 总发送消息数
	totalRecv    uint64    // 总接收消息数
	totalLatency uint64    // 总延迟(纳秒)
	minLatency   uint64    // 最小延迟(纳秒)
	maxLatency   uint64    // 最大延迟(纳秒)
	resultChan   chan *model.RequestResult
	wg           sync.WaitGroup
	errors       []error
	errorMutex   sync.Mutex
}

// NewStatistics 创建新的统计收集器
func NewStatistics(concurrency, totalReqs uint64) *Statistics {
	return &Statistics{
		concurrency: concurrency,
		totalReqs:   totalReqs,
		resultChan:  make(chan *model.RequestResult, concurrency*10),
		minLatency:  ^uint64(0), // 最大值
		errors:      make([]error, 0),
	}
}

// Start 开始统计
func (s *Statistics) Start() {
	s.startTime = time.Now()
	s.wg.Add(1)
	go s.collect()
}

// AddResult 添加结果
func (s *Statistics) AddResult(result *model.RequestResult) {
	s.resultChan <- result
}

// Stop 停止统计
func (s *Statistics) Stop() {
	close(s.resultChan)
	s.wg.Wait()
	s.endTime = time.Now()
}

// collect 收集结果
func (s *Statistics) collect() {
	defer s.wg.Done()

	for result := range s.resultChan {
		if result.Success {
			atomic.AddUint64(&s.successCount, uint64(result.MessagesSent))
			atomic.AddUint64(&s.totalSent, uint64(result.MessagesSent))
			atomic.AddUint64(&s.totalRecv, uint64(result.MessagesRecv))

			// 更新延迟统计 (这里是会话总延迟)
			latency := uint64(result.Duration.Nanoseconds())
			atomic.AddUint64(&s.totalLatency, latency)

			// 更新最小/最大延迟统计
			// 注意：这里的延迟统计是平均每条消息的延迟，还是整个会话？
			// 为了保持一致性，我们统计整个会话的延迟分布
			// 或者更好的办法是在 PrintReport 中处理

			// 更新最小延迟
			for {
				old := atomic.LoadUint64(&s.minLatency)
				if latency >= old {
					break
				}
				if atomic.CompareAndSwapUint64(&s.minLatency, old, latency) {
					break
				}
			}

			// 更新最大延迟
			for {
				old := atomic.LoadUint64(&s.maxLatency)
				if latency <= old {
					break
				}
				if atomic.CompareAndSwapUint64(&s.maxLatency, old, latency) {
					break
				}
			}
		} else {
			// 如果失败了，计算失败的请求数
			// 假设预期的每用户请求数是 s.totalReqs
			failed := s.totalReqs - uint64(result.MessagesSent)
			if failed == 0 && !result.Success {
				failed = s.totalReqs // 整个流程失败 (可能是 Bind 失败)
			}
			atomic.AddUint64(&s.failureCount, failed)
			// 同时也要累加已经发送和接收的消息
			atomic.AddUint64(&s.totalSent, uint64(result.MessagesSent))
			atomic.AddUint64(&s.totalRecv, uint64(result.MessagesRecv))

			if result.Error != nil {
				s.errorMutex.Lock()
				s.errors = append(s.errors, result.Error)
				s.errorMutex.Unlock()
			}
		}
	}
}

// PrintReport 打印报告
func (s *Statistics) PrintReport() {
	duration := s.endTime.Sub(s.startTime)
	totalRequests := s.concurrency * s.totalReqs

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("         压力测试结果汇总")
	fmt.Println("========================================")
	fmt.Printf("并发用户数:      %d\n", s.concurrency)
	fmt.Printf("每用户请求数:    %d\n", s.totalReqs)
	fmt.Printf("总请求数:        %d\n", totalRequests)
	fmt.Println("----------------------------------------")
	fmt.Printf("成功:            %d (%.2f%%)\n", s.successCount, float64(s.successCount)*100/float64(totalRequests))
	fmt.Printf("失败:            %d (%.2f%%)\n", s.failureCount, float64(s.failureCount)*100/float64(totalRequests))
	fmt.Println("----------------------------------------")
	fmt.Printf("消息发送:        %d\n", s.totalSent)
	fmt.Printf("消息接收:        %d\n", s.totalRecv)
	fmt.Println("----------------------------------------")
	fmt.Printf("测试时长:        %.2f 秒\n", duration.Seconds())
	fmt.Printf("吞吐量:          %.2f 请求/秒\n", float64(s.successCount)/duration.Seconds())
	fmt.Printf("用户吞吐:        %.2f 用户/秒\n", float64(s.concurrency)/duration.Seconds())
	fmt.Println("----------------------------------------")

	if s.successCount > 0 {
		avgLatency := time.Duration(s.totalLatency / s.successCount)
		minLatency := time.Duration(s.minLatency)
		maxLatency := time.Duration(s.maxLatency)

		fmt.Printf("平均延迟:        %v\n", avgLatency)
		fmt.Printf("最小延迟:        %v\n", minLatency)
		fmt.Printf("最大延迟:        %v\n", maxLatency)
		fmt.Println("----------------------------------------")
	}

	if s.failureCount > 0 {
		fmt.Printf("\n❌ 失败详情 (显示前10个):\n")
		errorCount := len(s.errors)
		if errorCount > 10 {
			errorCount = 10
		}
		for i := 0; i < errorCount; i++ {
			fmt.Printf("   %d. %v\n", i+1, s.errors[i])
		}
		fmt.Println()
	}

	fmt.Println("========================================")

	if s.failureCount == 0 {
		log.Println("✅ 测试通过！所有请求成功完成")
	} else {
		log.Printf("⚠️  测试完成，但有 %d 个请求失败", s.failureCount)
	}
}

// GetSuccessRate 获取成功率
func (s *Statistics) GetSuccessRate() float64 {
	total := s.successCount + s.failureCount
	if total == 0 {
		return 0
	}
	return float64(s.successCount) * 100 / float64(total)
}
