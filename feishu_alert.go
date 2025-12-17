package feishu_alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// todo 后面考虑对不同对错误码做监控，超过一定比例错误（错误/正常请求>0.1），比如10%，就报警。对于常见错误码，再加白名单

type PanicInfo struct {
	Method     string
	PanicValue interface{}
	Stack      string
}

type HTTPPanicInfo struct {
	Method     string
	URL        string
	RemoteAddr string
	PanicValue interface{}
	Stack      string
}

// DefaultFeishuWebhookURL 默认的飞书 webhook URL
const DefaultFeishuWebhookURL = "https://open.feishu.cn/open-apis/bot/v2/hook/040742e7-0e22-43ce"

// BusinessAlertType 业务告警类型
type BusinessAlertType string

const (
	AlertTypeRateLimit   BusinessAlertType = "限流"     // 限流告警
	AlertTypeSMSQuota    BusinessAlertType = "短信额度超限" // 短信额度超限
	AlertTypeSlowRequest BusinessAlertType = "慢请求"    // 超时时间超过1s
	AlertTypeHighError   BusinessAlertType = "错误率过高"  // 错误率过高
	AlertTypeResource    BusinessAlertType = "资源不足"   // 资源不足
	AlertTypeCustom      BusinessAlertType = "自定义"    // 自定义告警
)

// BusinessAlert 业务告警信息
type BusinessAlert struct {
	Type        BusinessAlertType      `json:"type"`
	Title       string                 `json:"title"`       // 告警标题
	Description string                 `json:"description"` // 详细描述
	Service     string                 `json:"service"`     // 服务名称
	Method      string                 `json:"method"`      // 相关方法或接口
	Metrics     map[string]interface{} `json:"metrics"`     // 相关指标
	Severity    string                 `json:"severity"`    // 严重程度: low, medium, high, critical
}

// FeishuAlertCollector 飞书告警 3分钟内只发送1条告警
type FeishuAlertCollector struct {
	webhookURL string
	mu         sync.Mutex
	lastSent   time.Time
	interval   time.Duration
	isProd     bool
}

// FeishuBusinessAlertCollector 飞书业务告警收集器
type FeishuBusinessAlertCollector struct {
	webhookURL string
	mu         sync.RWMutex
	lastSent   map[BusinessAlertType]time.Time // 按类型记录最后发送时间
	interval   time.Duration
	isProd     bool
}

// NewFeishuAlertCollector 创建飞书告警
func NewFeishuAlertCollector(webhookURL string, isProd bool) *FeishuAlertCollector {
	if webhookURL == "" {
		webhookURL = DefaultFeishuWebhookURL
	}

	return &FeishuAlertCollector{
		webhookURL: webhookURL,
		interval:   3 * time.Minute, // 3分钟限流
		isProd:     isProd,
	}
}

// NewFeishuBusinessAlertCollector 创建飞书业务告警收集器
func NewFeishuBusinessAlertCollector(webhookURL string, isProd bool) *FeishuBusinessAlertCollector {
	if webhookURL == "" {
		webhookURL = DefaultFeishuWebhookURL
	}

	return &FeishuBusinessAlertCollector{
		webhookURL: webhookURL,
		lastSent:   make(map[BusinessAlertType]time.Time),
		interval:   3 * time.Minute, // 3分钟限流
		isProd:     isProd,
	}
}

// Collect 收集 RPC panic 信息并发送飞书告警
func (c *FeishuAlertCollector) Collect(info PanicInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否在限流时间内
	if time.Since(c.lastSent) < c.interval {
		logx.Infof("丢弃此次告警，上次发送时间: %s", c.lastSent.Format("2006-01-02 15:04:05"))
		return
	}

	message := c.buildRPCMessage(info)

	// 发送告警
	if err := c.sendToFeishu(message); err != nil {
		logx.Errorf("发送飞书告警失败: %v", err)
		return
	}

	// 更新最后发送时间
	c.lastSent = time.Now()
}

