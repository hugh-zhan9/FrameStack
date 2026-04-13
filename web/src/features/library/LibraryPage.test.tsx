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
        selectedFileId={8}
        onSelectFile={() => {}}
        filters={{
          query: "",
          mediaType: "",
          qualityTier: "",
          status: "",
          sort: "updated_desc"
        }}
        onFiltersChange={() => {}}
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
  });
});
