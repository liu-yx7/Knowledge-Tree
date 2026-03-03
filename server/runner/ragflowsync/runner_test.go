// filepath: /Users/yuxuanli/Desktop/Project/Knowtree/Knowledge-Tree/server/runner/ragflowsync/runner_test.go
package ragflowsync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/store"
)

// ==================== NewRunner 测试 ====================

func TestNewRunner_NilConfig(t *testing.T) {
	// 当配置为 nil 时，应返回 nil
	runner := NewRunner(nil, nil)
	assert.Nil(t, runner, "配置为 nil 时应返回 nil Runner")
}

func TestNewRunner_WithConfig(t *testing.T) {
	config := &ragflow.Config{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-api-key",
	}

	// 注意：这里 store 为 nil，但 NewRunner 不检查 store
	runner := NewRunner(nil, config)

	require.NotNil(t, runner, "有效配置应返回非 nil Runner")
	assert.NotNil(t, runner.config, "config 应被设置")
	assert.NotNil(t, runner.ragflowClient, "ragflowClient 应被创建")
	assert.NotNil(t, runner.orchestrator, "orchestrator 应被创建")
	assert.NotNil(t, runner.stopCh, "stopCh 应被初始化")
	assert.Equal(t, 5*time.Minute, runner.syncInterval, "默认同步间隔应为 5 分钟")
	assert.Equal(t, 50, runner.batchSize, "默认批量大小应为 50")
}

// ==================== 生命周期管理测试 ====================

func TestRunner_IsRunning_NilRunner(t *testing.T) {
	var runner *Runner
	assert.False(t, runner.IsRunning(), "nil Runner 的 IsRunning 应返回 false")
}

func TestRunner_IsRunning_NotStarted(t *testing.T) {
	config := &ragflow.Config{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-api-key",
	}
	runner := NewRunner(nil, config)

	assert.False(t, runner.IsRunning(), "未启动的 Runner 的 IsRunning 应返回 false")
}

func TestRunner_Stop_NilRunner(t *testing.T) {
	var runner *Runner
	// 不应 panic
	assert.NotPanics(t, func() {
		runner.Stop()
	}, "nil Runner 的 Stop 不应 panic")
}

func TestRunner_Run_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()

	// 不应 panic
	assert.NotPanics(t, func() {
		runner.Run(ctx)
	}, "nil Runner 的 Run 不应 panic")
}

func TestRunner_Run_NilConfig(t *testing.T) {
	runner := &Runner{config: nil}
	ctx := context.Background()

	// 不应 panic，应直接返回
	assert.NotPanics(t, func() {
		runner.Run(ctx)
	}, "config 为 nil 时 Run 不应 panic")
}

func TestRunner_Run_ContextCancellation(t *testing.T) {
	config := &ragflow.Config{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-api-key",
	}
	runner := NewRunner(nil, config)
	runner.syncInterval = 100 * time.Millisecond // 缩短间隔以加快测试

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runner.Run(ctx)
	}()

	// 等待 Runner 启动
	time.Sleep(50 * time.Millisecond)
	assert.True(t, runner.IsRunning(), "Runner 应该正在运行")

	// 取消上下文
	cancel()

	// 等待 Runner 停止
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("Runner 未能在超时时间内停止")
	}

	assert.False(t, runner.IsRunning(), "Runner 应该已停止")
}

func TestRunner_Run_StopSignal(t *testing.T) {
	config := &ragflow.Config{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-api-key",
	}
	runner := NewRunner(nil, config)
	runner.syncInterval = 100 * time.Millisecond

	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runner.Run(ctx)
	}()

	// 等待 Runner 启动
	time.Sleep(50 * time.Millisecond)
	assert.True(t, runner.IsRunning(), "Runner 应该正在运行")

	// 发送停止信号
	runner.Stop()

	// 等待 Runner 停止
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("Runner 未能在超时时间内停止")
	}

	assert.False(t, runner.IsRunning(), "Runner 应该已停止")
}

