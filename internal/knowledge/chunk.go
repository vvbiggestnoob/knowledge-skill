/**
 * @date 2026/8/12
 * @description 实现文档分片管理命令，包括分片查询、新增、更新、删除、关联分片、关联问题和关联图片。
 * @since v0.2.0
 */

package knowledge

import (
	"fmt"

	"github.com/spf13/cobra"
)

// --- chunk parent command ---

var (
	chunkKnowledgeID   string
	chunkKnowledgeName string
	chunkDocID         string
	chunkID            string
)

func newChunkCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chunk",
		Short: "Manage document chunks",
		Long:  `Commands for managing document chunks: get, add, update, delete, relate, and manage associated questions/images.`,
	}

	cmd.AddCommand(newChunkGetCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkAddCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkAddAutoCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkUpdateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkDeleteCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkRelateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkQuestionCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkImageCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))

	return cmd
}

// --- chunk get ---

func newChunkGetCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a chunk by ID",
		Long: `Get detailed information about a document chunk by document ID and chunk ID.

API: GET /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/{docId}/{chunkId}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}
			_ = id // knowledgeId not needed in path but resolved for validation

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			path := fmt.Sprintf("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/%s/%s", chunkDocID, chunkID)
			resp, err := c.Get(path)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkID, "id", "", "Chunk ID (required)")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- chunk add ---

var (
	chunkAddContent string
	chunkAddTitle   string
)

func newChunkAddCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new chunk to a document",
		Long: `Add a new custom chunk to a document.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/add`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId":  id,
				"docId":        chunkDocID,
				"chunkContent": chunkAddContent,
			}
			if chunkAddTitle != "" {
				reqBody["chunkTitle"] = chunkAddTitle
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/add", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkAddContent, "content", "", "Chunk content (required)")
	cmd.Flags().StringVar(&chunkAddTitle, "title", "", "Chunk title")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("content")

	return cmd
}

// --- chunk add-auto (auto-find or create custom-questions.md document) ---

var (
	chunkAddAutoContent string
	chunkAddAutoTitle   string
	chunkAddAutoDocName string
)

func newChunkAddAutoCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-auto",
		Short: "Add a chunk to custom-questions.md (auto-create doc if needed)",
		Long: `Add a chunk to the custom-questions.md document in the knowledge base.
If the document does not exist, it is created automatically.

This is a convenience command that combines doc create + chunk add.
No --doc-id is needed; the document is found or created automatically.

API:
  POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/list (search)
  POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/create (if not found)
  POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/add`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			// Find or create the custom-questions document
			docID, err := findOrCreateCustomQuestionsDoc(c, id, chunkAddAutoDocName)
			if err != nil {
				return fmt.Errorf("find or create doc: %w", err)
			}

			fmt.Printf("✅ 使用文档 %s (docId: %s)\n", chunkAddAutoDocName, docID)

			// Add chunk to the document
			reqBody := map[string]interface{}{
				"knowledgeId":  id,
				"docId":        docID,
				"chunkContent": chunkAddAutoContent,
			}
			if chunkAddAutoTitle != "" {
				reqBody["chunkTitle"] = chunkAddAutoTitle
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/add", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkAddAutoContent, "content", "", "Chunk content (required)")
	cmd.Flags().StringVar(&chunkAddAutoTitle, "title", "", "Chunk title")
	cmd.Flags().StringVar(&chunkAddAutoDocName, "doc-name", "custom-questions.md", "Document name for storing Q&A chunks")
	cmd.MarkFlagRequired("content")

	return cmd
}

// --- chunk update ---

var (
	chunkUpdateContent string
	chunkUpdateStatus  string
)

func newChunkUpdateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a chunk",
		Long: `Update a chunk's content and/or status.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			chunkObj := map[string]interface{}{
				"chunkId": chunkID,
			}
			if chunkUpdateStatus != "" {
				chunkObj["chunkStatus"] = chunkUpdateStatus
			}
			if chunkUpdateContent != "" {
				chunkObj["chunkContent"] = chunkUpdateContent
			}

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       chunkDocID,
				"chunks":      []map[string]interface{}{chunkObj},
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/update", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkID, "id", "", "Chunk ID (required)")
	cmd.Flags().StringVar(&chunkUpdateContent, "content", "", "New chunk content")
	cmd.Flags().StringVar(&chunkUpdateStatus, "status", "", "New chunk status (e.g. 1=enabled, 2=disabled)")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- chunk delete ---

var chunkDeleteIDs string

func newChunkDeleteCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Batch delete chunks",
		Long: `Batch delete document chunks by their IDs.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/batchDelete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       chunkDocID,
				"chunkIds":    splitCSV(chunkDeleteIDs),
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/batchDelete", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkDeleteIDs, "chunk-ids", "", "Chunk IDs (comma-separated, required)")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("chunk-ids")

	return cmd
}

// --- chunk relate ---

var (
	chunkRelatePrevious int
	chunkRelateNext     int
)

func newChunkRelateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relate",
		Short: "Get related chunks",
		Long: `Get chunks related to a given chunk (previous and next neighbors).

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/relate-list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       chunkDocID,
				"chunkId":     chunkID,
				"previous":    chunkRelatePrevious,
				"next":        chunkRelateNext,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/relate-list", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkID, "id", "", "Chunk ID (required)")
	cmd.Flags().IntVar(&chunkRelatePrevious, "previous", 1, "Number of previous chunks to retrieve")
	cmd.Flags().IntVar(&chunkRelateNext, "next", 1, "Number of next chunks to retrieve")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- chunk question (parent) ---

