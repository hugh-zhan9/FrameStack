import { useEffect, useState, type ReactNode } from "react";
import { fetchFileDetail, fetchFiles } from "../../lib/api";
import { useAsync } from "../../lib/useAsync";
import { LibraryPage } from "./LibraryPage";
import type { FileItem } from "./types";

type LibraryFilters = {
  query: string;
  mediaType: string;
  qualityTier: string;
  status: string;
  sort: string;
};

const defaultFilters: LibraryFilters = {
  query: "",
  mediaType: "",
  qualityTier: "",
  status: "",
  sort: "updated_desc"
};

export function LibraryRoute() {
  const [files, setFiles] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedFileId, setSelectedFileId] = useState<number | null>(null);
  const [filters, setFilters] = useState<LibraryFilters>(defaultFilters);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    fetchFiles(filters)
      .then((items) => {
        if (cancelled) {
          return;
        }
        setFiles(items);
        setSelectedFileId((current) => {
          if (current && items.some((item) => item.id === current)) {
            return current;
          }
          return items[0]?.id ?? null;
        });
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return;
        }
        setFiles([]);
        setError(err instanceof Error ? err.message : "加载资源失败");
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [filters]);

  const detailState = useAsync(
    () => {
      if (selectedFileId == null) {
        return Promise.resolve(null);
      }
      return fetchFileDetail(selectedFileId);
    },
    [selectedFileId]
  );
  const detail = detailState.data;

  return (
    <div className="library-layout">
      <LibraryPage
        files={files}
        loading={loading}
        selectedFileId={selectedFileId}
        onSelectFile={setSelectedFileId}
        filters={filters}
        onFiltersChange={(patch) => setFilters((current) => ({ ...current, ...patch }))}
        error={error}
      />
      <aside className="detail-panel">
        <h3>文件详情</h3>
        {detailState.loading ? <p>正在加载详情…</p> : null}
        {detailState.error ? <p>{detailState.error}</p> : null}
        {detail ? (
          <div className="detail-stack">
            <img
              src={`/api/files/${detail.id}/preview`}
              alt={`${detail.file_name}-detail`}
              className="detail-preview"
            />
            <strong>{detail.file_name}</strong>
            <span className="detail-path">{detail.abs_path}</span>
            <div className="detail-meta">
              <span>{detail.media_type === "video" ? "视频" : "图片"}</span>
              <span>{detail.quality_tier ?? "unknown"}</span>
              <span>{detail.status}</span>
            </div>
            <div className="detail-grid">
              <DetailStat label="分辨率" value={formatResolution(detail.width, detail.height)} />
              <DetailStat label="时长" value={formatDuration(detail.duration_ms)} />
              <DetailStat label="格式" value={detail.format || detail.container || "-"} />
              <DetailStat label="体积" value={formatBytes(detail.size_bytes)} />
              <DetailStat label="FPS" value={detail.fps ? `${detail.fps}` : "-"} />
              <DetailStat
                label="码率"
                value={detail.bitrate ? `${Math.round(detail.bitrate / 1000)} kbps` : "-"}
              />
            </div>
            <DetailSection title="标签">
              <div className="media-tags">
                {(detail.tags ?? []).map((tag) => (
                  <span key={`${tag.namespace}:${tag.name}`} className="media-tag">
                    {tag.display_name || tag.name}
                  </span>
                ))}
              </div>
            </DetailSection>
            <DetailSection title="当前分析">
              <ul className="detail-list">
                {(detail.current_analyses ?? []).map((analysis) => (
                  <li key={`${analysis.analysis_type}:${analysis.created_at}`}>
                    <strong>{analysis.analysis_type}</strong>
                    <span>{analysis.summary || "无摘要"}</span>
                  </li>
                ))}
              </ul>
            </DetailSection>
            <DetailSection title="聚类">
              <ul className="detail-list">
                {(detail.clusters ?? []).map((cluster) => (
                  <li key={`${cluster.cluster_type}:${cluster.id}`}>
                    <strong>{cluster.title || cluster.cluster_type}</strong>
                    <span>
                      {cluster.cluster_type} / {cluster.status}
                    </span>
                  </li>
                ))}
              </ul>
            </DetailSection>
            <DetailSection title="Embedding">
              <ul className="detail-list">
                {(detail.embeddings ?? []).map((embedding) => (
                  <li key={`${embedding.embedding_type}:${embedding.model_name}`}>
                    <strong>{embedding.embedding_type}</strong>
                    <span>
                      {embedding.provider || "unknown"} / {embedding.model_name || "unknown"} / {embedding.vector_count}
                    </span>
                  </li>
                ))}
              </ul>
            </DetailSection>
            {detail.video_frames?.length ? (
              <DetailSection title="关键帧">
                <div className="frame-grid">
                  {detail.video_frames.map((frame, index) => (
                    <figure key={`${frame.timestamp_ms}:${index}`} className="frame-card">
                      <img
                        src={`/api/files/${detail.id}/frames/${index}/preview`}
                        alt={`frame-${index}`}
                        className="frame-preview"
                      />
                      <figcaption>
                        <strong>{frame.frame_role}</strong>
                        <span>{formatDuration(frame.timestamp_ms)}</span>
                      </figcaption>
                    </figure>
                  ))}
                </div>
              </DetailSection>
            ) : null}
            <DetailSection title="最近操作">
              <ul className="detail-list">
                {(detail.review_actions ?? []).map((action, index) => (
                  <li key={`${action.action_type}:${action.created_at}:${index}`}>
                    <strong>{action.action_type}</strong>
                    <span>{action.note || "无备注"}</span>
                  </li>
                ))}
              </ul>
            </DetailSection>
            <DetailSection title="路径历史">
              <ul className="detail-list">
                {(detail.path_history ?? []).map((entry) => (
                  <li key={`${entry.abs_path}:${entry.seen_at}`}>
                    <strong>{entry.event_type}</strong>
                    <span>{entry.abs_path}</span>
                  </li>
                ))}
              </ul>
            </DetailSection>
          </div>
        ) : null}
      </aside>
    </div>
  );
}

function DetailSection(props: { title: string; children: ReactNode }) {
  return (
    <div className="detail-section">
      <h4>{props.title}</h4>
      {props.children}
    </div>
  );
}

function DetailStat(props: { label: string; value: string }) {
  return (
    <div className="detail-stat">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}

function formatResolution(width?: number, height?: number) {
  if (!width || !height) {
    return "-";
  }
  return `${width} × ${height}`;
}

function formatDuration(durationMS?: number) {
  if (!durationMS || durationMS <= 0) {
    return "-";
  }
  const totalSeconds = Math.floor(durationMS / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  const hours = Math.floor(minutes / 60);
  if (hours > 0) {
    return `${hours}h ${minutes % 60}m ${seconds}s`;
  }
  return `${minutes}m ${seconds}s`;
}

function formatBytes(size: number) {
  if (!size) {
    return "-";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = size;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}
