package strategy

import (
	"context"
	"gateway/internal/pkg"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"strings"
	"testing"
	"time"
)

func TestPrometheusStrategy(t *testing.T) {
	// 创建模拟配置
	config := &pkg.Config{
		Strategy: []pkg.StrategyConfig{
			{
				Enable: true,
				Type:   "prometheus",
				Para: map[string]interface{}{
					"port":     8080,
					"endpoint": "/metrics",
				},
			},
		},
	}

	// 创建模拟的 context 和 logger
	ctx := pkg.WithConfig(context.Background(), config)
	ctx = pkg.WithLogger(ctx, logger)

	// 创建一个 PrometheusStrategy
	strategy, err := NewPrometheusStrategy(ctx)
	if err != nil {
		t.Fatalf("Failed to create PrometheusStrategy: %v", err)
	}
	promStrategy := strategy.(*PrometheusStrategy)
	// 创建模拟的 Point 数据
	point := pkg.Point{
		DeviceType: "testDevice",
		DeviceName: "testName",
		Field: map[string]interface{}{
			"temperature": 24.5,     // float64 类型
			"status":      "online", // string 类型
		},
		Ts: time.Now(),
	}

	// 调用 Publish 方法
	err = promStrategy.Publish(point)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// 使用 testutil 来验证 Prometheus 指标
	expectedMetric := `
	# HELP testDevice_testName_fields Metrics for testDevice testName
	# TYPE testDevice_testName_fields gauge
	testDevice_testName_fields{field="temperature"} 24.5
	`
	if err := testutil.CollectAndCompare(promStrategy.metrics["testDevice_testName_fields"], strings.NewReader(expectedMetric)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	// 验证其他字段没有注册或记录错误
	if _, exists := promStrategy.metrics["status"]; exists {
		t.Errorf("status field should not be registered as a metric")
	}
}
