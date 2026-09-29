---
name: knowledge-cli
version: 0.4.0
description: 通过自然语言搜索海康 MaaS 知识库，支持多工作空间、知识库管理、文档管理、分片管理、标签管理和未命中问题上传。
---

# knowledge-cli — 海康 MaaS 知识库 CLI

可执行文件位置：`C:\Users\pengzuo\.zcode\skills\knowledge-cli\knowledge-cli.exe`

## 前置条件

- 已编译可执行文件：`knowledge-cli.exe` 位于上述目录中
- 已配置 `conf/knowledge_bases.json`（含 `workspaces` 多工作空间配置，每个工作空间有独立的 `api-key` 和 `knowledgeBases` 列表）
- 配置文件中至少一个工作空间的 `api-key` 已设置为有效的 MaaS API Key

## 工作目录

所有命令均需先切换到技能目录：

```bash
cd "C:\Users\pengzuo\.zcode\skills\knowledge-cli"
```

下文示例中省略 `cd` 步骤，均假设已在该目录下执行 `./knowledge-cli.exe`。

## 能力边界

| 命令 | 能力 |
|------|------|
| `knowledge-cli init` | 初始化配置：拉取多工作空间知识库，合并更新配置文件 |
| `knowledge-cli switch` | 切换默认知识库（同时更新 defaultKnowledgeId 和 defaultKnowledgeName） |
| `knowledge-cli list` | 列出当前空间下的知识库，支持名称模糊搜索 |
| `knowledge-cli get` | 根据 ID 获取知识库详情 |
| `knowledge-cli create` | 创建知识库，支持图谱配置（灰度中） |
| `knowledge-cli update` | 修改知识库名称、描述、图谱配置 |
| `knowledge-cli delete` | 删除知识库 |
| `knowledge-cli graph create` | 批量创建知识库图谱（灰度中） |
| `knowledge-cli graph delete` | 批量删除知识库图谱（灰度中） |
| `knowledge-cli doc list` | 列出知识库中的文档，支持文档名关键字搜索 |
| `knowledge-cli doc get` | 根据 ID 获取文档详情（含分片配置、解析状态） |
| `knowledge-cli doc chunks` | 查看文档分片列表，支持关键字搜索分片内容 |
| `knowledge-cli doc upload` | 上传文档文件到 MaaS 平台 |
| `knowledge-cli doc import` | 将已上传文档导入知识库 |
| `knowledge-cli doc create` | 创建空白文档（.custom） |
| `knowledge-cli doc enable` | 启用文档（支持批量） |
| `knowledge-cli doc disable` | 禁用文档（支持批量） |
| `knowledge-cli doc delete` | 删除文档 |
| `knowledge-cli doc view` | 获取文档预览链接 |
| `knowledge-cli doc download` | 获取文档下载链接 |
| `knowledge-cli doc graph-restart` | 重启文档图谱创建（灰度中） |
| `knowledge-cli chunk get` | 根据 ID 获取分片详情 |
| `knowledge-cli chunk add` | 新增文档分片 |
| `knowledge-cli chunk add-auto` | 向 custom-questions.md 自动添加分片（不存在则创建文档） |
| `knowledge-cli chunk update` | 更新分片内容和状态 |
| `knowledge-cli chunk delete` | 批量删除分片 |
| `knowledge-cli chunk relate` | 获取关联分片（前后邻居） |
| `knowledge-cli chunk question add` | 新增分片关联问题 |
| `knowledge-cli chunk question update` | 更新分片关联问题 |
| `knowledge-cli chunk question delete` | 删除分片关联问题 |
| `knowledge-cli chunk image update` | 更新分片关联图片描述 |
| `knowledge-cli chunk image delete` | 删除分片关联图片 |
| `knowledge-cli label add` | 新增标签 |
| `knowledge-cli label list` | 分页获取标签 |
| `knowledge-cli label update` | 更新标签（增删标签值） |
| `knowledge-cli label delete` | 删除标签及标签值 |
| `knowledge-cli search` | 分片检索：语义/全文/混合检索，支持 rerank |
| `knowledge-cli upload-unanswered` | 将 cache 中已审核的未命中文档上传知识库，上传成功后删除本地文件 |

## 自然语言搜索流程

当用户用自然语言提问（如"XX 是否支持""XX 怎么配置""有没有 XX 功能"）时，按以下流程执行：

### 第 1 步：解析用户意图

从用户问题中提取：
- **核心关键词**：去掉疑问词（是否、怎么、有没有、如何），提取业务关键词
- **搜索范围**：判断是否需要限定知识库（如果用户提到了特定产品/平台名称）

