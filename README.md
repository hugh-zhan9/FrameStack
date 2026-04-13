# FrameStack / 影栈

FrameStack（影栈）是一个本地离线的图片与视频资产治理系统。

本目录当前是 FrameStack 的第一版工程骨架。

## 当前结构

- `cmd/server`
  Go 主服务入口
- `internal/config`
  基础配置加载
- `internal/httpserver`
  最小 HTTP 路由
- `internal/app`
  应用装配层
- `worker`
  Python AI worker 协议骨架
- `db/migrations`
  数据库 migration
- `docs/superpowers/specs`
  设计文档

## 当前可用能力

- Go 配置加载
- `/healthz` 进程存活探针
- `/readyz` 真实就绪探针
  - 返回 `system status` 快照
  - 当必需依赖未就绪时返回 `503`
- `/api/system-status` 系统依赖状态接口
  - 返回当前 `database` / `worker` 检查结果
- `/api/task-summary` 最小任务摘要接口
- `/api/jobs` 最近任务列表接口
- `POST /api/jobs` 开发期任务入队接口
- `POST /api/jobs/{id}/retry` 失败任务重试接口
- `/api/jobs/{id}/events` 任务事件时间线接口
- `/api/volumes` 卷列表与创建接口
- `POST /api/volumes/{id}/scan`
  - 为指定 volume 正式下发 `scan_volume` 任务
  - 用于 UI 和联调脚本，不必再手写底层 jobs payload
- `/api/tags` 标签列表接口
  - 支持 `namespace` 过滤
  - 支持 `limit` 限制
- `/api/clusters` 候选聚类列表接口
  - 支持 `cluster_type`、`status`、`limit`
- `/api/clusters/{id}` 单个候选聚类详情接口
  - 返回成员列表和成员质量等级
- `POST /api/clusters/{id}/review-actions`
  - 记录聚类级人工动作
  - 当前支持整组 `keep`、`favorite`、`trash_candidate`
- `POST /api/clusters/{id}/status`
  - 更新聚类审核状态
  - 当前支持 `candidate`、`confirmed`、`ignored`
- `/api/files` 最近文件列表接口
  - 支持 `q` 查询参数，基于 `search_documents.tsv` 做全文搜索
  - 支持 `media_type`、`quality_tier`、`review_action`、`status`、`volume_id`、`tag_namespace`、`tag`、`cluster_type`、`cluster_status` 结构化筛选
  - 支持 `offset` 分页和 `sort` 排序
  - `sort` 当前支持 `updated_desc`、`quality_desc`、`size_desc`、`size_asc`、`name_asc`
- `/api/files/{id}` 单文件详情接口
  - 返回基础元数据
  - 视频会额外返回 `fps`、`bitrate`、`video_codec`、`audio_codec`
  - 视频会返回最多 6 条关键帧摘要：`timestamp_ms`、`frame_role`、`phash`
  - 返回 `file_path_history`
  - 返回当前分析摘要、质量分数/等级、标签和最近人工操作
  - 返回所属 `same_content` / `same_series` 聚类
- `/api/files/{id}/content`
  - 流式返回文件原始内容
  - 图片可直接预览
  - 视频可直接在浏览器中播放
- `/api/files/{id}/preview`
  - 图片优先返回本地缩略图缓存，不存在时回退原图
  - 视频优先返回 `poster`
  - 供文件列表卡片和轻量浏览使用
- `/api/files/{id}/frames/{index}/preview`
  - 返回视频关键帧预览
  - 当前按详情页里同样的关键帧顺序读取
  - 供视频详情里的关键帧审核使用
- `POST /api/files/{id}/tags`
  - 新增手工标签
  - 当前支持直接写入 `content / quality / management / sensitive / person`
  - `person` 标签会即时触发 `same_person` 重聚类
- `DELETE /api/files/{id}/tags?namespace=...&name=...`
  - 删除手工标签
  - 当前只删除 `source=human` 的标签
  - 删除 `person` 标签会即时触发对应 `same_person` 重聚类
