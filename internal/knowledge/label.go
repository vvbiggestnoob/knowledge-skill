/**
 * @date 2026/8/12
 * @description 实现标签管理命令，包括新增、分页查询、更新和删除标签。
 * @since v0.2.0
 */

package knowledge

import (
	"strings"

	"github.com/spf13/cobra"
)

// --- label parent command ---

var (
	labelID        string
	labelName      string
	labelValues    string
	labelPageStart int
	labelPageSize  int
)

func newLabelCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage labels",
		Long:  `Commands for managing knowledge base labels: add, list, update, delete.`,
	}

	cmd.AddCommand(newLabelAddCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newLabelListCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newLabelUpdateCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))
	cmd.AddCommand(newLabelDeleteCmd(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr))

	return cmd
}

// --- label add ---

func newLabelAddCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a label",
		Long: `Create a new label with a name and one or more values.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/label/add`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			values := make([]map[string]string, 0)
			for _, v := range splitCSV(labelValues) {
				values = append(values, map[string]string{"labelValue": v})
			}

			reqBody := map[string]interface{}{
				"label": map[string]interface{}{
					"labelName": labelName,
					"values":    values,
				},
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/label/add", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&labelName, "name", "", "Label name (required)")
	cmd.Flags().StringVar(&labelValues, "values", "", "Label values (comma-separated, required)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("values")

	return cmd
}

// --- label list ---

var (
	labelKeyword string
	labelListID  string
)

func newLabelListCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List labels",
		Long: `List labels with optional keyword search and pagination.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/label/list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			reqBody := map[string]interface{}{
				"pageStart": labelPageStart,
				"pageSize":  labelPageSize,
			}
			if labelKeyword != "" {
				reqBody["keyword"] = labelKeyword
			}
			if labelListID != "" {
				reqBody["labelId"] = labelListID
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/label/list", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&labelKeyword, "keyword", "", "Label name keyword search")
	cmd.Flags().StringVar(&labelListID, "label-id", "", "Label ID")
	cmd.Flags().IntVar(&labelPageStart, "page-start", 1, "Page start")
	cmd.Flags().IntVar(&labelPageSize, "page-size", 10, "Page size")

	return cmd
}

// --- label update ---

var (
	labelUpdateID        string
	labelUpdateAddValues string
	labelUpdateDelValues string
)

func newLabelUpdateCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a label",
		Long: `Update a label: add new values and/or remove existing values.

Use --add-values to add new label values (comma-separated).
Use --delete-values to remove existing label values (comma-separated "valueId:labelValue" pairs).

Note: the MaaS label/update API does not support renaming a label (the labelName field is ignored and the call still returns success), so there is no --name flag.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/label/update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			labelObj := map[string]interface{}{
				"labelId": labelUpdateID,
			}

			// Parse add values
			if labelUpdateAddValues != "" {
				addVals := make([]map[string]string, 0)
				for _, v := range splitCSV(labelUpdateAddValues) {
					addVals = append(addVals, map[string]string{"labelValue": v})
				}
				labelObj["addValues"] = addVals
			}

			// Parse delete values: "valueId:labelValue" pairs
			if labelUpdateDelValues != "" {
				delVals := make([]map[string]string, 0)
				for _, pair := range splitCSV(labelUpdateDelValues) {
					parts := strings.SplitN(pair, ":", 2)
					entry := map[string]string{"valueId": parts[0]}
					if len(parts) > 1 {
						entry["labelValue"] = parts[1]
					}
					delVals = append(delVals, entry)
				}
				labelObj["deleteValues"] = delVals
			}

			reqBody := map[string]interface{}{
				"label": labelObj,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/label/update", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&labelUpdateID, "label-id", "", "Label ID (required)")
	cmd.Flags().StringVar(&labelUpdateAddValues, "add-values", "", "Label values to add (comma-separated)")
	cmd.Flags().StringVar(&labelUpdateDelValues, "delete-values", "", "Label values to delete (comma-separated \"valueId:labelValue\" pairs)")
	cmd.MarkFlagRequired("label-id")

	return cmd
}

// --- label delete ---

var labelDeleteID string

func newLabelDeleteCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a label",
		Long: `Delete a label and all its values by label ID.

⚠️  This is a destructive operation and cannot be undone.

API: POST /api/maas-model-api/api/datacenter/v1/knowledge/label/third-delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newMaasClient(apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)

			reqBody := map[string]interface{}{
				"labelId": labelDeleteID,
			}

			resp, err := c.Post("/api/maas-model-api/api/datacenter/v1/knowledge/label/third-delete", reqBody)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}

	cmd.Flags().StringVar(&labelDeleteID, "label-id", "", "Label ID (required)")
	cmd.MarkFlagRequired("label-id")

	return cmd
}
