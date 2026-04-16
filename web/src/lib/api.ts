import type { FileItem } from "../features/library/types";
import type {
  ClusterDetail,
  ClusterItem,
  ClusterSummary,
  FileDetail,
  JobEvent,
  JobItem,
  AIPromptSettings,
  SystemStatus,
  TaskSummary,
  VolumeItem
} from "./contracts";

export async function fetchJSON<T>(input: string): Promise<T> {
  const response = await fetch(input);
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  return (await response.json()) as T;
}

export type FetchFilesOptions = {
  query?: string;
  mediaType?: string;
  qualityTier?: string;
  status?: string;
  reviewAction?: string;
  hasTags?: string;
  tag?: string;
  sort?: string;
  cursor?: string;
};

export type FileListPage = {
  items: FileItem[];
  next_cursor?: string;
  has_more: boolean;
};

export async function fetchFiles(options: FetchFilesOptions = {}): Promise<FileListPage> {
  const search = new URLSearchParams({
    limit: "24",
    sort: options.sort?.trim() || "updated_desc"
  });
  if (options.query?.trim()) {
    search.set("q", options.query.trim());
  }
  if (options.mediaType?.trim()) {
    search.set("media_type", options.mediaType.trim());
  }
  if (options.qualityTier?.trim()) {
    search.set("quality_tier", options.qualityTier.trim());
  }
  if (options.status?.trim()) {
    search.set("status", options.status.trim());
  }
  if (options.reviewAction?.trim()) {
    search.set("review_action", options.reviewAction.trim());
  }
  if (options.hasTags?.trim()) {
    search.set("has_tags", options.hasTags.trim());
  }
  if (options.tag?.trim()) {
    search.set("tag", options.tag.trim());
  }
  if (options.cursor?.trim()) {
    search.set("cursor", options.cursor.trim());
  }
  return fetchJSON<FileListPage>(`/api/files?${search.toString()}`);
}

export async function fetchFileDetail(fileId: number): Promise<FileDetail> {
  return fetchJSON<FileDetail>(`/api/files/${fileId}`);
}

export async function openFileWithDefaultApp(fileId: number): Promise<void> {
  const response = await fetch(`/api/files/${fileId}/open`, { method: "POST" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function moveFileToTrash(fileId: number): Promise<void> {
  const response = await fetch(`/api/files/${fileId}/trash`, { method: "POST" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function recomputeFileEmbeddings(fileId: number): Promise<void> {
  const response = await fetch(`/api/files/${fileId}/recompute-embeddings`, { method: "POST" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function generateFilePreview(fileId: number): Promise<void> {
  const response = await fetch(`/api/files/${fileId}/generate-preview`, { method: "POST" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function createFileTag(input: {
  fileId: number;
  namespace: string;
  name: string;
  display_name?: string;
}): Promise<void> {
  const response = await fetch(`/api/files/${input.fileId}/tags`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      namespace: input.namespace,
      name: input.name,
      display_name: input.display_name?.trim() || undefined
    })
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function deleteFileTag(input: { fileId: number; namespace: string; name: string }): Promise<void> {
  const search = new URLSearchParams({
    namespace: input.namespace,
    name: input.name
  });
  const response = await fetch(`/api/files/${input.fileId}/tags?${search.toString()}`, {
    method: "DELETE"
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function replaceFileTag(input: {
  fileId: number;
  current_namespace: string;
  current_name: string;
  namespace: string;
  name: string;
  display_name?: string;
}): Promise<void> {
  const response = await fetch(`/api/files/${input.fileId}/tags`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      current_namespace: input.current_namespace,
      current_name: input.current_name,
      namespace: input.namespace,
      name: input.name,
      display_name: input.display_name?.trim() || undefined
    })
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function fetchSystemStatus(): Promise<SystemStatus> {
  return fetchJSON<SystemStatus>("/api/system-status");
}

export async function fetchAIPromptSettings(): Promise<AIPromptSettings> {
  return fetchJSON<AIPromptSettings>("/api/system/ai-prompt-settings");
}

export async function updateAIPromptSettings(input: AIPromptSettings): Promise<AIPromptSettings> {
  const response = await fetch("/api/system/ai-prompt-settings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input)
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  return (await response.json()) as AIPromptSettings;
}

export async function pickDirectory(): Promise<string> {
  const response = await fetch("/api/system/pick-directory", { method: "POST" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  const payload = (await response.json()) as { path: string };
  return payload.path;
}

export async function fetchTaskSummary(): Promise<TaskSummary> {
  return fetchJSON<TaskSummary>("/api/task-summary");
}

export async function fetchClusterSummary(): Promise<ClusterSummary[]> {
  return fetchJSON<ClusterSummary[]>("/api/cluster-summary");
}

export async function fetchVolumes(): Promise<VolumeItem[]> {
  return fetchJSON<VolumeItem[]>("/api/volumes");
}

export async function createVolume(input: { display_name: string; mount_path: string }): Promise<VolumeItem> {
  const response = await fetch("/api/volumes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input)
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  return (await response.json()) as VolumeItem;
}

export async function scanVolume(volumeId: number): Promise<void> {
  const response = await fetch(`/api/volumes/${volumeId}/scan`, { method: "POST" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function deleteVolume(volumeId: number): Promise<void> {
  const response = await fetch(`/api/volumes/${volumeId}`, { method: "DELETE" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function fetchJobs(): Promise<JobItem[]> {
  return fetchJSON<JobItem[]>("/api/jobs?limit=12");
}

export async function fetchJobEvents(jobId: number): Promise<JobEvent[]> {
  return fetchJSON<JobEvent[]>(`/api/jobs/${jobId}/events`);
}

export async function fetchClusters(clusterType = "", clusterStatus = ""): Promise<ClusterItem[]> {
  const search = new URLSearchParams({ limit: "12" });
  if (clusterType.trim() !== "") {
    search.set("cluster_type", clusterType);
  }
  if (clusterStatus.trim() !== "") {
    search.set("status", clusterStatus);
  }
  return fetchJSON<ClusterItem[]>(`/api/clusters?${search.toString()}`);
}

export async function fetchClusterDetail(clusterId: number): Promise<ClusterDetail> {
  return fetchJSON<ClusterDetail>(`/api/clusters/${clusterId}`);
}

export async function applyClusterReviewAction(clusterId: number, input: { action_type: string; note?: string }): Promise<void> {
  const response = await fetch(`/api/clusters/${clusterId}/review-actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input)
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function applyFileReviewAction(fileId: number, input: { action_type: string; note?: string }): Promise<void> {
  const response = await fetch(`/api/files/${fileId}/review-actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input)
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function updateClusterStatus(clusterId: number, status: string): Promise<void> {
  const response = await fetch(`/api/clusters/${clusterId}/status`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ status })
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

async function readError(response: Response) {
  const text = (await response.text()).trim();
  if (text) {
    return text;
  }
  return `request failed: ${response.status}`;
}