- `POST /api/files/{id}/reveal`
  - 在 macOS Finder 中定位当前文件
- `POST /api/files/{id}/recompute-embeddings`
  - 先重新入队 `infer_tags`，再按文件媒体类型重新入队 embedding 任务
  - 这样 `semantic` provider 会基于最新理解结果刷新语义向量
  - 图片重新入队 `embed_image`
  - 视频重新入队 `embed_video_frames`
- `POST /api/files/{id}/recluster`
  - 重新入队 `same_content` / `same_series` / `same_person`
  - 便于切换 provider、修正手工标签或调规则后做局部重算
- `POST /api/files/{id}/review-actions`
  - 记录文件级人工动作
  - 当前支持 `keep`、`favorite`、`ignore`、`hide`、`trash_candidate`
- `POST /api/files/{id}/trash`
  - 调用 macOS 废纸篓
  - 更新 `files.status=trashed`
  - 写入 `review_actions.deleted_to_trash`
- 可选 PostgreSQL 接入开关
- `scan_volume` 真实扫描执行：
  - 遍历 volume 挂载目录
  - 识别图片与视频文件
  - 写入 `files`
  - 写入 `file_path_history`
  - 标记缺失文件为 `missing`
  - 对新增或变化文件幂等下发 `hash_file` 与 `extract_image_features` / `extract_video_features`
- `hash_file` 真实执行：
  - 读取文件内容
  - 计算 `sha256`
  - 计算 `quick_hash`
  - 回写 `files.sha256` 与 `files.quick_hash`
  - 幂等下发 `cluster_same_content`
- `extract_image_features` 真实执行：
  - 读取 `files`
  - 提取图片基础元数据
  - 计算图片 `pHash`
  - 生成本地缩略图缓存
  - 写入 `image_assets`
  - 幂等下发 `recompute_search_doc`、`infer_tags`、`infer_quality`、`embed_image`、`cluster_same_content`、`cluster_same_series`
- `extract_video_features` 真实执行：
  - 读取 `files`
  - 优先通过 `ffprobe` 提取视频基础元数据
  - 如果本机存在 `ffmpeg`，会额外生成 `poster` 和 `understanding` 关键帧
  - 会为关键帧计算 `pHash`
  - 会写入 `video_frames`
  - `ffprobe` 不可用时仍写入最小 `video_assets` 记录
  - 幂等下发 `recompute_search_doc`、`infer_tags`、`infer_quality`、`embed_video_frames`、`cluster_same_content`、`cluster_same_series`
- `embed_image` 真实执行：
  - 读取 `image_assets.phash`
  - 调用 Python worker 的 `embed_media`
  - 请求会显式带上 `embedding_type=image_visual`
  - 当前默认由 worker 使用 `ffmpeg` 生成 `8x8` 灰度视觉向量
  - 也支持 `semantic` provider：复用 `understand_media` 的 VLM 输出生成 `64` 维语义哈希向量
  - `ffmpeg` 不可用或读取失败时，会回退为基于 `pHash` 的 `64` 维稳定占位向量
  - 会保留 worker 返回的真实 `model_name`
  - 写入最新 `embeddings(image_visual)`
- `embed_video_frames` 真实执行：
  - 读取 `video_frames(frame_role=understanding)`
  - 调用 Python worker 的 `embed_media`
  - 请求会显式带上 `embedding_type=video_frame_visual`
  - 当前默认由 worker 使用关键帧图片生成 `8x8` 灰度视觉向量
  - 也支持 `semantic` provider：逐帧复用 `understand_media` 的 VLM 输出生成 `64` 维语义哈希向量
  - `ffmpeg` 不可用或读取失败时，会回退为基于关键帧 `pHash` 的 `64` 维稳定占位向量
  - 替换帧向量时会先清空该文件下同类型旧 embedding，避免不同 model 混存
  - 写入最新 `embeddings(video_frame_visual)`
