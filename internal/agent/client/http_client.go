package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/platform/models"
)

// HTTPClient HTTP客户端实现
type HTTPClient struct {
	config     *config.AgentConfig
	logger     *logrus.Logger
	httpClient *http.Client
	baseURL    string
}

// NewHTTPClient 创建HTTP客户端
func NewHTTPClient(cfg *config.AgentConfig, logger *logrus.Logger) (*HTTPClient, error) {
	// 解析基础URL
	baseURL, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("解析服务器URL失败: %w", err)
	}
	
	// 创建HTTP传输层
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 5,
	}
	
	// 配置TLS
	if cfg.TLSEnabled {
		tlsConfig, err := createTLSConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("创建TLS配置失败: %w", err)
		}
		transport.TLSClientConfig = tlsConfig
	}
	
	// 创建HTTP客户端
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.RequestTimeout,
	}
	
	client := &HTTPClient{
		config:     cfg,
		logger:     logger,
		httpClient: httpClient,
		baseURL:    baseURL.String(),
	}
	
	return client, nil
}

// Register 注册Agent
func (c *HTTPClient) Register(ctx context.Context, agent *models.Agent) error {
	c.logger.Debug("发送注册请求")
	
	// 构建请求
	req := map[string]interface{}{
		"agent_id":         agent.AgentID,
		"hostname":         agent.Hostname,
		"ip":               agent.IP,
		"logstash_version": agent.LogstashVersion,
	}
	
	// 发送POST请求
	resp, err := c.doRequest(ctx, "POST", "/api/v1/agents/register", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("注册失败: %s - %s", resp.Status, string(body))
	}
	
	return nil
}

// SendHeartbeat 发送心跳
func (c *HTTPClient) SendHeartbeat(ctx context.Context, agentID string) error {
	c.logger.Debug("发送心跳")
	
	// 构建请求
	req := map[string]interface{}{
		"timestamp": time.Now().Unix(),
	}
	
	// 发送POST请求
	path := fmt.Sprintf("/api/v1/agents/%s/heartbeat", agentID)
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("心跳失败: %s - %s", resp.Status, string(body))
	}
	
	return nil
}

// ReportStatus 上报状态
func (c *HTTPClient) ReportStatus(ctx context.Context, agent *models.Agent) error {
	c.logger.Debug("上报状态")
	
	// 发送PUT请求
	path := fmt.Sprintf("/api/v1/agents/%s/status", agent.AgentID)
	resp, err := c.doRequest(ctx, "PUT", path, agent)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("状态上报失败: %s - %s", resp.Status, string(body))
	}
	
	return nil
}

// GetConfig 获取配置
func (c *HTTPClient) GetConfig(ctx context.Context, configID string) (*models.Config, error) {
	c.logger.WithField("config_id", configID).Debug("获取配置")
	
	// 发送GET请求
	path := fmt.Sprintf("/api/v1/configs/%s", configID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取配置失败: %s - %s", resp.Status, string(body))
	}
	
	// 解析响应
	var config models.Config
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("解析配置响应失败: %w", err)
	}
	
	return &config, nil
}

// ReportConfigApplied 上报配置应用结果
func (c *HTTPClient) ReportConfigApplied(ctx context.Context, agentID string, applied *models.AppliedConfig) error {
	c.logger.WithField("config_id", applied.ConfigID).Debug("上报配置应用结果")
	
	// 构建请求
	req := map[string]interface{}{
		"config_id":  applied.ConfigID,
		"version":    applied.Version,
		"applied_at": applied.AppliedAt,
		"status":     "success",
	}
	
	// 发送POST请求
	path := fmt.Sprintf("/api/v1/agents/%s/configs", agentID)
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("上报配置应用结果失败: %s - %s", resp.Status, string(body))
	}
	
	return nil
}

// ReportMetrics 上报指标
func (c *HTTPClient) ReportMetrics(ctx context.Context, agentID string, metrics interface{}) error {
	c.logger.Debug("上报指标")
	
	// 发送POST请求
	path := fmt.Sprintf("/api/v1/agents/%s/metrics", agentID)
	resp, err := c.doRequest(ctx, "POST", path, metrics)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("上报指标失败: %s - %s", resp.Status, string(body))
	}
	
	return nil
}

// doRequest 执行HTTP请求
func (c *HTTPClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	// 构建完整URL
	fullURL := c.baseURL + path
	
	// 准备请求体
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}
	
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	
	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("LogstashAgent/%s", c.config.AgentID))
	
	// 设置认证
	if c.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.Token)
	}
	
	// 记录请求
	c.logger.WithFields(logrus.Fields{
		"method": method,
		"url":    fullURL,
	}).Debug("发送HTTP请求")
	
	// 执行请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行请求失败: %w", err)
	}
	
	// 记录响应
	c.logger.WithFields(logrus.Fields{
		"status": resp.StatusCode,
		"url":    fullURL,
	}).Debug("收到HTTP响应")
	
	return resp, nil
}

// Close 关闭客户端
func (c *HTTPClient) Close() error {
	// HTTP客户端不需要特殊的关闭操作
	return nil
}

// createTLSConfig 创建TLS配置
func createTLSConfig(cfg *config.AgentConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.TLSSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}
	
	// 加载客户端证书
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载客户端证书失败: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	
	// 加载CA证书
	if cfg.TLSCAFile != "" && !cfg.TLSSkipVerify {
		caCert, err := ioutil.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("读取CA证书失败: %w", err)
		}
		
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("解析CA证书失败")
		}
		
		tlsConfig.RootCAs = caCertPool
	}
	
	return tlsConfig, nil
}