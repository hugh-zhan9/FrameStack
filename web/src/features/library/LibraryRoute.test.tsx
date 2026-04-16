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

    fireEvent.click(screen.getByRole("button", { name: "重新 AI 分析" }));
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files/7/recompute-embeddings", { method: "POST" })
    );
    await waitFor(() => expect(screen.getByText("已提交 AI 分析任务，可到任务页查看进度。")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "默认程序打开" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/files/7/open", { method: "POST" }));

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
    fireEvent.click(screen.getByRole("button", { name: "移动到废纸篓" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/files/7/trash", { method: "POST" }));
  });
});
