import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { LibraryRoute } from "./LibraryRoute";

describe("LibraryRoute", () => {
  it("loads files from the backend and supports cursor-based load more", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
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
                quality_tier: "high"
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
            file_name: "real-a.jpg",
            abs_path: "/Volumes/media/real-a.jpg",
            media_type: "image",
            status: "active",
            size_bytes: 1024,
            updated_at: "2026-04-13T12:00:00Z",
            has_preview: true,
            tag_names: ["单人"],
            quality_tier: "high",
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
      if (url === "/api/files/7/open") {
        return new Response(null, { status: 204 });
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<LibraryRoute />);

    expect(screen.getByText("正在加载资源…")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("real-a.jpg")).toBeInTheDocument());
    expect(screen.getAllByAltText(/real-a\.jpg/).length).toBeGreaterThan(0);
    await waitFor(() => expect(screen.getByText("文件详情")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("button", { name: "默认程序打开" })).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "默认程序打开" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/files/7/open", { method: "POST" }));
    fireEvent.click(screen.getByRole("button", { name: "加载更多" }));
    await waitFor(() => expect(screen.getByText("real-b.jpg")).toBeInTheDocument());
  });

  it("filters cleanup candidates and supports moving them to trash", async () => {
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
                review_action: "trash_candidate"
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
                review_action: "trash_candidate"
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

    await waitFor(() => expect(screen.getByText("cleanup-a.mp4")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("处理动作筛选"), { target: { value: "trash_candidate" } });
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files?limit=24&sort=updated_desc&review_action=trash_candidate")
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "移动到废纸篓" })).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "移动到废纸篓" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/files/7/trash", { method: "POST" }));
  });
});
