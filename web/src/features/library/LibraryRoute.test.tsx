import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { LibraryRoute } from "./LibraryRoute";

describe("LibraryRoute", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("loads files from the backend, supports rerunning analysis, and auto-loads the next page when the sentinel enters view", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      if (url === "/api/files?limit=24&sort=updated_desc") {
        return new Response(
          JSON.stringify({
            items: [
              {
                id: 7,
                file_name: "real-a.jpg",
                abs_path: "/Volumes/media/real-a.jpg",
                media_type: "image",
                status: "active",
                size_bytes: 1024,
                updated_at: "2026-04-13T12:00:00Z",
                has_preview: true,
                tag_names: ["单人"],
                quality_tier: "high",
                width: 1200,
                height: 1800
              }
            ],
            next_cursor: "cursor-a",
            has_more: true
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files?limit=24&sort=updated_desc&cursor=cursor-a") {
        return new Response(
          JSON.stringify({
            items: [
              {
                id: 8,
                file_name: "real-b.jpg",
                abs_path: "/Volumes/media/real-b.jpg",
                media_type: "image",
                status: "active",
                size_bytes: 2048,
                updated_at: "2026-04-13T11:00:00Z",
                has_preview: false,
                tag_names: ["双人"],
                quality_tier: "medium",
                width: 1800,
                height: 1200
              }
            ],
            next_cursor: "",
            has_more: false
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/7") {
        return new Response(
          JSON.stringify({
            id: 7,
            file_name: "real-a.jpg",
            abs_path: "/Volumes/media/real-a.jpg",
            media_type: "image",
            status: "active",
            size_bytes: 1024,
            updated_at: "2026-04-13T12:00:00Z",
            has_preview: true,
            tag_names: ["单人"],
            quality_tier: "high",
            width: 1200,
            height: 1800,
            tags: [
              {
                namespace: "content",
                name: "单人写真",
                display_name: "单人写真",
                source: "ai",
                confidence: 1
              }
            ],
            current_analyses: [],
            path_history: [],
            review_actions: [],
            embeddings: [],
            video_frames: [],
            clusters: []
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/8") {
        return new Response(
          JSON.stringify({
            id: 8,
            file_name: "real-b.jpg",
            abs_path: "/Volumes/media/real-b.jpg",
            media_type: "image",
            status: "active",
            size_bytes: 2048,
            updated_at: "2026-04-13T11:00:00Z",
            has_preview: false,
            tag_names: ["双人"],
            quality_tier: "medium",
            width: 1800,
            height: 1200,
            tags: [],
            current_analyses: [],
            path_history: [],
            review_actions: [],
            embeddings: [],
            video_frames: [],
            clusters: []
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/7/tags" && method === "POST") {
        return new Response(null, { status: 201 });
      }
      if (url === "/api/files/7/tags" && method === "PUT") {
        return new Response(null, { status: 200 });
      }
      if (url === "/api/files/7/tags?namespace=content&name=%E9%85%92%E5%BA%97%E5%8D%95%E4%BA%BA%E5%86%99%E7%9C%9F" && method === "DELETE") {
        return new Response(null, { status: 204 });
      }
      if (url === "/api/files/7/open" && method === "POST") {
        return new Response(null, { status: 204 });
      }
      if (url === "/api/files/7/recompute-embeddings" && method === "POST") {
        return new Response(null, { status: 202 });
      }
      throw new Error(`unexpected url: ${method} ${url}`);
    });

    let observerCallback: IntersectionObserverCallback | undefined;
    const observe = vi.fn();
    const disconnect = vi.fn();

    class MockIntersectionObserver implements IntersectionObserver {
      readonly root = null;
      readonly rootMargin = "600px 0px 600px 0px";
      readonly thresholds = [0];

      constructor(callback: IntersectionObserverCallback) {
        observerCallback = callback;
      }

      disconnect = disconnect;
      observe = observe;
      takeRecords = () => [];
      unobserve = vi.fn();
    }

    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);

    render(<LibraryRoute />);

    expect(screen.getByText("正在加载资源…")).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText("real-a.jpg").length).toBeGreaterThan(0));
    expect(screen.queryByRole("button", { name: "加载更多" })).not.toBeInTheDocument();
    await waitFor(() => expect(observe).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByRole("button", { name: "重新 AI 分析" })).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("单人写真")).toBeInTheDocument());
    expect(screen.getByRole("option", { name: "内容标签" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重新 AI 分析" }));
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files/7/recompute-embeddings", { method: "POST" })
    );
    await waitFor(() => expect(screen.getByText("已提交 AI 分析任务，可到任务页查看进度。")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "默认程序打开" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/files/7/open", { method: "POST" }));

    fireEvent.change(screen.getByLabelText("新增标签名"), { target: { value: "酒店场景" } });
    fireEvent.click(screen.getByRole("button", { name: "添加标签" }));
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files/7/tags", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          namespace: "content",
          name: "酒店场景"
        })
      })
    );
    await waitFor(() => expect(screen.getByLabelText("新增标签名")).toHaveValue(""));

    await waitFor(() => expect(screen.getByRole("button", { name: "编辑标签 单人写真" })).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "编辑标签 单人写真" }));
    fireEvent.change(screen.getByLabelText("编辑标签名"), { target: { value: "酒店单人写真" } });
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files/7/tags", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          current_namespace: "content",
          current_name: "单人写真",
          namespace: "content",
          name: "酒店单人写真"
        })
      })
    );

    fireEvent.click(screen.getByRole("button", { name: "删除标签 酒店单人写真" }));
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files/7/tags?namespace=content&name=%E9%85%92%E5%BA%97%E5%8D%95%E4%BA%BA%E5%86%99%E7%9C%9F", {
        method: "DELETE"
      })
    );
    await waitFor(() => expect(screen.queryByText("酒店单人写真")).not.toBeInTheDocument());

    act(() => {
      observerCallback?.(
        [
          {
            isIntersecting: true,
            target: observe.mock.calls[0][0],
            intersectionRatio: 1,
            time: 0,
            boundingClientRect: {} as DOMRectReadOnly,
            intersectionRect: {} as DOMRectReadOnly,
            rootBounds: null
          }
        ],
        {} as IntersectionObserver
      );
    });

    await waitFor(() => expect(screen.getByText("real-b.jpg")).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith("/api/files?limit=24&sort=updated_desc&cursor=cursor-a");

    fireEvent.change(screen.getByLabelText("新增标签名"), { target: { value: "切换前草稿" } });
    fireEvent.click(screen.getAllByText("real-b.jpg")[0]);
    await waitFor(() => expect(screen.getByLabelText("新增标签名")).toHaveValue(""));
  });

  it("filters cleanup candidates, renders video preview metadata, and supports moving them to trash", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      if (url === "/api/files?limit=24&sort=updated_desc") {
        return new Response(
          JSON.stringify({
            items: [
              {
                id: 7,
                file_name: "cleanup-a.mp4",
                abs_path: "/Volumes/media/cleanup-a.mp4",
                media_type: "video",
                status: "active",
                size_bytes: 1024,
                updated_at: "2026-04-13T12:00:00Z",
                has_preview: true,
                tag_names: ["重复候选"],
                quality_tier: "medium",
                review_action: "trash_candidate",
                width: 1080,
                height: 1920,
                duration_ms: 12000
              }
            ],
            next_cursor: "",
            has_more: false
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files?limit=24&sort=updated_desc&review_action=trash_candidate") {
        return new Response(
          JSON.stringify({
            items: [
              {
                id: 7,
                file_name: "cleanup-a.mp4",
                abs_path: "/Volumes/media/cleanup-a.mp4",
                media_type: "video",
                status: "active",
                size_bytes: 1024,
                updated_at: "2026-04-13T12:00:00Z",
                has_preview: true,
                tag_names: ["重复候选"],
                quality_tier: "medium",
                review_action: "trash_candidate",
                width: 1080,
                height: 1920,
                duration_ms: 12000
              }
            ],
            next_cursor: "",
            has_more: false
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/7") {
        return new Response(
          JSON.stringify({
            id: 7,
            file_name: "cleanup-a.mp4",
            abs_path: "/Volumes/media/cleanup-a.mp4",
            media_type: "video",
            status: "active",
            size_bytes: 1024,
            updated_at: "2026-04-13T12:00:00Z",
            has_preview: true,
            tag_names: ["重复候选"],
            quality_tier: "medium",
            review_action: "trash_candidate",
            width: 1080,
            height: 1920,
            duration_ms: 12000,
            tags: [],
            current_analyses: [],
            path_history: [],
            review_actions: [],
            embeddings: [],
            video_frames: [],
            clusters: []
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/7/trash" && method === "POST") {
        return new Response(null, { status: 204 });
      }
      if (url === "/api/files/7/generate-preview" && method === "POST") {
        return new Response(null, { status: 202 });
      }
      throw new Error(`unexpected url: ${method} ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<LibraryRoute />);

    await waitFor(() => expect(screen.getAllByText("cleanup-a.mp4").length).toBeGreaterThan(0));
    await waitFor(() => expect(screen.getByLabelText("cleanup-a.mp4 视频预览")).toBeInTheDocument());
    expect(screen.getAllByText("中规格").length).toBeGreaterThan(0);
    expect(screen.getAllByText("在库").length).toBeGreaterThan(0);
    expect(screen.getAllByText("待清理").length).toBeGreaterThan(0);
    expect(screen.queryByText("medium")).not.toBeInTheDocument();
    expect(screen.queryByText("active")).not.toBeInTheDocument();
    expect(screen.queryByText("trash_candidate")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("处理动作筛选"), { target: { value: "trash_candidate" } });
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files?limit=24&sort=updated_desc&review_action=trash_candidate")
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "移动到废纸篓" })).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "生成预览图" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "生成预览图" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/files/7/generate-preview", { method: "POST" }));
    await waitFor(() => expect(screen.getByText("已提交视频预览图生成任务，完成后会自动显示。")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "移动到废纸篓" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/files/7/trash", { method: "POST" }));
  });

  it("passes tag presence and tag filters to the backend", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/files?limit=24&sort=updated_desc") {
        return new Response(
          JSON.stringify({
            items: [
              {
                id: 7,
                file_name: "tagged-a.mp4",
                abs_path: "/Volumes/media/tagged-a.mp4",
                media_type: "video",
                status: "active",
                size_bytes: 1024,
                updated_at: "2026-04-13T12:00:00Z",
                has_preview: true,
                tag_names: ["深色制服少女"],
                quality_tier: "medium"
              }
            ],
            next_cursor: "",
            has_more: false
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files?limit=24&sort=updated_desc&has_tags=true&tag=%E6%B7%B1%E8%89%B2%E5%88%B6%E6%9C%8D%E5%B0%91%E5%A5%B3") {
        return new Response(
          JSON.stringify({
            items: [
              {
                id: 7,
                file_name: "tagged-a.mp4",
                abs_path: "/Volumes/media/tagged-a.mp4",
                media_type: "video",
                status: "active",
                size_bytes: 1024,
                updated_at: "2026-04-13T12:00:00Z",
                has_preview: true,
                tag_names: ["深色制服少女"],
                quality_tier: "medium"
              }
            ],
            next_cursor: "",
            has_more: false
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/7") {
        return new Response(
          JSON.stringify({
            id: 7,
            file_name: "tagged-a.mp4",
            abs_path: "/Volumes/media/tagged-a.mp4",
            media_type: "video",
            status: "active",
            size_bytes: 1024,
            updated_at: "2026-04-13T12:00:00Z",
            has_preview: true,
            tag_names: ["深色制服少女"],
            quality_tier: "medium",
            tags: [],
            current_analyses: [],
            path_history: [],
            review_actions: [],
            embeddings: [],
            video_frames: [],
            clusters: []
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<LibraryRoute />);

    await waitFor(() => expect(screen.getByText("tagged-a.mp4")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("标签存在筛选"), { target: { value: "true" } });
    fireEvent.change(screen.getByLabelText("标签筛选"), { target: { value: "深色制服少女" } });
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/files?limit=24&sort=updated_desc&has_tags=true&tag=%E6%B7%B1%E8%89%B2%E5%88%B6%E6%9C%8D%E5%B0%91%E5%A5%B3"
      )
    );
  });
});
