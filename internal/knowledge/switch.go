/**
 * @date 2026/8/13
 * @description 实现 knowledge-cli switch 命令，切换默认知识库（同时更新 defaultKnowledgeId 和 defaultKnowledgeName）。
 * @since v0.4.0
 */

package knowledge

import (
	"fmt"

	"github.com/spf13/cobra"
	"knowledge-cli/internal/config"
)

// --- switch ---

var switchKnowledgeName string

func newSwitchCmd(apiKeyPtr func() *string, baseURLPtr func() *string, timeoutPtr func() *int, insecurePtr func() *bool, configPtr func() *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch",
		Short: "Switch default knowledge base",
		Long: `Switch the default knowledge base used when --knowledge or --knowledge-id is not specified.

Updates both defaultKnowledgeId and defaultKnowledgeName in the config file.

Examples:
  # Switch default knowledge base by name
  knowledge-cli switch --knowledge "电力运检辅助平台"

  # Show current default and all available knowledge bases
  knowledge-cli switch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := *configPtr()
			cfg, err := config.LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// If no --knowledge specified, just show current status
			if switchKnowledgeName == "" {
				fmt.Printf("当前默认知识库：%s\n", cfg.DefaultKnowledgeName)
				fmt.Printf("默认知识库 ID：%s\n\n", cfg.DefaultKnowledgeId)
				fmt.Println("可用知识库：")
				for _, ws := range cfg.Workspaces {
					fmt.Printf("  工作空间 (API Key: %s...%s)\n", ws.APIKey[:6], ws.APIKey[len(ws.APIKey)-4:])
					for _, kb := range ws.KnowledgeBases {
						marker := ""
						if kb.ID == cfg.DefaultKnowledgeId || kb.Name == cfg.DefaultKnowledgeName {
							marker = " ← 当前默认"
						}
						fmt.Printf("    - %s (%s)%s\n", kb.Name, kb.ID, marker)
					}
				}
				return nil
			}

			// Find the knowledge base by name
			id, _, ok := cfg.ResolveKnowledgeByName(switchKnowledgeName)
			if !ok {
				return fmt.Errorf("knowledge base %q not found in config (available: %v)", switchKnowledgeName, cfg.AllNames())
			}

			// Update both fields
			cfg.DefaultKnowledgeId = id
			cfg.DefaultKnowledgeName = switchKnowledgeName

			if err := config.SaveConfig(cfgPath, cfg); err != nil {
				return err
			}

			fmt.Printf("✅ 默认知识库已切换为：%s (%s)\n", switchKnowledgeName, id)
			return nil
		},
	}

	cmd.Flags().StringVar(&switchKnowledgeName, "knowledge", "", "Knowledge base name to switch to")

	return cmd
}
