# FrameStack 收尾开发计划

## 目标

补齐两块剩余核心能力：

1. `embedding` 从过渡方案升级为正式的分层方案
2. `same_person` 从标签/弱信号驱动升级为更稳定的人物候选方案

## 阶段

| 阶段 | 状态 | 说明 |
| --- | --- | --- |
| 1 | complete | ~~建立正式 embedding 分层：引入 `person_visual` 类型、请求协议、存储与索引边界~~ |
| 2 | complete | ~~让 `same_content` / `same_series` 持续消费正式 `image_visual` / `video_frame_visual`，清理过渡耦合~~ |
| 3 | complete | ~~为 `same_person` 增加独立人物向量通道和候选召回路径~~ |
| 4 | complete | ~~把 `same_person` 打分、解释和 UI 展示迁移到新人物通道~~ |
| 5 | pending | 做真实环境联调：数据库、worker provider、真实样本回归 |

## 当前决策

- ~~`image_visual` 和 `video_frame_visual` 继续保留，避免影响已有聚类主链路~~
- ~~新增 `person_visual`，不和通用视觉向量混用~~
- ~~先做基础设施，再把 `same_person` 改到新通道~~
- ~~每个阶段先补失败测试，再做最小实现~~

## 阶段 1 进度

- ~~`embed_media` 请求显式带上 `embedding_type`~~
- ~~引入 `person_visual` 类型常量~~
- ~~数据库 migration 支持 `person_visual` 类型与索引~~
- ~~`same_person` 读取侧优先消费 `person_visual`~~
- ~~`person_visual` 实际写入任务与执行链路~~

## 阶段 3 进度

- ~~`same_person` 读取侧在图片/视频帧上优先消费 `person_visual`~~
- ~~`same_person` 候选召回显式感知 embedding 类型，不再把 `person_visual` 和通用视觉向量混用~~
- ~~`same_person` 打分已让 `person_visual` 证据强于通用视觉证据~~

## 阶段 4 进度

- ~~cluster detail 已透传 `embedding_type` 到成员级别~~
- ~~`same_person` 成员卡片已明确标注 `Person vector evidence` / `Generic visual fallback`~~
- ~~cluster detail 已补充 `person_visual` / 通用视觉覆盖数量和 `top_evidence_type` 摘要~~
- ~~cluster member 已透传 `has_face / subject_count / capture_type`~~
- ~~`same_person` 成员卡片已展示结构化证据轨迹~~
- ~~聚类列表已透传 `person_visual_count / generic_visual_count / top_evidence_type`~~
- ~~`same_person` 审核队列已在列表层显示人物证据摘要~~

## 阶段 2 进度

- ~~`same_content` 已显式透传 `image_visual / video_frame_visual` 的 `embedding_type`~~
- ~~`same_series` 已显式透传 `image_visual / video_frame_visual` 的 `embedding_type`~~
- ~~`same_content` 现在按 `embedding_type + model_name` 双重校验，不再只靠模型名~~
- ~~`same_series` 现在按 `embedding_type + model_name` 双重校验，不再只靠模型名~~
- ~~相关 Postgres 读取、服务判定和测试夹具已统一到正式通道契约~~

## 完成标准

- `person_visual` 具备独立类型、存储、索引和读取边界
- `same_person` 不再主要依赖标签和弱语义信号
- 新旧向量类型不会混比
- 至少完成一轮真实 provider 的端到端验证