func TestRunner_Run_PreventDoubleStart(t *testing.T) {
	config := &ragflow.Config{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-api-key",
	}
	runner := NewRunner(nil, config)
	runner.syncInterval = 1 * time.Hour // 长间隔防止自动同步

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动第一个 Run
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runner.Run(ctx)
	}()

	// 等待 Runner 启动
	time.Sleep(50 * time.Millisecond)
	assert.True(t, runner.IsRunning(), "第一次 Run 应该正在运行")

	// 尝试第二次 Run（应该立即返回）
	secondRunDone := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(secondRunDone)
	}()

	select {
	case <-secondRunDone:
		// 第二次 Run 应该立即返回
	case <-time.After(1 * time.Second):
		t.Fatal("第二次 Run 应该立即返回")
	}

	// 清理
	cancel()
	wg.Wait()
}

// ==================== TriggerSync 测试 ====================

func TestRunner_TriggerSync_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()

	err := runner.TriggerSync(ctx)
	assert.NoError(t, err, "nil Runner 的 TriggerSync 应返回 nil error")
}

func TestRunner_TriggerSync_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()

	err := runner.TriggerSync(ctx)
	assert.NoError(t, err, "orchestrator 为 nil 时 TriggerSync 应返回 nil error")
}

// ==================== 事件处理测试 ====================

func TestRunner_OnMemoCreated_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()
	memo := &store.Memo{UID: "test-uid", CreatorID: 1}

	// 不应 panic
	assert.NotPanics(t, func() {
		runner.OnMemoCreated(ctx, memo)
	}, "nil Runner 的 OnMemoCreated 不应 panic")
}

func TestRunner_OnMemoCreated_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()
	memo := &store.Memo{UID: "test-uid", CreatorID: 1}

	// 不应 panic
	assert.NotPanics(t, func() {
		runner.OnMemoCreated(ctx, memo)
	}, "orchestrator 为 nil 时 OnMemoCreated 不应 panic")
}

func TestRunner_OnMemoUpdated_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()
	memo := &store.Memo{UID: "test-uid", CreatorID: 1}

	assert.NotPanics(t, func() {
		runner.OnMemoUpdated(ctx, memo)
	}, "nil Runner 的 OnMemoUpdated 不应 panic")
}

func TestRunner_OnMemoUpdated_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()
	memo := &store.Memo{UID: "test-uid", CreatorID: 1}

	assert.NotPanics(t, func() {
		runner.OnMemoUpdated(ctx, memo)
	}, "orchestrator 为 nil 时 OnMemoUpdated 不应 panic")
}

func TestRunner_OnMemoDeleted_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()

	assert.NotPanics(t, func() {
		runner.OnMemoDeleted(ctx, "test-uid")
	}, "nil Runner 的 OnMemoDeleted 不应 panic")
}

func TestRunner_OnMemoDeleted_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()

	assert.NotPanics(t, func() {
		runner.OnMemoDeleted(ctx, "test-uid")
	}, "orchestrator 为 nil 时 OnMemoDeleted 不应 panic")
}

func TestRunner_OnMemoVisibilityChanged_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()

	assert.NotPanics(t, func() {
		runner.OnMemoVisibilityChanged(ctx, "test-uid", store.Public)
	}, "nil Runner 的 OnMemoVisibilityChanged 不应 panic")
}

func TestRunner_OnMemoVisibilityChanged_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()

	assert.NotPanics(t, func() {
		runner.OnMemoVisibilityChanged(ctx, "test-uid", store.Public)
	}, "orchestrator 为 nil 时 OnMemoVisibilityChanged 不应 panic")
}

func TestRunner_OnAttachmentCreated_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()
	attachment := &store.Attachment{UID: "test-uid", CreatorID: 1}

	assert.NotPanics(t, func() {
		runner.OnAttachmentCreated(ctx, attachment)
	}, "nil Runner 的 OnAttachmentCreated 不应 panic")
}

func TestRunner_OnAttachmentCreated_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()
	attachment := &store.Attachment{UID: "test-uid", CreatorID: 1}

	assert.NotPanics(t, func() {
		runner.OnAttachmentCreated(ctx, attachment)
	}, "orchestrator 为 nil 时 OnAttachmentCreated 不应 panic")
}

func TestRunner_OnAttachmentDeleted_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()

	assert.NotPanics(t, func() {
		runner.OnAttachmentDeleted(ctx, "test-uid")
	}, "nil Runner 的 OnAttachmentDeleted 不应 panic")
}

func TestRunner_OnAttachmentDeleted_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()

	assert.NotPanics(t, func() {
		runner.OnAttachmentDeleted(ctx, "test-uid")
	}, "orchestrator 为 nil 时 OnAttachmentDeleted 不应 panic")
}

