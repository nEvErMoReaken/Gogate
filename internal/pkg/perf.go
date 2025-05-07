package pkg

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// PerformanceMetrics 存储性能指标数据
type PerformanceMetrics struct {
	// 系统指标
	StartTime      time.Time
	GoroutineCount int64
	MemStats       runtime.MemStats

	// 应用指标
	RequestCount   int64
	ErrorCount     int64
	ProcessingTime int64 // 纳秒
	ProcessedItems int64

	// 消息处理指标 - 使用原子操作或细粒度锁替代全局锁
	msgStats *concurrentMsgStats
}

// concurrentMsgStats 使用分离锁保护不同类型的消息统计
type concurrentMsgStats struct {
	received  sync.Map // string -> *int64
	processed sync.Map // string -> *int64
	errors    sync.Map // string -> *int64
}

// 全局性能指标实例
var (
	perfMetrics *PerformanceMetrics
	once        sync.Once
)

// GetPerformanceMetrics 返回性能指标实例
func GetPerformanceMetrics() *PerformanceMetrics {
	once.Do(func() {
		perfMetrics = &PerformanceMetrics{
			StartTime: time.Now(),
			msgStats:  &concurrentMsgStats{},
		}

		// 开始定期收集系统指标
		go perfMetrics.collectSystemMetrics()
	})
	return perfMetrics
}

// collectSystemMetrics 定期收集系统指标
func (pm *PerformanceMetrics) collectSystemMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 更新协程数
		atomic.StoreInt64(&pm.GoroutineCount, int64(runtime.NumGoroutine()))

		// 更新内存统计
		runtime.ReadMemStats(&pm.MemStats)
	}
}

// IncRequestCount 增加请求计数并返回当前值
func (pm *PerformanceMetrics) IncRequestCount() int64 {
	return atomic.AddInt64(&pm.RequestCount, 1)
}

// IncErrorCount 增加错误计数并返回当前值
func (pm *PerformanceMetrics) IncErrorCount() int64 {
	return atomic.AddInt64(&pm.ErrorCount, 1)
}

// AddProcessingTime 添加处理时间并返回累计时间
func (pm *PerformanceMetrics) AddProcessingTime(duration time.Duration) int64 {
	return atomic.AddInt64(&pm.ProcessingTime, int64(duration))
}

// IncProcessedItems 增加处理项目数并返回当前值
func (pm *PerformanceMetrics) IncProcessedItems() int64 {
	return atomic.AddInt64(&pm.ProcessedItems, 1)
}

// 从sync.Map中获取计数器，如果不存在则创建
func getOrCreateCounter(m *sync.Map, key string) *int64 {
	if val, ok := m.Load(key); ok {
		return val.(*int64)
	}

	// 不存在则创建新计数器
	counter := new(int64)
	if actual, loaded := m.LoadOrStore(key, counter); loaded {
		return actual.(*int64)
	}
	return counter
}

// IncMsgReceived 增加特定类型的接收消息计数并返回当前值
func (pm *PerformanceMetrics) IncMsgReceived(msgType string) int64 {
	counter := getOrCreateCounter(&pm.msgStats.received, msgType)
	return atomic.AddInt64(counter, 1)
}

// IncMsgProcessed 增加特定类型的处理消息计数并返回当前值
func (pm *PerformanceMetrics) IncMsgProcessed(msgType string) int64 {
	counter := getOrCreateCounter(&pm.msgStats.processed, msgType)
	return atomic.AddInt64(counter, 1)
}

// IncMsgErrors 增加特定类型的错误消息计数并返回当前值
func (pm *PerformanceMetrics) IncMsgErrors(msgType string) int64 {
	counter := getOrCreateCounter(&pm.msgStats.errors, msgType)
	return atomic.AddInt64(counter, 1)
}

