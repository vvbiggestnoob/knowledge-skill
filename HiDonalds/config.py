import os

# MaaS 平台 API Key：建议通过环境变量 MAAS_API_KEY 注入，或直接替换下面的占位符
API_KEY = os.environ.get("MAAS_API_KEY", "sk-xxx")
# 目标知识库名称：建议通过环境变量 MAAS_KNOWLEDGE_NAME 注入，或直接替换下面的占位符
KNOWLEDGE_NAME = os.environ.get("MAAS_KNOWLEDGE_NAME", "你的知识库名称")
BASE_URL = "https://maas.hikvision.com.cn/v1"
MODEL_QWEN = "Qwen3.6-35B-A3B-FP8"
IMAGER_PROMPT = (
            "你是一个图片信息提取专家，首要任务是针对上传的数据，提取并总结其中的信息。"
            "总结应该保持段落性总结，而不是分点。"
            "总结需要说明细节，需要详细总结。"
            "非常重要！如果判定为聊天记录信息，你需要先输出具体的聊天内容，然后再输出总结。"
            "请使用结构化的格式输出结果。"
        )

TEXT_EXTENSIONS_B = {'.txt', '.md', '.json'}
# 文本类文件：pdf, txt, doc, docx, md, pptx, ppt, json, html
TEXT_EXTENSIONS = {'.pdf', '.doc', '.docx', '.html', '.pptx', '.ppt'}
# 表格类文件：xlsx, xls, csv
TABLE_EXTENSIONS = {'.xlsx', '.xls', '.csv'}
# 图片类文件：png, jpg, jpeg
IMAGE_EXTENSIONS = {'.png', '.jpg', '.jpeg'}
