# 评估功能 API

[返回目录](./README.md)

| 方法 | 路径           | 描述                  |
| ---- | -------------- | --------------------- |
| GET  | `/evaluation/` | 获取评估任务结果       |
| POST | `/evaluation/` | 创建评估任务          |
| GET | `/evaluation/wiki/datasets` | 获取可用于 Wiki 评测的数据集 |
| POST | `/evaluation/wiki/runs` | 创建 Wiki 评测任务 |
| GET | `/evaluation/wiki/runs` | 分页查询 Wiki 评测历史 |
| GET | `/evaluation/wiki/runs/:id` | 获取 Wiki 评测详情 |
| GET | `/evaluation/wiki/runs/:id/report` | 下载 Wiki 评测报告 |
| DELETE | `/evaluation/wiki/runs/:id` | 删除终态 Wiki 评测记录 |

> 注：服务端路由带尾斜杠（Gin 会自动从 `/evaluation` 重定向到 `/evaluation/`），下方示例为方便阅读用了 `/evaluation`。

## GET `/evaluation` - 获取评估任务结果

**参数说明（查询参数）**:

| 字段     | 类型   | 必填 | 说明                                                |
| -------- | ------ | ---- | --------------------------------------------------- |
| task_id  | string | 是   | 从 `POST /evaluation` 返回的任务 ID                  |

**请求**:

```bash
curl --location 'http://localhost:8080/api/v1/evaluation?task_id=c34563ad-b09f-4858-b72e-e92beb80becb' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json'
```

**响应**:

```json
{
    "data": {
        "task": {
            "id": "c34563ad-b09f-4858-b72e-e92beb80becb",
            "tenant_id": 1,
            "dataset_id": "default",
            "start_time": "2025-08-12T14:54:26.221804768+08:00",
            "status": 2,
            "total": 1,
            "finished": 1
        },
        "params": {
            "session_id": "",
            "knowledge_base_id": "2ef57434-8c8d-4442-b967-2f7fc578a2fc",
            "vector_threshold": 0.5,
            "keyword_threshold": 0.3,
            "embedding_top_k": 10,
            "vector_database": "",
            "rerank_model_id": "b30171a1-787b-426e-a293-735cd5ac16c0",
            "rerank_top_k": 5,
            "rerank_threshold": 0.7,
            "chat_model_id": "8aea788c-bb30-4898-809e-e40c14ffb48c",
            "summary_config": {
                "max_tokens": 0,
                "repeat_penalty": 1,
                "top_k": 0,
                "top_p": 0,
                "frequency_penalty": 0,
                "presence_penalty": 0,
                "prompt": "这是用户和助手之间的对话。",
                "context_template": "你是一个专业的智能信息检索助手",
                "no_match_prefix": "<think>\n</think>\nNO_MATCH",
                "temperature": 0.3,
                "seed": 0,
                "max_completion_tokens": 2048
            },
            "fallback_strategy": "",
            "fallback_response": "抱歉，我无法回答这个问题。"
        },
        "metric": {
            "retrieval_metrics": {
                "precision": 0,
                "recall": 0,
                "ndcg3": 0,
                "ndcg10": 0,
                "mrr": 0,
                "map": 0
            },
            "generation_metrics": {
                "bleu1": 0.037656734016532384,
                "bleu2": 0.04067392145167686,
                "bleu4": 0.048963321289052536,
                "rouge1": 0,
                "rouge2": 0,
                "rougel": 0
            }
        }
    },
    "success": true
}
```

## Wiki 评测

Wiki 评测会创建隔离的临时知识库，导入 EnterpriseRAG 的 85 篇文档，并用指定 Chat 模型运行现有 Wiki 生成流程。评分阶段不调用 Chat 模型：系统先按同类型节点进行规范化名称和别名精确匹配，再用指定 Embedding 模型补充语义匹配，最后计算实体、概念、总体覆盖率及有向图边 Precision、Recall、F1。成功或失败收尾后都会删除临时知识库，运行配置、冻结结果和报告继续保留。

### 创建任务

```bash
curl --location 'http://localhost:8080/api/v1/evaluation/wiki/runs' \
  --header 'X-API-Key: sk-xxxxx' \
  --header 'Content-Type: application/json' \
  --data '{
    "dataset_id": "enterprise_rag",
    "chat_id": "chat-model-id",
    "embedding_id": "embedding-model-id",
    "semantic_threshold": 0.80
  }'
```

