/**
 * @date 2026/8/10
 * @description 实现 knowledge doc 命令，提供知识库文档列表、详情、分片查看、上传和导入功能。
 * @since v1.0
 */

package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"knowledge-cli/internal/client"
	"knowledge-cli/internal/config"
	"knowledge-cli/internal/model"
)

// --- knowledge doc ---

var (
	docKnowledgeID   string
	docKnowledgeName string
	docContent       string
	docPageStart     int
	docPageSize      int
	docID            string
)

func newDocCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Manage knowledge base documents",
		Long:  `Commands for listing, viewing, uploading, importing documents and their chunks in a knowledge base.`,
	}

	cmd.AddCommand(newDocListCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocGetCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocChunksCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocUploadCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocImportCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocCreateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocEnableCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocDisableCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocDeleteCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocViewCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocDownloadCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newDocGraphRestartCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))

	return cmd
}

// --- knowledge doc list ---

func newDocListCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List documents in a knowledge base",
		Long: `List documents in a knowledge base. Supports keyword search by document name.

Use --knowledge to specify a knowledge base by name (from config), or --knowledge-id for a direct ID.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"pageStart":   docPageStart,
				"pageSize":    docPageSize,
			}
			if docContent != "" {
				reqBody["content"] = docContent
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/list", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docContent, "content", "", "Document name keyword search")
	cmd.Flags().IntVar(&docPageStart, "page-start", 1, "Page start")
	cmd.Flags().IntVar(&docPageSize, "page-size", 20, "Page size")

	return cmd
}

// --- knowledge doc get ---

func newDocGetCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a document by ID",
		Long: `Get detailed information about a document by knowledge base ID and document ID.

Use --knowledge to specify a knowledge base by name (from config), or --knowledge-id for a direct ID.

API: GET /api/maas-model-api/api/datacenter/v1/knowledge/doc/{knowledgeId}/{docId}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			path := fmt.Sprintf("/api/maas-model-api/api/datacenter/v1/knowledge/doc/%s/%s", id, docID)
			resp, err := c.Get(path)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docID, "id", "", "Document ID (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- knowledge doc chunks ---

var (
	chunkKeyword string
)

func newDocChunksCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chunks",
		Short: "List document chunks",
		Long: `List chunks of a document. Supports keyword search within chunks.

Use --knowledge to specify a knowledge base by name (from config), or --knowledge-id for a direct ID.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       docID,
				"pageStart":   docPageStart,
				"pageSize":    docPageSize,
			}
			if chunkKeyword != "" {
				reqBody["keyword"] = chunkKeyword
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/list", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docID, "id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkKeyword, "keyword", "", "Chunk content keyword search")
	cmd.Flags().IntVar(&docPageStart, "page-start", 1, "Page start")
	cmd.Flags().IntVar(&docPageSize, "page-size", 10, "Page size")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- knowledge doc upload ---
//
// Upload follows the "精细上传" (refined upload) mode ported from the HiDonalds
// Python tool: files are uploaded first, then grouped by type and imported with
// a type-specific chunkConfig. This lets each document type (text/table/image/
// txt) be parsed with its own parsing rules instead of the MaaS platform default.

var (
	uploadFiles       []string
	uploadKnowledgeID string
)

func newDocUploadCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload documents to MaaS, grouped by type with per-type chunk config",
		Long: `Upload local files to the MaaS platform, then (if --knowledge or --knowledge-id is
given) import them grouped by file type, each group using its own chunkConfig
(text / table / image / txt). Files are uploaded first, then imported in the
same command.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/upload/create
     POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/import`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve knowledge ID if specified
			var kid, kbAPIKey string
			if uploadKnowledgeID != "" || docKnowledgeName != "" {
				var err error
				kid, kbAPIKey, err = resolveKnowledge(docKnowledgeName, uploadKnowledgeID, *configPtr())
				if err != nil {
					return err
				}
			}

			c := newMaasClientWithKey(kbAPIKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			// Visual model settings for image analysis (falls back to defaults).
			cfg, _ := config.LoadConfig(*configPtr())
			analyzer := cfg.AnalyzerOrDefault()

			// Upload each file. Images are first analyzed by the visual model to
			// produce a text summary + embedded image, assembled into a temp docx,
			// and that docx is uploaded instead of the raw image. Other types are
			// uploaded as-is. Group uploaded doc info by type.
			groups := make(map[int][]map[string]interface{})
			for _, filePath := range uploadFiles {
				pt := getFileParseType(filePath)
				info, err := uploadDocForUpload(c, filePath, pt, kbAPIKey, analyzer)
				if err != nil {
					fmt.Printf("❌ 上传失败: %v\n", err)
					continue
				}
				groups[pt] = append(groups[pt], info)
			}

			if kid == "" {
				// No knowledge base given: upload only, report what was uploaded.
				fmt.Println("未指定知识库，仅上传文件（未导入）")
				total := 0
				for _, items := range groups {
					total += len(items)
				}
				fmt.Printf("已上传 %d 个文件\n", total)
				return nil
			}

			// Import each type group with its own chunkConfig.
			order := []int{1, 0, 2, 3} // 文本, txt(B), 表格, 图片
			for _, pt := range order {
				items := groups[pt]
				if len(items) == 0 {
					continue
				}
				cc, label := chunkConfigForParseType(pt)
				if err := importDocs(c, kid, items, cc, label); err != nil {
					fmt.Printf("❌ %s 导入失败: %v\n", label, err)
				} else {
					fmt.Printf("✅ %s 导入完成: %d 个\n", label, len(items))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&uploadFiles, "files", nil, "Local file paths to upload (repeatable or comma-separated)")
	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config, optional)")
	cmd.Flags().StringVar(&uploadKnowledgeID, "knowledge-id", "", "Knowledge base ID (optional, overrides --knowledge)")
	cmd.MarkFlagRequired("files")

	return cmd
}

// uploadDocFile uploads a single file and returns its doc info map
// {docName, storageId, docHash} parsed from the MaaS upload response.
// It uploads WITHOUT a knowledgeId so no auto-import happens here; the actual
// import is performed separately per type group in the two-phase flow (this
// mirrors HiDonalds upload_file + import_docs and avoids "文档 id 重复").
func uploadDocFile(c *client.MaasClient, filePath string) (map[string]interface{}, error) {
	resp, err := c.UploadFile("/api/maas-model-api/api/datacenter/v1/knowledge/doc/upload/create", filePath, "")
	if err != nil {
		return nil, err
	}
	info, err := parseUploadedDocInfo(resp.Body)
	if err != nil {
		return nil, err
	}
	if info["docName"] == nil {
		info["docName"] = filepath.Base(filePath)
	}
	return info, nil
}

// uploadDocForUpload uploads a single file according to its type. For images it
// first runs the visual model (analyzeImage) to get a text summary plus the
// original image, assembles a temp docx (generateDocx), and uploads that docx.
// For all other types it uploads the raw file. Returns the uploaded doc info map.
func uploadDocForUpload(c *client.MaasClient, filePath string, pt int, apiKey string, analyzer *config.LLMAnalyzer) (map[string]interface{}, error) {
	if pt != parseTypeImage || apiKey == "" || analyzer == nil {
		// Non-image, or no visual model configured: upload the raw file.
		return uploadDocFile(c, filePath)
	}

	// Image pipeline (mirrors HiDonalds batch_analyze_images_to_temp): analyze,
	// generate a docx carrying the text + image, then upload that docx.
	res, err := analyzeImage(apiKey, filePath, analyzer)
	if err != nil {
		return nil, err
	}

	docxBytes, err := generateDocx(res.Text, res.ImageB64, res.ImageExt)
	if err != nil {
		return nil, err
	}

	// Name the docx after the original image file so the imported document keeps
	// the image's name (the platform names the doc after the uploaded file's
	// basename). Use <imageBase>.docx as the temp file name.
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	imageBase := base
	if ext != "" {
		imageBase = strings.TrimSuffix(base, ext)
	}
	tmpName := filepath.Join(os.TempDir(), imageBase+".docx")
	if err := os.WriteFile(tmpName, docxBytes, 0o644); err != nil {
		return nil, err
	}
	defer os.Remove(tmpName)

	return uploadDocFile(c, tmpName)
}

// parseUploadedDocInfo extracts {docName, storageId, docHash} from the upload
// response body. The MaaS success payload is under data, an array whose first
// element holds the fields.
func parseUploadedDocInfo(body []byte) (map[string]interface{}, error) {
	var result model.APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析上传响应失败: %w", err)
	}
	if !isCodeOK(result.Code) {
		return nil, fmt.Errorf("上传失败: code=%s message=%s", result.Code, result.Message)
	}

	raw, err := json.Marshal(result.Data)
	if err != nil {
		return nil, fmt.Errorf("解析上传响应 data: %w", err)
	}

	// data is usually a single-element array, e.g. [{docName, storageId, docHash, ...}]
	var arr []map[string]interface{}
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr[0], nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj, nil
	}

	return nil, fmt.Errorf("无法解析上传响应结构: %s", string(raw))
}

// importDocs imports a group of uploaded docs into the knowledge base with the
// given chunkConfig. docs are [{docName, storageId, docHash}, ...].
func importDocs(c *client.MaasClient, knowledgeID string, docs []map[string]interface{}, chunkConfig map[string]interface{}, label string) error {
	reqBody := map[string]interface{}{
		"knowledgeId": knowledgeID,
		"docs":        docs,
	}
	if len(chunkConfig) > 0 {
		reqBody["chunkConfig"] = chunkConfig
	}

	resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/import", reqBody)
	if err != nil {
		return err
	}
	return printResponse(resp)
}

// --- file type grouping & per-type chunkConfig ---
//
// These replicate HiDonalds' get_file_parse_type / _chunk_config_* logic so the
// refined upload imports each document type with its own parsing rules.

// fileParseType constants (mirror the MaaS / HiDonalds values).
const (
	parseTypeTxt   = 0 // txt 文本(B)
	parseTypeText  = 1 // 文本
	parseTypeTable = 2 // 表格
	parseTypeImage = 3 // 图片
)

var (
	textExtensions  = []string{".doc", ".docx", ".pdf", ".pptx", ".ppt", ".md", ".custom"}
	txtExtensions   = []string{".txt"}
	tableExtensions = []string{".xlsx", ".xls", ".csv"}
	imageExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}
)

// getFileParseType returns the file parse type for a file path, by extension.
// Mirrors HiDonalds hikvision_api.get_file_parse_type.
func getFileParseType(filePath string) int {
	ext := strings.ToLower(filepath.Ext(filePath))
	if contains(textExtensions, ext) {
		return parseTypeText
	}
	if contains(tableExtensions, ext) {
		return parseTypeTable
	}
	if contains(imageExtensions, ext) {
		return parseTypeImage
	}
	if contains(txtExtensions, ext) {
		return parseTypeTxt
	}
	fmt.Printf("⚠️ 未知文件类型: %s，默认按文本文件处理\n", ext)
	return parseTypeText
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// chunkConfigForParseType returns the chunkConfig and a human label for a given
// file parse type. These defaults mirror the HiDonalds _chunk_config_* builders.
func chunkConfigForParseType(pt int) (map[string]interface{}, string) {
	switch pt {
	case parseTypeTable:
		// table docs keep native table chunking, with image (VLM) parsing enabled.
		return map[string]interface{}{
			"fileParseType":     "2",
			"imageEnable":       true,
			"imageParseType":    3,
			"chunkMethod":       1,
			"tableChunkMode":    2,
			"enableRefFile":     true,
			"enableRefTitle":    true,
			"generateQuestions": true,
		}, "表格"
	case parseTypeImage:
		// Placeholder: image is imported with the image-type chunkConfig. The Qwen
		// image analysis step (generating a temp docx) from HiDGE is NOT implemented
		// yet — TODO: 接入 Qwen 识图，先按图片型导入原图。
		return map[string]interface{}{
			"fileParseType":     "1",
			"imageEnable":       true,
			"imageParseType":    3,
			"chunkType":         2,
			"chunkLength":       2000,
			"maxOverlapRatio":   20,
			"enableRefFile":     true,
			"enableRefTitle":    true,
			"titleDepth":        1,
			"generateQuestions": true,
		}, "图片"
	case parseTypeTxt:
		// txt 文本(B): image parsing disabled.
		return map[string]interface{}{
			"fileParseType":     "1",
			"imageEnable":       false,
			"tableEnable":       true,
			"formulaEnable":     true,
			"chunkType":         2,
			"chunkLength":       1000,
			"maxOverlapRatio":   20,
			"enableRefFile":     true,
			"enableRefTitle":    true,
			"titleDepth":        3,
			"generateQuestions": true,
		}, "txt文本"
	default: // text
		return map[string]interface{}{
			"fileParseType":     "1",
			"imageEnable":       true,
			"imageParseType":    3,
			"tableEnable":       true,
			"formulaEnable":     true,
			"chunkType":         2,
			"chunkLength":       1000,
			"maxOverlapRatio":   20,
			"enableRefFile":     true,
			"enableRefTitle":    true,
			"titleDepth":        3,
			"generateQuestions": true,
		}, "文本"
	}
}

// --- knowledge doc import ---

var (
	importStorageID string
	importDocHash   string
	importDocName   string
	importParseType int
)

func newDocImportCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an uploaded document into a knowledge base",
		Long: `Import an already-uploaded document (identified by storageId and docHash) into a knowledge base.

