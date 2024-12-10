package strategy

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"net/http"
)

// 初始化函数，注册 Prometheus 策略
func init() {
	// 注册 Prometheus 发送策略
	Register("prometheus", NewPrometheusStrategy)
}

// PrometheusStrategy 实现将数据发布到 Prometheus 的逻辑
type PrometheusStrategy struct {
	info    PrometheusInfo
	ctx     context.Context
	logger  *zap.Logger
	metrics map[string]*prometheus.GaugeVec // 使用 Prometheus 的 GaugeVec 作为指标类型
}

// PrometheusInfo Prometheus 的专属配置
type PrometheusInfo struct {
	Port     int    `mapstructure:"port"`
	Endpoint string `mapstructure:"endpoint"`
}

// NewPrometheusStrategy Step.0 构造函数
func NewPrometheusStrategy(ctx context.Context) (Template, error) {
	log := pkg.LoggerFromContext(ctx)
	config := pkg.ConfigFromContext(ctx)
	var info PrometheusInfo
	for _, strategyConfig := range config.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "prometheus" {
			// 将 map 转换为结构体
			if err := mapstructure.Decode(strategyConfig.Para, &info); err != nil {
				log.Error("Error decoding map to struct", zap.Error(err))
				return nil, fmt.Errorf("[NewPrometheusStrategy] Error decoding map to struct: %v", err)
			}
		}
	}

	// 创建 Prometheus 指标集合
	metrics := make(map[string]*prometheus.GaugeVec)

	// 启动 HTTP 服务器，暴露 Prometheus 指标
	http.Handle(info.Endpoint, promhttp.Handler())
	go func() {
		log.Info("Starting Prometheus HTTP server", zap.Int("port", info.Port), zap.String("endpoint", info.Endpoint))
		if err := http.ListenAndServe(fmt.Sprintf(":%d", info.Port), nil); err != nil {
			log.Error("Prometheus HTTP server failed to start", zap.Error(err))
		}
	}()

	return &PrometheusStrategy{
		info:    info,
		ctx:     ctx,
		logger:  log,
		metrics: metrics,
	}, nil
}

// GetCore Step.1
func (p *PrometheusStrategy) GetType() string {
	return "prometheus"
}

// Start Step.2
func (p *PrometheusStrategy) Start(pointChan chan pkg.Point) {
	p.logger.Info("===PrometheusStrategy started===")

	for {
		select {
		case <-p.ctx.Done():
			p.Stop()
			pkg.LoggerFromContext(p.ctx).Info("===PrometheusStrategy stopped===")
		case point := <-pointChan:
			err := p.Publish(point)
			if err != nil {
				pkg.ErrChanFromContext(p.ctx) <- fmt.Errorf("PrometheusStrategy error occurred: %w", err)
			}
		}
	}
}

// Publish 将数据发布到 Prometheus
func (p *PrometheusStrategy) Publish(point pkg.Point) error {
	// 创建或更新指标
	metricName := fmt.Sprintf("%s_fields", point.Device)
	gauge, exists := p.metrics[metricName]
	if !exists {
		gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: fmt.Sprintf("Metrics for %s", point.Device),
			},
			[]string{"field"},
		)
		// 注册指标
		if err := prometheus.Register(gauge); err != nil {
			p.logger.Error("Failed to register Prometheus metric", zap.Error(err))
			return fmt.Errorf("注册 Prometheus 指标失败: %+v", err)
		}
		p.metrics[metricName] = gauge
	}

	// 设置指标值
	for key, valuePtr := range point.Field {
		if valuePtr == nil {
			continue // 跳过 nil 值
		}
		// 使用类型断言和 type switch 来处理不同类型
		switch value := valuePtr.(type) {
		case float64:
			// 如果是 float64 类型，更新 Prometheus 指标
			gauge.With(prometheus.Labels{"field": key}).Set(value)
		case string:
			// 如果是 string 类型，可以根据业务需求进行处理，Prometheus 通常只接受数值类型
			// 这里我们打印日志，如果需要其他处理方式（如转换为数值），也可以实现
			p.logger.Warn("Received string type, but Prometheus expects numerical values", zap.String("field", key), zap.String("value", value))
		default:
			// 其他类型也可以根据需要进行处理
			p.logger.Warn("Unsupported data type for Prometheus metrics", zap.String("field", key), zap.Any("value", value))
		}
	}
	p.logger.Debug("[PrometheusStrategy] 发布指标", zap.String("metricName", metricName))
	return nil
}

// Stop 停止 PrometheusStrategy
func (p *PrometheusStrategy) Stop() {
	p.logger.Info("Stopping PrometheusStrategy")
}
