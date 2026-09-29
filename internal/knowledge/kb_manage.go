/**
 * @date 2026/8/12
 * @description 实现知识库创建、修改、删除及图谱批量管理命令。
 * @since v0.2.0
 */

package knowledge

import (
	"strings"

	"github.com/spf13/cobra"
)

// --- knowledge create ---

var (
	createName             string
	createDetail           string
	createGraph            bool
	createGraphEntityTypes string
	createGraphMethod      int
	createGraphResolution  bool
	createGraphCommunity   bool
)

func newCreateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a knowledge base",
		Long: `Create a new knowledge base with a name and description.

Optionally configure knowledge graph (灰度中).

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			reqBody := map[string]interface{}{
				"name":   createName,
				"detail": createDetail,
			}

			if createGraph || createGraphEntityTypes != "" || createGraphMethod > 0 {
				kgConfig := map[string]interface{}{
					"knowledgeGraph": createGraph,
				}
				if createGraphEntityTypes != "" {
					kgConfig["graphEntityTypes"] = splitCSV(createGraphEntityTypes)
				}
				if createGraphMethod > 0 {
					kgConfig["graphMethod"] = createGraphMethod
				}
				kgConfig["graphWithResolution"] = createGraphResolution
				kgConfig["graphWithCommunity"] = createGraphCommunity
				reqBody["knowledgeGraphConfig"] = kgConfig
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/create", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&createName, "name", "", "Knowledge base name (required)")
	cmd.Flags().StringVar(&createDetail, "detail", "", "Knowledge base description (required)")
	cmd.Flags().BoolVar(&createGraph, "graph", false, "Enable knowledge graph (灰度中)")
	cmd.Flags().StringVar(&createGraphEntityTypes, "graph-entity-types", "", "Knowledge graph entity types (comma-separated)")
	cmd.Flags().IntVar(&createGraphMethod, "graph-method", 0, "Knowledge graph method: 1=GraphRAG, 2=LightRAG")
	cmd.Flags().BoolVar(&createGraphResolution, "graph-resolution", false, "Knowledge graph entity resolution")
	cmd.Flags().BoolVar(&createGraphCommunity, "graph-community", false, "Knowledge graph community report generation")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("detail")

	return cmd
}

// --- knowledge update ---

var (
	updateID               string
	updateName             string
	updateDetail           string
	updateGraph            bool
	updateGraphEntityTypes string
	updateGraphMethod      int
	updateGraphResolution  bool
	updateGraphCommunity   bool
)

func newUpdateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a knowledge base",
		Long: `Update a knowledge base name, description, and optional graph config.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			reqBody := map[string]interface{}{
				"id": updateID,
			}
			if updateName != "" {
				reqBody["name"] = updateName
			}
			if updateDetail != "" {
				reqBody["detail"] = updateDetail
			}

			if cmd.Flags().Changed("graph") || updateGraphEntityTypes != "" || updateGraphMethod > 0 {
				kgConfig := map[string]interface{}{
					"knowledgeGraph": updateGraph,
				}
				if updateGraphEntityTypes != "" {
					kgConfig["graphEntityTypes"] = splitCSV(updateGraphEntityTypes)
				}
				if updateGraphMethod > 0 {
					kgConfig["graphMethod"] = updateGraphMethod
				}
				kgConfig["graphWithResolution"] = updateGraphResolution
				kgConfig["graphWithCommunity"] = updateGraphCommunity
				reqBody["knowledgeGraphConfig"] = kgConfig
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/update", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&updateID, "id", "", "Knowledge base ID (required)")
	cmd.Flags().StringVar(&updateName, "name", "", "New knowledge base name")
	cmd.Flags().StringVar(&updateDetail, "detail", "", "New knowledge base description")
	cmd.Flags().BoolVar(&updateGraph, "graph", false, "Enable knowledge graph (灰度中)")
	cmd.Flags().StringVar(&updateGraphEntityTypes, "graph-entity-types", "", "Knowledge graph entity types (comma-separated)")
	cmd.Flags().IntVar(&updateGraphMethod, "graph-method", 0, "Knowledge graph method: 1=GraphRAG, 2=LightRAG")
	cmd.Flags().BoolVar(&updateGraphResolution, "graph-resolution", false, "Knowledge graph entity resolution")
	cmd.Flags().BoolVar(&updateGraphCommunity, "graph-community", false, "Knowledge graph community report generation")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- knowledge delete ---

var deleteID string

func newDeleteCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a knowledge base",
		Long: `Delete a knowledge base by its ID.

⚠️  This is a destructive operation and cannot be undone.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			reqBody := map[string]interface{}{
				"knowledgeId": deleteID,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/delete", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&deleteID, "id", "", "Knowledge base ID (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

// --- knowledge graph ---

var graphIDs string

func newGraphCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Manage knowledge base graphs",
		Long:  `Batch create or delete knowledge graphs for knowledge bases (灰度中).`,
	}

	cmd.AddCommand(newGraphCreateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newGraphDeleteCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))

	return cmd
}

func newGraphCreateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Batch create knowledge base graphs",
		Long: `Batch create knowledge graphs for the specified knowledge bases (灰度中).

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/graph/batch-create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			reqBody := map[string]interface{}{
				"knowledgeBaseIds": splitCSV(graphIDs),
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/graph/batch-create", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&graphIDs, "ids", "", "Knowledge base IDs (comma-separated, required)")
	cmd.MarkFlagRequired("ids")

	return cmd
}

func newGraphDeleteCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Batch delete knowledge base graphs",
		Long: `Batch delete knowledge graphs for the specified knowledge bases (灰度中).

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/graph/batch-delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			reqBody := map[string]interface{}{
				"knowledgeBaseIds": splitCSV(graphIDs),
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/graph/batch-delete", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&graphIDs, "ids", "", "Knowledge base IDs (comma-separated, required)")
	cmd.MarkFlagRequired("ids")

	return cmd
}

// splitCSV splits a comma-separated string into a string slice, trimming whitespace from each element.
func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