Use --knowledge to specify a knowledge base by name (from config), or --knowledge-id for a direct ID.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/import`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docs": []map[string]interface{}{
					{
						"docName":   importDocName,
						"storageId": importStorageID,
						"docHash":   importDocHash,
					},
				},
			}

			// When --parse-type is provided, apply the full per-type chunkConfig
			// (not just fileParseType) for refined parsing rules.
			if cmd.Flags().Changed("parse-type") {
				cc, _ := chunkConfigForParseType(importParseType)
				reqBody["chunkConfig"] = cc
			} else if importParseType > 0 {
				reqBody["chunkConfig"] = map[string]interface{}{
					"fileParseType": importParseType,
				}
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/import", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&importDocName, "doc-name", "", "Document name (required)")
	cmd.Flags().StringVar(&importStorageID, "storage-id", "", "Storage ID from upload response (required)")
	cmd.Flags().StringVar(&importDocHash, "doc-hash", "", "Document hash from upload response (required)")
	cmd.Flags().IntVar(&importParseType, "parse-type", 1, "File parse type: 1=text, 2=table, 3=QA, 4=custom, 6=image, 7=video, 8=audio")
	cmd.MarkFlagRequired("doc-name")
	cmd.MarkFlagRequired("storage-id")
	cmd.MarkFlagRequired("doc-hash")

	return cmd
}

// --- knowledge doc create (blank document) ---

var (
	docCreateName        string
	docCreateImageEnable bool
	docCreateImageParse  int
)

func newDocCreateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a blank document in a knowledge base",
		Long: `Create a blank document (.custom) in a knowledge base.

