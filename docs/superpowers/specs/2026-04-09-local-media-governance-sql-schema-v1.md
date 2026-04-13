# 本地离线媒体治理系统 SQL Schema v1

- 日期: 2026-04-09
- 状态: Draft
- 适用数据库: PostgreSQL 15+
- 扩展依赖: `pgvector`

## 1. 目标

本文件定义第一版数据库结构草案，重点覆盖：

- 多 volume 资源索引
- 文件稳定身份与路径迁移
- 标签体系与标签关系
- 多版本分析结果
- 图片 / 视频特征
- 聚类候选
- 人工审核动作
- 任务队列与任务恢复

本 schema 设计原则：

- 尽量规范化
- 允许适度冗余
- 优先保证可追溯性、可恢复性、可扩展性

## 2. 约定

### 2.1 ID 策略

第一版统一使用 `bigserial` 主键。

原因：

- 本地单机足够简单
- 排序和调试方便
- 不引入 UUID 额外复杂度

### 2.2 枚举策略

第一版优先使用：

- `text + check constraint`

不强依赖 PostgreSQL 原生 enum。

原因：

- 变更成本更低
- 后续新增状态或类型更灵活

### 2.3 时间字段

统一使用：

- `timestamptz`

### 2.4 JSON 字段

以下场景优先使用 `jsonb`：

- EXIF
- subtitle_info
- structured_attributes
- raw_model_output
- evidence
- payload

## 3. 扩展

```sql
create extension if not exists vector;
```

## 4. 核心表

## 4.1 `volumes`

记录硬盘、分区或外接卷。

```sql
create table if not exists volumes (
  id bigserial primary key,
  volume_uuid text,
  display_name text not null,
  mount_path text not null,
  filesystem text,
  capacity_bytes bigint,
  is_online boolean not null default true,
  last_seen_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint chk_volumes_capacity_bytes_nonnegative
    check (capacity_bytes is null or capacity_bytes >= 0)
);

create unique index if not exists uq_volumes_mount_path
  on volumes (mount_path);

create unique index if not exists uq_volumes_volume_uuid
  on volumes (volume_uuid)
  where volume_uuid is not null and volume_uuid <> '';
```

说明：

- `mount_path` 在当前机器上唯一
- `volume_uuid` 若能可靠拿到，优先作为长期识别辅助

## 4.2 `files`

系统中的稳定文件实体。  
`files.id` 是内部稳定身份，路径不是身份本身。

```sql
create table if not exists files (
  id bigserial primary key,
  volume_id bigint not null references volumes(id),
  abs_path text not null,
  parent_path text not null,
  file_name text not null,
  extension text not null,
  media_type text not null,
  size_bytes bigint not null,
  mtime timestamptz,
  ctime timestamptz,
  device_id text,
  inode_hint text,
  sha256 text,
  quick_hash text,
  status text not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint chk_files_media_type
    check (media_type in ('image', 'video')),
  constraint chk_files_status
    check (status in ('active', 'missing', 'ignored', 'trashed')),
  constraint chk_files_size_bytes_nonnegative
    check (size_bytes >= 0)
);

create unique index if not exists uq_files_volume_path
  on files (volume_id, abs_path);

create index if not exists idx_files_media_type
  on files (media_type);

create index if not exists idx_files_status
  on files (status);

create index if not exists idx_files_quick_hash
  on files (quick_hash)
  where quick_hash is not null and quick_hash <> '';

create index if not exists idx_files_sha256
  on files (sha256)
  where sha256 is not null and sha256 <> '';

create index if not exists idx_files_size_mtime
  on files (size_bytes, mtime);
```

说明：

- `uq_files_volume_path` 约束当前路径唯一
- 迁移识别发生时更新路径，但尽量保留同一个 `files.id`

## 4.3 `file_path_history`

记录文件路径发现、移动、重命名和丢失历史。

```sql
create table if not exists file_path_history (
  id bigserial primary key,
  file_id bigint not null references files(id) on delete cascade,
  volume_id bigint not null references volumes(id),
  abs_path text not null,
  event_type text not null,
  seen_at timestamptz not null default now(),
  constraint chk_file_path_history_event_type
    check (event_type in ('discovered', 'moved', 'renamed', 'missing'))
);

create index if not exists idx_file_path_history_file_seen
  on file_path_history (file_id, seen_at desc);

create index if not exists idx_file_path_history_volume_path
  on file_path_history (volume_id, abs_path);
```

## 5. 媒体特征表

## 5.1 `image_assets`

