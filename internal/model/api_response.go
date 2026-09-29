/**
 * @date 2026/8/10
 * @description 实现 MaaS API 通用响应模型。
 * @since v1.0
 */

package model

// APIResponse is the common JSON envelope returned by MaaS platform APIs.
type APIResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}