- `embed_person_image` / `embed_person_video_frames` 真实执行：
  - 复用现有 worker `embed_media`
  - 显式带上 `embedding_type=person_visual`
  - 当前沿用现有 provider 与 fallback 逻辑
  - 写入最新 `embeddings(person_visual)`
- `recompute_search_doc` 真实执行：
  - 联表读取 `files`、`image_assets`、`video_assets`
  - 生成基础全文搜索文档
  - 写入 `search_documents`
  - 写入 `analysis_results(search_doc)` 与 `file_current_analysis`
- `infer_tags` 真实执行：
  - 读取 `files`
  - 视频会额外读取 `understanding` 关键帧路径
  - 调用 Python worker 的 `understand_media`
  - prompt 会显式约束 `canonical_candidates` 只使用 `content / quality / sensitive / person / management`
  - prompt 会要求输出更稳定的 `structured_attributes`，并鼓励产出可重复使用的 `person` 外观候选标签
  - 写入 `analysis_results(understanding)` 与 `file_current_analysis`
  - 写入 `tags` 与 `file_tags`
  - 当结果包含 `person` 标签或明显人物结构化信号时，会条件性下发 `embed_person_image` / `embed_person_video_frames`
  - 幂等下发 `cluster_same_person`
- `infer_quality` 真实执行：
  - 读取 `files`、`image_assets`、`video_assets`
  - 按分辨率和视频码率生成基础质量评分
  - 视频会额外纳入 `fps`、`duration_ms`、`video_codec`、`container`
  - 写入 `analysis_results(quality)` 与 `file_current_analysis`
- Python worker 基础协议：
  - `health_check`
  - `list_models`
  - `understand_media` 支持 `placeholder`、`ollama`、`lm_studio`
  - `embed_media` 支持 `pixel`、`semantic`、`placeholder`
  - `embed_media` 默认走 `pixel` provider，使用 `ffmpeg` 生成本地视觉向量
  - `semantic` provider 会复用 `understand_media` 当前配置的 VLM，把标签/摘要/结构化属性压成稳定的 `64` 维语义向量
  - `embed_media` 失败时自动回退 `placeholder` 向量
  - provider 调用失败时自动回退占位结果
  - 未知消息错误返回
- migration 文件发现与顺序执行骨架
- embedding 相关 migration 已补充索引
  - 最新向量读取索引
  - `image_visual` / `video_frame_visual` 的 `ivfflat` 距离索引
  - schema 已预留 `person_visual` 类型及其独立索引，供后续 `same_person` 正式化使用
- 首页数据面板：
  - System Status
  - 任务摘要
  - Volumes
  - Volume 直接触发扫描
  - Top Tags
  - Top Tags namespace 切换
  - Recent Clusters
  - Cluster type / status 过滤
  - Cluster detail 与成员预览
  - Cluster detail 审核摘要与成员卡片化展示
  - Cluster detail 直接 `Confirm` / `Ignore` / `Reset Candidate`
  - Cluster detail 整组 `Keep` / `Favorite` / `Trash Candidate`
  - 最近任务
  - 最近文件
  - 最近文件搜索框
  - 最近文件结构化筛选
  - 最近文件质量等级筛选
  - 最近文件人工动作筛选
  - 最近文件标签命名空间筛选
  - 最近文件标签精确筛选
  - 最近文件按聚类类型和聚类状态筛选
  - 最近文件标签摘要
  - 最近文件质量等级、质量分数和最新人工动作摘要
  - 最近文件按质量分数排序
  - 最近文件分页和排序
  - 最近文件卡片封面预览
  - 最近文件照片管理器式卡片网格
  - 最近文件多选与批量审核
    - `Keep`
    - `Favorite`
    - `Trash Candidate`
  - 最近文件当前筛选条件可视化
  - 工作台式首页信息架构
    - 顶部摘要区
    - 中央浏览区
    - 右侧详情与时间线侧栏
  - 文件详情、当前分析、标签与路径历史
  - 文件详情所属聚类
  - 文件详情最近人工操作历史
  - 文件详情直接在 Finder 中定位
  - 文件详情直接标记 `Keep` / `Favorite`
  - 文件详情直接移动到废纸篓
  - 选中任务的事件时间线
  - 文件详情展示 `quality` 分析结果
  - 文件详情展示当前 embedding 摘要
    - `embedding_type`
    - `provider`
    - `model_name`
    - `vector_count`
  - 文件详情展示视频关键帧摘要
  - 文件详情展示视频关键帧预览
  - 文件详情直接触发 `Recompute Embeddings` / `Recluster`
  - 文件详情直接预览图片或视频
  - 文件详情直接新增手工标签
  - 文件详情可删除手工标签
  - 聚类详情的成员列表展示当前 embedding 摘要
    - `provider`
    - `model_name`
    - `vector_count`