### 第 2 步：调用 search 命令

```bash
cd "C:\Users\pengzuo\.zcode\skills\knowledge-cli" && ./knowledge-cli.exe search --query "<关键词>" --top-k 10
```

**参数说明**：
- `--query`：搜索关键词（必填），将用户自然语言问题精简为关键词
- `--top-k`：返回结果数，默认 10，复杂问题可调至 15-20
- `--similarity`：相似度阈值，默认 0.6，如需扩大召回可调低（如 0.1），精确匹配可调高（如 0.9）
- `--strategy`：检索策略，1=语义检索，2=全文检索，3=混合检索（默认）
- `--knowledge`：指定知识库名称（从配置文件查找），如 `--knowledge "电力运检辅助平台"`
- `--knowledge-id`：直接指定知识库 ID，支持逗号分隔多个 ID
- `--unanswered-threshold`：未命中相似度阈值，默认 0.9。当所有结果相似度低于此值时，自动记录到 `cache/unanswered-questions-时间戳.md`，待人工审核后上传知识库

### 第 3 步：分析结果并回答

1. **检查相似度**：`similarity > 0.8` 为高度相关，`0.5-0.8` 为中度相关，`< 0.5` 为弱相关
2. **提取关键信息**：从 `chunkContent` 中提取与用户问题相关的具体内容
3. **引用来源**：标注来自哪个文档（`docName`）和分片标题（`chunkTitle`）
4. **判断结论**：
   - 如果多条结果明确描述该功能 → "✅ 支持，详见..."
   - 如果结果涉及相关但不完全匹配 → "⚠️ 部分支持，具体是..."
   - 如果没有相关结果或相似度很低 → "❌ 未找到相关功能说明"
5. **未命中自动记录**：如果所有结果 `similarity < 0.9`，系统会自动将该问题记录到 `cache/unanswered-questions-时间戳.md` 文件（同天问题汇总到同一文件），并提示待人工审核。人工审核后可补充标准答案，通过 `upload-unanswered` 命令上传到知识库

### 第 4 步：必要时深入查看

如果 search 结果不够明确，可以进一步：

```bash
# 查看文档完整分片列表
./knowledge-cli.exe doc chunks --id <docId> --page-size 20

# 查看文档详情
./knowledge-cli.exe doc get --id <docId>

# 用不同关键词再次搜索
./knowledge-cli.exe search --query "<调整后的关键词>" --top-k 10
```

## 命令参考

### 初始化配置

```bash
# 拉取多个工作空间的知识库并合并更新配置文件
./knowledge-cli.exe init --api-keys "sk-xxx1,sk-xxx2"

# 单工作空间拉取
./knowledge-cli.exe init --api-key "sk-xxx1"

# 从已有配置文件中的 API Key 重新拉取
./knowledge-cli.exe init

# 指定拉取数量
./knowledge-cli.exe init --api-keys "sk-xxx1,sk-xxx2" --page-size 200
```

init 命令会：
1. 对每个 API Key 创建 client 调用 MaaS API 获取该工作空间下所有知识库
2. 以表格形式展示知识库名称和 ID
3. 构建 `workspaces` 数组，每个工作空间包含 api-key 和知识库列表
4. 保留已有 `defaultKnowledgeId`/`defaultKnowledgeName`（如果该知识库仍在列表中），否则设为第一个知识库
5. 全量更新 `conf/knowledge_bases.json`

### 切换默认知识库

```bash
# 切换默认知识库
./knowledge-cli.exe switch --knowledge "电力运检辅助平台"

# 查看当前默认和所有可用知识库
./knowledge-cli.exe switch
```

### 知识库管理

```bash
# 列出所有知识库
./knowledge-cli.exe list

# 按名称模糊搜索知识库
./knowledge-cli.exe list --name "电力"

# 查看知识库详情
./knowledge-cli.exe get --id <knowledgeId>

# 创建知识库
./knowledge-cli.exe create --name "新知识库" --detail "描述"

# 创建知识库（启用图谱，灰度中）
./knowledge-cli.exe create --name "图谱知识库" --detail "描述" --graph --graph-method 1

# 修改知识库
./knowledge-cli.exe update --id <knowledgeId> --name "新名称" --detail "新描述"

# 删除知识库
./knowledge-cli.exe delete --id <knowledgeId>

# 批量创建知识库图谱（灰度中）
./knowledge-cli.exe graph create --ids "id1,id2"

# 批量删除知识库图谱（灰度中）
./knowledge-cli.exe graph delete --ids "id1,id2"
```