// GetMsgCount 获取特定类型的消息计数
func (pm *PerformanceMetrics) GetMsgCount(msgType string, statsType string) int64 {
	var m *sync.Map
	switch statsType {
	case "received":
		m = &pm.msgStats.received
	case "processed":
		m = &pm.msgStats.processed
	case "errors":
		m = &pm.msgStats.errors
	default:
		return 0
	}

	if val, ok := m.Load(msgType); ok {
		return atomic.LoadInt64(val.(*int64))
	}
	return 0
}

// GetMetricsReport 获取性能指标报告
func (pm *PerformanceMetrics) GetMetricsReport() string {
	uptime := time.Since(pm.StartTime)
	var avgProcessingTime float64
	if atomic.LoadInt64(&pm.ProcessedItems) > 0 {
		avgProcessingTime = float64(atomic.LoadInt64(&pm.ProcessingTime)) / float64(atomic.LoadInt64(&pm.ProcessedItems)) / float64(time.Millisecond)
	}

	report := fmt.Sprintf(
		"系统运行时间: %s\n"+
			"协程数: %d\n"+
			"内存使用: %d MB (已分配: %d MB, 系统: %d MB)\n"+
			"GC次数: %d\n"+
			"请求计数: %d\n"+
			"错误计数: %d\n"+
			"平均处理时间: %.2f ms\n\n",
		uptime,
		atomic.LoadInt64(&pm.GoroutineCount),
		pm.MemStats.Alloc/1024/1024,
		pm.MemStats.TotalAlloc/1024/1024,
		pm.MemStats.Sys/1024/1024,
		pm.MemStats.NumGC,
		atomic.LoadInt64(&pm.RequestCount),
		atomic.LoadInt64(&pm.ErrorCount),
		avgProcessingTime,
	)

	report += "消息统计:\n"
	// 收集所有消息类型
	msgTypes := make(map[string]struct{})
	pm.msgStats.received.Range(func(key, _ interface{}) bool {
		msgTypes[key.(string)] = struct{}{}
		return true
	})

	// 生成报告
	for msgType := range msgTypes {
		received := pm.GetMsgCount(msgType, "received")
		processed := pm.GetMsgCount(msgType, "processed")
		errors := pm.GetMsgCount(msgType, "errors")

		report += fmt.Sprintf("- %s: 接收=%d, 处理=%d, 错误=%d\n",
			msgType, received, processed, errors)
	}

	return report
}

// LogMetrics 将性能指标写入日志
func (pm *PerformanceMetrics) LogMetrics(logger *zap.Logger) {
	runtime.ReadMemStats(&pm.MemStats)

	logger.Info("性能指标统计",
		zap.Duration("uptime", time.Since(pm.StartTime)),
		zap.Int64("goroutines", atomic.LoadInt64(&pm.GoroutineCount)),
		zap.Uint64("memory_mb", pm.MemStats.Alloc/1024/1024),
		zap.Uint64("gc_count", uint64(pm.MemStats.NumGC)),
		zap.Int64("requests", atomic.LoadInt64(&pm.RequestCount)),
		zap.Int64("errors", atomic.LoadInt64(&pm.ErrorCount)),
	)
}

// Timer 简单的计时器结构体
type Timer struct {
	start   time.Time
	metrics *PerformanceMetrics
	name    string
}

// NewTimer 创建一个新的计时器
func (pm *PerformanceMetrics) NewTimer(name string) *Timer {
	return &Timer{
		start:   time.Now(),
		metrics: pm,
		name:    name,
	}
}

// Stop 停止计时器并记录时间
func (t *Timer) Stop() time.Duration {
	duration := time.Since(t.start)

	// 更新性能指标
	t.metrics.AddProcessingTime(duration)
	t.metrics.IncProcessedItems()

	return duration
}

// StopAndLog 停止计时器并记录到日志
func (t *Timer) StopAndLog(logger *zap.Logger) time.Duration {
	duration := t.Stop()
	logger.Debug("操作计时",
		zap.String("operation", t.name),
		zap.Duration("duration", duration),
	)
	return duration
}
