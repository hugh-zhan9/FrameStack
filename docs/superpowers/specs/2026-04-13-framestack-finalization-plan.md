# FrameStack 收尾计划

## 1. 目标

FrameStack 当前已经具备可运行的 MVP，但距离“核心版完成”还差两块能力：

- 正式的分层 embedding 方案
- 更稳定的 `same_person` 正式方案

本计划的目标，是在不破坏当前可运行主链路的前提下，补齐这两块能力。

## 2. 里程碑

### 2.1 Embedding 正式化

- 建立 `image_visual` / `video_frame_visual` / `person_visual` 三类正式边界
- `embed_media` 请求显式带上 `embedding_type`
- 存储、读取、索引按类型分层
- 禁止不同类型和不同模型直接混比

### 2.2 same_person 正式化

- `same_person` 增加独立人物向量通道
- 标签和弱语义信号从主入口降级为辅助召回
- 打分逻辑转向“人物向量为主、结构化信号为辅”
- UI 能解释候选强弱来源

### 2.3 联调与收尾

- 真实 provider 联调
- 真实样本阈值回归
- 清理文档与配置示例

## 3. 开发顺序

1. 建立 `person_visual` 基础设施
2. 规范 worker 协议中的 `embedding_type`
3. 为 `same_person` 新增人物向量读取和候选路径
4. 重构 `same_person` 打分与展示
5. 做真实环境联调

## 4. 约束

- 不回退当前已可用的 `same_content` / `same_series` / `same_person` 主链路
- 每次改动都先补失败测试
- 文档、测试、flight recorder 同步更新
