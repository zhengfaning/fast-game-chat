package model

import "time"

// RequestResult 单次请求的结果
type RequestResult struct {
	UserID       int32         // 用户ID
	Success      bool          // 是否成功
	Error        error         // 错误信息
	Duration     time.Duration // 请求耗时
	MessagesSent int           // 发送的消息数
	MessagesRecv int           // 接收的消息数
}

// UserStats 单个用户的统计信息
type UserStats struct {
	UserID          int32         // 用户ID
	TotalRequests   int           // 总请求数
	SuccessRequests int           // 成功请求数
	FailedRequests  int           // 失败请求数
	MessagesSent    int           // 发送的消息数
	MessagesRecv    int           // 接收的消息数
	TotalDuration   time.Duration // 总耗时
	MinDuration     time.Duration // 最小耗时
	MaxDuration     time.Duration // 最大耗时
}
