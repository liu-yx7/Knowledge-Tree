package sync

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/usememos/memos/plugin/ragflow"
)

// ==================== Health Checker 定义 ====================

// HealthChecker RAGFlow 服务健康检查与熔断器
// 职责：监控 RAGFlow 服务可用性，在服务不可用时实施熔断保护
type HealthChecker struct {
	client *ragflow.Client

	mu            sync.RWMutex
	isHealthy     bool      // 当前健康状态
	lastCheckTime time.Time // 最后检查时间
	failureCount  int       // 连续失败次数
	circuitOpen   bool      // 熔断器是否打开
	circuitOpenAt time.Time // 熔断器打开时间

	// 配置
	checkInterval    time.Duration // 健康检查间隔
	failureThreshold int           // 触发熔断的失败阈值
	recoveryTimeout  time.Duration // 熔断恢复超时
}

// HealthCheckerConfig 健康检查器配置
type HealthCheckerConfig struct {
	CheckInterval    time.Duration // 默认 30 秒
	FailureThreshold int           // 默认 3 次
	RecoveryTimeout  time.Duration // 默认 60 秒
}

// DefaultHealthCheckerConfig 返回默认配置
func DefaultHealthCheckerConfig() *HealthCheckerConfig {
	return &HealthCheckerConfig{
		CheckInterval:    30 * time.Second,
		FailureThreshold: 3,
		RecoveryTimeout:  60 * time.Second,
	}
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(client *ragflow.Client, cfg *HealthCheckerConfig) *HealthChecker {
	if cfg == nil {
		cfg = DefaultHealthCheckerConfig()
	}
	return &HealthChecker{
		client:           client,
		isHealthy:        true, // 初始假设健康
		checkInterval:    cfg.CheckInterval,
		failureThreshold: cfg.FailureThreshold,
		recoveryTimeout:  cfg.RecoveryTimeout,
	}
}

// ==================== 健康状态查询 ====================

// IsHealthy 返回 RAGFlow 服务是否健康
// 如果距离上次检查超过 checkInterval，会触发新的检查
func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
	h.mu.RLock()
	timeSinceLastCheck := time.Since(h.lastCheckTime)
	cachedHealthy := h.isHealthy
	circuitOpen := h.circuitOpen
	h.mu.RUnlock()

	// 如果熔断器打开，检查是否应该尝试恢复
	if circuitOpen {
		return h.tryRecovery(ctx)
	}

	// 如果检查间隔已过，进行新的检查
	if timeSinceLastCheck >= h.checkInterval {
		return h.performCheck(ctx)
	}

	return cachedHealthy
}

// IsCircuitOpen 返回熔断器是否打开
func (h *HealthChecker) IsCircuitOpen() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.circuitOpen
}

// GetStatus 获取详细的健康状态
func (h *HealthChecker) GetStatus() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return HealthStatus{
		IsHealthy:     h.isHealthy,
		CircuitOpen:   h.circuitOpen,
		FailureCount:  h.failureCount,
		LastCheckTime: h.lastCheckTime,
		CircuitOpenAt: h.circuitOpenAt,
	}
}

// HealthStatus 健康状态详情
type HealthStatus struct {
	IsHealthy     bool
	CircuitOpen   bool
	FailureCount  int
	LastCheckTime time.Time
	CircuitOpenAt time.Time
}

// ==================== 内部方法 ====================

// performCheck 执行健康检查
func (h *HealthChecker) performCheck(ctx context.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastCheckTime = time.Now()

	// 执行健康检查
	err := h.client.HealthCheck(ctx)
	if err != nil {
		h.failureCount++
		h.isHealthy = false
		slog.Warn("RAGFlow 健康检查失败",
			slog.Int("failureCount", h.failureCount),
			slog.Any("error", err))

		// 检查是否需要触发熔断
		if h.failureCount >= h.failureThreshold {
			h.openCircuit()
		}
		return false
	}

	// 检查成功，重置失败计数
	h.failureCount = 0
	h.isHealthy = true
	slog.Debug("RAGFlow 健康检查通过")
	return true
}

// openCircuit 打开熔断器
func (h *HealthChecker) openCircuit() {
	h.circuitOpen = true
	h.circuitOpenAt = time.Now()
	slog.Warn("RAGFlow 熔断器已打开",
		slog.Int("failureThreshold", h.failureThreshold),
		slog.Duration("recoveryTimeout", h.recoveryTimeout))
}

// tryRecovery 尝试熔断恢复
func (h *HealthChecker) tryRecovery(ctx context.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查是否已经过了恢复超时时间
	if time.Since(h.circuitOpenAt) < h.recoveryTimeout {
		return false
	}

	slog.Info("RAGFlow 熔断器尝试恢复...")
	h.lastCheckTime = time.Now()

	// 尝试健康检查
	err := h.client.HealthCheck(ctx)
	if err != nil {
		// 恢复失败，重置熔断时间
		h.circuitOpenAt = time.Now()
		slog.Warn("RAGFlow 熔断恢复失败，继续保持熔断状态", slog.Any("error", err))
		return false
	}

	// 恢复成功
	h.circuitOpen = false
	h.failureCount = 0
	h.isHealthy = true
	slog.Info("RAGFlow 熔断器已恢复，服务可用")
	return true
}

// ==================== 手动控制方法 ====================

// RecordSuccess 记录一次成功的操作
func (h *HealthChecker) RecordSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failureCount = 0
	h.isHealthy = true
	if h.circuitOpen {
		h.circuitOpen = false
		slog.Info("RAGFlow 操作成功，熔断器已关闭")
	}
}

// RecordFailure 记录一次失败的操作
func (h *HealthChecker) RecordFailure(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failureCount++
	slog.Warn("RAGFlow 操作失败",
		slog.Int("failureCount", h.failureCount),
		slog.Any("error", err))

	if h.failureCount >= h.failureThreshold && !h.circuitOpen {
		h.openCircuit()
	}
}

// ForceOpen 强制打开熔断器（用于紧急情况）
func (h *HealthChecker) ForceOpen() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.circuitOpen = true
	h.circuitOpenAt = time.Now()
	h.isHealthy = false
	slog.Warn("RAGFlow 熔断器被强制打开")
}

// ForceClose 强制关闭熔断器
func (h *HealthChecker) ForceClose() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.circuitOpen = false
	h.failureCount = 0
	h.isHealthy = true
	slog.Info("RAGFlow 熔断器被强制关闭")
}

// ==================== 后台检查任务 ====================

// StartBackgroundCheck 启动后台定期健康检查
// 返回一个取消函数用于停止检查
func (h *HealthChecker) StartBackgroundCheck(ctx context.Context) context.CancelFunc {
	checkCtx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(h.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-checkCtx.Done():
				slog.Info("RAGFlow 健康检查后台任务已停止")
				return
			case <-ticker.C:
				h.IsHealthy(checkCtx)
			}
		}
	}()

	return cancel
}
