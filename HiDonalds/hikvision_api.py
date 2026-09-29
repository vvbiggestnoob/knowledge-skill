"""
海康知识库上传 —— 纯 HTTP 版（无 JVM）

替代 main.py 里基于 JPype + JVM + 海康 Java SDK 的实现，改成直接调海康
MaaS 的 HTTP 接口。这样不再需要 JVM / jpype / libs/*.jar / JRE，整个
CLI 可以打包成只含 Python + requests 的小 exe（几十 MB），目标机无需
安装 Java。

三个 HTTP 接口（同域名、同认证 header `api-key`）：
  1. 上传文件   POST {BASE}/knowledge/doc/upload/create   multipart file=@<路径>
                -> 返回 {docName, storageId, docHash}
  2. 查知识库ID POST {BASE}/knowledge/list                 json {name, pageStart, pageSize}
                -> 返回列表，取 name 匹配项的 knowledgeId
  3. 导入文档   POST {BASE}/knowledge/doc/import          json {knowledgeId, docs[], chunkConfig}

本文件复刻 main.hikvision_sdk 的整体流程（过滤加域/不存在文件、按类型分组、
图片先分析生成临时 docx、分四组导入），返回结构与 main.py 完全一致，保证
cli.py 不用改。
"""
import os
import sys
import time
import base64
import tempfile
import threading
from io import BytesIO
from typing import List

import requests
from docx import Document
from docx.shared import Cm

from config import (
    API_KEY,
    KNOWLEDGE_NAME,
    TEXT_EXTENSIONS,
    TABLE_EXTENSIONS,
    IMAGE_EXTENSIONS,
    TEXT_EXTENSIONS_B,
)
from LLMTool import analyze_image

# 知识库 HTTP API 基址（与 Qwen 图片分析的 BASE_URL 不同，后者是 OpenAI 兼容的 /v1）
KNOWLEDGE_API_BASE = "https://maas.hikvision.com.cn/api/maas-model-api/api/datacenter/v1"

# HTTP 请求超时（秒）：上传/导入可能较慢，给宽一点
TIMEOUT = 120

# 心跳间隔（秒）。长操作（上传/导入/图片分析）进行中每隔这么久往 stderr 打
# 一行，让调用方（Claude 的 PowerShell 工具）持续收到输出，不被默认的
# 2 分钟静默超时强制中断。走 stderr，不污染 stdout 末尾的 JSON 结果
# （skill 靠 stdout 末行 JSON 汇报成功/失败）。
HEARTBEAT_INTERVAL = 15

# 心跳停止信号：全局 Event，一次只跑一个心跳（上传/导入不会并发）。
_heartbeat_stop = threading.Event()


def _start_heartbeat(label: str) -> threading.Thread:
    """启动后台心跳线程，每 HEARTBEAT_INTERVAL 秒往 stderr 打一行进度。

    label: 当前操作描述（如「上传 xxx.pdf（1234KB）」）。
    返回线程对象；操作结束后用 _stop_heartbeat(thread) 停掉。
    用 threading.Event.wait(interval) 做定时，既能周期唤醒又能被立即停止。
    """
    _heartbeat_stop.clear()
    start = time.monotonic()

    def _beat():
        while not _heartbeat_stop.wait(HEARTBEAT_INTERVAL):
            elapsed = int(time.monotonic() - start)
            print(f"💓 {label}（已等待 {elapsed}s）", file=sys.stderr, flush=True)

    t = threading.Thread(target=_beat, daemon=True)
    t.start()
    return t


def _stop_heartbeat(thread: threading.Thread):
    """停止心跳线程。"""
    _heartbeat_stop.set()
    if thread is not None:
        thread.join(timeout=1)
    _heartbeat_stop.clear()


def _api_headers(json_body=True):
    """构造请求头。上传文件时不要设 Content-Type（requests 会自动带 multipart boundary）。"""
    h = {
        "api-key": API_KEY,
        "Accept": "*/*",
        "Connection": "keep-alive",
        "User-Agent": "HiDonalds-cli/1.0",
    }
    if json_body:
        h["Content-Type"] = "application/json"
    return h


