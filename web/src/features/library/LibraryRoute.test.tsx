import { render, screen, waitFor } from "@testing-library/react";
import { LibraryRoute } from "./LibraryRoute";

describe("LibraryRoute", () => {
  it("loads files from the backend and renders preview URLs", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/files?")) {
        return new Response(
          JSON.stringify([
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
          ]),
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
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<LibraryRoute />);

    expect(screen.getByText("正在加载资源…")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("real-a.jpg")).toBeInTheDocument());
    expect(screen.getAllByAltText(/real-a\.jpg/).length).toBeGreaterThan(0);
    await waitFor(() => expect(screen.getByText("文件详情")).toBeInTheDocument());
  });
});
