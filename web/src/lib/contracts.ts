export type SystemCheck = {
  name: string;
  status: string;
  message?: string;
};

export type SystemStatus = {
  status: string;
  checks: SystemCheck[];
};

export type TaskSummary = {
  pending: number;
  running: number;
  failed: number;
  dead: number;
  succeeded: number;
};

export type AIPromptSettings = {
  understanding_extra_prompt: string;
};

export type ClusterSummary = {
  cluster_type: string;
  status: string;
  cluster_count: number;
  member_count: number;
};

export type VolumeItem = {
  id: number;
  display_name: string;
  mount_path: string;
  is_online: boolean;
};

export type JobItem = {
  id: number;
  job_type: string;
  status: string;
  target_type?: string;
  target_id?: number;
  last_error?: string;
  started_at?: string;
  finished_at?: string;
};

export type JobEvent = {
  id: number;
  level: string;
  message: string;
  created_at: string;
};

export type ClusterItem = {
  id: number;
  cluster_type: string;
  title: string;
  status: string;
  member_count: number;
  strong_member_count?: number;
  top_member_score?: number;
  person_visual_count?: number;
  generic_visual_count?: number;
  top_evidence_type?: string;
};

export type ClusterMember = {
  file_id: number;
  file_name: string;
  abs_path: string;
  media_type: string;
  role: string;
  score?: number;
  quality_tier?: string;
  has_face?: boolean;
  subject_count?: string;
  capture_type?: string;
  embedding_type?: string;
  embedding_provider?: string;
  embedding_model?: string;
  embedding_vector_count?: number;
};

export type ClusterDetail = ClusterItem & {
  members: ClusterMember[];
};

export type FileTag = {
  namespace: string;
  name: string;
  display_name: string;
  source: string;
  confidence?: number;
};

export type AnalysisItem = {
  analysis_type: string;
  status: string;
  summary: string;
  quality_score?: number;
  quality_tier?: string;
  created_at: string;
};

export type PathHistory = {
  abs_path: string;
  event_type: string;
  seen_at: string;
};

export type ReviewAction = {
  action_type: string;
  note: string;
  created_at: string;
};

export type EmbeddingInfo = {
  embedding_type: string;
  provider: string;
  model_name: string;
  vector_count: number;
};

export type VideoFrame = {
  timestamp_ms: number;
  frame_role: string;
  phash: string;
};

export type FileCluster = {
  id: number;
  cluster_type: string;
  title: string;
  status: string;
};

export type FileDetail = import("../features/library/types").FileItem & {
  tags: FileTag[];
  current_analyses: AnalysisItem[];
  path_history: PathHistory[];
  review_actions: ReviewAction[];
  embeddings: EmbeddingInfo[];
  video_frames: VideoFrame[];
  clusters: FileCluster[];
};