def _unwrap(resp_json):
    """海康接口返回通常包在 {code, msg, data} 里，取 data；没有就原样返回。

    字段名跑通时确认（可能是 data / result / 直接平铺）。
    """
    if not isinstance(resp_json, dict):
        return resp_json
    for key in ("data", "result", "payload"):
        if key in resp_json and resp_json[key] is not None:
            return resp_json[key]
    return resp_json


# 业务成功的 code 值集合（HTTP 200 不代表业务成功，要看 code）。
# 实测小文件 upload 成功返回 code="0"；错误如加域文档 code="0x04040020"。
_SUCCESS_CODES = {"0", "0x0", "0x00000000", "success", "200"}


def _check_response(resp):
    """检查业务返回码。HTTP 200 不代表成功，海康用 JSON 里的 code 字段表示。

    code 不在成功集合里 → 抛 RuntimeError，把服务端 message 带出来。
    这样"加域文档无法解析"等服务端拒绝会被正确识别为失败，而非假成功。
    """
    try:
        body = resp.json()
    except Exception:
        return  # 非 JSON 响应，交给上层处理
    if not isinstance(body, dict):
        return
    code = body.get("code")
    if code is None:
        return  # 无 code 字段，按 HTTP 状态码为准（已 raise_for_status 过）
    code_s = str(code).strip()
    if code_s in _SUCCESS_CODES:
        return  # 业务成功
    message = body.get("message") or body.get("msg") or "未知错误"
    raise RuntimeError(f"[{code_s}] {message}")


def upload_file(file_path):
    """第 1 步：上传文件，返回 {docName, storageId, docHash}。

    对应 main.py 的 app.upload(file_path)。
    实测返回结构：{code, data: [{docId, docType, docName, docSize, storageId, docHash, duplicate}], message}
    data 是列表，取第一个元素。

    新增：检查文件大小，超过 200MB 直接抛异常。
    """
    # 检查文件大小（200MB = 200 * 1024 * 1024 字节）
    MAX_FILE_SIZE = 200 * 1024 * 1024  # 200MB
    file_size = os.path.getsize(file_path)
    if file_size > MAX_FILE_SIZE:
        raise RuntimeError("文件大小超限")

    url = f"{KNOWLEDGE_API_BASE}/knowledge/doc/upload/create"
    label = f"上传 {os.path.basename(file_path)}（{file_size // 1024}KB）"
    hb = _start_heartbeat(label)
    try:
        with open(file_path, "rb") as f:
            resp = requests.post(
                url,
                headers=_api_headers(json_body=False),
                files={"file": (os.path.basename(file_path), f)},
                timeout=TIMEOUT,
            )
    finally:
        _stop_heartbeat(hb)
    resp.raise_for_status()
    _check_response(resp)  # 检查业务码：加域文档等会在此抛带原因的异常
    data = _unwrap(resp.json())
    # data 是列表，取第一个元素
    if isinstance(data, list) and data:
        item = data[0]
    elif isinstance(data, dict):
        item = data
    else:
        raise RuntimeError(f"upload 返回结构异常: {resp.json()}")
    return {
        "docName": item.get("docName") or os.path.basename(file_path),
        "storageId": item.get("storageId"),
        "docHash": item.get("docHash"),
    }


def find_knowledge_id(name):
    """第 2 步：按名字查知识库 ID。对应 main.py 的 app.getByName(name)。

    返回匹配 name 的知识库的 knowledgeId；找不到返回 None。
    """
    url = f"{KNOWLEDGE_API_BASE}/knowledge/list"
    resp = requests.post(
        url,
        headers=_api_headers(),
        json={"name": name, "pageStart": 1, "pageSize": 50},
        timeout=TIMEOUT,
    )
    resp.raise_for_status()
    _check_response(resp)
    data = _unwrap(resp.json())
    # data 可能是列表，也可能是 {list: [...], total: N}
    items = data if isinstance(data, list) else (data.get("list") or data.get("records") or [])
    for item in items:
        if not isinstance(item, dict):
            continue
        if item.get("name") == name:
            return item.get("knowledgeId") or item.get("id")
    # 退而求其次：返回第一个的 id
    for item in items:
        if isinstance(item, dict):
            return item.get("knowledgeId") or item.get("id")
    return None


