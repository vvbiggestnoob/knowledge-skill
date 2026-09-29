/**
 * @date 2026/8/10
 * @description 实现 knowledge search 命令，提供知识库分片检索功能，支持语义/全文/混合检索。
 *              v0.1.1 新增：搜索未命中时自动记录问题到 cache 目录，供人工审核后上传知识库。
 * @since v0.1.1
 */

package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"knowledge-cli/internal/config"
	"knowledge-cli/internal/model"
)

// --- knowledge search ---

var (
	searchKnowledgeID      string
	searchKnowledgeName    string
	searchQuery            string
	searchTopK             int
	searchSimilarity       float64
	searchStrategy         int
	searchEnableRerank     bool
	searchRerankModel      string
	searchUnansweredThresh float64
)

func newSearchCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search chunks in a knowledge base",
		Long: `Search chunks in a knowledge base using semantic, full-text, or hybrid retrieval.

Use --knowledge to specify a knowledge base by name (from config), or --knowledge-id for a direct ID.
If neither is specified, the default knowledge base from config is used.

Retrieval strategies:
  1 - Semantic retrieval (vector similarity)
  2 - Full-text retrieval
  3 - Hybrid retrieval (default, combines semantic + full-text)

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/trial/test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveSearchKnowledge(searchKnowledgeName, searchKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId":         id,
				"query":               searchQuery,
				"topK":                searchTopK,
				"similarityThreshold": searchSimilarity,
				"retrieveStrategy":    searchStrategy,
				"enableRerankModel":   searchEnableRerank,
			}

			if searchRerankModel != "" {
				reqBody["rerankModel"] = searchRerankModel
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/trial/test", reqBody)
			if err != nil {
				return err
			}
			if err := printResponse(resp); err != nil {
				return err
			}

			// Check for unanswered: if all results have similarity below threshold, record the question
			checkUnanswered(resp.Body, searchQuery, searchKnowledgeName, searchKnowledgeID, id, searchTopK, searchUnansweredThresh)
			return nil
		},
	}

	cmd.Flags().StringVar(&searchKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&searchKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge, supports comma-separated multiple IDs)")
	cmd.Flags().StringVar(&searchQuery, "query", "", "Search query keyword (required)")
	cmd.Flags().IntVar(&searchTopK, "top-k", 10, "Maximum number of chunks to return")
	cmd.Flags().Float64Var(&searchSimilarity, "similarity", 0.6, "Similarity threshold (0.0-1.0), only chunks above this score are returned")
	cmd.Flags().IntVar(&searchStrategy, "strategy", 3, "Retrieval strategy: 1=semantic, 2=full-text, 3=hybrid")
	cmd.Flags().BoolVar(&searchEnableRerank, "rerank", true, "Enable rerank model for better ordering")
	cmd.Flags().StringVar(&searchRerankModel, "rerank-model", "", "Rerank model: bge-reranker-base, bge-reranker-v2-m3, bge-reranker-large, Qwen3-Reranker-0.6B")
	cmd.Flags().Float64Var(&searchUnansweredThresh, "unanswered-threshold", 0.9, "Similarity threshold for unanswered detection; if all results are below this, the question is recorded for manual review")

	cmd.MarkFlagRequired("query")

	return cmd
}

// --- unanswered question recording ---

// resolveSearchKnowledge resolves the search target knowledge base(s).
// It supports multiple knowledge bases via --knowledge-id "id1,id2". Each ID is
// resolved against the config; the resolved IDs are re-joined with commas into
// the single knowledgeId field the MaaS trial/test endpoint expects. A single
// api-key header is used (from the workspace of the first ID), so all IDs should
// belong to one workspace when searching across multiple bases.
func resolveSearchKnowledge(knowledgeName, knowledgeID, configPath string) (string, string, error) {
	// Single-ID (or name/default) search: delegate to the shared resolver.
	if strings.TrimSpace(knowledgeID) == "" || !strings.Contains(knowledgeID, ",") {
		return resolveKnowledge(knowledgeName, knowledgeID, configPath)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return "", "", fmt.Errorf("cannot load config: %w", err)
	}

	ids := splitCSV(knowledgeID)
	joined := make([]string, 0, len(ids))
	var apiKey string
	for _, one := range ids {
		id, key, ok := cfg.ResolveKnowledgeByID(one)
		if !ok {
			return "", "", fmt.Errorf("knowledge base ID %q not found in config (available: %v)", one, cfg.AllNames())
		}
		if apiKey == "" {
			apiKey = key
		}
		joined = append(joined, id)
	}
	return strings.Join(joined, ","), apiKey, nil
}

// searchResultItem is a flexible struct to extract similarity from search response items.
type searchResultItem struct {
	Similarity float64 `json:"similarity"`
}

// extractSimilarities parses the search response Data field and extracts similarity scores.
// It flexibly handles paginated wrappers { "chunks": [...] } / { "list": [...] } / { "records": [...] } and bare array [...] formats.
func extractSimilarities(data interface{}) []float64 {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	// Try paginated wrapper with "chunks" key (MaaS search actual format)
	var chunksWrapper struct {
		Chunks []searchResultItem `json:"chunks"`
	}
	if err := json.Unmarshal(raw, &chunksWrapper); err == nil && len(chunksWrapper.Chunks) > 0 {
		sims := make([]float64, len(chunksWrapper.Chunks))
		for i, r := range chunksWrapper.Chunks {
			sims[i] = r.Similarity
		}
		return sims
	}

	// Try paginated wrapper with "list" key
	var listWrapper struct {
		List []searchResultItem `json:"list"`
	}
	if err := json.Unmarshal(raw, &listWrapper); err == nil && len(listWrapper.List) > 0 {
		sims := make([]float64, len(listWrapper.List))
		for i, r := range listWrapper.List {
			sims[i] = r.Similarity
		}
		return sims
	}

	// Try paginated wrapper with "records" key
	var recordsWrapper struct {
		Records []searchResultItem `json:"records"`
	}
	if err := json.Unmarshal(raw, &recordsWrapper); err == nil && len(recordsWrapper.Records) > 0 {
		sims := make([]float64, len(recordsWrapper.Records))
		for i, r := range recordsWrapper.Records {
			sims[i] = r.Similarity
		}
		return sims
	}

	// Try bare array: [...]
	var records []searchResultItem
	if err := json.Unmarshal(raw, &records); err == nil && len(records) > 0 {
		sims := make([]float64, len(records))
		for i, r := range records {
			sims[i] = r.Similarity
		}
		return sims
	}

	return nil
}

// checkUnanswered inspects search results and records the question if all similarities are below threshold.
func checkUnanswered(respBody []byte, query, knowledgeName, knowledgeID, resolvedID string, topK int, threshold float64) {
	var result model.APIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return
	}

	similarities := extractSimilarities(result.Data)

	// Determine max similarity
	maxSim := 0.0
	for _, s := range similarities {
		if s > maxSim {
			maxSim = s
		}
	}

	// If no results or all below threshold → unanswered
	if len(similarities) == 0 || maxSim < threshold {
		kbLabel := knowledgeName
		if kbLabel == "" {
			kbLabel = knowledgeID
		}
		if kbLabel == "" {
			kbLabel = resolvedID
		}

		filename, err := recordUnanswered(query, kbLabel, topK, threshold)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  未命中（最高相似度 %.4f < %.2f），但记录失败：%v\n", maxSim, threshold, err)
			return
		}
		fmt.Fprintf(os.Stderr, "⚠️  未命中（最高相似度 %.4f < %.2f），已记录到 cache/%s，待人工审核\n", maxSim, threshold, filename)
	}
}

// recordUnanswered appends an unanswered question to cache/unanswered-questions-YYYYMMDD.md.
// Same-day questions are appended to the same file.
func recordUnanswered(query, knowledgeLabel string, topK int, threshold float64) (string, error) {
	cacheDir := filepath.Join(".", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	now := time.Now()
	timestamp := now.Format("20060102")
	filename := fmt.Sprintf("unanswered-questions-%s.md", timestamp)
	filePath := filepath.Join(cacheDir, filename)

	content := fmt.Sprintf(`## 未命中问题

- **时间**：%s
- **查询问题**：%s
- **知识库**：%s
- **Top-K**：%d
- **未命中阈值**：%.2f

### 人工审核

- **审核结论**：（待填写：已回答 / 需补充 / 无意义）
- **补充答案**：（待填写：针对该问题的标准答案，将用于上传知识库）

---

`, now.Format("2006-01-02 15:04:05"), query, knowledgeLabel, topK, threshold)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("open unanswered file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return "", fmt.Errorf("write unanswered record: %w", err)
	}

	return filename, nil
}