```sql
create table if not exists image_assets (
  file_id bigint primary key references files(id) on delete cascade,
  width integer,
  height integer,
  format text,
  orientation text,
  exif jsonb not null default '{}'::jsonb,
  thumbnail_path text,
  phash text,
  quality_score numeric(5,2),
  quality_tier text,
  quality_flags text[] not null default '{}',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint chk_image_assets_width_nonnegative
    check (width is null or width >= 0),
  constraint chk_image_assets_height_nonnegative
    check (height is null or height >= 0)
);

create index if not exists idx_image_assets_quality_tier
  on image_assets (quality_tier);

create index if not exists idx_image_assets_phash
  on image_assets (phash)
  where phash is not null and phash <> '';
```

## 5.2 `video_assets`

```sql
create table if not exists video_assets (
  file_id bigint primary key references files(id) on delete cascade,
  duration_ms bigint,
  width integer,
  height integer,
  fps numeric(8,3),
  container text,
  video_codec text,
  audio_codec text,
  bitrate bigint,
  poster_path text,
  subtitle_info jsonb not null default '{}'::jsonb,
  quality_score numeric(5,2),
  quality_tier text,
  quality_flags text[] not null default '{}',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint chk_video_assets_duration_nonnegative
    check (duration_ms is null or duration_ms >= 0),
  constraint chk_video_assets_width_nonnegative
    check (width is null or width >= 0),
  constraint chk_video_assets_height_nonnegative
    check (height is null or height >= 0),
  constraint chk_video_assets_bitrate_nonnegative
    check (bitrate is null or bitrate >= 0)
);

create index if not exists idx_video_assets_quality_tier
  on video_assets (quality_tier);

create index if not exists idx_video_assets_duration_ms
  on video_assets (duration_ms);
```

## 5.3 `video_frames`

关键帧元数据。  
第一版每个视频会抽取候选帧、索引帧和理解帧。

```sql
create table if not exists video_frames (
  id bigserial primary key,
  file_id bigint not null references files(id) on delete cascade,
  timestamp_ms bigint not null,
  frame_path text not null,
  phash text,
  frame_role text not null,
  created_at timestamptz not null default now(),
  constraint chk_video_frames_timestamp_nonnegative
    check (timestamp_ms >= 0),
  constraint chk_video_frames_role
    check (frame_role in ('candidate', 'index', 'understanding'))
);

create unique index if not exists uq_video_frames_file_timestamp_role
  on video_frames (file_id, timestamp_ms, frame_role);

create index if not exists idx_video_frames_file_role
  on video_frames (file_id, frame_role);
```

## 5.4 `embeddings`

统一存储向量。  
图片、视频帧、搜索文档可以共用。

> `vector(768)` 只是示例，最终维度应与选定模型一致。

```sql
create table if not exists embeddings (
  id bigserial primary key,
  file_id bigint references files(id) on delete cascade,
  frame_id bigint references video_frames(id) on delete cascade,
  embedding_type text not null,
  model_name text not null,
  vector vector(768) not null,
  created_at timestamptz not null default now(),
  constraint chk_embeddings_type
    check (embedding_type in ('image_visual', 'video_frame_visual', 'face', 'search_text')),
  constraint chk_embeddings_target
    check (
      (file_id is not null and frame_id is null)
      or (file_id is null and frame_id is not null)
    )
);

create index if not exists idx_embeddings_type_model
  on embeddings (embedding_type, model_name);
```

说明：

- `face` 向量后续可单独用于 `same_person`
- 若后面需要多模型并存，这张表天然支持

## 6. 标签体系

## 6.1 `tags`

正式标签定义表。

```sql
create table if not exists tags (
  id bigserial primary key,
  namespace text not null,
  name text not null,
  display_name text not null,
  description text,
  is_system boolean not null default false,
  is_sensitive boolean not null default false,
  created_at timestamptz not null default now()
);

create unique index if not exists uq_tags_namespace_name
  on tags (namespace, name);

create index if not exists idx_tags_namespace
  on tags (namespace);
```

建议第一版 namespace：

- `content`
- `quality`
- `management`
- `sensitive`
- `person`

## 6.2 `file_tags`

文件和标签关系。  
允许同一个文件 + 同一个标签出现多种来源。

```sql
create table if not exists file_tags (
  id bigserial primary key,
  file_id bigint not null references files(id) on delete cascade,
  tag_id bigint not null references tags(id) on delete cascade,
  source text not null,
  confidence numeric(5,4),
  evidence jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  constraint chk_file_tags_source
    check (source in ('ai', 'rule', 'human'))
);

create unique index if not exists uq_file_tags_file_tag_source
  on file_tags (file_id, tag_id, source);

create index if not exists idx_file_tags_file_id
  on file_tags (file_id);

create index if not exists idx_file_tags_tag_id
  on file_tags (tag_id);
```

说明：

- 人工标签不覆盖 AI 标签
- 前端展示时人工优先

## 7. 分析结果