The document name must end with ".custom".

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docs": []map[string]interface{}{
					{"docName": docCreateName},
				},
			}

			if cmd.Flags().Changed("image-enable") || docCreateImageParse > 0 {
				chunkConfig := map[string]interface{}{}
				if cmd.Flags().Changed("image-enable") {
					chunkConfig["imageEnable"] = docCreateImageEnable
				}
				if docCreateImageParse > 0 {
					chunkConfig["imageParseType"] = docCreateImageParse
				}
				reqBody["chunkConfig"] = chunkConfig
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/create", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docCreateName, "doc-name", "", "Document name, must end with .custom (required)")
	cmd.Flags().BoolVar(&docCreateImageEnable, "image-enable", false, "Enable image parsing")
	cmd.Flags().IntVar(&docCreateImageParse, "image-parse-type", 0, "Image parse type: 2=OCR, 3=VLM (requires --image-enable)")
	cmd.MarkFlagRequired("doc-name")

	return cmd
}

// --- knowledge doc enable ---

func newDocEnableCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable a document",
		Long: `Enable a document in a knowledge base. Supports batch by comma-separating doc IDs.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/enable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       docID,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/enable", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docID, "id", "", "Document ID(s), comma-separated for batch (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- knowledge doc disable ---

func newDocDisableCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a document",
		Long: `Disable a document in a knowledge base. Supports batch by comma-separating doc IDs.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/disable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       docID,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/disable", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docID, "id", "", "Document ID(s), comma-separated for batch (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- knowledge doc delete ---

func newDocDeleteCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a document",
		Long: `Delete a document from a knowledge base.

⚠️  This is a destructive operation and cannot be undone.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       docID,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/delete", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docID, "id", "", "Document ID (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- knowledge doc view (preview link) ---

func newDocViewCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Get document preview link",
		Long: `Get a preview link for a document in a knowledge base.

API: GET /api/maas-model-api/api/datacenter/v1/knowledge/doc/view/{knowledgeId}/{docId}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			path := fmt.Sprintf("/api/maas-model-api/api/datacenter/v1/knowledge/doc/view/%s/%s", id, docID)
			resp, err := c.Get(path)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docID, "id", "", "Document ID (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- knowledge doc download ---

func newDocDownloadCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Get document download link",
		Long: `Get a download link for a document in a knowledge base.

If the download request fails (e.g. HTTP 403), the response body is inspected
for a download URL and displayed if available.

API: GET /api/maas-model-api/api/datacenter/v1/knowledge/doc/downloadUrl/{knowledgeId}/{docId}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			path := fmt.Sprintf("/api/maas-model-api/api/datacenter/v1/knowledge/doc/downloadUrl/%s/%s", id, docID)
			resp, err := c.Get(path)
			if err != nil {
				// client.doJSON returns both resp and err when HTTP status is non-2xx
				if resp != nil && len(resp.Body) > 0 {
					// Try to extract downloadUrl from the response body
					if downloadURL := extractDownloadURL(resp.Body); downloadURL != "" {
						fmt.Printf("⚠️  下载请求返回 HTTP %d，但获取到下载链接：\n%s\n", resp.StatusCode, downloadURL)
						return nil
					}
					// Cannot extract URL, print raw response body
					fmt.Printf("⚠️  下载请求失败 (HTTP %d)：\n%s\n", resp.StatusCode, string(resp.Body))
					return nil
				}
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docID, "id", "", "Document ID (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

// extractDownloadURL tries to extract a downloadUrl field from the API response body.
func extractDownloadURL(body []byte) string {
	var result model.APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		return ""
	}

	// Try {"downloadUrl": "..."} format
	var wrapper struct {
		DownloadURL string `json:"downloadUrl"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.DownloadURL != "" {
		return wrapper.DownloadURL
	}

	return ""
}

// --- knowledge doc graph-restart ---

var docGraphRestartIDs string

func newDocGraphRestartCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph-restart",
		Short: "Restart document graph creation",
		Long: `Restart knowledge graph creation for specific documents in a knowledge base (灰度中).

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/graph/restart`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(docKnowledgeName, docKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docIds":      splitCSV(docGraphRestartIDs),
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/graph/restart", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&docKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&docKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&docGraphRestartIDs, "doc-ids", "", "Document IDs (comma-separated, required)")
	cmd.MarkFlagRequired("doc-ids")

	return cmd
}
