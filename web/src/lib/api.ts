import type { FileItem } from "../features/library/types";
import type {
  ClusterDetail,
  ClusterItem,
  ClusterSummary,
  FileDetail,
  JobEvent,
  JobItem,
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
  sort?: string;
};

export async function fetchFiles(options: FetchFilesOptions = {}): Promise<FileItem[]> {
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
  return fetchJSON<FileItem[]>(`/api/files?${search.toString()}`);
}

export async function fetchFileDetail(fileId: number): Promise<FileDetail> {
  return fetchJSON<FileDetail>(`/api/files/${fileId}`);
}

export async function fetchSystemStatus(): Promise<SystemStatus> {
  return fetchJSON<SystemStatus>("/api/system-status");
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

export async function fetchJobs(): Promise<JobItem[]> {
  return fetchJSON<JobItem[]>("/api/jobs?limit=12");
}

export async function fetchJobEvents(jobId: number): Promise<JobEvent[]> {
  return fetchJSON<JobEvent[]>(`/api/jobs/${jobId}/events`);
}

export async function fetchClusters(clusterType = ""): Promise<ClusterItem[]> {
  const search = new URLSearchParams({ limit: "12" });
  if (clusterType.trim() !== "") {
    search.set("cluster_type", clusterType);
  }
  return fetchJSON<ClusterItem[]>(`/api/clusters?${search.toString()}`);
}

export async function fetchClusterDetail(clusterId: number): Promise<ClusterDetail> {
  return fetchJSON<ClusterDetail>(`/api/clusters/${clusterId}`);
}

async function readError(response: Response) {
  const text = (await response.text()).trim();
  if (text) {
    return text;
  }
  return `request failed: ${response.status}`;
}