## 7.1 `analysis_results`

多版本分析结果快照。  
不直接维护正式标签关系。

```sql
create table if not exists analysis_results (
  id bigserial primary key,
  file_id bigint not null references files(id) on delete cascade,
  analysis_type text not null,
  status text not null default 'succeeded',
  summary text,
  caption text,
  structured_attributes jsonb not null default '{}'::jsonb,
  quality_score numeric(5,2),
  quality_tier text,
  quality_flags text[] not null default '{}',
  quality_reasons text[] not null default '{}',
  raw_model_output jsonb not null default '{}'::jsonb,
  provider text,
  model_name text,
  prompt_version text,
  analysis_version integer not null default 1,
  created_at timestamptz not null default now(),
  constraint chk_analysis_results_type
    check (analysis_type in ('understanding', 'quality', 'search_doc')),
  constraint chk_analysis_results_status
    check (status in ('pending', 'running', 'succeeded', 'failed'))
);

create index if not exists idx_analysis_results_file_type_created
  on analysis_results (file_id, analysis_type, created_at desc);

create index if not exists idx_analysis_results_type_status
  on analysis_results (analysis_type, status);
```

说明：

- `raw_model_output` 第一版尽量完整保留
- 需在应用层做大小保护

## 7.2 `file_current_analysis`

每个文件、每种分析类型只指向一个当前生效结果。

```sql
create table if not exists file_current_analysis (
  file_id bigint not null references files(id) on delete cascade,
  analysis_type text not null,
  analysis_result_id bigint not null references analysis_results(id) on delete cascade,
  updated_at timestamptz not null default now(),
  primary key (file_id, analysis_type),
  constraint chk_file_current_analysis_type
    check (analysis_type in ('understanding', 'quality', 'search_doc'))
);
```

## 7.3 `search_documents`

为全文搜索提供派生文档。  
第一版建议独立存，而不是完全依赖在线拼接。

```sql
create table if not exists search_documents (
  file_id bigint primary key references files(id) on delete cascade,
  document_text text not null,
  tsv tsvector not null,
  updated_at timestamptz not null default now()
);

create index if not exists idx_search_documents_tsv
  on search_documents using gin (tsv);
```

说明：

- `document_text` 由文件名、路径、标签、描述、部分原始标签拼装
- `search_doc` 分析结果可作为其来源之一

## 8. 聚类

## 8.1 `clusters`

```sql
create table if not exists clusters (
  id bigserial primary key,
  cluster_type text not null,
  title text,
  confidence numeric(5,4),
  status text not null default 'candidate',
  cover_file_id bigint references files(id) on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint chk_clusters_type
    check (cluster_type in ('same_content', 'same_person', 'same_series')),
  constraint chk_clusters_status
    check (status in ('candidate', 'confirmed', 'ignored'))
);

create index if not exists idx_clusters_type_status
  on clusters (cluster_type, status);
```

## 8.2 `cluster_members`

```sql
create table if not exists cluster_members (
  id bigserial primary key,
  cluster_id bigint not null references clusters(id) on delete cascade,
  file_id bigint not null references files(id) on delete cascade,
  score numeric(5,4),
  role text not null default 'member',
  created_at timestamptz not null default now(),
  constraint chk_cluster_members_role
    check (role in ('cover', 'member', 'best_quality', 'duplicate_candidate'))
);

create unique index if not exists uq_cluster_members_cluster_file
  on cluster_members (cluster_id, file_id);

create index if not exists idx_cluster_members_file_id
  on cluster_members (file_id);
```

说明：

- `same_content / same_person / same_series` 都走这套关系
- `same_content` 中视频片段不直接并入完整原片时，可不建 member，而是走关联候选表

## 8.3 `file_relationships`

用于表达“不宜直接并组但值得提示”的关联关系。

```sql
create table if not exists file_relationships (
  id bigserial primary key,
  from_file_id bigint not null references files(id) on delete cascade,
  to_file_id bigint not null references files(id) on delete cascade,
  relation_type text not null,
  confidence numeric(5,4),
  evidence jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  constraint chk_file_relationships_type
    check (relation_type in ('same_source_video_candidate', 'possible_migration', 'weak_same_series'))
);

create unique index if not exists uq_file_relationships_unique
  on file_relationships (from_file_id, to_file_id, relation_type);
```

## 9. 人工审核

## 9.1 `review_actions`

```sql
create table if not exists review_actions (
  id bigserial primary key,
  file_id bigint references files(id) on delete cascade,
  cluster_id bigint references clusters(id) on delete cascade,
  action_type text not null,
  note text,
  created_at timestamptz not null default now(),
  constraint chk_review_actions_type
    check (action_type in ('keep', 'trash_candidate', 'ignore', 'favorite', 'hide', 'deleted_to_trash')),
  constraint chk_review_actions_target
    check (file_id is not null or cluster_id is not null)
);

create index if not exists idx_review_actions_file_id
  on review_actions (file_id);

create index if not exists idx_review_actions_cluster_id
  on review_actions (cluster_id);
```

