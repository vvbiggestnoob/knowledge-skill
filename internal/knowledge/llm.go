/**
 * @date 2026/8/20
 * @description 图片分析：调用 OpenAI 兼容（Qwen）视觉模型识别本地图片。
 *              复用知识库工作空间的 api-key，结合 config 提供的 baseUrl/model/prompt。
 *              ported from HiDonalds/LLMTool.py analyze_image.
 */

package knowledge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"knowledge-cli/internal/config"
)

// ImageAnalysis is the result of analyzing one image: the model's text summary
// plus the original image bytes encoded as base64 (reused when embedding it in
// the generated docx).
type ImageAnalysis struct {
	Text        string
	ImageB64    string
	ImageExt    string
	ContentType string
}

// analyzeImage sends a single image to the configured visual model and returns
// its text summary plus the base64 image bytes. It mirrors LLMTool.analyze_image.
//
// apiKey is the MaaS knowledge-base workspace key (which also works for the
// visual model). analyzer provides baseUrl/model/prompt.
func analyzeImage(apiKey, imagePath string, analyzer *config.LLMAnalyzer) (*ImageAnalysis, error) {
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("读取图片失败: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(imageBytes)

	// Detect the actual content type from magic bytes (a file's extension can
	// be misleading, e.g. a .png containing JPEG bytes). This is needed both for
	// the model request and for embedding the image in the generated docx.
	ext, contentType := detectImageType(imageBytes)

	url := strings.TrimRight(analyzer.BaseURL, "/") + "/chat/completions"

	reqBody := map[string]interface{}{
		"model": analyzer.Model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "image_url",
						"image_url": map[string]interface{}{
							"url": "data:" + contentType + ";base64," + b64,
						},
					},
					{
						"type": "text",
						"text": analyzer.Prompt,
					},
				},
			},
		},
		"max_tokens": 1024,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("构建请求体失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用视觉模型失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析视觉模型响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := ""
		if result.Error != nil {
			msg = result.Error.Message
		}
		return nil, fmt.Errorf("视觉模型 HTTP %d: %s", resp.StatusCode, msg)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("视觉模型未返回内容")
	}

	return &ImageAnalysis{
		Text:        result.Choices[0].Message.Content,
		ImageB64:    b64,
		ImageExt:    ext,
		ContentType: contentType,
	}, nil
}

// detectImageType returns the file extension and MIME content type based on the
// image magic bytes, falling back to JPEG when unrecognized.
func detectImageType(imageBytes []byte) (string, string) {
	if len(imageBytes) >= 8 &&
		imageBytes[0] == 0x89 && imageBytes[1] == 0x50 && imageBytes[2] == 0x4E && imageBytes[3] == 0x47 {
		return "png", "image/png"
	}
	if len(imageBytes) >= 3 && imageBytes[0] == 0xFF && imageBytes[1] == 0xD8 && imageBytes[2] == 0xFF {
		return "jpg", "image/jpeg"
	}
	if len(imageBytes) >= 6 &&
		imageBytes[0] == 'G' && imageBytes[1] == 'I' && imageBytes[2] == 'F' && imageBytes[3] == '8' {
		return "gif", "image/gif"
	}
	if len(imageBytes) >= 12 &&
		imageBytes[8] == 'W' && imageBytes[9] == 'E' && imageBytes[10] == 'B' && imageBytes[11] == 'P' {
		return "webp", "image/webp"
	}
	// Default
	return "jpg", "image/jpeg"
}
