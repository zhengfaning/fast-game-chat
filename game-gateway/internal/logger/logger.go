package logger

import (
	"log"
	"os"
	"sync"
)

// LogLevel 定义日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// LogTag 定义日志标签
type LogTag string

const (
	TagSession  LogTag = "SESSION"  // Session管理相关
	TagRouter   LogTag = "ROUTER"   // 路由相关
	TagMQ       LogTag = "MQ"       // 消息队列相关
	TagBackend  LogTag = "BACKEND"  // 后端连接相关
	TagProtocol LogTag = "PROTOCOL" // 协议解析相关
	TagPerf     LogTag = "PERF"     // 性能统计相关
)

// Logger 可配置的日志记录器
type Logger struct {
	mu          sync.RWMutex
	enabledTags map[LogTag]bool
	minLevel    LogLevel
	logger      *log.Logger
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Init 初始化全局日志记录器
func Init() {
	once.Do(func() {
		defaultLogger = &Logger{
			enabledTags: make(map[LogTag]bool),
			minLevel:    INFO,
			logger:      log.New(os.Stdout, "", log.LstdFlags),
		}

		// 默认启用的标签
		defaultLogger.EnableTag(TagSession)
		defaultLogger.EnableTag(TagRouter)
		defaultLogger.EnableTag(TagMQ)
		defaultLogger.EnableTag(TagBackend)
	})
}

// EnableTag 启用指定标签的日志
func (l *Logger) EnableTag(tag LogTag) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabledTags[tag] = true
}

// DisableTag 禁用指定标签的日志
func (l *Logger) DisableTag(tag LogTag) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabledTags[tag] = false
}

// IsTagEnabled 检查标签是否启用
func (l *Logger) IsTagEnabled(tag LogTag) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabledTags[tag]
}

// SetLevel 设置最小日志级别
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// Debug 输出调试日志
func (l *Logger) Debug(tag LogTag, format string, v ...interface{}) {
	if l.IsTagEnabled(tag) && l.minLevel <= DEBUG {
		l.logger.Printf("[DEBUG][%s] "+format, append([]interface{}{tag}, v...)...)
	}
}

// Info 输出信息日志
func (l *Logger) Info(tag LogTag, format string, v ...interface{}) {
	if l.IsTagEnabled(tag) && l.minLevel <= INFO {
		l.logger.Printf("[INFO][%s] "+format, append([]interface{}{tag}, v...)...)
	}
}

// Warn 输出警告日志
func (l *Logger) Warn(tag LogTag, format string, v ...interface{}) {
	if l.IsTagEnabled(tag) && l.minLevel <= WARN {
		l.logger.Printf("[WARN][%s] "+format, append([]interface{}{tag}, v...)...)
	}
}

// Error 输出错误日志
func (l *Logger) Error(tag LogTag, format string, v ...interface{}) {
	if l.IsTagEnabled(tag) && l.minLevel <= ERROR {
		l.logger.Printf("[ERROR][%s] "+format, append([]interface{}{tag}, v...)...)
	}
}

// 全局便捷函数
func EnableTag(tag LogTag)    { defaultLogger.EnableTag(tag) }
func DisableTag(tag LogTag)   { defaultLogger.DisableTag(tag) }
func SetLevel(level LogLevel) { defaultLogger.SetLevel(level) }

func Debug(tag LogTag, format string, v ...interface{}) { defaultLogger.Debug(tag, format, v...) }
func Info(tag LogTag, format string, v ...interface{})  { defaultLogger.Info(tag, format, v...) }
func Warn(tag LogTag, format string, v ...interface{})  { defaultLogger.Warn(tag, format, v...) }
func Error(tag LogTag, format string, v ...interface{}) { defaultLogger.Error(tag, format, v...) }
