import type { CSSProperties, ChangeEvent, Ref } from "react";
import {
  formatMediaType,
  getFileMetaBadges,
  QUALITY_TIER_NOTE
} from "./presentation";
import type { FileItem } from "./types";

type LibraryFilters = {
  query: string;
  mediaType: string;
  qualityTier: string;
  status: string;
  reviewAction: string;
  sort: string;
};

type Props = {
  files: FileItem[];
  selectedFile?: FileItem | null;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  totalLoaded: number;
  selectedFileId: number | null;
  onSelectFile: (fileId: number) => void;
  filters: LibraryFilters;
  onFiltersChange: (patch: Partial<LibraryFilters>) => void;
  loadMoreRef?: Ref<HTMLDivElement>;
  error?: string | null;
};

export function LibraryPage(props: Props) {
  return (
    <section className="page-shell library-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">Library</p>
          <h2>资源库</h2>
          <p className="page-subtitle">预览优先的本地媒体工作台，按技术规格、文件状态和处理动作快速筛选与检查。</p>
        </div>
      </header>

      <div className="library-hero">
        <div className="library-hero-copy">
          <span className="library-section-label">Workbench</span>
          <h3>把筛选、浏览和检查收敛到一个连续工作流里</h3>
          <p>
            当前版本优先解决遮挡、层级冲突和操作路径断裂的问题，让筛选与详情检查不再互相抢视觉焦点。
          </p>
        </div>
        <div className="library-hero-stats" aria-label="资源库状态摘要">
          <div className="library-stat-card">
            <span>已加载</span>
            <strong>{props.totalLoaded}</strong>
            <small>当前命中资源</small>
          </div>
          <div className="library-stat-card">
            <span>浏览模式</span>
            <strong>瀑布流</strong>
            <small>预览优先</small>
          </div>
          <div className="library-stat-card">
            <span>续页方式</span>
            <strong>{props.hasMore ? "自动" : "完成"}</strong>
            <small>{props.hasMore ? "滚动触发加载" : "当前结果已到底"}</small>
          </div>
          <div className="library-stat-card">
            <span>当前焦点</span>
            <strong>{props.selectedFile ? `#${props.selectedFile.id}` : "--"}</strong>
            <small>{props.selectedFile ? props.selectedFile.file_name : "尚未选择资源"}</small>
          </div>
        </div>
      </div>

      <div className="library-controls-panel">
        <div className="library-panel-heading">
          <div>
            <span className="library-section-label">Filter Console</span>
            <h3>筛选控制台</h3>
          </div>
          <p>搜索栏独占一行，技术规格、生命周期状态和处理动作分开表达，避免语义混在一起。</p>
        </div>

        <div className="library-toolbar">
          <input
            type="search"
            value={props.filters.query}
            onChange={(event: ChangeEvent<HTMLInputElement>) => props.onFiltersChange({ query: event.target.value })}
            placeholder="搜索文件名、路径、格式、标签"
            aria-label="搜索文件"
            className="library-search"
          />
          <select
            value={props.filters.mediaType}
            onChange={(event) => props.onFiltersChange({ mediaType: event.target.value })}
            aria-label="媒体类型筛选"
          >
            <option value="">全部类型</option>
            <option value="image">图片</option>
            <option value="video">视频</option>
          </select>
          <select
            value={props.filters.qualityTier}
            onChange={(event) => props.onFiltersChange({ qualityTier: event.target.value })}
            aria-label="技术规格等级筛选"
          >
            <option value="">全部规格</option>
            <option value="high">高规格</option>
            <option value="medium">中规格</option>
            <option value="low">基础规格</option>
          </select>
          <select
            value={props.filters.status}
            onChange={(event) => props.onFiltersChange({ status: event.target.value })}
            aria-label="文件状态筛选"
          >
            <option value="">全部状态</option>
            <option value="active">在库</option>
            <option value="missing">源文件缺失</option>
            <option value="ignored">已忽略</option>
            <option value="trashed">已移入废纸篓</option>
          </select>
          <select
            value={props.filters.reviewAction}
            onChange={(event) => props.onFiltersChange({ reviewAction: event.target.value })}
            aria-label="处理动作筛选"
          >
            <option value="">全部处理动作</option>
            <option value="trash_candidate">待清理</option>
            <option value="keep">保留</option>
            <option value="favorite">收藏</option>
            <option value="ignore">忽略</option>
            <option value="hide">隐藏</option>
            <option value="deleted_to_trash">已移入废纸篓</option>
          </select>
          <select
            value={props.filters.sort}
            onChange={(event) => props.onFiltersChange({ sort: event.target.value })}
            aria-label="排序方式"
          >
            <option value="updated_desc">最近更新</option>
            <option value="quality_desc">质量优先</option>
            <option value="size_desc">体积从大到小</option>
            <option value="size_asc">体积从小到大</option>
            <option value="name_asc">名称</option>
          </select>
        </div>
        <p className="library-filter-note">{QUALITY_TIER_NOTE}</p>
      </div>

      {!props.loading ? (
        <div className="library-status-bar">
          <span>当前已加载 {props.totalLoaded} 条资源</span>
          <span>{props.hasMore ? "继续滚动会自动加载后续结果" : "当前筛选结果已经全部加载完成"}</span>
        </div>
      ) : null}

      {props.error ? (
        <div className="empty-state" role="alert">
          {props.error}
        </div>
      ) : null}

      {props.loading ? (
        <div className="empty-state">正在加载资源…</div>
      ) : props.files.length === 0 ? (
        <div className="empty-state">当前没有可展示的文件。</div>
      ) : (
        <div className="library-results-panel">
          <div className="library-panel-heading">
            <div>
              <span className="library-section-label">Media Wall</span>
              <h3>资源预览墙</h3>
            </div>
            <p>卡片强调预览、尺寸和体积信息，减少路径与徽标相互抢注意力。</p>
          </div>
          <div className="media-grid">
            {props.files.map((file) => (
              <button
                key={file.id}
                type="button"
                className={file.id === props.selectedFileId ? "media-card selected" : "media-card"}
                data-orientation={getOrientation(file)}
                onClick={() => props.onSelectFile(file.id)}
              >
                <div
                  className="media-preview"
                  data-testid="library-media-preview"
                  data-orientation={getOrientation(file)}
                  style={getPreviewStyle(file)}
                >
                  {file.has_preview ? (
                    <img
                      src={`/api/files/${file.id}/preview`}
                      alt={file.file_name}
                      aria-label={`${file.file_name} ${file.media_type === "video" ? "视频" : "图片"}预览`}
                      className="media-preview-image"
                    />
                  ) : (
                    <div className="media-preview-fallback">无预览</div>
                  )}
                  <span className="media-preview-label">{file.has_preview ? "可预览" : "无预览"}</span>
                  <span className="media-badge">{formatMediaType(file.media_type)}</span>
                </div>
                <div className="media-body">
                  <div className="media-card-title">
                    <strong>{file.file_name}</strong>
                    <span className="media-card-id">#{file.id}</span>
                  </div>
                  <span className="media-path">{file.abs_path}</span>
                  <div className="media-meta">
                    {getFileMetaBadges(file).map((badge) => (
                      <span key={`${file.id}:${badge}`}>{badge}</span>
                    ))}
                  </div>
                  <div className="media-facts">
                    <span className="media-fact">{formatResolution(file.width, file.height)}</span>
                    {file.media_type === "video" && file.duration_ms ? (
                      <span className="media-fact">{formatDuration(file.duration_ms)}</span>
                    ) : null}
                    <span className="media-fact">{formatBytes(file.size_bytes)}</span>
                    {file.format || file.container ? (
                      <span className="media-fact">{(file.format || file.container || "-").toUpperCase()}</span>
                    ) : null}
                  </div>
                  <div className="media-tags">
                    {(file.tag_names ?? []).slice(0, 4).map((tag) => (
                      <span key={tag} className="media-tag">
                        {tag}
                      </span>
                    ))}
                  </div>
                </div>
              </button>
            ))}
          </div>
          {props.hasMore ? (
            <div className="library-auto-load-sentinel" ref={props.loadMoreRef} aria-live="polite">
              <span className="library-load-more-hint">
                {props.loadingMore ? "正在加载更多资源…" : "继续向下滚动，自动加载更多资源"}
              </span>
            </div>
          ) : (
            <div className="library-load-more library-load-more-done">
              <span className="library-load-more-hint">当前筛选结果已经全部展示完成</span>
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function getOrientation(file: FileItem) {
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

function getPreviewStyle(file: FileItem): CSSProperties {
  if (!file.width || !file.height) {
    return {};
  }
  return {
    "--preview-aspect": `${file.width} / ${file.height}`
  } as CSSProperties;
}

function formatResolution(width?: number, height?: number) {
  if (!width || !height) {
    return "未识别尺寸";
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
