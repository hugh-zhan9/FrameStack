import { render, screen } from "@testing-library/react";
import { LibraryPage } from "./LibraryPage";
import type { FileItem } from "./types";

const files: FileItem[] = [
  {
    id: 7,
    file_name: "sample-a.jpg",
    abs_path: "/Volumes/media/sample-a.jpg",
    media_type: "image",
    status: "active",
    size_bytes: 1024,
    updated_at: "2026-04-13T12:00:00Z",
    has_preview: true,
    tag_names: ["单人", "写真"],
    quality_tier: "high"
  },
  {
    id: 8,
    file_name: "sample-b.mp4",
    abs_path: "/Volumes/media/sample-b.mp4",
    media_type: "video",
    status: "active",
    size_bytes: 2048,
    updated_at: "2026-04-13T12:10:00Z",
    has_preview: true,
    tag_names: ["视频"],
    quality_tier: "medium"
  }
];

describe("LibraryPage", () => {
  it("renders preview-first media cards with media badges", () => {
    render(
      <LibraryPage
        files={files}
        loading={false}
        loadingMore={false}
        hasMore={true}
        totalLoaded={files.length}
        selectedFileId={8}
        onSelectFile={() => {}}
        filters={{
          query: "",
          mediaType: "",
          qualityTier: "",
          status: "",
          reviewAction: "",
          sort: "updated_desc"
        }}
        onFiltersChange={() => {}}
        onLoadMore={() => {}}
      />
    );

    expect(screen.getByRole("heading", { name: "资源库" })).toBeInTheDocument();
    expect(screen.getByText("sample-a.jpg")).toBeInTheDocument();
    expect(screen.getByText("sample-b.mp4")).toBeInTheDocument();
    expect(screen.getAllByText("可预览").length).toBeGreaterThan(0);
    expect(screen.getAllByText("图片").length).toBeGreaterThan(0);
    expect(screen.getAllByText("视频").length).toBeGreaterThan(0);
    expect(screen.getByLabelText("媒体类型筛选")).toBeInTheDocument();
    expect(screen.getByLabelText("排序方式")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "加载更多" })).toBeInTheDocument();
    expect(screen.getByText("当前已加载 2 条资源")).toBeInTheDocument();
    expect(screen.getByText("还有更多结果可继续加载")).toBeInTheDocument();
  });

  it("shows done state when all results are loaded", () => {
    render(
      <LibraryPage
        files={files}
        loading={false}
        loadingMore={false}
        hasMore={false}
        totalLoaded={files.length}
        selectedFileId={8}
        onSelectFile={() => {}}
        filters={{
          query: "",
          mediaType: "",
          qualityTier: "",
          status: "",
          reviewAction: "",
          sort: "updated_desc"
        }}
        onFiltersChange={() => {}}
        onLoadMore={() => {}}
      />
    );

    expect(screen.getByText("已加载完当前筛选结果")).toBeInTheDocument();
    expect(screen.getByText("当前筛选结果已经全部展示完成")).toBeInTheDocument();
  });
});