服务返回 HTTP `202 Accepted`。`semantic_threshold` 取值范围为 `[0,1]`，省略时使用默认值 `0.80`；显式传 `0` 会保留零阈值。

### 查询历史和详情

```bash
curl 'http://localhost:8080/api/v1/evaluation/wiki/runs?page=1&page_size=20' \
  --header 'X-API-Key: sk-xxxxx'

curl 'http://localhost:8080/api/v1/evaluation/wiki/runs/<run-id>' \
  --header 'X-API-Key: sk-xxxxx'
```

运行状态为 `0=pending`、`1=running`、`2=success`、`3=failed`、`4=interrupted`。详情中的 `stage` 给出当前阶段，`failure_stage` 保留首次发生业务错误的阶段。成功结果的 `metric` 包含：

- `entity`、`concept`、`overall`：Gold 总数、精确命中数、语义命中数、未命中数和覆盖率。
- `graph`：正确、缺失、多余有向边数量及 Precision、Recall、F1。
- `generation_cost`、`scoring_cost`：按生成和评分 request group 分开的模型调用统计。

`result` 保存逐节点匹配、正确/缺失/多余/未评分边以及参与评分的冻结页面。失败发生在完整评分前时，`metric` 和 `result` 为空，避免把不完整结果显示成零分。

### 下载报告

```bash
curl -OJ 'http://localhost:8080/api/v1/evaluation/wiki/runs/<run-id>/report?format=json' \
  --header 'X-API-Key: sk-xxxxx'

curl -OJ 'http://localhost:8080/api/v1/evaluation/wiki/runs/<run-id>/report?format=markdown' \
  --header 'X-API-Key: sk-xxxxx'
```

报告完全从已持久化的运行快照渲染，下载时不会重新读取已删除的临时知识库，也不会再次调用模型。

## POST `/evaluation` - 创建评估任务

**参数说明（请求体）**:

| 字段              | 类型   | 必填 | 说明                                            |
| ----------------- | ------ | ---- | ----------------------------------------------- |
| dataset_id        | string | 是   | 评估数据集，目前仅支持 `default`（官方测试集）   |
| knowledge_base_id | string | 是   | 评估使用的知识库 ID                              |
| chat_id           | string | 是   | 评估使用的对话模型 ID                            |
| rerank_id         | string | 是   | 评估使用的重排序模型 ID                          |

**请求**:

```bash
curl --location 'http://localhost:8080/api/v1/evaluation' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json' \
--data '{
    "dataset_id": "default",
    "knowledge_base_id": "kb-00000001",
    "chat_id": "8aea788c-bb30-4898-809e-e40c14ffb48c",
    "rerank_id": "b30171a1-787b-426e-a293-735cd5ac16c0"
}'
```

**响应**:

```json
{
    "data": {
        "task": {
            "id": "c34563ad-b09f-4858-b72e-e92beb80becb",
            "tenant_id": 1,
            "dataset_id": "default",
            "start_time": "2025-08-12T14:54:26.221804768+08:00",
            "status": 1
        },
        "params": {
            "session_id": "",
            "knowledge_base_id": "2ef57434-8c8d-4442-b967-2f7fc578a2fc",
            "vector_threshold": 0.5,
            "keyword_threshold": 0.3,
            "embedding_top_k": 10,
            "vector_database": "",
            "rerank_model_id": "b30171a1-787b-426e-a293-735cd5ac16c0",
            "rerank_top_k": 5,
            "rerank_threshold": 0.7,
            "chat_model_id": "8aea788c-bb30-4898-809e-e40c14ffb48c",
            "summary_config": {
                "max_tokens": 0,
                "repeat_penalty": 1,
                "top_k": 0,
                "top_p": 0,
                "frequency_penalty": 0,
                "presence_penalty": 0,
                "prompt": "这是用户和助手之间的对话。",
                "context_template": "你是一个专业的智能信息检索助手，xxx",
                "no_match_prefix": "<think>\n</think>\nNO_MATCH",
                "temperature": 0.3,
                "seed": 0,
                "max_completion_tokens": 2048
            },
            "fallback_strategy": "",
            "fallback_response": "抱歉，我无法回答这个问题。"
        }
    },
    "success": true
}
```