### 文档管理

```bash
# 列出知识库中的文档（使用默认知识库）
./knowledge-cli.exe doc list

# 按文档名搜索
./knowledge-cli.exe doc list --content "用户手册"

# 指定知识库
./knowledge-cli.exe doc list --knowledge "电力运检辅助平台"

# 查看文档详情
./knowledge-cli.exe doc get --id <docId>

# 查看文档分片
./knowledge-cli.exe doc chunks --id <docId> --page-size 10

# 在分片中搜索关键字
./knowledge-cli.exe doc chunks --id <docId> --keyword "配置"

# 上传文档（一步上传+导入，--files 支持多个文件，逗号分隔或重复传参）
./knowledge-cli.exe doc upload --files "D:\docs\产品介绍.pdf" --knowledge "电力运检辅助平台"

# 导入已上传文档
./knowledge-cli.exe doc import --knowledge "电力运检辅助平台" --doc-name "产品介绍.pdf" --storage-id "<storageId>" --doc-hash "<docHash>"

# 创建空白文档（.custom 结尾）
./knowledge-cli.exe doc create --knowledge "电力运检辅助平台" --doc-name "空白文档.custom"

# 启用文档（支持批量，逗号分隔）
./knowledge-cli.exe doc enable --id "docId1,docId2"

# 禁用文档（支持批量，逗号分隔）
./knowledge-cli.exe doc disable --id "docId1,docId2"

# 删除文档
./knowledge-cli.exe doc delete --id <docId>

# 获取文档预览链接
./knowledge-cli.exe doc view --id <docId>

# 获取文档下载链接（若下载失败则展示返回的 URL 地址）
./knowledge-cli.exe doc download --id <docId>

# 重启文档图谱创建（灰度中）
./knowledge-cli.exe doc graph-restart --doc-ids "docId1,docId2"
```

### 分片管理

```bash
# 获取分片详情
./knowledge-cli.exe chunk get --doc-id <docId> --id <chunkId>

# 新增分片
./knowledge-cli.exe chunk add --doc-id <docId> --content "分片内容" --title "分片标题"

# 自动添加分片（自动查找/创建 custom-questions.md 文档）
./knowledge-cli.exe chunk add-auto --content "分片内容" --title "分片标题"

# 自动添加分片（指定知识库和文档名）
./knowledge-cli.exe chunk add-auto --knowledge "电力运检辅助平台" --content "分片内容" --title "分片标题" --doc-name "custom-questions.md"

# 更新分片（内容和状态）
./knowledge-cli.exe chunk update --doc-id <docId> --id <chunkId> --content "新内容" --status "1"

# 批量删除分片
./knowledge-cli.exe chunk delete --doc-id <docId> --chunk-ids "chunkId1,chunkId2"

# 获取关联分片（前后邻居）
./knowledge-cli.exe chunk relate --doc-id <docId> --id <chunkId> --previous 1 --next 1

# 分片关联问题管理
./knowledge-cli.exe chunk question add --doc-id <docId> --id <chunkId> --question-content "问题内容"
./knowledge-cli.exe chunk question update --doc-id <docId> --id <chunkId> --question-id <quesId> --question-content "新内容"
./knowledge-cli.exe chunk question delete --doc-id <docId> --id <chunkId> --question-id <quesId>

# 分片关联图片管理
./knowledge-cli.exe chunk image update --doc-id <docId> --id <chunkId> --image-id <imgId> --image-desc "图片描述"
./knowledge-cli.exe chunk image delete --doc-id <docId> --id <chunkId> --image-id <imgId>
```

### 标签管理

```bash
# 新增标签
./knowledge-cli.exe label add --name "标签名" --values "值1,值2"

# 分页获取标签
./knowledge-cli.exe label list --page-size 10

# 按关键字搜索标签
./knowledge-cli.exe label list --keyword "测试"

# 更新标签（增加/删除标签值；后端不支持改名，无 --name 参数）
./knowledge-cli.exe label update --label-id <labelId> --add-values "新值1,新值2"
./knowledge-cli.exe label update --label-id <labelId> --delete-values "valueId:labelValue"

# 删除标签
./knowledge-cli.exe label delete --label-id <labelId>
```

### 分片检索