def import_docs(knowledge_id, docs, chunk_config, label="文档"):
    """第 3 步：把文档导入知识库。对应 main.py 的 knowledge.importDocument(doc_list, config)。

    docs: [{docName, storageId, docHash}, ...]
    chunk_config: dict（按 setter 名映射的字段）
    label: 导入类型名（图片/txt文本/文本/表格），仅用于心跳进度展示。
    """
    url = f"{KNOWLEDGE_API_BASE}/knowledge/doc/import"
    body = {
        "knowledgeId": knowledge_id,
        "docs": docs,
        "chunkConfig": chunk_config,
        # labels: main.py 没设，这里也不传（可选字段）
    }
    hb = _start_heartbeat(f"导入 {len(docs)} 个{label}文档")
    try:
        resp = requests.post(url, headers=_api_headers(), json=body, timeout=TIMEOUT)
    finally:
        _stop_heartbeat(hb)
    resp.raise_for_status()
    _check_response(resp)
    return resp.json()


def get_file_parse_type(file_path):
    """根据文件扩展名返回对应的 fileParseType（与 main.py 一致）。"""
    ext = os.path.splitext(file_path)[1].lower()
    if ext in TEXT_EXTENSIONS:
        return 1  # 文本文件
    elif ext in TABLE_EXTENSIONS:
        return 2  # 表格文件
    elif ext in IMAGE_EXTENSIONS:
        return 3  # 图片类
    elif ext in TEXT_EXTENSIONS_B:
        return 0
    else:
        print(f"⚠️ 未知文件类型: {ext}，默认按文本文件处理")
        return 1  # 默认文本文件


def _is_domain_file(file_path: str) -> bool:
    """判断文件是否位于域控同步目录（加域文件）。从 main.py 原样搬来，无 JVM 依赖。"""
    domain_markers = [
        "\\\\domain",
        "\\\\corp",
        "\\\\ad",
        "\\\\ldap",
        "dfs",
        "sysvol",
        "\\\\192.168",
        "\\\\10.",
        "netlogon",
    ]
    lower_path = file_path.lower()
    for marker in domain_markers:
        if marker in lower_path:
            return True
    if file_path.startswith("\\\\") and not file_path.startswith("\\\\?\\"):
        return True
    return False


def _chunk_config_image():
    """图片类导入的 chunkConfig（对应 main.py 图片分支的 ChunkConfig 设置）。"""
    return {
        "fileParseType": "1",
        "imageEnable": True,
        "imageParseType": 3,
        "chunkType": 2,
        "chunkLength": 2000,
        "maxOverlapRatio": 20,
        "enableRefFile": True,
        "enableRefTitle": True,
        "titleDepth": 1,
        "generateQuestions": True,
    }


def _chunk_config_text_b():
    """txt 文本类导入的 chunkConfig（对应 main.py document_list_b 分支）。"""
    return {
        "fileParseType": "1",
        "imageEnable": False,
        "tableEnable": True,
        "formulaEnable": True,
        "chunkType": 2,
        "chunkLength": 1000,
        "maxOverlapRatio": 20,
        "enableRefFile": True,
        "enableRefTitle": True,
        "titleDepth": 3,
        "generateQuestions": True,
    }


def _chunk_config_text():
    """文本类导入的 chunkConfig（对应 main.py document_list 分支）。"""
    return {
        "fileParseType": "1",
        "imageEnable": True,
        "imageParseType": 3,
        "tableEnable": True,
        "formulaEnable": True,
        "chunkType": 2,
        "chunkLength": 1000,
        "maxOverlapRatio": 20,
        "enableRefFile": True,
        "enableRefTitle": True,
        "titleDepth": 3,
        "generateQuestions": True,
    }


def _chunk_config_table():
    """表格类导入的 chunkConfig（对应 main.py table_list 分支）。"""
    return {
        "fileParseType": "2",
        "imageEnable": True,
        "imageParseType": 3,
        "chunkMethod": 1,
        "tableChunkMode": 2,
        "enableRefFile": True,
        "enableRefTitle": True,
        "generateQuestions": True,
    }


