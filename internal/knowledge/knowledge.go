/**
 * @date 2026/8/10
 * @description 实现 knowledge-cli 命令，提供知识库列表、详情、文档管理和搜索功能。
 * @since v1.0
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

// RegisterCommands adds all knowledge-cli commands to the root command.
func RegisterCommands(rootCmd *cobra.Command, apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) {
	rootCmd.AddCommand(newListCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newGetCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newCreateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newUpdateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newDeleteCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newGraphCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newDocCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newChunkCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newLabelCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newSearchCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newInitCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newUploadUnansweredCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	rootCmd.AddCommand(newSwitchCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
}

func newMaasClient(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *client.MaasClient {
	apiKey := *apiKeyPtr()
	if apiKey == "" && configPtr != nil {
		cfg, err := config.LoadConfig(*configPtr())
		if err == nil {
			// Use the first workspace's API key as fallback
			if len(cfg.Workspaces) > 0 && cfg.Workspaces[0].APIKey != "" {
				apiKey = cfg.Workspaces[0].APIKey
			}
		}
	}
	return client.NewMaasClient(client.Config{
		BaseURL:  *baseURLPtr(),
		APIKey:   apiKey,
		Timeout:  time.Duration(*timeoutPtr()) * time.Second,
		Insecure: *insecurePtr(),
	})
}

// newMaasClientWithKey creates a client using the resolved API key for a specific knowledge base.
// If --api-key flag is set, it takes priority; otherwise the resolved key from config is used.
func newMaasClientWithKey(resolvedAPIKey string, apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool) *client.MaasClient {
	apiKey := *apiKeyPtr()
	if apiKey == "" {
		apiKey = resolvedAPIKey
	}
	return client.NewMaasClient(client.Config{
		BaseURL:  *baseURLPtr(),
		APIKey:   apiKey,
		Timeout:  time.Duration(*timeoutPtr()) * time.Second,
		Insecure: *insecurePtr(),
	})
}

func mustJSON(v interface{}) string {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(out)
}

func isCodeOK(code string) bool {
	return code == "0"
}

func printResponse(resp *client.Response) error {
	var result model.APIResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		fmt.Println(string(resp.Body))
		return nil
	}
	fmt.Println(mustJSON(result))
	if !isCodeOK(result.Code) {
		return fmt.Errorf("API error: code=%s, message=%s", result.Code, result.Message)
	}
	return nil
}

// resolveKnowledge resolves a knowledge base ID and its API key from the --knowledge flag or --knowledge-id flag.
// Priority: --knowledge-id (explicit ID) > --knowledge (name from config) > config default (defaultKnowledgeId first, then defaultKnowledgeName).
// Returns (id, apiKey, error).
func resolveKnowledge(knowledgeName, knowledgeID, configPath string) (string, string, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return "", "", fmt.Errorf("cannot load config: %w", err)
	}

	if knowledgeID != "" {
		id, apiKey, ok := cfg.ResolveKnowledgeByID(knowledgeID)
		if !ok {
			return "", "", fmt.Errorf("knowledge base ID %q not found in config (available: %v)", knowledgeID, cfg.AllNames())
		}
		return id, apiKey, nil
	}

	if knowledgeName != "" {
		id, apiKey, ok := cfg.ResolveKnowledgeByName(knowledgeName)
		if !ok {
			return "", "", fmt.Errorf("knowledge base %q not found in config (available: %v)", knowledgeName, cfg.AllNames())
		}
		return id, apiKey, nil
	}

	id, apiKey, err := cfg.DefaultKnowledge()
	if err != nil {
		return "", "", err
	}
	return id, apiKey, nil
}

// findDocByName searches a doc list API response for a document with the exact matching name.
// Returns the doc ID and true if found, empty string and false otherwise.
func findDocByName(respBody []byte, targetName string) (string, bool) {
	var result model.APIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", false
	}

	raw, err := json.Marshal(result.Data)
	if err != nil {
		return "", false
	}

	// Try {"list": [...]} format (doc list API response)
	var listWrapper struct {
		List []struct {
			ID      string `json:"id"`
			DocName string `json:"docName"`
		} `json:"list"`
	}
	if err := json.Unmarshal(raw, &listWrapper); err == nil {
		for _, doc := range listWrapper.List {
			if doc.DocName == targetName {
				return doc.ID, true
			}
		}
	}

	// Try bare array [...] format
	var docs []struct {
		ID      string `json:"id"`
		DocName string `json:"docName"`
	}
	if err := json.Unmarshal(raw, &docs); err == nil {
		for _, doc := range docs {
			if doc.DocName == targetName {
				return doc.ID, true
			}
		}
	}

	return "", false
}

// findOrCreateCustomQuestionsDoc finds a document by name in the knowledge base, or creates it if not found.
// Returns the doc ID. If creation with the original name fails (e.g. format error), retries with .custom suffix.
func findOrCreateCustomQuestionsDoc(c *client.MaasClient, knowledgeID, docName string) (string, error) {
	// Search for existing document
	listReqBody := map[string]interface{}{
		"knowledgeId": knowledgeID,
		"pageStart":   1,
		"pageSize":    100,
		"content":     docName,
	}
	resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/list", listReqBody)
	if err != nil {
		return "", fmt.Errorf("search for existing doc: %w", err)
	}

	if docID, found := findDocByName(resp.Body, docName); found {
		return docID, nil
	}

	// Document not found, create it
	createReqBody := map[string]interface{}{
		"knowledgeId": knowledgeID,
		"docs": []map[string]interface{}{
			{"docName": docName},
		},
	}
	createResp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/create", createReqBody)
	if err != nil {
		// If creation fails (e.g. format error), retry with .custom suffix
		customName := docName
		if !strings.HasSuffix(customName, ".custom") {
			// Replace the extension with .custom
			if idx := strings.LastIndex(customName, "."); idx > 0 {
				customName = customName[:idx] + ".custom"
			} else {
				customName = customName + ".custom"
			}
		}
		if customName == docName {
			return "", fmt.Errorf("create doc: %w", err)
		}
		createReqBody["docs"] = []map[string]interface{}{
			{"docName": customName},
		}
		createResp, err = c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/create", createReqBody)
		if err != nil {
			return "", fmt.Errorf("create doc (retry with .custom): %w", err)
		}
	}

	// Try to extract docId from create response
	docID := extractDocIDFromResponse(createResp.Body)
	if docID != "" {
		return docID, nil
	}

	// If create response doesn't include docId, re-list to find it
	createdName := docName
	if createResp != nil {
		// Try to find the actual created doc name from response
		var result model.APIResponse
		if json.Unmarshal(createResp.Body, &result) == nil {
			if name := extractDocNameFromResponse(result.Data); name != "" {
				createdName = name
			}
		}
	}
	listReqBody["content"] = createdName
	resp, err = c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/list", listReqBody)
	if err != nil {
		return "", fmt.Errorf("re-list after create: %w", err)
	}
	if docID, found := findDocByName(resp.Body, createdName); found {
		return docID, nil
	}

	return "", fmt.Errorf("doc created but could not find docId")
}

// extractDocIDFromResponse tries to extract a docId from the create response data.
func extractDocIDFromResponse(body []byte) string {
	var result model.APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		return ""
	}

	// Try {"docs": [{"docId": "..."}]} format
	var docsWrapper struct {
		Docs []struct {
			DocID string `json:"docId"`
		} `json:"docs"`
	}
	if err := json.Unmarshal(raw, &docsWrapper); err == nil && len(docsWrapper.Docs) > 0 {
		return docsWrapper.Docs[0].DocID
	}

	// Try {"docId": "..."} format
	var docWrapper struct {
		DocID string `json:"docId"`
	}
	if err := json.Unmarshal(raw, &docWrapper); err == nil && docWrapper.DocID != "" {
		return docWrapper.DocID
	}

	// Try bare array [{"docId": "..."}] format
	var docs []struct {
		DocID string `json:"docId"`
	}
	if err := json.Unmarshal(raw, &docs); err == nil && len(docs) > 0 {
		return docs[0].DocID
	}

	return ""
}

// extractDocNameFromResponse tries to extract a docName from the create response data.
func extractDocNameFromResponse(data interface{}) string {
	raw, err := json.Marshal(data)
	if err != nil {
		return ""
	}

	// Try {"docs": [{"docName": "..."}]} format
	var docsWrapper struct {
		Docs []struct {
			DocName string `json:"docName"`
		} `json:"docs"`
	}
	if err := json.Unmarshal(raw, &docsWrapper); err == nil && len(docsWrapper.Docs) > 0 {
		return docsWrapper.Docs[0].DocName
	}

	return ""
}

// --- list ---

var listName string
var listPageStart int
var listPageSize int

func newListCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List knowledge bases",
		Long:  `List knowledge bases in the current workspace. Supports fuzzy search by name.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			reqBody := map[string]interface{}{
				"pageStart": listPageStart,
				"pageSize":  listPageSize,
			}
			if listName != "" {
				reqBody["name"] = listName
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/list", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&listName, "name", "", "Knowledge base name (fuzzy search)")
	cmd.Flags().IntVar(&listPageStart, "page-start", 1, "Page start")
	cmd.Flags().IntVar(&listPageSize, "page-size", 10, "Page size")

	return cmd
}

// --- get ---

var getID string

func newGetCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a knowledge base by ID",
		Long:  `Get detailed information about a knowledge base by its ID.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			resp, err := c.Get("/api/maas-model-api/api/datacenter/v1/knowledge/" + getID)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&getID, "id", "", "Knowledge base ID (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}
