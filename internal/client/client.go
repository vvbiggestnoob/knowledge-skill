/**
 * @date 2026/8/10
 * @description 实现 MaaS HTTP 客户端，使用 api-key 认证。
 * @since v1.0
 */

package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Config holds the client configuration.
type Config struct {
	BaseURL  string
	APIKey   string
	Timeout  time.Duration
	Insecure bool
}

// MaasClient is the HTTP client for MaaS platform APIs.
type MaasClient struct {
	config     Config
	httpClient *http.Client
}

// NewMaasClient creates a new MaaS API client.
func NewMaasClient(config Config) *MaasClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Insecure},
	}
	return &MaasClient{
		config: config,
		httpClient: &http.Client{
			Timeout:   config.Timeout,
			Transport: tr,
		},
	}
}

// Response wraps an HTTP response.
type Response struct {
	StatusCode int
	Body       []byte
}

// ParseJSON unmarshals the response body into the given value.
func (r *Response) ParseJSON(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// Get performs a GET request to the MaaS API.
func (c *MaasClient) Get(path string) (*Response, error) {
	return c.doJSON("GET", path, nil)
}

// Post performs a POST request to the MaaS API with a JSON body.
func (c *MaasClient) Post(path string, data interface{}) (*Response, error) {
	return c.doJSON("POST", path, data)
}

// Delete performs a DELETE request to the MaaS API.
func (c *MaasClient) Delete(path string) (*Response, error) {
	return c.doJSON("DELETE", path, nil)
}

// UploadFile uploads a file via multipart/form-data to the MaaS API.
// If knowledgeID is non-empty, the file is uploaded and imported into the knowledge base in one step.
func (c *MaasClient) UploadFile(path string, filePath string, knowledgeID string) (*Response, error) {
	fullURL := c.config.BaseURL + path

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", filePath, err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copy file content: %w", err)
	}

	if knowledgeID != "" {
		if err := writer.WriteField("knowledgeId", knowledgeID); err != nil {
			return nil, fmt.Errorf("write knowledgeId field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", fullURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("api-key", c.config.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Response{StatusCode: resp.StatusCode, Body: respBody},
			fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return &Response{StatusCode: resp.StatusCode, Body: respBody}, nil
}

func (c *MaasClient) doJSON(method, path string, data interface{}) (*Response, error) {
	fullURL := c.config.BaseURL + path

	var body io.Reader
	if data != nil {
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("api-key", c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Response{StatusCode: resp.StatusCode, Body: respBody},
			fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return &Response{StatusCode: resp.StatusCode, Body: respBody}, nil
}