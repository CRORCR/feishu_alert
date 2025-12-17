package feishu_alert

import (
	"fmt"
	"time"
)

// ExampleUsage 展示如何使用业务告警功能
func ExampleUsage() {
	// 1. 初始化业务告警收集器
	webhookURL := "" // 使用默认URL或设置自定义URL
	isProd := true
	collector := GetGlobalBusinessAlertCollector(webhookURL, isProd)

	// 或者创建独立的收集器实例
	// collector := NewFeishuBusinessAlertCollector(webhookURL, isProd)

	// 2. 限流告警示例
	SendBusinessAlert(
		AlertTypeRateLimit,
		"接口限流触发",
		"用户登录接口达到限流阈值",
		"user-service",
		"Login",
		"medium",
		map[string]interface{}{
			"限流阈值":  "100req/min",
			"当前QPS": "120",
			"持续时间":  "30s",
		},
	)

	// 3. 短信额度超限告警示例
	SendBusinessAlert(
		AlertTypeSMSQuota,
		"短信额度不足",
		"短信发送额度即将用完，请及时充值",
		"notification-service",
		"SendSMS",
		"high",
		map[string]interface{}{
			"剩余额度": "50",
			"总额度":  "10000",
			"已使用":  "9950",
		},
	)

	// 4. 慢请求告警示例
	SendBusinessAlert(
		AlertTypeSlowRequest,
		"慢请求检测",
		"数据库查询响应时间超过阈值",
		"order-service",
		"GetOrderById",
		"medium",
		map[string]interface{}{
			"响应时间": "1.5s",
			"阈值":   "1.0s",
			"数据库":  "MySQL",
		},
	)

	// 5. 错误率过高告警示例
	SendBusinessAlert(
		AlertTypeHighError,
		"错误率过高",
		"支付接口错误率异常升高",
		"payment-service",
		"ProcessPayment",
		"critical",
		map[string]interface{}{
			"错误率":   "15%",
			"阈值":    "5%",
			"总请求数":  "1000",
			"错误请求数": "150",
		},
	)

	// 6. 自定义告警示例
	SendBusinessAlert(
		AlertTypeCustom,
		"自定义业务监控",
		"用户注册量突然下降",
		"user-service",
		"UserRegistration",
		"high",
		map[string]interface{}{
			"当前注册率": "10/min",
			"正常水平":  "50/min",
			"下降幅度":  "80%",
			"持续时间":  "5min",
		},
	)

	// 7. 直接使用收集器实例发送告警
	alert := BusinessAlert{
		Type:        AlertTypeResource,
		Title:       "CPU使用率过高",
		Description: "服务器CPU使用率达到85%，可能影响服务性能",
		Service:     "api-gateway",
		Method:      "ProxyRequest",
		Severity:    "high",
		Metrics: map[string]interface{}{
			"CPU使用率": "85%",
			"内存使用率":  "60%",
			"服务器IP":  "10.0.1.100",
		},
	}
	collector.Collect(alert)

	// 模拟重复告警（会被限流）
	for i := 0; i < 5; i++ {
		SendBusinessAlert(
			AlertTypeRateLimit,
			"重复的限流告警",
			"这个告警应该被限流",
			"test-service",
			"TestMethod",
			"low",
			map[string]interface{}{
				"计数": i + 1,
			},
		)
		time.Sleep(1 * time.Second) // 只有第一个会发送，后续的被限流
	}
}

// 在实际业务代码中的使用示例

// RateLimitExample 限流业务场景
func RateLimitExample(service, method string, currentQPS int, threshold int) {
	if currentQPS > threshold {
		SendBusinessAlert(
			AlertTypeRateLimit,
			"服务触发限流保护",
			fmt.Sprintf("%s 服务的 %s 方法触发限流，当前QPS: %d，阈值: %d", service, method, currentQPS, threshold),
			service,
			method,
			"medium",
			map[string]interface{}{
				"当前QPS": currentQPS,
				"限流阈值":  threshold,
				"限流策略":  "滑动窗口",
			},
		)
	}
}

// SMSQuotaExample 短信额度监控
func SMSQuotaExample(remainingQuota, totalQuota int) {
	usageRate := float64(totalQuota-remainingQuota) / float64(totalQuota) * 100
	if usageRate > 90 { // 使用率超过90%时告警
		severity := "high"
		if usageRate > 95 {
			severity = "critical"
		}

		SendBusinessAlert(
			AlertTypeSMSQuota,
			"短信额度使用率过高",
			fmt.Sprintf("短信额度使用率已达到%.1f%%，请及时充值", usageRate),
			"notification-service",
			"CheckSMSQuota",
			severity,
			map[string]interface{}{
				"使用率":  fmt.Sprintf("%.1f%%", usageRate),
				"剩余额度": remainingQuota,
				"总额度":  totalQuota,
			},
		)
	}
}

// SlowRequestExample 慢请求监控
func SlowRequestExample(service, method string, duration time.Duration, threshold time.Duration) {
	if duration > threshold {
		SendBusinessAlert(
			AlertTypeSlowRequest,
			"检测到慢请求",
			fmt.Sprintf("%s 服务的 %s 方法响应时间过长", service, method),
			service,
			method,
			"medium",
			map[string]interface{}{
				"响应时间": duration.String(),
				"阈值":   threshold.String(),
				"超时倍数": float64(duration) / float64(threshold),
			},
		)
	}
}