def batch_analyze_images_to_temp(image_path_list: List[str]):
    """批量分析图片并保存为临时 docx（文本总结 + 嵌入原始图片）。从 main.py 原样搬来，无 JVM 依赖。"""
    docx_path_list = []

    for i, image_path in enumerate(image_path_list):
        try:
            print(f"正在分析第 {i+1}/{len(image_path_list)} 张图片: {image_path}")
            image_name = os.path.basename(image_path).split(".")[0]
            result = analyze_image(image_path)
            print(result["text"])

            doc = Document()
            text = result.get("text", "")
            if text:
                for para in text.split("\n"):
                    if para.strip():
                        doc.add_paragraph(para)
                    else:
                        doc.add_paragraph("")

            image_b64 = result.get("image_b64")
            if image_b64:
                try:
                    image_bytes = base64.b64decode(image_b64)
                    doc.add_picture(BytesIO(image_bytes), width=Cm(15))
                except Exception as img_err:
                    print(f"嵌入图片 {image_path} 失败: {img_err}")

            temp_dir = tempfile.gettempdir()
            temp_file_path = os.path.join(temp_dir, f"{image_name}.docx")
            doc.save(temp_file_path)

            docx_path_list.append(temp_file_path)
            print(f"已保存临时文件: {temp_file_path}")

            sleep_interval = 1.2
            if i < len(image_path_list) - 1:
                time.sleep(sleep_interval)

        except Exception as e:
            print(f"处理图片 {image_path} 失败: {e}")
            docx_path_list.append(None)

    return docx_path_list