var (
	chunkQuesID      string
	chunkQuesContent string
)

func newChunkQuestionCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "question",
		Short: "Manage chunk associated questions",
		Long:  `Commands for managing questions associated with a document chunk.`,
	}

	cmd.AddCommand(newChunkQuestionAddCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkQuestionUpdateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkQuestionDeleteCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))

	return cmd
}

// --- chunk question add ---

func newChunkQuestionAddCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an associated question to a chunk",
		Long: `Add a new associated question to a document chunk.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/question/add`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			questionObj := map[string]interface{}{
				"questionContent": chunkQuesContent,
			}
			if chunkQuesID != "" {
				questionObj["questionId"] = chunkQuesID
			}

			reqBody := map[string]interface{}{
				"knowledgeId":      id,
				"docId":            chunkDocID,
				"chunkId":          chunkID,
				"questionInfoList": []map[string]interface{}{questionObj},
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/question/add", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkID, "id", "", "Chunk ID (required)")
	cmd.Flags().StringVar(&chunkQuesID, "question-id", "", "Question ID")
	cmd.Flags().StringVar(&chunkQuesContent, "question-content", "", "Question content (required)")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("question-content")

	return cmd
}

// --- chunk question update ---

func newChunkQuestionUpdateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an associated question of a chunk",
		Long: `Update the content of an associated question for a document chunk.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/question/update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			questionObj := map[string]interface{}{
				"questionContent": chunkQuesContent,
			}
			if chunkQuesID != "" {
				questionObj["questionId"] = chunkQuesID
			}

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       chunkDocID,
				"chunkId":     chunkID,
				"question":    questionObj,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/question/update", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkID, "id", "", "Chunk ID (required)")
	cmd.Flags().StringVar(&chunkQuesID, "question-id", "", "Question ID (optional)")
	cmd.Flags().StringVar(&chunkQuesContent, "question-content", "", "New question content (required)")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("question-content")

	return cmd
}

// --- chunk question delete ---

func newChunkQuestionDeleteCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an associated question from a chunk",
		Long: `Delete an associated question from a document chunk by question ID.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/question/delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       chunkDocID,
				"chunkId":     chunkID,
				"questionId":  chunkQuesID,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/question/delete", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkID, "id", "", "Chunk ID (required)")
	cmd.Flags().StringVar(&chunkQuesID, "question-id", "", "Question ID (required)")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("question-id")

	return cmd
}

// --- chunk image (parent) ---

var (
	chunkImageID   string
	chunkImageDesc string
)

func newChunkImageCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage chunk associated images",
		Long:  `Commands for managing images associated with a document chunk.`,
	}

	cmd.AddCommand(newChunkImageUpdateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newChunkImageDeleteCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))

	return cmd
}

// --- chunk image update ---

func newChunkImageUpdateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an associated image of a chunk",
		Long: `Update the description of an image associated with a document chunk.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/image/update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       chunkDocID,
				"chunkId":     chunkID,
				"image": map[string]interface{}{
					"imageId":   chunkImageID,
					"imageDesc": chunkImageDesc,
				},
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/image/update", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkID, "id", "", "Chunk ID (required)")
	cmd.Flags().StringVar(&chunkImageID, "image-id", "", "Image ID (required)")
	cmd.Flags().StringVar(&chunkImageDesc, "image-desc", "", "New image description (required)")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("image-id")
	cmd.MarkFlagRequired("image-desc")

	return cmd
}

// --- chunk image delete ---

func newChunkImageDeleteCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an associated image from a chunk",
		Long: `Delete an image associated with a document chunk by image ID.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/image/update (delete semantics)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, apiKey, err := resolveKnowledge(chunkKnowledgeName, chunkKnowledgeID, *configPtr())
			if err != nil {
				return err
			}

			c := newMaasClientWithKey(apiKey, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr)

			// The delete image API uses the same endpoint as update, with imageId only
			reqBody := map[string]interface{}{
				"knowledgeId": id,
				"docId":       chunkDocID,
				"chunkId":     chunkID,
				"image": map[string]interface{}{
					"imageId": chunkImageID,
				},
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/doc/chunk/image/update", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&chunkKnowledgeName, "knowledge", "", "Knowledge base name (from config)")
	cmd.Flags().StringVar(&chunkKnowledgeID, "knowledge-id", "", "Knowledge base ID (overrides --knowledge)")
	cmd.Flags().StringVar(&chunkDocID, "doc-id", "", "Document ID (required)")
	cmd.Flags().StringVar(&chunkID, "id", "", "Chunk ID (required)")
	cmd.Flags().StringVar(&chunkImageID, "image-id", "", "Image ID (required)")
	cmd.MarkFlagRequired("doc-id")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("image-id")

	return cmd
}
