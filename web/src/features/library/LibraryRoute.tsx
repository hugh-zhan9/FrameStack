import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  createFileTag,
  deleteFileTag,
  fetchFileDetail,
  fetchFiles,
  generateFilePreview,
  moveFileToTrash,
  openFileWithDefaultApp,
  replaceFileTag,
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
  formatUnknown
} from "./presentation";
import { useAsync } from "../../lib/useAsync";
import { LibraryPage } from "./LibraryPage";
import type { FileDetail, FileTag } from "../../lib/contracts";
import type { FileItem } from "./types";

type LibraryFilters = {
  query: string;
  mediaType: string;
  qualityTier: string;
  status: string;
  reviewAction: string;
  hasTags: string;
  tag: string;
  sort: string;
};

const defaultFilters: LibraryFilters = {
  query: "",
  mediaType: "",
  qualityTier: "",
  status: "",
  reviewAction: "",
  hasTags: "",
  tag: "",
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
  const [previewPending, setPreviewPending] = useState(false);
  const [detailRefreshToken, setDetailRefreshToken] = useState(0);
  const [tagPending, setTagPending] = useState(false);
  const [tagDraft, setTagDraft] = useState({
    namespace: "content",
    name: ""
  });
  const [editingTagKey, setEditingTagKey] = useState<string | null>(null);
  const [tagEditDraft, setTagEditDraft] = useState({
    currentNamespace: "",
    currentName: "",
    namespace: "content",
    name: ""
  });
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
  const [detail, setDetail] = useState<FileDetail | null>(null);

  useEffect(() => {
    setDetail(null);
  }, [selectedFileId]);

  useEffect(() => {
    if (detailState.data) {
      setDetail(detailState.data);
    }
  }, [detailState.data]);

  useEffect(() => {
    setTagDraft((current) => ({ ...current, name: "" }));
    setEditingTagKey(null);
    setTagEditDraft({
      currentNamespace: "",
      currentName: "",
      namespace: "content",
      name: ""
    });
  }, [selectedFileId]);

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

  async function handleGeneratePreview() {
    if (!detail || previewPending || detail.media_type !== "video") {
      return;
    }
    setPreviewPending(true);
    setError(null);
    setNotice(null);
    try {
      await generateFilePreview(detail.id);
      setNotice("已提交视频预览图生成任务，完成后会自动显示。");
      setDetailRefreshToken((current) => current + 1);
    } catch (err) {
      setError(err instanceof Error ? err.message : "提交预览图生成任务失败");
    } finally {
      setPreviewPending(false);
    }
  }

  async function handleCreateTag() {
    if (!detail || tagPending) {
      return;
    }
    const namespace = tagDraft.namespace.trim();
    const name = tagDraft.name.trim();
    if (!namespace || !name) {
      setError("请填写标签命名空间和标签名");
      return;
    }
    setTagPending(true);
    setError(null);
    setNotice(null);
    try {
      await createFileTag({
        fileId: detail.id,
        namespace,
        name
      });
      setNotice("标签已添加。");
      setTagDraft((current) => ({ ...current, name: "" }));
      const nextTag = buildLocalTag(namespace, name);
      setDetail((current) => (current ? applyTagsToDetail(current, upsertLocalTag(current.tags ?? [], nextTag)) : current));
      setFiles((current) => updateFileListTags(current, detail.id, upsertLocalTag(detail.tags ?? [], nextTag)));
    } catch (err) {
      setError(err instanceof Error ? err.message : "添加标签失败");
    } finally {
      setTagPending(false);
    }
  }

  async function handleDeleteTag(namespace: string, name: string) {
    if (!detail || tagPending) {
      return;
    }
    setTagPending(true);
    setError(null);
    setNotice(null);
    try {
      await deleteFileTag({
        fileId: detail.id,
        namespace,
        name
      });
      setNotice("标签已删除。");
      const nextTags = removeLocalTag(detail.tags ?? [], namespace, name);
      setDetail((current) => (current ? applyTagsToDetail(current, nextTags) : current));
      setFiles((current) => updateFileListTags(current, detail.id, nextTags));
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除标签失败");
    } finally {
      setTagPending(false);
    }
  }

  function handleStartEditTag(tag: FileTag) {
    setEditingTagKey(`${tag.namespace}:${tag.name}`);
    setTagEditDraft({
      currentNamespace: tag.namespace,
      currentName: tag.name,
      namespace: tag.namespace,
      name: tag.name
    });
    setError(null);
    setNotice(null);
  }

  function handleCancelEditTag() {
    setEditingTagKey(null);
    setTagEditDraft({
      currentNamespace: "",
      currentName: "",
      namespace: "content",
      name: ""
    });
  }

  async function handleReplaceTag() {
    if (!detail || tagPending || !editingTagKey) {
      return;
    }
    const currentNamespace = tagEditDraft.currentNamespace.trim();
    const currentName = tagEditDraft.currentName.trim();
    const namespace = tagEditDraft.namespace.trim();
    const name = tagEditDraft.name.trim();
    if (!currentNamespace || !currentName || !namespace || !name) {
      setError("请填写完整的原标签和新标签信息");
      return;
    }
    setTagPending(true);
    setError(null);
    setNotice(null);
    try {
      await replaceFileTag({
        fileId: detail.id,
        current_namespace: currentNamespace,
        current_name: currentName,
        namespace,
        name
      });
      setNotice("标签已更新。");
      const nextTag = buildLocalTag(namespace, name);
      const nextTags = upsertLocalTag(removeLocalTag(detail.tags ?? [], currentNamespace, currentName), nextTag);
      setDetail((current) => (current ? applyTagsToDetail(current, nextTags) : current));
      setFiles((current) => updateFileListTags(current, detail.id, nextTags));
      handleCancelEditTag();
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新标签失败");
    } finally {
      setTagPending(false);
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
                  {detail.media_type === "video" ? (
                    <button
                      type="button"
                      className="secondary-button"
                      onClick={handleGeneratePreview}
                      disabled={previewPending}
                    >
                      {previewPending ? "提交中…" : "生成预览图"}
                    </button>
                  ) : null}
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
              </section>

              <DetailSection title="基础指标" className="detail-overview-card">
                <div className="detail-grid detail-grid-compact">
                  <DetailStat label="规格等级" value={formatQualityTier(detail.quality_tier)} />
                  <DetailStat label="技术评分" value={formatQualityScore(detail.quality_score)} />
                  <DetailStat label="文件状态" value={formatFileStatus(detail.status)} />
                  <DetailStat label="分辨率" value={formatResolution(detail.width, detail.height, detail.media_type)} />
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
                  <div className="detail-tag-form">
                    <div className="detail-tag-form-fields">
                      <select
                        value={tagDraft.namespace}
                        onChange={(event) => setTagDraft((current) => ({ ...current, namespace: event.target.value }))}
                        aria-label="新增标签命名空间"
                      >
                        <option value="content">内容标签</option>
                        <option value="quality">质量标签</option>
                        <option value="sensitive">敏感标签</option>
                        <option value="person">人物标签</option>
                        <option value="management">管理标签</option>
                      </select>
                      <input
                        type="text"
                        value={tagDraft.name}
                        onChange={(event) => setTagDraft((current) => ({ ...current, name: event.target.value }))}
                        placeholder="标签名"
                        aria-label="新增标签名"
                      />
                    </div>
                    <div className="detail-tag-form-actions">
                      <button type="button" className="secondary-button" onClick={handleCreateTag} disabled={tagPending}>
                        {tagPending ? "提交中…" : "添加标签"}
                      </button>
                    </div>
                  </div>
                  {editingTagKey ? (
                    <div className="detail-tag-editor-card">
                      <div className="detail-tag-editor-header">
                        <strong>编辑标签</strong>
                        <span>{tagEditDraft.currentNamespace}:{tagEditDraft.currentName}</span>
                      </div>
                      <div className="detail-tag-editor-form">
                        <div className="detail-tag-editor-fields">
                          <select
                            value={tagEditDraft.namespace}
                            onChange={(event) => setTagEditDraft((current) => ({ ...current, namespace: event.target.value }))}
                            aria-label="编辑标签命名空间"
                          >
                            <option value="content">内容标签</option>
                            <option value="quality">质量标签</option>
                            <option value="sensitive">敏感标签</option>
                            <option value="person">人物标签</option>
                            <option value="management">管理标签</option>
                          </select>
                          <input
                            type="text"
                            value={tagEditDraft.name}
                            onChange={(event) => setTagEditDraft((current) => ({ ...current, name: event.target.value }))}
                            placeholder="新的标签名"
                            aria-label="编辑标签名"
                          />
                        </div>
                        <div className="detail-tag-editor-actions">
                          <button type="button" className="secondary-button" onClick={handleReplaceTag} disabled={tagPending}>
                            {tagPending ? "保存中…" : "保存"}
                          </button>
                          <button type="button" className="secondary-button" onClick={handleCancelEditTag} disabled={tagPending}>
                            取消
                          </button>
                        </div>
                      </div>
                    </div>
                  ) : null}
                  <div className="media-tags detail-tag-list">
                    {(detail.tags ?? []).length ? (
                      (detail.tags ?? []).slice(0, 12).map((tag) => (
                        <span key={`${tag.namespace}:${tag.name}:${tag.source}`} className="media-tag media-tag-editable">
                          <span>{tag.name}</span>
                          <button
                            type="button"
                            className="media-tag-action"
                            onClick={() => handleStartEditTag(tag)}
                            disabled={tagPending}
                            aria-label={`编辑标签 ${tag.name}`}
                          >
                            编辑
                          </button>
                          <button
                            type="button"
                            className="media-tag-remove"
                            onClick={() => void handleDeleteTag(tag.namespace, tag.name)}
                            disabled={tagPending}
                            aria-label={`删除标签 ${tag.name}`}
                          >
                            ×
                          </button>
                        </span>
                      ))
                    ) : (
                      <span className="detail-note">当前还没有标签。</span>
                    )}
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
                        loading="lazy"
                        decoding="async"
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

function buildLocalTag(namespace: string, name: string): FileTag {
  return {
    namespace,
    name,
    display_name: name,
    source: "human",
    confidence: 1
  };
}

function upsertLocalTag(tags: FileTag[], nextTag: FileTag): FileTag[] {
  return [nextTag, ...tags.filter((tag) => !(tag.namespace === nextTag.namespace && tag.name === nextTag.name))];
}

function removeLocalTag(tags: FileTag[], namespace: string, name: string): FileTag[] {
  return tags.filter((tag) => !(tag.namespace === namespace && tag.name === name));
}

function applyTagsToDetail(detail: FileDetail, tags: FileTag[]): FileDetail {
  return {
    ...detail,
    tags,
    tag_names: extractTagNames(tags)
  };
}

function updateFileListTags(files: FileItem[], fileId: number, tags: FileTag[]): FileItem[] {
  const tagNames = extractTagNames(tags);
  return files.map((file) => (file.id === fileId ? { ...file, tag_names: tagNames } : file));
}

function extractTagNames(tags: FileTag[]): string[] {
  return Array.from(new Set(tags.map((tag) => tag.name)));
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

function formatResolution(width?: number, height?: number, mediaType?: string) {
  if (!width || !height) {
    return mediaType === "image" ? "尚未提取尺寸" : "尚未提取分辨率";
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
