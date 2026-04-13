# 本地离线媒体治理系统 Provider Adapter v1

- 日期: 2026-04-09
- 状态: Draft

## 1. 目标

本文件定义第一版 AI Provider Adapter 接口和运行时边界，解决以下问题：

- Go 主服务如何调用 Python AI Worker
- Python Worker 如何适配不同本地推理后端
- 不同 Provider 的请求/响应差异如何被屏蔽
- Provider 不可用时如何降级

第一版适用 Provider：

- `Ollama`
- `LM Studio`

## 2. 总体结构

第一版采用三层结构：

1. `Go 主服务`
2. `Python AI Worker`
3. `Provider Adapter`

关系如下：

- Go 主服务负责任务调度、数据落库、状态管理
- Python Worker 负责理解任务编排和模型调用
- Provider Adapter 负责把统一请求翻译成具体 Provider 请求

## 3. 第一版关键决策

### 3.1 Worker 进程形态

第一版采用：

- `长驻 Python sidecar`

不采用：

- 每次任务都起一个新 Python 进程

原因：

- 冷启动成本更低
- 更适合连续批处理
- 更容易复用 provider 连接和模型状态

### 3.2 Go 与 Worker 通信方式

第一版采用：

- `stdio JSON lines`

即：

- Go 启动 Worker 进程
- 双方通过 stdin/stdout 传递 JSON 行
- 每个请求带唯一 `request_id`

不采用：

- Go 与 Worker 之间再起一层本地 HTTP 服务

原因：

- 少一个额外监听端口
- 少一层服务生命周期管理
- 对单机本地系统更直接

### 3.3 Provider 调用方式

对外统一由 Worker 暴露协议，对内各 Provider 自己实现。

建议：

- `Ollama Adapter` 走 Ollama 原生 REST API
- `LM Studio Adapter` 走 LM Studio 本地 OpenAI-compatible API

这样做的原因：

- Ollama 原生 API 对视觉输入和本地运行更直接
- LM Studio 官方对 OpenAI-like 接口支持成熟
- Worker 层负责屏蔽差异

## 4. Worker 对外协议

### 4.1 基本消息结构

所有请求与响应都带：

- `request_id`
- `type`

请求示例：

```json
{
  "request_id": "req_0001",
  "type": "understand_media",
  "payload": {
    "file_id": 123,
    "media_type": "video",
    "file_path": "/Volumes/D1/media/a.mp4",
    "frame_paths": [
      "/tmp/frames/a_001.jpg",
      "/tmp/frames/a_002.jpg"
    ],
    "context": {
      "allow_sensitive_labels": true,
      "max_tags": 20,
      "language": "zh-CN"
    }
  }
}
```

响应示例：

```json
{
  "request_id": "req_0001",
  "type": "result",
  "ok": true,
  "payload": {
    "raw_tags": ["自拍做爱", "室内", "竖屏手机拍摄"],
    "canonical_candidates": [
      {"namespace": "content", "name": "自拍做爱", "confidence": 0.92},
      {"namespace": "sensitive", "name": "做爱", "confidence": 0.90}
    ],
    "summary": "手机竖屏自拍性爱视频，室内，单镜头。",
    "sensitive_tags": ["做爱"],
    "quality_hints": ["竖屏自拍", "清晰度中等"],
    "structured_attributes": {
      "subject_count": "2",
      "capture_type": "selfie",
      "orientation": "portrait",
      "is_sensitive": true
    },
    "confidence": 0.88,
    "provider": "ollama",
    "model": "qwen3-vl-8b",
    "raw_response": {
      "provider_trace_id": "local"
    }
  }
}
```

### 4.2 支持的消息类型

第一版 Worker 至少支持：

- `health_check`
- `understand_media`
- `list_models`

可选支持：

- `reload_config`

### 4.3 流式状态消息

当任务较长时，Worker 可主动输出阶段性状态：

```json
{
  "request_id": "req_0001",
  "type": "progress",
  "payload": {
    "stage": "provider_inference",
    "message": "正在调用视觉模型",
    "percent": 72
  }
}
```

### 4.4 错误响应

```json
{
  "request_id": "req_0001",
  "type": "error",
  "ok": false,
  "error": {
    "code": "provider_unavailable",
    "message": "Ollama local server is not reachable",
    "retryable": true
  }
}
```

## 5. 统一任务输入输出

## 5.1 `understand_media` 输入

建议字段：

- `file_id`
- `media_type`
- `file_path`
- `thumbnail_paths`
- `frame_paths`
- `context`

其中：

