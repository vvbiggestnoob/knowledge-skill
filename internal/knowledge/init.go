/**
 * @date 2026/8/12
 * @description 实现 knowledge-cli init 命令，支持多工作空间 API Key 批量拉取知识库并合并到配置文件。
 * @since v0.4.0
 */

package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"knowledge-cli/internal/client"
	"knowledge-cli/internal/config"
	"knowledge-cli/internal/model"
)

// --- init ---

var (
	initAPIKeys  string
	initPageSize int
)

func newInitCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize config from remote knowledge bases",
		Long: `Fetch knowledge bases from the MaaS platform and update the local config file.

Supports multiple workspaces: pass comma-separated API keys via --api-keys.
Each workspace's knowledge bases are fetched and merged into the config file.

Examples:
  # Fetch from multiple workspaces
  knowledge-cli init --api-keys "sk-key1,sk-key2"

  # Fetch from a single workspace (uses --api-key flag)
  knowledge-cli init --api-key "sk-key1"

  # Re-fetch from existing config's API keys
  knowledge-cli init

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := *configPtr()

			// Collect API keys to fetch
			var apiKeys []string
			if initAPIKeys != "" {
				apiKeys = splitCSV(initAPIKeys)
			} else if *apiKeyPtr() != "" {
				apiKeys = []string{*apiKeyPtr()}
			} else {
				// Read existing config's workspace API keys
				existingCfg, err := config.LoadConfig(cfgPath)
				if err != nil {
					return fmt.Errorf("no --api-keys or --api-key provided, and cannot read existing config: %w", err)
				}
				for _, ws := range existingCfg.Workspaces {
					if ws.APIKey != "" {
						apiKeys = append(apiKeys, ws.APIKey)
					}
				}
				if len(apiKeys) == 0 {
					return fmt.Errorf("no API keys found in --api-keys, --api-key, or existing config")
				}
			}

			// Fetch knowledge bases for each API key
			var workspaces []config.Workspace
			var allBases []config.KnowledgeBase

			for _, key := range apiKeys {
				c := client.NewMaasClient(client.Config{
					BaseURL:  *baseURLPtr(),
					APIKey:   key,
					Timeout:  time.Duration(*timeoutPtr()) * time.Second,
					Insecure: *insecurePtr(),
				})

				reqBody := map[string]interface{}{
					"pageStart": 1,
					"pageSize":  initPageSize,
				}

				resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/list", reqBody)
				if err != nil {
					fmt.Printf("⚠️  API Key %s...%s 拉取失败：%v\n", key[:6], key[len(key)-4:], err)
					continue
				}

				var result model.APIResponse
				if err := json.Unmarshal(resp.Body, &result); err != nil {
					fmt.Printf("⚠️  API Key %s...%s 解析失败：%v\n", key[:6], key[len(key)-4:], err)
					continue
				}
				if !isCodeOK(result.Code) {
					fmt.Printf("⚠️  API Key %s...%s API 错误：code=%s, message=%s\n", key[:6], key[len(key)-4:], result.Code, result.Message)
					continue
				}

				bases, err := extractKnowledgeBases(result.Data)
				if err != nil {
					fmt.Printf("⚠️  API Key %s...%s 提取知识库失败：%v\n", key[:6], key[len(key)-4:], err)
					continue
				}
				if len(bases) == 0 {
					fmt.Printf("⚠️  API Key %s...%s 未获取到任何知识库\n", key[:6], key[len(key)-4:])
					continue
				}

				fmt.Printf("✅ API Key %s...%s：获取到 %d 个知识库\n", key[:6], key[len(key)-4:], len(bases))
				printKnowledgeBaseTable(bases)

				workspaces = append(workspaces, config.Workspace{
					APIKey:         key,
					KnowledgeBases: bases,
				})
				allBases = append(allBases, bases...)
			}

			if len(workspaces) == 0 {
				return fmt.Errorf("no knowledge bases fetched from any workspace")
			}

			// Preserve existing default if still present, otherwise set to first KB
			var defaultID, defaultName string

			// Try to preserve existing default
			existingCfg, err := config.LoadConfig(cfgPath)
			if err == nil && existingCfg.DefaultKnowledgeId != "" {
				// Check if existing default ID is still in the new list
				for _, kb := range allBases {
					if kb.ID == existingCfg.DefaultKnowledgeId {
						defaultID = kb.ID
						defaultName = kb.Name
						break
					}
				}
			}
			if defaultID == "" && err == nil && existingCfg.DefaultKnowledgeName != "" {
				// Try by name
				for _, kb := range allBases {
					if kb.Name == existingCfg.DefaultKnowledgeName {
						defaultID = kb.ID
						defaultName = kb.Name
						break
					}
				}
			}
			if defaultID == "" {
				// Fall back to first KB
				defaultID = allBases[0].ID
				defaultName = allBases[0].Name
			}

			newCfg := &config.Config{
				DefaultKnowledgeId:   defaultID,
				DefaultKnowledgeName: defaultName,
				Workspaces:           workspaces,
			}

			if err := config.SaveConfig(cfgPath, newCfg); err != nil {
				return err
			}

			fmt.Printf("\n✅ 配置文件已更新：%s（共 %d 个工作空间，%d 个知识库，默认：%s）\n",
				cfgPath, len(workspaces), len(allBases), defaultName)
			return nil
		},
	}

	cmd.Flags().StringVar(&initAPIKeys, "api-keys", "", "Multiple workspace API keys (comma-separated)")
	cmd.Flags().IntVar(&initPageSize, "page-size", 100, "Page size for fetching knowledge bases")

	return cmd
}

// kbRecord is a flexible struct to unmarshal a single knowledge base from the API response.
// It supports both id/name and knowledgeId/knowledgeName field variants.
type kbRecord struct {
	ID            string `json:"id"`
	KnowledgeID   string `json:"knowledgeId"`
	Name          string `json:"name"`
	KnowledgeName string `json:"knowledgeName"`
}

func (r kbRecord) ToKnowledgeBase() config.KnowledgeBase {
	id := r.ID
	if id == "" {
		id = r.KnowledgeID
	}
	name := r.Name
	if name == "" {
		name = r.KnowledgeName
	}
	return config.KnowledgeBase{Name: name, ID: id}
}

// extractKnowledgeBases flexibly parses the Data field of the list API response.
// It tries paginated wrappers { "list": [...] } / { "records": [...] } first, then bare array [...].
func extractKnowledgeBases(data interface{}) ([]config.KnowledgeBase, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("re-marshal data: %w", err)
	}

	// Try paginated wrapper with "list" key (MaaS actual format)
	var listWrapper struct {
		List []kbRecord `json:"list"`
	}
	if err := json.Unmarshal(raw, &listWrapper); err == nil && len(listWrapper.List) > 0 {
		return convertRecords(listWrapper.List), nil
	}

	// Try paginated wrapper with "records" key
	var recordsWrapper struct {
		Records []kbRecord `json:"records"`
	}
	if err := json.Unmarshal(raw, &recordsWrapper); err == nil && len(recordsWrapper.Records) > 0 {
		return convertRecords(recordsWrapper.Records), nil
	}

	// Try bare array: [...]
	var records []kbRecord
	if err := json.Unmarshal(raw, &records); err == nil && len(records) > 0 {
		return convertRecords(records), nil
	}

	// Fallback: try nested "data" key (some APIs double-wrap)
	var nested struct {
		Data interface{} `json:"data"`
	}
	if err := json.Unmarshal(raw, &nested); err == nil && nested.Data != nil {
		nestedRaw, err := json.Marshal(nested.Data)
		if err == nil {
			if err := json.Unmarshal(nestedRaw, &listWrapper); err == nil && len(listWrapper.List) > 0 {
				return convertRecords(listWrapper.List), nil
			}
			if err := json.Unmarshal(nestedRaw, &recordsWrapper); err == nil && len(recordsWrapper.Records) > 0 {
				return convertRecords(recordsWrapper.Records), nil
			}
			if err := json.Unmarshal(nestedRaw, &records); err == nil && len(records) > 0 {
				return convertRecords(records), nil
			}
		}
	}

	return nil, fmt.Errorf("could not parse knowledge base list from response data")
}

func convertRecords(records []kbRecord) []config.KnowledgeBase {
	bases := make([]config.KnowledgeBase, 0, len(records))
	for _, r := range records {
		kb := r.ToKnowledgeBase()
		// Skip entries without an ID (not valid knowledge bases)
		if kb.ID == "" {
			continue
		}
		bases = append(bases, kb)
	}
	return bases
}

// printKnowledgeBaseTable prints a formatted table of knowledge bases.
func printKnowledgeBaseTable(bases []config.KnowledgeBase) {
	// Calculate column widths
	nameWidth := len("NAME")
	idWidth := len("ID")
	for _, kb := range bases {
		if len(kb.Name) > nameWidth {
			nameWidth = len(kb.Name)
		}
		if len(kb.ID) > idWidth {
			idWidth = len(kb.ID)
		}
	}

	// Header
	fmt.Printf("%s | %s\n", padRight("NAME", nameWidth), padRight("ID", idWidth))
	fmt.Printf("%s-+-%s\n", strings.Repeat("-", nameWidth), strings.Repeat("-", idWidth))

	// Rows
	for _, kb := range bases {
		fmt.Printf("%s | %s\n", padRight(kb.Name, nameWidth), padRight(kb.ID, idWidth))
	}
	fmt.Println()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