## 测试

```bash
go test ./...
python3 -m unittest discover -s worker -p '*_test.py'
```

也可以直接用：

```bash
make test
```

联调前建议先跑：

```bash
make doctor
```

如果需要 JSON：

```bash
go run ./cmd/devdoctor --json
```

生成一份本地样本图片：

```bash
make sample-media
```

## 运行服务

```bash
go run ./cmd/server
```

默认监听：

```txt
:8080
```

常用快捷命令：

```bash
make doctor
make sample-media
make dev
make db-dev
make db-migrate
```

## 最小端到端验证

1. 生成样本素材：

```bash
make sample-media
```

默认会生成到 `./tmp/dev-media`，其中包含：

- 一组完全重复图片
- 一组同目录同批次图片
- 一张额外 poster 图片

2. 启动数据库 migration：

```bash
make db-migrate
```

3. 启动服务：

```bash
make db-dev
```

4. 注册样本目录为 volume：

```bash
curl -X POST http://127.0.0.1:8080/api/volumes \
  -H 'Content-Type: application/json' \
  -d '{"display_name":"dev-sample","mount_path":"./tmp/dev-media"}'
```

5. 触发扫描：

```bash
curl -X POST http://127.0.0.1:8080/api/volumes/1/scan
```

6. 打开浏览器访问：

```txt
http://127.0.0.1:8080
```

建议依次验证：

- `System Status` 是否为 `ready`
- `Volumes` 中扫描任务是否入队
- `Recent Files` 是否出现样本图片
- 文件详情是否能直接预览图片
- `same_content` 是否识别出重复对
- `same_series` 是否识别出同目录近时间图片
- 给某张图手工加 `person` 标签后，`same_person` 是否出现候选组

## 主要环境变量

```bash
IDEA_HTTP_ADDR=:8080
IDEA_DATABASE_URL=postgres://localhost:5432/framestack?sslmode=disable
IDEA_DEFAULT_PROVIDER=ollama
IDEA_ENABLE_DATABASE=false
IDEA_RUN_JOB_WORKER=true
IDEA_JOB_WORKER_NAME=local-server
IDEA_JOB_POLL_INTERVAL=2s
IDEA_MIGRATIONS_DIR=db/migrations
IDEA_RUN_MIGRATIONS=false
IDEA_WORKER_COMMAND=python3
IDEA_WORKER_SCRIPT=worker/main.py
IDEA_WORKER_PROVIDER=placeholder
IDEA_WORKER_PROVIDER_TIMEOUT_SEC=60
IDEA_WORKER_OLLAMA_URL=http://127.0.0.1:11434
IDEA_WORKER_OLLAMA_MODEL=qwen3-vl-8b
IDEA_WORKER_LM_STUDIO_URL=http://127.0.0.1:1234
IDEA_WORKER_LM_STUDIO_MODEL=qwen2.5-vl-7b
```

启用数据库后，服务会尝试：

- 打开 PostgreSQL 连接
- 可选执行 migration
- 用 `jobs` 表驱动 `/api/task-summary`
- 用 `jobs` 表驱动 `/api/jobs`
- 用 `jobs` 表支持基础入队和重试
- 用 `volumes` / `files` 表驱动首页卷列表和最近文件
- 用 `search_documents` 支撑 `/api/files?q=...` 搜索
- 可选启动后台 job runner 自动消费队列

