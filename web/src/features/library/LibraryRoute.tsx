import { useEffect, useState, type ReactNode } from "react";
import {
  fetchFileDetail,
  fetchFiles,
  moveFileToTrash,
  openFileWithDefaultApp,
  recomputeFileEmbeddings
} from "../../lib/api";
import { useAsync } from "../../lib/useAsync";
import { LibraryPage } from "./LibraryPage";
import type { FileItem } from "./types";

type LibraryFilters = {
  query: string;
  mediaType: string;
  qualityTier: string;
  status: string;
  reviewAction: string;
  sort: string;
};

const defaultFilters: LibraryFilters = {
  query: "",
  mediaType: "",
  qualityTier: "",
  status: "",
  reviewAction: "",
  sort: "updated_desc"
};

export function LibraryRoute() {
  const [files, setFiles] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [selectedFileId, setSelectedFileId] = useState<number | null>(null);
  const [filters, setFilters] = useState<LibraryFilters>(defaultFilters);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [openPending, setOpenPending] = useState(false);
  const [trashPending, setTrashPending] = useState(false);
  const [analysisPending, setAnalysisPending] = useState(false);
  const [detailRefreshToken, setDetailRefreshToken] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setNotice(null);

    fetchFiles(filters)
      .then((page) => {
        if (cancelled) {
          return;
        }
        const items = Array.isArray(page.items) ? page.items : [];
        setFiles(items);
        setNextCursor(page.next_cursor ?? null);
        setHasMore(Boolean(page.has_more));
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
        setNextCursor(null);
        setHasMore(false);
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

  async function handleLoadMore() {
    if (!nextCursor || loadingMore) {
      return;
    }
    setLoadingMore(true);
    setError(null);
    try {
      const page = await fetchFiles({ ...filters, cursor: nextCursor });
      const items = Array.isArray(page.items) ? page.items : [];
      setFiles((current) => [...current, ...items]);
      setNextCursor(page.next_cursor ?? null);
      setHasMore(Boolean(page.has_more));
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载更多失败");
    } finally {
      setLoadingMore(false);
    }
  }

  const detailState = useAsync(
    () => {
      if (selectedFileId == null) {
        return Promise.resolve(null);
      }
      return fetchFileDetail(selectedFileId);
    },
    [selectedFileId, detailRefreshToken]
  );
  const detail = detailState.data;

  async function handleOpenFile() {
    if (!detail || openPending) {
      return;
    }
    setOpenPending(true);
    setError(null);
    setNotice(null);
    try {
      await openFileWithDefaultApp(detail.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "默认程序打开失败");
    } finally {
      setOpenPending(false);
    }
  }

  async function handleMoveToTrash() {
    if (!detail || trashPending) {
      return;
    }
    setTrashPending(true);
    setError(null);
    setNotice(null);
    try {
      await moveFileToTrash(detail.id);
      setFiles((current) => current.filter((item) => item.id !== detail.id));
      setSelectedFileId((current) => {
        if (current !== detail.id) {
          return current;
        }
        const remaining = files.filter((item) => item.id !== detail.id);
        return remaining[0]?.id ?? null;
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "移动到废纸篓失败");
    } finally {
      setTrashPending(false);
    }
  }

  async function handleRecomputeAnalysis() {
    if (!detail || analysisPending) {
      return;
    }
    setAnalysisPending(true);
    setError(null);
    setNotice(null);
    try {
      await recomputeFileEmbeddings(detail.id);
      setNotice("已提交 AI 分析任务，可到任务页查看进度。");
      setDetailRefreshToken((current) => current + 1);
    } catch (err) {
      setError(err instanceof Error ? err.message : "提交 AI 分析任务失败");
    } finally {
      setAnalysisPending(false);
    }
  }

  return (
    <div className="library-layout">
      <LibraryPage
        files={files}
        loading={loading}
        loadingMore={loadingMore}
        hasMore={hasMore}
        totalLoaded={files.length}
        selectedFileId={selectedFileId}
        onSelectFile={setSelectedFileId}
        filters={filters}
        onFiltersChange={(patch) => setFilters((current) => ({ ...current, ...patch }))}
        onLoadMore={handleLoadMore}
        error={error}
      />
      <aside className="detail-panel">
        <h3>文件详情</h3>
        {notice ? <p>{notice}</p> : null}
        {detailState.loading ? <p>正在加载详情…</p> : null}
        {detailState.error ? <p>{detailState.error}</p> : null}
        {detail ? (
          <div className="detail-stack">
            <img
              src={`/api/files/${detail.id}/preview`}
              alt={`${detail.file_name}-detail`}
              className="detail-preview"
            />
            <div className="detail-actions">
              <button
                type="button"
                className="primary-button"
                onClick={handleRecomputeAnalysis}
                disabled={analysisPending}
              >
                {analysisPending ? "提交中…" : "重新 AI 分析"}
              </button>
              <button type="button" className="primary-button" onClick={handleOpenFile} disabled={openPending}>
                {openPending ? "打开中…" : "默认程序打开"}
              </button>
              {detail.review_action === "trash_candidate" ? (
                <button type="button" className="secondary-button" onClick={handleMoveToTrash} disabled={trashPending}>
                  {trashPending ? "移动中…" : "移动到废纸篓"}
                </button>
              ) : null}
            </div>
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
