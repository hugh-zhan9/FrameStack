begin;

create extension if not exists vector;

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

create table if not exists embeddings (
  id bigserial primary key,
  file_id bigint references files(id) on delete cascade,
  frame_id bigint references video_frames(id) on delete cascade,
  embedding_type text not null,
  model_name text not null,
  vector vector(64) not null,
  created_at timestamptz not null default now(),
  constraint chk_embeddings_type
    check (embedding_type in ('image_visual', 'video_frame_visual', 'person_visual', 'face', 'search_text')),
  constraint chk_embeddings_target
    check (
      (file_id is not null and frame_id is null)
      or (file_id is null and frame_id is not null)
    )
);

create index if not exists idx_embeddings_type_model
  on embeddings (embedding_type, model_name);

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

create table if not exists file_current_analysis (
  file_id bigint not null references files(id) on delete cascade,
  analysis_type text not null,
  analysis_result_id bigint not null references analysis_results(id) on delete cascade,
  updated_at timestamptz not null default now(),
  primary key (file_id, analysis_type),
  constraint chk_file_current_analysis_type
    check (analysis_type in ('understanding', 'quality', 'search_doc'))
);

create table if not exists search_documents (
  file_id bigint primary key references files(id) on delete cascade,
  document_text text not null,
  tsv tsvector not null,
  updated_at timestamptz not null default now()
);

create index if not exists idx_search_documents_tsv
  on search_documents using gin (tsv);

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

create table if not exists cluster_members (
  id bigserial primary key,
  cluster_id bigint not null references clusters(id) on delete cascade,
  file_id bigint not null references files(id) on delete cascade,
  score numeric(5,4),
  role text not null default 'member',
  created_at timestamptz not null default now(),
  constraint chk_cluster_members_role
    check (role in ('cover', 'member', 'best_quality', 'duplicate_candidate', 'series_focus'))
);

create unique index if not exists uq_cluster_members_cluster_file
  on cluster_members (cluster_id, file_id);

create index if not exists idx_cluster_members_file_id
  on cluster_members (file_id);

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

commit;