当前系统状态语义：

- `database`
  - `IDEA_ENABLE_DATABASE=false` 时标记为 `disabled`
  - 启用后按 PostgreSQL `Ping` 结果判定
- `worker`
  - `IDEA_RUN_JOB_WORKER=false` 时标记为 `disabled`
  - 启用后按 Python worker `health_check` 结果判定
- `/healthz`
  - 只表示 HTTP 进程存活
- `/readyz`
  - 只要存在 `not_ready` 的必需依赖，就返回 `503`

如果同时开启 `IDEA_ENABLE_DATABASE=true` 和 `IDEA_RUN_JOB_WORKER=true`，当前 runner 会真实执行：

- `scan_volume`
- `extract_image_features`
- `extract_video_features`
- `hash_file`
- `cluster_same_content`
- `cluster_same_series`
- `embed_image`
- `embed_video_frames`
- `recompute_search_doc`
- `infer_tags`
- `infer_quality`
- `cluster_same_content` 会基于完全相同的 `sha256` 产出 `same_content` 候选聚类
- 当图片存在相近的 `pHash` 时，也会基于图片 `pHash` 产出 `same_content` 候选聚类
  - 当前实现先按 `pHash` 前缀召回候选，再按汉明距离做保守过滤
  - 对图片 `pHash` 命中的候选，也会额外校验方向与纵横比兼容性，避免把横图和竖图这类构图明显冲突的图片误并成同内容
  - 图片方向过滤现在使用真实文件分辨率作为锚点，不依赖候选返回顺序
- 当图片 `pHash` 不足以成组、且存在最新的 `image_visual` embedding 时，也会基于图片 embedding 做保守候选聚类
  - 当前图片 embedding 召回已从字符串前缀升级为向量距离过滤
  - 当前图片 embedding 在读取、传输和判定上都显式使用 `embedding_type=image_visual`
  - 只有 `model_name` 相同的图片向量才会直接做距离比较
  - 只有 `embedding_type` 和 `model_name` 同时一致的图片向量才会直接做距离比较
  - 对仅靠图片 embedding 命中的候选，还会额外校验纵横比兼容性，避免把明显不同构图的图片误并成同内容
  - 对仅靠图片 embedding 命中的候选，还会额外校验分辨率比例，避免把极小缩略图和原图直接并成同内容
  - 图片 embedding 的方向与尺寸过滤同样使用真实文件分辨率作为锚点，不依赖候选返回顺序
- 当视频存在至少两张 `understanding` 关键帧 `pHash` 完全一致时，也会产出保守的 `same_content` 候选聚类
  - 对视频 `pHash` 命中的候选，也会额外校验时长兼容性，避免把时长差过大的视频误并成同内容
  - 对视频 `pHash` 命中的候选，只要存在分辨率信息，也会额外校验画幅方向和画幅比例兼容性，避免把横屏和竖屏视频、或 `16:9` 与 `4:3` 这类差异明显的视频直接并成同内容
- 当视频关键帧 `pHash` 不足以成组、且存在最新的 `video_frame_visual` embedding 时，也会基于关键帧 embedding 做保守候选聚类
  - 当前视频 embedding 召回也已升级为向量距离过滤，并要求至少两帧命中
  - 当前视频帧 embedding 在读取、传输和判定上都显式使用 `embedding_type=video_frame_visual`
  - 只有 `model_name` 相同的视频帧向量才会直接做距离比较
  - 只有 `embedding_type` 和 `model_name` 同时一致的视频帧向量才会直接做距离比较
  - 对仅靠视频 embedding 命中的候选，还会额外校验时长兼容性，避免把时长差过大的视频误并成同内容
  - 对仅靠视频 embedding 命中的候选，只要存在分辨率信息，也会额外校验画幅方向和画幅比例兼容性，避免把横屏和竖屏视频、或 `16:9` 与 `4:3` 这类差异明显的视频直接并成同内容
  - 视频方向过滤现在使用真实文件分辨率作为锚点，不依赖候选返回顺序
