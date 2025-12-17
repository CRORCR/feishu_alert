## feishu_alert 功能概述

飞书告警功能支持各种业务场景的通知，如panic，限流、资源不足、性能问题等。每种类型的告警独立限流，3分钟内只发送一次。

## 主要特性

- 🚀 **即开即用**: 简单的API调用，快速集成
- 🎯 **按类型限流**: 每种告警类型独立3分钟限流
- 📊 **丰富信息**: 支持标题、描述、指标、严重程度等信息
- 🎨 **可视化**: 根据严重程度显示不同图标
- 🔄 **单例模式**: 支持全局单例，便于统一管理

## 支持的告警类型

| 类型 | 常量 | 说明 |
|------|------|------|
| 限流告警 | `AlertTypeRateLimit` | 接口达到限流阈值 |
| 短信额度超限 | `AlertTypeSMSQuota` | 短信发送额度不足 |
| 慢请求 | `AlertTypeSlowRequest` | 响应时间超过1秒 |
| 错误率过高 | `AlertTypeHighError` | 接口错误率异常 |
| 资源不足 | `AlertTypeResource` | CPU、内存等资源不足 |
| 自定义 | `AlertTypeCustom` | 自定义业务场景 |

## 严重程度

- `critical`: 🚨 严重影响，需要立即处理
- `high`: ⚠️ 高优先级，需要尽快处理
- `medium`: ⚡ 中等优先级，需要关注
- `low`: ℹ️ 低优先级，仅供参考

## 快速开始

### 1. 初始化

```go
import "your-project/middleware"

// 方式1: 使用全局单例
collector := middleware.GetGlobalBusinessAlertCollector("", true) // 使用默认webhook

// 方式2: 创建独立实例
collector := middleware.NewFeishuBusinessAlertCollector("your-webhook-url", true)
```

### 2. 发送告警

#### 便捷方法
```go
// 限流告警
middleware.SendBusinessAlert(
    middleware.AlertTypeRateLimit,
    "接口限流触发",
    "用户登录接口达到限流阈值",
    "user-service",
    "Login",
    "medium",
    map[string]interface{}{
        "限流阈值": "100req/min",
        "当前QPS": "120",
        "持续时间": "30s",
    },
)
```

#### 直接使用收集器
```go
alert := middleware.BusinessAlert{
    Type:        middleware.AlertTypeSlowRequest,
    Title:       "慢请求检测",
    Description: "数据库查询响应时间超过阈值",
    Service:     "order-service",
    Method:      "GetOrderById",
    Severity:    "medium",
    Metrics: map[string]interface{}{
        "响应时间": "1.5s",
        "阈值": "1.0s",
        "数据库": "MySQL",
    },
}
collector.Collect(alert)
```

## 实际业务场景示例

### 限流监控
```go
func checkRateLimit(service, method string, currentQPS int, threshold int) {
    if currentQPS > threshold {
        middleware.SendBusinessAlert(
            middleware.AlertTypeRateLimit,
            "服务触发限流保护",
            fmt.Sprintf("%s 服务的 %s 方法触发限流", service, method),
            service,
            method,
            "medium",
            map[string]interface{}{
                "当前QPS": currentQPS,
                "限流阈值": threshold,
            },
        )
    }
}
```

### 短信额度监控
```go
func checkSMSQuota(remainingQuota, totalQuota int) {
    usageRate := float64(totalQuota-remainingQuota) / float64(totalQuota) * 100
    if usageRate > 90 {
        severity := "high"
        if usageRate > 95 {
            severity = "critical"
        }

        middleware.SendBusinessAlert(
            middleware.AlertTypeSMSQuota,
            "短信额度使用率过高",
            fmt.Sprintf("短信额度使用率已达到%.1f%%", usageRate),
            "notification-service",
            "CheckSMSQuota",
            severity,
            map[string]interface{}{
                "使用率": fmt.Sprintf("%.1f%%", usageRate),
                "剩余额度": remainingQuota,
            },
        )
    }
}
```

### 慢请求监控
```go
func monitorSlowRequest(service, method string, duration time.Duration) {
    if duration > time.Second {
        middleware.SendBusinessAlert(
            middleware.AlertTypeSlowRequest,
            "检测到慢请求",
            fmt.Sprintf("%s 服务的 %s 方法响应时间过长", service, method),
            service,
            method,
            "medium",
            map[string]interface{}{
                "响应时间": duration.String(),
                "阈值": "1s",
            },
        )
    }
}
```

## 消息格式示例

发送的业务告警在飞书中会显示为：

```
🚨 业务告警

时间: 2024-12-17 15:30:45
类型: 限流
标题: 接口限流触发
服务: user-service
严重程度: medium
方法: Login
指标: 限流阈值: 100req/min, 当前QPS: 120, 持续时间: 30s

详细描述:
用户登录接口达到限流阈值，可能影响用户体验
```

## 注意事项

1. **限流机制**: 每种告警类型独立限流，3分钟内重复发送会被丢弃
2. **初始化**: 使用便捷方法前需要先调用 `GetGlobalBusinessAlertCollector` 初始化
3. **Webhook URL**: 可以使用默认URL或配置自定义的飞书webhook
4. **指标格式**: metrics中的value可以是任意类型，会自动转换为字符串显示
5. **线程安全**: 所有方法都是线程安全的，可以在goroutine中安全调用

## 配置建议

1. **开发环境**: 设置 `isProd = false`，便于区分测试和生产告警
2. **Webhook配置**: 建议为不同环境配置不同的飞书群组
3. **告警级别**: 合理设置告警严重程度，避免告警疲劳
4. **指标信息**: 提供足够详细的指标信息，便于快速定位问题
