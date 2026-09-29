import base64
import requests
import json
from openai import OpenAI
from typing import Optional, Dict, Any
from config import API_KEY, BASE_URL, MODEL_QWEN, IMAGER_PROMPT

class ImageAnalyzer:
    """图片分析接口类，用于提取和总结图片信息"""
    
    def __init__(
        self,
        api_key: str = API_KEY,
        base_url: str = BASE_URL,
        model: str = MODEL_QWEN,
        max_tokens: int = 1024
    ):
        """
        初始化图片分析器
        
        Args:
            api_key: API密钥
            base_url: API基础URL
            model: 模型名称
            max_tokens: 最大输出token数
        """
        self.client = OpenAI(api_key=api_key, base_url=base_url)
        self.model = model
        self.max_tokens = max_tokens
    
    def _get_http_b64(self, image_url: str) -> str:
        """从HTTP URL获取图片的base64编码"""
        response = requests.get(image_url)
        if response.status_code == 200:
            image_data = response.content
            b64 = base64.b64encode(image_data).decode('utf-8')
            return b64
        raise Exception(f"图像获取失败，状态码: {response.status_code}, 响应: {response.text}")
    
    def _get_local_b64(self, image_path: str) -> str:
        """从本地路径获取图片的base64编码"""
        with open(image_path, "rb") as f:
            image_data = f.read()
        b64 = base64.b64encode(image_data).decode('utf-8')
        return b64
    
    def _get_b64(self, image_url: str) -> str:
        """获取图片的base64编码，支持HTTP URL和本地路径"""
        if image_url.startswith(('http://', 'https://')):
            return self._get_http_b64(image_url)
        else:
            return self._get_local_b64(image_url)
    
    def _build_messages(self, text: str, image_url: str) -> list:
        """构建消息格式"""
        b64 = self._get_b64(image_url)
        return [
            {
                "role": "user",
                "content": [
                    {
                        "type": "image_url",
                        "image_url": {
                            "url": f"data:image/jpeg;base64,{b64}"
                        },
                    },
                    {
                        "type": "text",
                        "text": text
                    },
                ],
            }
        ]
    
    def analyze(
        self,
        image_url: str,
        custom_prompt: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        分析图片并返回结果
        
        Args:
            image_url: 图片URL或本地路径
            custom_prompt: 自定义提示词，如果不提供则使用默认提示词
            
        Returns:
            Dict包含:
                - success: bool, 是否成功
                - markdown: str, Markdown格式的输出
                - raw_response: dict, 原始API响应
                - error: str, 错误信息(如果有)
        """
        # 默认提示词
        default_prompt = IMAGER_PROMPT
        
        prompt = custom_prompt if custom_prompt else default_prompt
        
        try:
            # 构建消息
            messages = self._build_messages(prompt, image_url)

            # 心跳：图片分析（Qwen OpenAI 调用）可能较慢，进行中每 15 秒往
            # stderr 打一行进度，避免调用方（Claude PowerShell 工具）静默
            # 超时中断。心跳工具定义在 hikvision_api，这里函数内 import
            # 避免模块加载期循环依赖（hikvision_api 顶层已 import LLMTool）。
            from hikvision_api import _start_heartbeat, _stop_heartbeat
            import os as _os
            _hb_label = f"分析图片 {_os.path.basename(image_url)}"
            _hb = _start_heartbeat(_hb_label)
            try:
                # 调用API
                completion = self.client.chat.completions.create(
                    model=self.model,
                    messages=messages,
                    max_tokens=self.max_tokens,
                )
            finally:
                _stop_heartbeat(_hb)
            
            # 获取原始响应
            raw_response = json.loads(completion.model_dump_json())
            
            # 提取文本内容
            content = completion.choices[0].message.content
            
            return {
                "success": True,
                "markdown": content,
                "raw_response": raw_response,
                "error": None
            }
            
        except Exception as e:
            return {
                "success": False,
                "markdown": "",
                "raw_response": None,
                "error": str(e)
            }


def analyze_image(
    image_url: str,
    custom_prompt: Optional[str] = None,
    api_key: str = API_KEY,
    base_url: str = BASE_URL,
    model: str = MODEL_QWEN,
    max_tokens: int = 1024
) -> Dict[str, Any]:
    """
    统一的图片分析接口函数
    
    Args:
        image_url: 图片URL或本地路径
        custom_prompt: 自定义提示词
        api_key: API密钥
        base_url: API基础URL
        model: 模型名称
        max_tokens: 最大输出token数
        
    Returns:
        Markdown格式的分析结果字符串
        
    Example:
        # 分析本地图片
        result = analyze_image("C:/Users/xxx/Desktop/image.png")
        print(result)
        
        # 分析网络图片
        result = analyze_image("https://example.com/image.jpg")
        print(result)
        
        # 使用自定义提示词
        result = analyze_image(
            "image.png",
            custom_prompt="请提取图片中的所有文字信息"
        )
    """
    analyzer = ImageAnalyzer(
        api_key=api_key,
        base_url=base_url,
        model=model,
        max_tokens=max_tokens
    )
    
    result = analyzer.analyze(image_url, custom_prompt)
    print()
    if result["success"]:
        try:
            # 获取原始图片的base64和扩展名
            b64 = analyzer._get_b64(image_url)
            ext = "png" if image_url.lower().endswith(".png") else "jpg"

            # 返回包含文本内容和图片信息的字典
            return {
                "text": result["markdown"],
                "image_b64": b64,
                "image_ext": ext,
                "success": True,
                "error": None
            }
        except Exception:
            # 如果无法获取图片base64，则只返回文本内容
            return {
                "text": result["markdown"],
                "image_b64": None,
                "image_ext": None,
                "success": True,
                "error": None
            }
    else:
        return {
            "text": f"# 错误\n\n分析失败：{result['error']}",
            "image_b64": None,
            "image_ext": None,
            "success": False,
            "error": result['error']
        }