- `cluster_same_series` 会基于同目录、同媒体类型、接近修改时间先召回候选
  - 当文件名家族一致时，也会在接近修改时间窗口内做有限的跨目录补召回
  - 图片优先看文件名家族，其次看图片 `pHash` 近似
  - 当存在 embedding 时，图片也会用最新的 `image_visual` 做保守收口；跨目录候选必须同时满足文件名家族一致和技术特征接近
  - `same_series` 的图片 embedding 现在也会显式校验 `embedding_type=image_visual`
  - 对图片候选还会额外校验纵横比兼容性，避免把横图和竖图这类明显不同构图的素材并成同一系列
  - 对图片候选还会额外校验分辨率比例，避免把原图和极小缩略图直接并成同一系列
  - 视频优先看文件名家族，其次看 `understanding` 关键帧 `pHash` 是否有重合
  - 当存在 embedding 时，视频也会用最新的 `video_frame_visual` 做保守收口；跨目录候选必须同时满足文件名家族一致和技术特征接近
  - `same_series` 的视频 embedding 现在也会显式校验 `embedding_type=video_frame_visual`
  - 对视频候选还会额外校验时长兼容性，避免把时长差过大的视频并成同一系列
  - 对视频候选只要存在分辨率信息，也会额外校验画幅方向、画幅比例和分辨率兼容性，避免把横屏和竖屏视频、`16:9` 与 `4:3` 这类差异明显的视频，或 4K 原片与极小转码版本直接并成同一系列
  - 如果候选和锚点在 `capture_type` 上明显冲突（如 `photo` vs `screenshot`），即使同目录同 family 也不会直接并入同系列
  - embedding 只在同 model 下比较，避免不同 provider 的向量空间混用
  - 目标是宁可拆散，也避免把无关素材并进同一系列
- `cluster_same_person` 会基于 `person` namespace 标签产出保守的 `same_person` 候选聚类
- 手工 `person` 标签会保持完整聚类
- `same_person` 读取 embedding 时会优先使用 `person_visual`，没有时再回退到 `image_visual / video_frame_visual`
- `same_person` 的 embedding 匹配现在会显式区分人物向量和通用视觉向量
  - `person_visual` 证据强于通用视觉证据
  - `person_visual` 不会和通用视觉向量直接混比
- cluster detail 现在会直接显示成员使用的 `embedding_type`
  - `same_person` 审核界面会标注 `Person vector evidence` 或 `Generic visual fallback`
  - cluster detail 还会汇总当前组内 `person_visual` 数量、通用视觉数量，以及 `top_evidence_type`
  - `same_person` 成员卡片还会展示结构化证据轨迹，例如 `has face`、`same family`、`capture selfie`
  - 结构化证据轨迹会以次级样式展示，不和主证据提示混淆
  - 聚类列表也会直接展示人物证据摘要，不必点进详情页才能区分强弱组
- AI `person` 标签在候选过大时，会按文件名家族和目录做保守收口，避免把明显无关素材并进同一组
  - 当存在 embedding 时，也会用最新的图片 `image_visual` 或视频 `video_frame_visual` 做额外保留，避免把跨目录但相似的候选过早过滤掉
  - embedding 只在同 model 下比较，避免 `pixel / semantic / placeholder` 结果直接混比
  - 对 AI 大候选集，图片还会额外校验方向、纵横比和分辨率兼容性，视频会额外校验时长、画幅方向、画幅比例和分辨率兼容性，避免明显不一致的候选仅凭 embedding 被保留下来