说明：

- `deleted_to_trash` 表示已执行移动到 macOS 废纸篓

## 10. 任务系统

## 10.1 `jobs`

```sql
create table if not exists jobs (
  id bigserial primary key,
  job_type text not null,
  priority integer not null default 100,
  status text not null default 'pending',
  target_type text,
  target_id bigint,
  payload jsonb not null default '{}'::jsonb,
  progress_percent numeric(5,2),
  progress_stage text,
  attempt_count integer not null default 0,
  max_attempts integer not null default 3,
  lease_owner text,
  lease_expires_at timestamptz,
  scheduled_at timestamptz not null default now(),
  started_at timestamptz,
  finished_at timestamptz,
  last_error text,
  worker_name text,
  provider text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint chk_jobs_status
    check (status in ('pending', 'leased', 'running', 'succeeded', 'failed', 'dead'))
);

create index if not exists idx_jobs_status_priority_scheduled
  on jobs (status, priority, scheduled_at);

create index if not exists idx_jobs_lease_expires_at
  on jobs (lease_expires_at)
  where lease_expires_at is not null;

create index if not exists idx_jobs_target
  on jobs (target_type, target_id);
```

建议 `job_type`：

- `scan_volume`
- `index_file`
- `extract_image_features`
- `extract_video_features`
- `embed_image`
- `embed_video_frames`
- `infer_tags`
- `infer_summary`
- `cluster_same_content`
- `cluster_same_person`
- `cluster_same_series`
- `recompute_search_doc`

## 10.2 `job_events`

用于记录任务状态变化和关键日志，便于任务中心展示。

```sql
create table if not exists job_events (
  id bigserial primary key,
  job_id bigint not null references jobs(id) on delete cascade,
  level text not null,
  message text not null,
  created_at timestamptz not null default now(),
  constraint chk_job_events_level
    check (level in ('info', 'warning', 'error'))
);

create index if not exists idx_job_events_job_id_created
  on job_events (job_id, created_at desc);
```

## 11. 推荐索引补充

除前面各表索引外，第一版还建议重点关注：

- `files(volume_id, abs_path)`
- `files(quick_hash)`
- `files(sha256)`
- `file_tags(file_id)`
- `file_tags(tag_id)`
- `analysis_results(file_id, analysis_type, created_at desc)`
- `clusters(cluster_type, status)`
- `jobs(status, priority, scheduled_at)`
- `search_documents using gin(tsv)`

向量索引可在实际模型维度明确后补充，例如：

```sql
-- 示例，实际参数需结合数据量和 pgvector 版本调整
create index if not exists idx_embeddings_vector_ivfflat
  on embeddings
  using ivfflat (vector vector_cosine_ops)
  with (lists = 100);
```

> 说明：向量索引应在数据量、维度和查询模式稳定后再正式调优。

## 12. 查询视图建议

第一版可增加几个只读视图，简化前端和后端查询。

## 12.1 `v_file_overview`

聚合文件、基础媒体信息、当前分析结果。

可包含：

- 文件基本信息
- 图片或视频质量分
- 当前 understanding 摘要
- 当前 quality 评分

## 12.2 `v_candidate_summary`

聚合候选组数量和状态。

用于首页数据面板：

- same_content 数量
- same_person 数量
- same_series 数量
- trash_candidate 数量

## 13. 实现建议

### 13.1 先建核心表

优先顺序建议：

1. `volumes`
2. `files`
3. `file_path_history`
4. `tags`
5. `file_tags`
6. `analysis_results`
7. `file_current_analysis`
8. `image_assets`
9. `video_assets`
10. `video_frames`
11. `embeddings`
12. `clusters`
13. `cluster_members`
14. `review_actions`
15. `jobs`
16. `job_events`
17. `search_documents`

### 13.2 应用层负责的约束

以下约束第一版建议在应用层保证，而不是完全依赖数据库：

- `raw_model_output` 大小保护
- 迁移识别逻辑
- 人工标签优先展示
- 当前分析结果指针更新
- 聚类整体重算与替换策略

## 14. 当前结论

这份 schema 已足够支撑第一版：

- 本地离线扫描
- 标签管理
- 多版本分析
- 图片视频统一索引
- 聚类候选
- 人工审核
- 任务恢复

后续如果进入实现阶段，可基于此再补：

- 真正可执行的 `.sql` migration 文件
- 视图定义
- 触发器或 `updated_at` 自动维护策略
- 向量维度最终确认
