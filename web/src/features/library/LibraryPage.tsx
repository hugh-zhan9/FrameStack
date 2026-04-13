import type { ChangeEvent } from "react";
import type { FileItem } from "./types";

type LibraryFilters = {
  query: string;
  mediaType: string;
  qualityTier: string;
  status: string;
  sort: string;
};

type Props = {
  files: FileItem[];
  loading: boolean;
  selectedFileId: number | null;
  onSelectFile: (fileId: number) => void;
  filters: LibraryFilters;
  onFiltersChange: (patch: Partial<LibraryFilters>) => void;
  error?: string | null;
};

export function LibraryPage(props: Props) {
  return (
    <section className="page-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">Library</p>
          <h2>资源库</h2>
          <p className="page-subtitle">预览优先的本地媒体浏览台。</p>
        </div>
      </header>

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
          aria-label="质量等级筛选"
        >
          <option value="">全部质量</option>
          <option value="high">高</option>
          <option value="medium">中</option>
          <option value="low">低</option>
        </select>
        <select
          value={props.filters.status}
          onChange={(event) => props.onFiltersChange({ status: event.target.value })}
          aria-label="文件状态筛选"
        >
          <option value="">全部状态</option>
          <option value="active">active</option>
          <option value="missing">missing</option>
          <option value="trashed">trashed</option>
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

      {props.error ? <div className="empty-state">{props.error}</div> : null}

      {props.loading ? (
        <div className="empty-state">正在加载资源…</div>
      ) : props.files.length === 0 ? (
        <div className="empty-state">当前没有可展示的文件。</div>
      ) : (
        <div className="media-grid">
          {props.files.map((file) => (
            <button
              key={file.id}
              type="button"
              className={file.id === props.selectedFileId ? "media-card selected" : "media-card"}
              onClick={() => props.onSelectFile(file.id)}
            >
              <div className="media-preview">
                {file.has_preview ? (
                  <img
                    src={`/api/files/${file.id}/preview`}
                    alt={file.file_name}
                    className="media-preview-image"
                  />
                ) : (
                  <div className="media-preview-fallback">无预览</div>
                )}
                <span className="media-preview-label">{file.has_preview ? "可预览" : "无预览"}</span>
                <span className="media-badge">{file.media_type === "video" ? "视频" : "图片"}</span>
              </div>
              <div className="media-body">
                <strong>{file.file_name}</strong>
                <span className="media-path">{file.abs_path}</span>
                <div className="media-meta">
                  <span>{file.quality_tier ?? "unknown"}</span>
                  <span>{file.status}</span>
                </div>
                <div className="media-tags">
                  {(file.tag_names ?? []).map((tag) => (
                    <span key={tag} className="media-tag">
                      {tag}
                    </span>
                  ))}
                </div>
              </div>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