- 图片任务主要使用 `thumbnail_paths`
- 视频任务主要使用 `frame_paths`

### 5.1.1 context 建议字段

- `allow_sensitive_labels`
- `language`
- `max_tags`
- `detail_level`
- `prompt_profile`

## 5.2 `understand_media` 输出

统一输出：

- `raw_tags`
- `canonical_candidates`
- `summary`
- `sensitive_tags`
- `quality_hints`
- `structured_attributes`
- `confidence`
- `provider`
- `model`
- `raw_response`

说明：

- 正式标签关系不在 Worker 内落库
- Worker 只返回建议结构
- 主服务负责规范化、落库和当前版本切换

## 6. Provider Adapter 最小接口

第一版 Python 内部建议定义抽象基类：

```python
class ProviderAdapter(Protocol):
    def health_check(self) -> dict: ...
    def list_models(self) -> list[dict]: ...
    def supports_vision(self) -> bool: ...
    def understand_media(self, req: UnderstandMediaRequest) -> UnderstandMediaResult: ...
```

第一版坚持最小接口，不把抽象做重。

## 7. Provider 实现建议

## 7.1 Ollama Adapter

建议：

- 使用 Ollama 原生本地 REST API
- 默认地址：`http://localhost:11434/api`
- 视觉任务优先使用 `POST /api/chat`

原因：

- 原生 API 对图像输入直接支持
- 官方明确支持 `images` 数组
- API 稳定且面向本地运行

输入适配：

- 将图片路径或 base64 图像写入消息体
- Worker 在本地读取文件并构造 Provider 请求

输出适配：

- 从 Ollama 响应中提取文本结果
- 再由 Worker 转为统一 JSON

## 7.2 LM Studio Adapter

建议：

- 使用 LM Studio 本地 OpenAI-compatible API
- 默认地址：`http://localhost:1234/v1`

原因：

- 官方明确支持本地 OpenAI-like endpoints
- 支持本地 server
- 适合复用 OpenAI 风格 client

输入适配：

- 使用 `responses` 或 `chat/completions` 风格请求
- 图片输入按 OpenAI-like 结构组织

输出适配：

- 将返回内容提取为统一结构
- Worker 继续负责结构化解析

## 7.3 选择顺序

第一版默认顺序：

1. `Ollama`
2. `LM Studio`

但实际生效的 provider 应可通过配置切换。

## 8. 结果解析策略

第一版不要求 Provider 直接返回完美 JSON。  
Worker 负责：

- 构造结构化 prompt
- 解析文本结果
- 尽量输出稳定字段

建议：

- 优先要求模型输出 JSON
- 若解析失败，则退回“宽松提取模式”
- 绝不能因为返回格式轻微偏差就丢弃整次理解结果

## 9. 错误分类

建议统一错误码：

- `provider_unavailable`
- `model_not_loaded`
- `model_not_found`
- `vision_not_supported`
- `request_timeout`
- `invalid_response`
- `worker_internal_error`

每个错误建议包含：

- `message`
- `retryable`

## 10. 降级策略

### 10.1 Provider 不可用

若默认 Provider 不可用：

- 尝试下一个可用 Provider
- 若仍不可用，则任务失败并可重试

### 10.2 视觉不支持

若当前模型不支持视觉：

- 不进行假推理
- 直接返回 `vision_not_supported`

### 10.3 Worker 不可用

若 Worker 崩溃：

- Go 主服务应重启 Worker
- 当前任务重新排队或按 lease 超时后重取

## 11. 配置建议

第一版建议 Worker 配置包含：

- `default_provider`
- `provider_priority`
- `ollama.base_url`
- `ollama.model`
- `lm_studio.base_url`
- `lm_studio.model`
- `timeout_seconds`
- `max_images_per_request`

## 12. 日志与调试

第一版建议：

- Worker 标准错误输出用于运行日志
- 标准输出只输出协议消息
- 请求和响应都带 `request_id`
- 失败时记录 provider、model、error_code

## 13. 第一版实现边界

第一版不做：

- 多 Provider 并行投票
- 自动模型下载与加载编排
- 复杂路由策略
- Provider 级流式多路复用

第一版要做到：

- 可切换 Provider
- 统一请求协议
- 统一返回结构
- 明确错误分类
- 明确降级路径

## 14. 当前结论

第一版最稳的实现方式是：

- Go 主服务通过 `stdio JSON lines` 驱动长驻 Python Worker
- Python Worker 内部通过 Provider Adapter 适配 `Ollama` 和 `LM Studio`
- Worker 对外只暴露少量统一任务协议
- 主服务负责真正的业务落库和状态切换