```bash
# 基础搜索
./knowledge-cli.exe search --query "巡检任务配置"

# 高精度搜索（提高相似度阈值）
./knowledge-cli.exe search --query "告警审核" --similarity 0.5

# 全文检索
./knowledge-cli.exe search --query "设备台账" --strategy 2

# 指定 rerank 模型
./knowledge-cli.exe search --query "智能巡视" --rerank-model "bge-reranker-v2-m3"

# 跨知识库搜索
./knowledge-cli.exe search --knowledge-id "<id1>,<id2>" --query "安全管理"
```

### 未命中问题上传

```bash
# 上传 cache 目录下所有已审核的未命中文件到知识库
./knowledge-cli.exe upload-unanswered

# 指定知识库
./knowledge-cli.exe upload-unanswered --knowledge "电力运检辅助平台"

# 预览模式（只解析不实际上传或删除）
./knowledge-cli.exe upload-unanswered --dry-run

# 指定 cache 目录
./knowledge-cli.exe upload-unanswered --cache-dir "./cache"
```

upload-unanswered 命令会：
1. 扫描 cache 目录下的 `.custom` 文件和 `unanswered-questions-*.md` 文件
2. `.custom` 文件：内容作为分片上传，文件名（去掉 .custom）作为标题
3. `unanswered-questions-*.md` 文件：解析已审核的问答块（补充答案非"待填写"的），逐条上传
4. 上传成功后删除本地文件；.md 文件中未审核的块会保留
5. 所有分片上传到 `custom-questions.md` 文档（不存在则自动创建）

## 配置文件

`conf/knowledge_bases.json` 结构：

```json
{
  "defaultKnowledgeName": "默认知识库名称",
  "defaultKnowledgeId": "knowledge-base-uuid",
  "workspaces": [
    {
      "api-key": "sk-xxx1",
      "knowledgeBases": [
        { "name": "知识库A", "id": "uuid-a" }
      ]
    },
    {
      "api-key": "sk-xxx2",
      "knowledgeBases": [
        { "name": "知识库B", "id": "uuid-b" }
      ]
    }
  ]
}
```

- `defaultKnowledgeName` / `defaultKnowledgeId`：默认知识库，以 `defaultKnowledgeId` 为准，当未指定 `--knowledge` 或 `--knowledge-id` 时使用
- `workspaces`：工作空间列表，每个工作空间有独立的 `api-key` 和 `knowledgeBases`，CLI 会根据知识库自动选择对应工作空间的 API Key
- `knowledgeBases`：知识库列表，通过 `--knowledge <名称>` 自动查找对应 ID

## 安全边界

- 默认只读：`list`、`get`、`doc list`、`doc get`、`doc chunks`、`doc view`、`doc download`、`chunk get`、`chunk relate`、`label list`、`search`、`switch`（无参数时）均为只读操作
- `init` 为写操作（更新 `conf/knowledge_bases.json` 配置文件），执行前需用户确认
- `switch`（带 `--knowledge` 时）为写操作（更新默认知识库配置），执行前需用户确认
- `doc upload`、`doc import`、`doc create` 为写操作，执行前需用户确认
- `create`、`update`、`delete` 为知识库写操作，执行前需用户确认
- `doc enable`、`doc disable`、`doc delete` 为文档写操作，执行前需用户确认
- `chunk add`、`chunk update`、`chunk delete` 为分片写操作，执行前需用户确认
- `chunk add-auto` 为写操作（可能创建文档+添加分片），执行前需用户确认
- `chunk question add/update/delete`、`chunk image update/delete` 为分片关联数据写操作，执行前需用户确认
- `label add`、`label update`、`label delete` 为标签写操作，执行前需用户确认
- `graph create`、`graph delete`、`doc graph-restart` 为图谱写操作（灰度中），执行前需用户确认
- `search` 的未命中记录为写操作（创建/追加 `cache/unanswered-questions-时间戳.md`），属于辅助记录，无需额外确认
- `upload-unanswered` 为写操作（批量上传分片+删除本地文件），执行前需用户确认
- API Key 优先从配置文件读取，不通过命令行传递
- 不在回答中输出完整的 API Key
- 搜索结果中的图片 URL 仅在用户明确需要时展示

## 回答规范

1. **先搜后答**：不要凭记忆回答，必须调用 `knowledge-cli search` 获取实际结果
2. **引用来源**：明确标注信息来自哪个文档、哪个章节
3. **区分确定性**：用 ✅/⚠️/❌ 标识功能支持程度
4. **简洁摘要**：不要直接粘贴原始 JSON，提取关键信息用表格或列表呈现
5. **关联推荐**：如果搜索结果中有相关的其他功能点，可简要提及供用户参考