- 当文件没有显式 `person` 标签时，会回退到 `content / sensitive` 里的弱人物信号（如 `单人写真`、`自拍`、`情侣` 等）做自动候选召回
  - 这类自动候选一定会经过 embedding / 路径 / 文件名家族收口，不会直接按泛标签硬并组
  - 同时也会消费 `understanding.structured_attributes` 里的结构化人物信号，例如 `subject_count=single`、`has_face=true`、`capture_type=selfie`
  - 这让没有显式 `person` 标签的视频和图片也能进入 `same_person` 候选
  - `情侣 / 多人 / AV / 做爱 / 口交` 这类弱自动信号不会仅凭同目录成组，至少要有文件名家族或 embedding 证据
  - 上述弱自动信号即使有 embedding 命中，也不会跨很长时间窗口直接成组
  - 上述弱自动信号在视频场景下还会校验时长兼容性，避免把时长差过大的视频继续并入同一人的候选
  - 上述弱自动视频候选还会校验画幅方向、画幅比例和分辨率兼容性，避免把横屏和竖屏视频、`16:9` 与 `5:4` 这类差异明显的视频，或 4K 原片与极小转码版本继续并入同一人的候选
  - 上述自动图片候选还会校验纵横比兼容性，避免把横图和竖图这类构图明显冲突的图片继续并入同一人的候选
  - 上述自动图片候选还会校验分辨率比例，避免把原图和极小缩略图继续并入同一人的候选
  - 如果弱自动信号候选和锚点在 `subject_count` 等结构化人物形态上明显冲突，也会被直接剔除
  - 如果弱自动信号候选和锚点在 `capture_type` 上明显冲突（如 `selfie` vs `screenshot`），也会被直接剔除
- `same_person` 的成员 score 现在有真实语义
  - 手工标签来源最高
  - AI `person` 标签次之
  - 自动人物信号最低
  - `has_face / subject_count / capture_type` 这类结构化人物信号也会抬高候选分
  - 时间更接近同一拍摄批次的候选会排得更靠前
  - 文件名家族一致、同目录、embedding 接近会继续抬高分数
  - UI 里可以直接利用这个 score 区分强候选和弱候选
- cluster 列表和详情也会直接暴露强度信息
  - `strong_member_count` 表示 score >= 0.80 的成员数量
  - `top_member_score` 表示组内最高候选分
- `same_content` 不再只是把重复版本聚在一起
  - 组内会按质量优先级排序
  - 当前优先级主要看：`quality_score`、分辨率、时长、文件大小
  - 视频还会额外纳入：`bitrate`、`fps`、`container`
  - 第一名会被标记为 `best_quality`
  - 其余成员标记为 `duplicate_candidate`
  - 前端审核界面会直接高亮 `Recommended Keep`，并把 `best_quality / duplicate_candidate` 做成不同的角色标识
- `same_series` 现在也有轻量审核焦点
  - 组内会挑一个时间中位点成员标记为 `series_focus`
  - 前端会显示 `Review Focus`，用于提示你先看哪一张/哪一段
  - 跨目录但同文件名家族的候选，仍然要求时间足够接近，避免把同名不同批次素材并成同一系列

其中：

- `scan_volume` 会索引文件并补下游任务
- `extract_image_features` 会写 `image_assets`
- `extract_video_features` 会写 `video_assets`
- `embed_image` / `embed_video_frames` 会写 `embeddings`
- `recompute_search_doc` 会写 `search_documents` 和 `search_doc` 分析结果
- `infer_tags` 会写 `understanding` 分析结果和 AI 标签
- `infer_quality` 会写 `quality` 分析结果

其中 `infer_tags` 当前支持：

- `IDEA_WORKER_PROVIDER=placeholder`
  - 始终返回本地占位理解结果
- `IDEA_WORKER_PROVIDER=ollama`
  - 调用 Ollama 本地 REST API
  - 当前请求目标：`/api/chat`
- `IDEA_WORKER_PROVIDER=lm_studio`
  - 调用 LM Studio 本地 OpenAI-compatible API
  - 当前请求目标：`/v1/chat/completions`

如果 provider 不可用、返回无效 JSON，worker 会自动回退到 placeholder 结果，保证任务链路不中断。

## 下一步建议

1. 为 `understanding` 接入更稳定的标签规范化与结构化属性映射
2. 把 `quality` 从基础规则评分扩展到更多客观指标
3. 为任务中心补充更细的进度和失败恢复信息
4. 在真实 embedding / 人脸链路就绪后再实现 `same_person`