def hikvision_sdk(file_path_list, on_progress=None):
    """纯 HTTP 版的批量上传主流程，复刻 main.py 的 hikvision_sdk，返回结构一致。

    Args:
        file_path_list: 文件路径列表
        on_progress: 进度回调函数，接收 (file_path, status, message) 参数
    """
    try:
        # 过滤加域文件 + 不存在文件，记录被跳过的
        domain_files = []
        filtered_list = []
        for file_path in file_path_list:
            if _is_domain_file(file_path):
                print(f"⏭️ 跳过加域文件（可能上传失败）: {file_path}")
                if on_progress:
                    on_progress(file_path, "skipped", "加域文件，跳过")
                domain_files.append(file_path)
            elif os.path.exists(file_path):
                filtered_list.append(file_path)
            else:
                print(f"⏭️ 文件不存在: {file_path}")
                if on_progress:
                    on_progress(file_path, "skipped", "文件不存在")
                domain_files.append(file_path)

        # 第 2 步：查知识库 ID
        knowledge_id = find_knowledge_id(KNOWLEDGE_NAME)
        if not knowledge_id:
            raise RuntimeError(f"未找到名为「{KNOWLEDGE_NAME}」的知识库")
        print(f"知识库 ID: {knowledge_id}")

        # 第 1 步：逐个上传，按类型分组
        document_list = []        # 文本类（fileParseType=1）
        document_list_b = []      # txt 文本类（fileParseType=0）
        image_list = []           # 图片类
        image_path_list = []      # 图片原始路径（供分析用）
        table_list = []           # 表格类
        upload_failed_files = []

        for file_path in filtered_list:
            try:
                file_base = upload_file(file_path)
            except Exception as e:
                reason = str(e)
                print(f"❌ 上传失败: {file_path}, 原因: {reason}")
                if on_progress:
                    on_progress(file_path, "failed", f"上传失败: {reason}")
                upload_failed_files.append({"file": file_path, "reason": reason})
                continue

            # 上传成功，调用进度回调
            if on_progress:
                on_progress(file_path, "uploaded", "上传成功")

            index = get_file_parse_type(file_path)
            if index == 1:
                document_list.append(file_base)
            elif index == 3:
                image_list.append(file_base)
                image_path_list.append(file_path)
            elif index == 2:
                table_list.append(file_base)
            elif index == 0:
                document_list_b.append(file_base)

        # 图片类：先分析生成临时 docx，再上传 docx，和原图片一起按文本类导入。
        # 用独立的 image_doc_list 装要导入的内容，不污染 document_list，
        # 否则后面 `if document_list:` 会把 docx 再导入一次（导致重复导入）。
        if image_list:
            docx_paths = batch_analyze_images_to_temp(image_path_list=image_path_list)
            image_doc_list = list(image_list)  # 原图片的 file_base
            image_doc_files = list(image_path_list)  # 对应的原始文件路径（导入失败时记录用）
            for docx_path in docx_paths:
                if docx_path is not None:
                    try:
                        docx_base = upload_file(docx_path)
                        image_doc_list.append(docx_base)
                    except Exception as e:
                        reason = str(e)
                        print(f"❌ 临时 docx 上传失败: {docx_path}, 原因: {reason}")
                        upload_failed_files.append({"file": docx_path, "reason": reason})

            try:
                import_docs(knowledge_id, image_doc_list, _chunk_config_image(), label="图片")
                print(f"✅ 图片分析结果导入成功: {len(image_list)} 个")
                # 图片类导入成功后，对所有图片文件调用进度回调
                if on_progress:
                    for fp in image_doc_files:
                        on_progress(fp, "imported", "导入成功")
            except Exception as e:
                reason = str(e)
                print(f"❌ 图片类导入失败, 原因: {reason}")
                if on_progress:
                    for fp in image_doc_files:
                        on_progress(fp, "failed", f"导入失败: {reason}")
                for fp in image_doc_files:
                    upload_failed_files.append({"file": fp, "reason": f"导入失败: {reason}"})

            # 清理临时文件
            for docx_path in docx_paths:
                if docx_path is not None and os.path.exists(docx_path):
                    try:
                        os.remove(docx_path)
                        print(f"已删除临时文件: {docx_path}")
                    except Exception as e:
                        print(f"删除临时文件失败 {docx_path}: {e}")

        if document_list_b:
            try:
                import_docs(knowledge_id, document_list_b, _chunk_config_text_b(), label="txt文本")
                print(f"✅ txt文本类文档导入成功: {len(document_list_b)} 个")
                if on_progress:
                    for fb in document_list_b:
                        on_progress(fb.get("docName", ""), "imported", "导入成功")
            except Exception as e:
                reason = str(e)
                print(f"❌ txt文本类导入失败, 原因: {reason}")
                if on_progress:
                    for fb in document_list_b:
                        on_progress(fb.get("docName", ""), "imported_failed", f"导入失败: {reason}")
                for fb in document_list_b:
                    upload_failed_files.append({"file": fb.get("docName", ""), "reason": f"导入失败: {reason}"})

        if document_list:
            try:
                import_docs(knowledge_id, document_list, _chunk_config_text(), label="文本")
                print(f"✅ 文本类文档导入成功: {len(document_list)} 个")
                if on_progress:
                    for fb in document_list:
                        on_progress(fb.get("docName", ""), "imported", "导入成功")
            except Exception as e:
                reason = str(e)
                print(f"❌ 文本类导入失败, 原因: {reason}")
                if on_progress:
                    for fb in document_list:
                        on_progress(fb.get("docName", ""), "imported_failed", f"导入失败: {reason}")
                for fb in document_list:
                    upload_failed_files.append({"file": fb.get("docName", ""), "reason": f"导入失败: {reason}"})

        if table_list:
            try:
                import_docs(knowledge_id, table_list, _chunk_config_table(), label="表格")
                print(f"✅ 表格类文档导入成功: {len(table_list)} 个")
                if on_progress:
                    for fb in table_list:
                        on_progress(fb.get("docName", ""), "imported", "导入成功")
            except Exception as e:
                reason = str(e)
                print(f"❌ 表格类导入失败, 原因: {reason}")
                if on_progress:
                    for fb in table_list:
                        on_progress(fb.get("docName", ""), "imported_failed", f"导入失败: {reason}")
                for fb in table_list:
                    upload_failed_files.append({"file": fb.get("docName", ""), "reason": f"导入失败: {reason}"})

        return {
            "status": "处理完毕",
            "success_count": len(filtered_list) - len(upload_failed_files),
            "total_count": len(file_path_list),
            "domain_skipped": domain_files,
            "upload_failed": upload_failed_files,
        }

    except Exception as e:
        print(f"错误: {e}")
        import traceback
        traceback.print_exc()
        return {
            "status": str(e),
            "success_count": 0,
            "total_count": len(file_path_list),
            "domain_skipped": domain_files if 'domain_files' in dir() else [],
            "upload_failed": upload_failed_files if 'upload_failed_files' in dir() else [],
        }
