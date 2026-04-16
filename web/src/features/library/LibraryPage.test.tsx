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
    quality_tier: "high",
    width: 900,
    height: 1600
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
    quality_tier: "medium",
    width: 1920,
    height: 1080,
    duration_ms: 73000
  }
];

describe("LibraryPage", () => {
  it("renders preview-first media cards with adaptive preview ratios and auto-load hint", () => {
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
      />
    );

    expect(screen.getByRole("heading", { name: "资源库" })).toBeInTheDocument();
    expect(screen.getByText("sample-a.jpg")).toBeInTheDocument();
    expect(screen.getByText("sample-b.mp4")).toBeInTheDocument();
    expect(screen.getByText("规格等级按分辨率、码率、帧率等技术指标粗分，不代表审美质量。")).toBeInTheDocument();
    expect(screen.getAllByText("可预览").length).toBeGreaterThan(0);
    expect(screen.getAllByText("图片").length).toBeGreaterThan(0);
    expect(screen.getAllByText("视频").length).toBeGreaterThan(0);
    expect(screen.getAllByText("高规格").length).toBeGreaterThan(0);
    expect(screen.getAllByText("中规格").length).toBeGreaterThan(0);
    expect(screen.getAllByText("在库").length).toBeGreaterThan(0);
    expect(screen.queryByText("high")).not.toBeInTheDocument();
    expect(screen.queryByText("medium")).not.toBeInTheDocument();
    expect(screen.queryByText("active")).not.toBeInTheDocument();
    expect(screen.getByLabelText("媒体类型筛选")).toBeInTheDocument();
    expect(screen.getByLabelText("技术规格等级筛选")).toBeInTheDocument();
    expect(screen.getByLabelText("排序方式")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "已忽略" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "加载更多" })).not.toBeInTheDocument();
    expect(screen.getByText("当前已加载 2 条资源")).toBeInTheDocument();
    expect(screen.getByText("继续向下滚动，自动加载更多资源")).toBeInTheDocument();
    expect(screen.getByText("900 × 1600")).toBeInTheDocument();
    expect(screen.getByText("1m 13s")).toBeInTheDocument();

    const previews = screen.getAllByTestId("library-media-preview");
    expect(previews[0]).toHaveStyle("--preview-aspect: 900 / 1600");
    expect(previews[1]).toHaveStyle("--preview-aspect: 1920 / 1080");
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
      />
    );

    expect(screen.getByText("当前筛选结果已经全部加载完成")).toBeInTheDocument();
    expect(screen.getByText("当前筛选结果已经全部展示完成")).toBeInTheDocument();
  });
});
