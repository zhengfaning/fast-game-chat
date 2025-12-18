package metrics

import (
	"log"
	"sync/atomic"
	"time"
)

// Metrics 性能指标收集器
type Metrics struct {
	// 连接统计
	TotalConnections    uint64
	ActiveConnections   uint64
	TotalDisconnections uint64

	// 消息统计
	MessagesReceived uint64
	MessagesSent     uint64
	MessagesRouted   uint64
	RoutingErrors    uint64

	// 性能统计
	SlowMessages uint64 // 处理时间 > 100ms 的消息数
}

var GlobalMetrics = &Metrics{}

// IncrementConnections 增加连接数
func (m *Metrics) IncrementConnections() {
	atomic.AddUint64(&m.TotalConnections, 1)
	atomic.AddUint64(&m.ActiveConnections, 1)
}

// DecrementConnections 减少连接数
func (m *Metrics) DecrementConnections() {
	atomic.AddUint64(&m.TotalDisconnections, 1)
	atomic.AddUint64(&m.ActiveConnections, ^uint64(0)) // -1
}

// IncrementMessagesReceived 增加接收消息数
func (m *Metrics) IncrementMessagesReceived() {
	atomic.AddUint64(&m.MessagesReceived, 1)
}

// IncrementMessagesSent 增加发送消息数
func (m *Metrics) IncrementMessagesSent() {
	atomic.AddUint64(&m.MessagesSent, 1)
}

// IncrementMessagesRouted 增加路由消息数
func (m *Metrics) IncrementMessagesRouted() {
	atomic.AddUint64(&m.MessagesRouted, 1)
}

// IncrementRoutingErrors 增加路由错误数
func (m *Metrics) IncrementRoutingErrors() {
	atomic.AddUint64(&m.RoutingErrors, 1)
}

// IncrementSlowMessages 增加慢消息数
func (m *Metrics) IncrementSlowMessages() {
	atomic.AddUint64(&m.SlowMessages, 1)
}

// PrintStats 打印统计信息
func (m *Metrics) PrintStats() {
	log.Printf("========== Gateway Metrics ==========")
	log.Printf("Connections:")
	log.Printf("  Total:        %d", atomic.LoadUint64(&m.TotalConnections))
	log.Printf("  Active:       %d", atomic.LoadUint64(&m.ActiveConnections))
	log.Printf("  Disconnected: %d", atomic.LoadUint64(&m.TotalDisconnections))
	log.Printf("Messages:")
	log.Printf("  Received:     %d", atomic.LoadUint64(&m.MessagesReceived))
	log.Printf("  Sent:         %d", atomic.LoadUint64(&m.MessagesSent))
	log.Printf("  Routed:       %d", atomic.LoadUint64(&m.MessagesRouted))
	log.Printf("  Routing Err:  %d", atomic.LoadUint64(&m.RoutingErrors))
	log.Printf("Performance:")
	log.Printf("  Slow Msgs:    %d", atomic.LoadUint64(&m.SlowMessages))
	log.Printf("=====================================")
}

// StartPeriodicReport 启动定期报告
func (m *Metrics) StartPeriodicReport(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			m.PrintStats()
		}
	}()
}
