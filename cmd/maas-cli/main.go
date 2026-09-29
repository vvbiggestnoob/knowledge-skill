/**
 * @date 2026/8/10
 * @description 实现 knowledge-cli 入口，基于 cobra 框架组织知识库相关命令。
 * @since v1.0
 */

package main

import (
	"fmt"
	"os"

	"knowledge-cli/internal/knowledge"
	"github.com/spf13/cobra"
)

var (
	apiKey     string
	baseURL    string
	timeout    int
	insecure   bool
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "knowledge-cli",
	Short: "Hikvision MaaS knowledge base CLI",
	Long: `knowledge-cli is a CLI tool for the Hikvision MaaS (Model as a Service) platform.
It provides commands for managing knowledge bases, viewing documents, and searching text.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "MaaS platform API key (can also be set in config file)")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "https://maas.hikvision.com.cn", "MaaS platform base URL")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 30, "Request timeout in seconds")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "conf/knowledge_bases.json", "Knowledge base configuration file path")

	knowledge.RegisterCommands(rootCmd, apiKeyPtr, baseURLPtr, timeoutPtr, insecurePtr, configPtr)
}

func apiKeyPtr() *string  { return &apiKey }
func baseURLPtr() *string { return &baseURL }
func timeoutPtr() *int    { return &timeout }
func insecurePtr() *bool  { return &insecure }
func configPtr() *string  { return &configPath }

func main() {
	Execute()
}