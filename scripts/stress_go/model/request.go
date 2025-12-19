package model

// Request 压测请求配置
type Request struct {
	Concurrency        uint64 // 并发用户数
	TotalNumber        uint64 // 每个用户的请求数
	URL                string // Gateway WebSocket URL
	StartUserID        int32  // 起始用户ID
	Debug              bool   // 调试模式
	ConnectionInterval int    // 连接间隔（毫秒），0 表示无间隔
}

// GetTotalRequests 获取总请求数
func (r *Request) GetTotalRequests() uint64 {
	return r.Concurrency * r.TotalNumber
}
