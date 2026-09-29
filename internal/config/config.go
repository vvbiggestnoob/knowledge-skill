/**
 * @date 2026/8/11
 * @description 实现知识库配置文件加载，支持多工作空间（workspaces）结构。
 *              每个工作空间有独立的 api-key 和知识库列表，顶层通过 defaultKnowledgeId/Name 指定默认知识库。
 * @since v0.4.0
 */

package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the knowledge base configuration from JSON.
// Uses a multi-workspace structure where each workspace has its own api-key and knowledge bases.
type Config struct {
	DefaultKnowledgeId   string       `json:"defaultKnowledgeId"`
	DefaultKnowledgeName string       `json:"defaultKnowledgeName"`
	Workspaces            []Workspace `json:"workspaces"`
	// LLMAnalyzer configures the visual model used for image analysis during
	// document upload. It is optional: when absent, DefaultLLMAnalyzer() falls
	// back to built-in defaults, so the user does not need to provide it.
	LLMAnalyzer *LLMAnalyzer `json:"llmAnalyzer,omitempty"`
}

// LLMAnalyzer holds OpenAI-compatible settings for image analysis. The API key
// is deliberately NOT stored here; it reuses the knowledge-base workspace api-key
// (the same key can also call the visual model).
type LLMAnalyzer struct {
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	Prompt  string `json:"prompt,omitempty"`
}

// Default visual model values, sourced from HiDonalds/config.py.
const (
	DefaultLLMBaseURL = "https://maas.hikvision.com.cn/v1"
	DefaultLLMModel   = "Qwen3.6-35B-A3B-FP8"
	DefaultLLMPrompt  = "你是一个图片信息提取专家，首要任务是针对上传的数据，提取并总结其中的信息。" +
		"总结应该保持段落性总结，而不是分点。总结需要说明细节，需要详细总结。" +
		"非常重要！如果判定为聊天记录信息，你需要先输出具体的聊天内容，然后再输出总结。" +
		"请使用结构化的格式输出结果。"
)

// DefaultLLMAnalyzer returns the built-in default analyzer settings, so image
// upload works without any user configuration.
func DefaultLLMAnalyzer() *LLMAnalyzer {
	return &LLMAnalyzer{
		BaseURL: DefaultLLMBaseURL,
		Model:   DefaultLLMModel,
		Prompt:  DefaultLLMPrompt,
	}
}

// AnalyzerOrDefault returns the configured analyzer, falling back to defaults.
func (c *Config) AnalyzerOrDefault() *LLMAnalyzer {
	if c != nil && c.LLMAnalyzer != nil && c.LLMAnalyzer.BaseURL != "" && c.LLMAnalyzer.Model != "" {
		a := c.LLMAnalyzer
		if a.Prompt == "" {
			a.Prompt = DefaultLLMPrompt
		}
		return a
	}
	return DefaultLLMAnalyzer()
}

// Workspace represents a single MaaS workspace with its own API key and knowledge bases.
type Workspace struct {
	APIKey         string          `json:"api-key"`
	KnowledgeBases []KnowledgeBase `json:"knowledgeBases"`
}

// KnowledgeBase represents a single knowledge base entry.
type KnowledgeBase struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// LoadConfig reads and parses the knowledge base configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	return &cfg, nil
}

// SaveConfig serializes and writes the configuration to the given path.
func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file %s: %w", path, err)
	}
	return nil
}

// ResolveKnowledgeByID looks up a knowledge base by ID across all workspaces.
// Returns the ID, the workspace's API key, and true if found.
func (c *Config) ResolveKnowledgeByID(id string) (string, string, bool) {
	for _, ws := range c.Workspaces {
		for _, kb := range ws.KnowledgeBases {
			if kb.ID == id {
				return kb.ID, ws.APIKey, true
			}
		}
	}
	return "", "", false
}

// ResolveKnowledgeByName looks up a knowledge base by name across all workspaces.
// Returns the ID, the workspace's API key, and true if found.
func (c *Config) ResolveKnowledgeByName(name string) (string, string, bool) {
	for _, ws := range c.Workspaces {
		for _, kb := range ws.KnowledgeBases {
			if kb.Name == name {
				return kb.ID, ws.APIKey, true
			}
		}
	}
	return "", "", false
}

// DefaultKnowledge resolves the default knowledge base.
// Priority: defaultKnowledgeId first, then defaultKnowledgeName.
// Returns the ID, API key, and error.
func (c *Config) DefaultKnowledge() (string, string, error) {
	if c.DefaultKnowledgeId != "" {
		if id, apiKey, ok := c.ResolveKnowledgeByID(c.DefaultKnowledgeId); ok {
			return id, apiKey, nil
		}
		return "", "", fmt.Errorf("defaultKnowledgeId %q not found in any workspace", c.DefaultKnowledgeId)
	}

	if c.DefaultKnowledgeName != "" {
		if id, apiKey, ok := c.ResolveKnowledgeByName(c.DefaultKnowledgeName); ok {
			return id, apiKey, nil
		}
		return "", "", fmt.Errorf("defaultKnowledgeName %q not found in any workspace", c.DefaultKnowledgeName)
	}

	return "", "", fmt.Errorf("no default knowledge base configured; use --knowledge or --knowledge-id")
}

// AllNames returns all knowledge base names across all workspaces.
func (c *Config) AllNames() []string {
	var names []string
	for _, ws := range c.Workspaces {
		for _, kb := range ws.KnowledgeBases {
			names = append(names, kb.Name)
		}
	}
	return names
}

// AllKnowledgeBases returns all knowledge bases across all workspaces as a flat list.
func (c *Config) AllKnowledgeBases() []KnowledgeBase {
	var bases []KnowledgeBase
	for _, ws := range c.Workspaces {
		bases = append(bases, ws.KnowledgeBases...)
	}
	return bases
}