// buildRPCMessage 构建 RPC panic 消息
func (c *FeishuAlertCollector) buildRPCMessage(info PanicInfo) FeishuMessage {
	// 截取堆栈信息（避免消息过长）
	//stack := info.Stack
	//if len(stack) > 500 {
	//	stack = stack[:500] + "\n... (堆栈过长，已截断)"
	//}

	content := fmt.Sprintf(
		"**🚨 RPC Panic 告警**\n\n"+
			"**时间**: %s\n"+
			"**生产环境**: %t\n"+
			"**方法**: %s\n"+
			"**错误**: %v\n\n",
		//"**堆栈跟踪**:\n```\n%s\n```",
		time.Now().Format("2006-01-02 15:04:05"),
		c.isProd,
		info.Method,
		info.PanicValue,
		//stack,
	)

	return FeishuMessage{
		MsgType: "text",
		Content: FeishuContent{
			Text: content,
		},
	}
}

// FeishuHTTPAlertCollector 飞书 HTTP 告警
type FeishuHTTPAlertCollector struct {
	*FeishuAlertCollector
}

// NewFeishuHTTPAlertCollector 创建飞书 HTTP 告警
func NewFeishuHTTPAlertCollector(webhookURL string, isProd bool) *FeishuHTTPAlertCollector {
	return &FeishuHTTPAlertCollector{
		FeishuAlertCollector: NewFeishuAlertCollector(webhookURL, isProd),
	}
}

// Collect 收集 HTTP panic 信息并发送飞书告警
func (c *FeishuHTTPAlertCollector) Collect(info HTTPPanicInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否在限流时间内
	if time.Since(c.lastSent) < c.interval {
		logx.Infof("丢弃此次告警，上次发送时间: %s", c.lastSent.Format("2006-01-02 15:04:05"))
		return
	}

	// 构建飞书消息
	message := c.buildHTTPMessage(info)

	// 发送告警
	if err := c.sendToFeishu(message); err != nil {
		logx.Errorf("发送飞书告警失败: %v", err)
		return
	}

	c.lastSent = time.Now()
}

// buildHTTPMessage 构建 HTTP panic 消息
func (c *FeishuHTTPAlertCollector) buildHTTPMessage(info HTTPPanicInfo) FeishuMessage {
	// 截取堆栈信息（避免消息过长）
	stack := info.Stack
	if len(stack) > 500 {
		stack = stack[:500] + "\n... (堆栈过长，已截断)"
	}

	content := fmt.Sprintf(
		"**🚨 HTTP Panic 告警**\n\n"+
			"**时间**: %s\n"+
			"**请求**: %s %s\n"+
			"**客户端**: %s\n"+
			"**错误**: %v\n\n"+
			"**堆栈跟踪**:\n```\n%s\n```",
		time.Now().Format("2006-01-02 15:04:05"),
		info.Method,
		info.URL,
		info.RemoteAddr,
		info.PanicValue,
		stack,
	)

	return FeishuMessage{
		MsgType: "text",
		Content: FeishuContent{
			Text: content,
		},
	}
}

// sendToFeishu 发送消息到飞书
func (c *FeishuAlertCollector) sendToFeishu(message FeishuMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	resp, err := http.Post(c.webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("飞书返回错误状态码: %d", resp.StatusCode)
	}

	var result FeishuResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("飞书返回错误: code=%d, msg=%s", result.Code, result.Msg)
	}

	return nil
}

type FeishuMessage struct {
	MsgType string        `json:"msg_type"`
	Content FeishuContent `json:"content"`
}

type FeishuContent struct {
	Text string `json:"text"`
}

type FeishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Collect 收集业务告警信息并发送飞书通知
func (c *FeishuBusinessAlertCollector) Collect(alert BusinessAlert) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否在限流时间内
	if lastSent, exists := c.lastSent[alert.Type]; exists {
		if time.Since(lastSent) < c.interval {
			logx.Infof("丢弃业务告警，类型: %s, 上次发送时间: %s", alert.Type, lastSent.Format("2006-01-02 15:04:05"))
			return
		}
	}

	// 构建飞书消息
	message := c.buildBusinessMessage(alert)

	// 发送告警
	if err := c.sendToFeishu(message); err != nil {
		logx.Errorf("发送飞书业务告警失败: %v", err)
		return
	}

	// 更新最后发送时间
	c.lastSent[alert.Type] = time.Now()
	logx.Infof("业务告警已发送，类型: %s", alert.Type)
}

// buildBusinessMessage 构建业务告警消息
func (c *FeishuBusinessAlertCollector) buildBusinessMessage(alert BusinessAlert) FeishuMessage {
	// 获取严重程度图标
	severityIcon := c.getSeverityIcon(alert.Severity)

	// 构建指标信息
	var metricsInfo string
	if len(alert.Metrics) > 0 {
		var metrics []string
		for key, value := range alert.Metrics {
			metrics = append(metrics, fmt.Sprintf("**%s**: %v", key, value))
		}
		metricsInfo = "**指标**: " + strings.Join(metrics, ", ") + "\n"
	}

	content := fmt.Sprintf(
		"**%s 业务告警**\n\n"+
			"**时间**: %s\n"+
			"**类型**: %s\n"+
			"**标题**: %s\n"+
			"**服务**: %s\n"+
			"**严重程度**: %s\n",
		severityIcon,
		time.Now().Format("2006-01-02 15:04:05"),
		alert.Type,
		alert.Title,
		alert.Service,
		alert.Severity,
	)

	// 添加可选信息
	if alert.Method != "" {
		content += fmt.Sprintf("**方法**: %s\n", alert.Method)
	}

	if metricsInfo != "" {
		content += metricsInfo
	}

	if alert.Description != "" {
		content += fmt.Sprintf("\n**详细描述**:\n%s", alert.Description)
	}

	return FeishuMessage{
		MsgType: "text",
		Content: FeishuContent{
			Text: content,
		},
	}
}

// getSeverityIcon 获取严重程度图标
func (c *FeishuBusinessAlertCollector) getSeverityIcon(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "🚨"
	case "high":
		return "⚠️"
	case "medium":
		return "⚡"
	case "low":
		return "ℹ️"
	default:
		return "📢"
	}
}

// sendToFeishu 发送消息到飞书
func (c *FeishuBusinessAlertCollector) sendToFeishu(message FeishuMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	resp, err := http.Post(c.webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("飞书返回错误状态码: %d", resp.StatusCode)
	}

	var result FeishuResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("飞书返回错误: code=%d, msg=%s", result.Code, result.Msg)
	}

	return nil
}

// 全局业务告警收集器实例
var globalBusinessAlertCollector *FeishuBusinessAlertCollector
var businessAlertOnce sync.Once

// GetGlobalBusinessAlertCollector 获取全局业务告警收集器（单例）
func GetGlobalBusinessAlertCollector(webhookURL string, isProd bool) *FeishuBusinessAlertCollector {
	businessAlertOnce.Do(func() {
		globalBusinessAlertCollector = NewFeishuBusinessAlertCollector(webhookURL, isProd)
	})
	return globalBusinessAlertCollector
}

// SendBusinessAlert 发送业务告警（便捷方法）
func SendBusinessAlert(alertType BusinessAlertType, title, description, service, method, severity string, metrics map[string]interface{}) {
	if globalBusinessAlertCollector == nil {
		logx.Error("业务告警收集器未初始化，请先调用 GetGlobalBusinessAlertCollector")
		return
	}

	alert := BusinessAlert{
		Type:        alertType,
		Title:       title,
		Description: description,
		Service:     service,
		Method:      method,
		Metrics:     metrics,
		Severity:    severity,
	}

	globalBusinessAlertCollector.Collect(alert)
}
