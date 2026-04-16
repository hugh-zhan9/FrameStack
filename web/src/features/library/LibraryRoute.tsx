import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  fetchFileDetail,
  fetchFiles,
  moveFileToTrash,
  openFileWithDefaultApp,
  recomputeFileEmbeddings
} from "../../lib/api";
import {
  formatAnalysisSummary,
  formatAnalysisType,
  formatClusterLabel,
  formatClusterMeta,
  formatFileStatus,
  formatMediaType,
  formatPathEvent,
  formatPathHistorySummary,
  formatQualityScore,
  formatQualityTier,
  formatReviewAction,
  formatReviewActionSummary,
  formatUnknown,
  QUALITY_TIER_NOTE
} from "./presentation";
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
  const loadMoreRef = useRef<HTMLDivElement | null>(null);

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

  useEffect(() => {
    if (!hasMore || loading || loadingMore || !nextCursor || !loadMoreRef.current || !("IntersectionObserver" in window)) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          void handleLoadMore();
        }
      },
      { rootMargin: "600px 0px 600px 0px" }
    );
    observer.observe(loadMoreRef.current);
    return () => observer.disconnect();
  }, [hasMore, loading, loadingMore, nextCursor, filters]);

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
        selectedFile={files.find((file) => file.id === selectedFileId) ?? null}
        loading={loading}
        loadingMore={loadingMore}
        hasMore={hasMore}
        totalLoaded={files.length}
        selectedFileId={selectedFileId}
        onSelectFile={setSelectedFileId}
        filters={filters}
        onFiltersChange={(patch) => setFilters((current) => ({ ...current, ...patch }))}
        loadMoreRef={loadMoreRef}
        error={error}
      />
      <aside className="detail-panel library-inspector">
        <div className="library-panel-heading">
          <div>
            <span className="library-section-label">Inspector</span>
            <h3>文件详情</h3>
          </div>
          <p>把关键信息拆成并列信息块，并把技术规格、状态、动作统一翻译成可读的产品语言。</p>
        </div>
        {notice ? <p className="detail-notice">{notice}</p> : null}
        {detailState.loading ? <p>正在加载详情…</p> : null}
        {detailState.error ? <p>{detailState.error}</p> : null}
        {detail ? (
          <div className="detail-stack">
            <div className="detail-overview-grid">
              <section className="detail-overview-card detail-overview-card-primary">
                <div className="detail-preview-shell" data-orientation={getOrientation(detail)}>
                  <img
                    src={`/api/files/${detail.id}/preview`}
                    alt={`${detail.file_name}-detail`}
                    className="detail-preview"
                  />
                </div>
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
                <div className="detail-title-row">
                  <strong>{detail.file_name}</strong>
                  <span className="media-card-id">#{detail.id}</span>
                </div>
                <span className="detail-path">{detail.abs_path}</span>
                <div className="detail-meta">
                  <span>{formatMediaType(detail.media_type)}</span>
                  <span>{formatQualityTier(detail.quality_tier)}</span>
                  <span>{formatFileStatus(detail.status)}</span>
                  {detail.review_action ? <span>{formatReviewAction(detail.review_action)}</span> : null}
                </div>
                <p className="detail-note">{QUALITY_TIER_NOTE}</p>
              </section>

              <DetailSection title="基础指标" className="detail-overview-card">
                <div className="detail-grid detail-grid-compact">
                  <DetailStat label="规格等级" value={formatQualityTier(detail.quality_tier)} />
                  <DetailStat label="技术评分" value={formatQualityScore(detail.quality_score)} />
                  <DetailStat label="文件状态" value={formatFileStatus(detail.status)} />
                  <DetailStat label="分辨率" value={formatResolution(detail.width, detail.height)} />
                  <DetailStat label="时长" value={formatDuration(detail.duration_ms)} />
                  <DetailStat label="格式" value={formatUnknown(detail.format || detail.container)} />
                  <DetailStat label="体积" value={formatBytes(detail.size_bytes)} />
                  <DetailStat label="FPS" value={detail.fps ? `${detail.fps}` : "-"} />
                  <DetailStat
                    label="码率"
                    value={detail.bitrate ? `${Math.round(detail.bitrate / 1000)} kbps` : "-"}
                  />
                </div>
              </DetailSection>

              <DetailSection title="标签与聚类" className="detail-overview-card">
                <div className="detail-inline-stack">
                  <div className="media-tags">
                    {(detail.tags ?? []).slice(0, 8).map((tag) => (
                      <span key={`${tag.namespace}:${tag.name}`} className="media-tag">
                        {tag.display_name || tag.name}
                      </span>
                    ))}
                  </div>
                  <ul className="detail-list detail-list-compact">
                    {(detail.clusters ?? []).slice(0, 4).map((cluster) => (
                      <li key={`${cluster.cluster_type}:${cluster.id}`}>
                        <strong>{formatClusterLabel(cluster)}</strong>
                        <span>{formatClusterMeta(cluster)}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </DetailSection>

              <DetailSection title="分析与向量" className="detail-overview-card">
                <div className="detail-two-column-list">
                  <ul className="detail-list detail-list-compact">
                    {(detail.current_analyses ?? []).slice(0, 4).map((analysis) => (
                      <li key={`${analysis.analysis_type}:${analysis.created_at}`}>
                        <strong>{formatAnalysisType(analysis.analysis_type)}</strong>
                        <span>{formatAnalysisSummary(analysis)}</span>
                      </li>
                    ))}
                  </ul>
                  <ul className="detail-list detail-list-compact">
                    {(detail.embeddings ?? []).slice(0, 4).map((embedding) => (
                      <li key={`${embedding.embedding_type}:${embedding.model_name}`}>
                        <strong>{embedding.embedding_type || "未识别向量"}</strong>
                        <span>
                          {formatUnknown(embedding.provider)} / {formatUnknown(embedding.model_name)} /{" "}
                          {embedding.vector_count}
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              </DetailSection>

              {detail.video_frames?.length ? (
                <DetailSection title="关键帧" className="detail-overview-card detail-overview-card-wide">
                  <div className="frame-grid frame-grid-compact">
                    {detail.video_frames.slice(0, 4).map((frame, index) => (
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

              <DetailSection title="最近操作与路径历史" className="detail-overview-card detail-overview-card-wide">
                <div className="detail-two-column-list">
                  <ul className="detail-list detail-list-compact">
                    {(detail.review_actions ?? []).slice(0, 4).map((action, index) => (
                      <li key={`${action.action_type}:${action.created_at}:${index}`}>
                        <strong>{formatReviewAction(action.action_type)}</strong>
                        <span>{formatReviewActionSummary(action)}</span>
                      </li>
                    ))}
                  </ul>
                  <ul className="detail-list detail-list-compact">
                    {(detail.path_history ?? []).slice(0, 4).map((entry) => (
                      <li key={`${entry.abs_path}:${entry.seen_at}`}>
                        <strong>{formatPathEvent(entry.event_type)}</strong>
                        <span>{formatPathHistorySummary(entry)}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </DetailSection>
            </div>
          </div>
        ) : (
          <div className="detail-notice">从左侧媒体墙选择一个文件后，这里会展示完整的检查信息。</div>
        )}
      </aside>
    </div>
  );
}

function DetailSection(props: { title: string; children: ReactNode; className?: string }) {
  return (
    <div className={props.className ? `detail-section ${props.className}` : "detail-section"}>
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

function getOrientation(file: Pick<FileItem, "width" | "height">) {
  if (!file.width || !file.height) {
    return "unknown";
  }
  if (file.height > file.width) {
    return "portrait";
  }
  if (file.width > file.height) {
    return "landscape";
  }
  return "square";
}
