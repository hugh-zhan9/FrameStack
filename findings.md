# Findings

## 2026-04-13

- 当前三条聚类线的技术过滤边界已基本对齐：
  - 视频：时长、方向、画幅比例、分辨率比例
  - 图片：方向/纵横比、分辨率比例
- `same_person` 现在已经具备独立人物向量通道：
  - `person_visual` 可独立写入、读取和索引
  - 候选召回和打分会显式区分 `person_visual` 与通用视觉向量
  - `person_visual` 证据强于通用视觉证据
- `embeddings` schema 现在已经支持 `person_visual` 类型，并带有独立索引
- `embed_media` 请求协议现在已显式带上 `embedding_type`，后续可以按类型扩展 provider 行为
- `same_person` 查询现在已经优先读取 `person_visual`，因此后续只要生产出该类型向量，就能立即被候选聚类消费
- `understand` 服务现在会按 `person` 标签和结构化人物信号条件性下发 `person_visual` 任务
- `embed_person_image` / `embed_person_video_frames` 已接入 jobexecutor、任务白名单和主进程装配
- `same_content` / `same_series` 现在都显式携带正式 embedding 通道信息：
  - 图片只走 `image_visual`
  - 视频关键帧只走 `video_frame_visual`
  - 服务判定按 `embedding_type + model_name` 双重校验
- 当前剩余缺口已经集中到阶段 5：
  - 真实环境下验证数据库、worker provider、样本数据的端到端表现