func TestRunner_OnUserDeleted_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()

	assert.NotPanics(t, func() {
		runner.OnUserDeleted(ctx, 1)
	}, "nil Runner 的 OnUserDeleted 不应 panic")
}

func TestRunner_OnUserDeleted_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()

	assert.NotPanics(t, func() {
		runner.OnUserDeleted(ctx, 1)
	}, "orchestrator 为 nil 时 OnUserDeleted 不应 panic")
}

// ==================== 状态查询测试 ====================

func TestRunner_GetHealthStatus_NilRunner(t *testing.T) {
	var runner *Runner
	status := runner.GetHealthStatus()
	assert.Nil(t, status, "nil Runner 的 GetHealthStatus 应返回 nil")
}

func TestRunner_GetHealthStatus_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	status := runner.GetHealthStatus()
	assert.Nil(t, status, "orchestrator 为 nil 时 GetHealthStatus 应返回 nil")
}

func TestRunner_GetSyncStats_NilRunner(t *testing.T) {
	var runner *Runner
	ctx := context.Background()

	stats, err := runner.GetSyncStats(ctx)
	assert.NoError(t, err, "nil Runner 的 GetSyncStats 应返回 nil error")
	assert.Nil(t, stats, "nil Runner 的 GetSyncStats 应返回 nil stats")
}

func TestRunner_GetSyncStats_NilOrchestrator(t *testing.T) {
	runner := &Runner{orchestrator: nil}
	ctx := context.Background()

	stats, err := runner.GetSyncStats(ctx)
	assert.NoError(t, err, "orchestrator 为 nil 时 GetSyncStats 应返回 nil error")
	assert.Nil(t, stats, "orchestrator 为 nil 时 GetSyncStats 应返回 nil stats")
}

func TestRunner_GetOrchestrator_NilRunner(t *testing.T) {
	var runner *Runner
	orch := runner.GetOrchestrator()
	assert.Nil(t, orch, "nil Runner 的 GetOrchestrator 应返回 nil")
}

func TestRunner_GetOrchestrator_ValidRunner(t *testing.T) {
	config := &ragflow.Config{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-api-key",
	}
	runner := NewRunner(nil, config)

	orch := runner.GetOrchestrator()
	assert.NotNil(t, orch, "有效 Runner 的 GetOrchestrator 应返回非 nil")
}

// ==================== 并发安全测试 ====================

func TestRunner_ConcurrentEventHandling(t *testing.T) {
	// 测试并发调用事件处理方法的安全性
	runner := &Runner{
		orchestrator: nil, // nil orchestrator 使方法直接返回
		wg:           sync.WaitGroup{},
	}

	ctx := context.Background()
	memo := &store.Memo{UID: "test-uid", CreatorID: 1}
	attachment := &store.Attachment{UID: "test-uid", CreatorID: 1}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(7)
		go func() {
			defer wg.Done()
			runner.OnMemoCreated(ctx, memo)
		}()
		go func() {
			defer wg.Done()
			runner.OnMemoUpdated(ctx, memo)
		}()
		go func() {
			defer wg.Done()
			runner.OnMemoDeleted(ctx, "test-uid")
		}()
		go func() {
			defer wg.Done()
			runner.OnMemoVisibilityChanged(ctx, "test-uid", store.Public)
		}()
		go func() {
			defer wg.Done()
			runner.OnAttachmentCreated(ctx, attachment)
		}()
		go func() {
			defer wg.Done()
			runner.OnAttachmentDeleted(ctx, "test-uid")
		}()
		go func() {
			defer wg.Done()
			runner.OnUserDeleted(ctx, 1)
		}()
	}

	// 等待所有 goroutine 完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 成功
	case <-time.After(5 * time.Second):
		t.Fatal("并发测试超时")
	}
}

func TestRunner_ConcurrentStatusQueries(t *testing.T) {
	// 使用 nil orchestrator 的 runner，避免调用实际的 store
	runner := &Runner{
		orchestrator: nil,
	}
	runner.running.Store(false)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			runner.IsRunning()
		}()
		go func() {
			defer wg.Done()
			runner.GetHealthStatus()
		}()
		go func() {
			defer wg.Done()
			_, _ = runner.GetSyncStats(ctx)
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 成功
	case <-time.After(5 * time.Second):
		t.Fatal("并发状态查询测试超时")
	}
}
