/**
 * @date 2026/8/12
 * @description 实现 upload-unanswered 命令，扫描 cache 目录下的未命中文件并上传到知识库。
 *              支持 .custom 文件和 unanswered-questions-*.md 文件，上传成功后删除本地文件。
 * @since v0.3.0
 */

package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"knowledge-cli/internal/client"
	"knowledge-cli/internal/model"
)

// --- upload-unanswered ---

var (
	uploadUnansweredKnowledgeID string
	uploadUnansweredKnowledge   string
	uploadUnansweredCacheDir    string
	uploadUnansweredDryRun      bool
)

func newUploadUnansweredCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload-unanswered",
		Short: "Upload reviewed unanswered questions to knowledge base",
		Long: `Scan the cache directory for unanswered question files and upload them to the knowledge base.

Two types of files are processed:
  1. *.custom files — content uploaded as a single chunk, filename (without .custom) as title
  2. unanswered-questions-*.md files — parsed for reviewed Q&A blocks, each block uploaded as a chunk

Successfully uploaded files are deleted from the cache directory.
Unreviewed or failed blocks in .md files are preserved by rewriting the file.

Use --dry-run to preview without uploading.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(uploadUnansweredKnowledge, uploadUnansweredKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			// Scan cache directory
			cacheDir := uploadUnansweredCacheDir
			if cacheDir == "" {
				cacheDir = "./cache"
			}

			entries, err := os.ReadDir(cacheDir)
			if err != nil {
				fmt.Printf("未找到待上传的未命中文件（目录 %s 不存在或无法访问）\n", cacheDir)
				return nil
			}

			// Collect files to process
			var customFiles []string
			var mdFiles []string
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasSuffix(name, ".custom") {
					customFiles = append(customFiles, filepath.Join(cacheDir, name))
				} else if strings.HasPrefix(name, "unanswered-questions-") && strings.HasSuffix(name, ".md") {
					mdFiles = append(mdFiles, filepath.Join(cacheDir, name))
				}
			}

			totalFiles := len(customFiles) + len(mdFiles)
			if totalFiles == 0 {
				fmt.Printf("未找到待上传的未命中文件（cache 目录中无 .custom 或 unanswered-questions-*.md 文件）\n")
				return nil
			}

			fmt.Printf("发现 %d 个待处理文件（%d 个 .custom，%d 个 .md）\n", totalFiles, len(customFiles), len(mdFiles))
			if uploadUnansweredDryRun {
				fmt.Println("🔍 dry-run 模式：仅预览，不实际上传")
			}

			// Find or create the custom-questions.md document
			docName := "custom-questions.md"
			var docID string
			if !uploadUnansweredDryRun {
				docID, err = findOrCreateCustomQuestionsDoc(c, id, docName)
				if err != nil {
					return fmt.Errorf("find or create doc: %w", err)
				}
				fmt.Printf("✅ 使用文档 %s (docId: %s)\n", docName, docID)
			}

			totalUploaded := 0
			totalSkipped := 0
			deletedFiles := 0

			// Process .custom files
			for _, filePath := range customFiles {
				uploaded, skip, err := processCustomFile(c, id, docID, filePath, uploadUnansweredDryRun)
				if err != nil {
					fmt.Printf("❌ 处理 %s 失败：%v\n", filepath.Base(filePath), err)
					continue
				}
				if uploaded {
					totalUploaded++
				}
				if skip {
					totalSkipped++
				}
				if uploaded && !uploadUnansweredDryRun {
					if err := os.Remove(filePath); err != nil {
						fmt.Printf("⚠️  上传成功但删除本地文件失败：%s (%v)\n", filepath.Base(filePath), err)
					} else {
						fmt.Printf("🗑️  已删除本地文件 %s\n", filepath.Base(filePath))
						deletedFiles++
					}
				}
			}

			// Process unanswered-questions-*.md files
			for _, filePath := range mdFiles {
				uploaded, skip, err := processUnansweredMDFile(c, id, docID, filePath, uploadUnansweredDryRun)
				if err != nil {
					fmt.Printf("❌ 处理 %s 失败：%v\n", filepath.Base(filePath), err)
					continue
				}
				totalUploaded += uploaded
				totalSkipped += skip
			}

			// Print summary
			fmt.Println("\n--- 汇总 ---")
			fmt.Printf("扫描文件数：%d\n", totalFiles)
			fmt.Printf("上传分片数：%d\n", totalUploaded)
			fmt.Printf("跳过（未审核）：%d\n", totalSkipped)
			fmt.Printf("删除文件数：%d\n", deletedFiles)
			if uploadUnansweredDryRun {
				fmt.Println("（dry-run 模式，未实际上传或删除）")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&uploadUnansweredKnowledge, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&uploadUnansweredKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&uploadUnansweredCacheDir, "cache-dir", "./cache", "Cache directory path")
	cmd.Flags().BoolVar(&uploadUnansweredDryRun, "dry-run", false, "Preview only, do not upload or delete")

	return cmd
}

// processCustomFile reads a .custom file and uploads its content as a single chunk.
// Returns (uploaded=true, skipped=false, error) on success.
func processCustomFile(c *client.MaasClient, knowledgeID, docID, filePath string, dryRun bool) (bool, bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, false, fmt.Errorf("read file: %w", err)
	}

	// Derive title from filename (strip .custom suffix)
	baseName := filepath.Base(filePath)
	title := strings.TrimSuffix(baseName, ".custom")

	fmt.Printf("\n📄 处理 .custom 文件：%s\n", baseName)
	fmt.Printf("   标题：%s\n", title)
	fmt.Printf("   内容长度：%d 字符\n", len(content))

	if dryRun {
		fmt.Println("   (dry-run) 跳过实际上传")
		return true, false, nil
	}

	// Upload as a chunk
	reqBody := map[string]interface{}{
		"knowledgeId":  knowledgeID,
		"docId":        docID,
		"chunkContent": string(content),
		"chunkTitle":   title,
	}
	resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/add", reqBody)
	if err != nil {
		return false, false, fmt.Errorf("chunk add: %w", err)
	}

	// Check if upload was successful
	var result model.APIResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil || result.Code != "0" {
		return false, false, fmt.Errorf("chunk add failed: %s", string(resp.Body))
	}

	fmt.Printf("   ✅ 上传成功\n")
	return true, false, nil
}

// unansweredEntry represents a parsed Q&A block from unanswered-questions-*.md
type unansweredEntry struct {
	RawBlock string // original markdown block text
	Query    string
	Answer   string
	Reviewed bool
}

// processUnansweredMDFile parses an unanswered-questions-*.md file and uploads reviewed blocks.
// Returns (uploadedCount, skippedCount, error).
func processUnansweredMDFile(c *client.MaasClient, knowledgeID, docID, filePath string, dryRun bool) (int, int, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0, 0, fmt.Errorf("read file: %w", err)
	}

	baseName := filepath.Base(filePath)
	fmt.Printf("\n📝 处理 .md 文件：%s\n", baseName)

	entries := parseUnansweredFile(content)
	fmt.Printf("   解析到 %d 个问题块\n", len(entries))

	uploadedCount := 0
	skippedCount := 0
	var remainingBlocks []string

	for i, entry := range entries {
		if !entry.Reviewed {
			skippedCount++
			remainingBlocks = append(remainingBlocks, entry.RawBlock)
			fmt.Printf("   [%d] 跳过（未审核）：%s\n", i+1, truncate(entry.Query, 40))
			continue
		}

		// Construct chunk content
		chunkContent := fmt.Sprintf("## 问题\n\n%s\n\n## 答案\n\n%s", entry.Query, entry.Answer)
		// Use first 50 chars of query as title
		title := truncate(entry.Query, 50)

		fmt.Printf("   [%d] 上传：%s\n", i+1, title)

		if dryRun {
			fmt.Println("   (dry-run) 跳过实际上传")
			uploadedCount++
			continue
		}

		// Upload as a chunk
		reqBody := map[string]interface{}{
			"knowledgeId":  knowledgeID,
			"docId":        docID,
			"chunkContent": chunkContent,
			"chunkTitle":   title,
		}
		resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/add", reqBody)
		if err != nil {
			fmt.Printf("   ❌ 上传失败：%v\n", err)
			remainingBlocks = append(remainingBlocks, entry.RawBlock)
			continue
		}

		// Check success
		var result model.APIResponse
		if err := json.Unmarshal(resp.Body, &result); err != nil || result.Code != "0" {
			fmt.Printf("   ❌ 上传失败：%s\n", string(resp.Body))
			remainingBlocks = append(remainingBlocks, entry.RawBlock)
			continue
		}

		fmt.Printf("   ✅ 上传成功\n")
		uploadedCount++
	}

	// Handle local file after processing
	if dryRun {
		return uploadedCount, skippedCount, nil
	}

	if len(remainingBlocks) == 0 {
		// All blocks uploaded, delete the file
		if err := os.Remove(filePath); err != nil {
			fmt.Printf("   ⚠️  删除本地文件失败：%v\n", err)
		} else {
			fmt.Printf("   🗑️  已删除本地文件 %s（全部上传成功）\n", baseName)
		}
	} else {
		// Rewrite file with remaining blocks
		newContent := strings.Join(remainingBlocks, "\n")
		if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
			fmt.Printf("   ⚠️  更新本地文件失败：%v\n", err)
		} else {
			fmt.Printf("   📝 已更新本地文件 %s（保留 %d 条未处理）\n", baseName, len(remainingBlocks))
		}
	}

	return uploadedCount, skippedCount, nil
}

// parseUnansweredFile parses an unanswered-questions-*.md file into entries.
// Each block starts with "## 未命中问题" and ends with "---".
func parseUnansweredFile(content []byte) []unansweredEntry {
	var entries []unansweredEntry

	// Split by the block delimiter "## 未命中问题"
	blocks := strings.Split(string(content), "## 未命中问题")

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		// Re-add the header for the raw block
		rawBlock := "## 未命中问题\n\n" + block
		// Ensure it ends with ---
		if !strings.HasSuffix(rawBlock, "---") {
			rawBlock = strings.TrimRight(rawBlock, "\n") + "\n\n---\n"
		}

		entry := unansweredEntry{
			RawBlock: rawBlock,
		}

		// Extract query: line starting with "- **查询问题**："
		queryRe := regexp.MustCompile(`(?m)-\s*\*\*查询问题\*\*[：:]\s*(.+)`)
		if m := queryRe.FindStringSubmatch(block); len(m) > 1 {
			entry.Query = strings.TrimSpace(m[1])
		}

		// Extract answer: line starting with "- **补充答案**："
		answerRe := regexp.MustCompile(`(?m)-\s*\*\*补充答案\*\*[：:]\s*(.+)`)
		if m := answerRe.FindStringSubmatch(block); len(m) > 1 {
			answer := strings.TrimSpace(m[1])
			// Check if answer is still the template placeholder
			if answer != "" && !strings.HasPrefix(answer, "（待填写") {
				entry.Answer = answer
				entry.Reviewed = true
			}
		}

		// Only add entries that have a query
		if entry.Query != "" {
			entries = append(entries, entry)
		}
	}

	return entries
}

// truncate returns s truncated to maxLen characters, with "..." suffix if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
