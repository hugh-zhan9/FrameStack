import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { TasksPage } from "./TasksPage";

describe("TasksPage", () => {
  it("loads jobs and shows event timeline", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/jobs?limit=12") {
        return new Response(
          JSON.stringify([
            {
              id: 21,
              job_type: "infer_tags",
              status: "running",
              target_type: "file",
              target_id: 7
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/system/ai-prompt-settings") {
        return new Response(
          JSON.stringify({
            understanding_extra_prompt: "优先输出明确中文标签"
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/7") {
        return new Response(
          JSON.stringify({
            id: 7,
            file_name: "poster.jpg",
            abs_path: "/Volumes/media/poster.jpg",
            media_type: "image",
            status: "active",
            size_bytes: 1024,
            updated_at: "2026-04-13T12:00:00Z",
            has_preview: true,
            tag_names: [],
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
      if (url === "/api/jobs/21/events") {
        return new Response(
          JSON.stringify([
            {
              id: 1,
              level: "info",
              message: "embedding started",
              created_at: "2026-04-13T12:00:00Z"
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<TasksPage />);

    await waitFor(() => expect(screen.getAllByText("AI 打标签").length).toBeGreaterThan(0));
    expect(screen.getAllByText("执行中").length).toBeGreaterThan(0);
    await waitFor(() => expect(screen.getByText("poster.jpg")).toBeInTheDocument());
    expect(screen.getByAltText("poster.jpg 预览")).toBeInTheDocument();
    expect(screen.getByDisplayValue("优先输出明确中文标签")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("embedding started")).toBeInTheDocument());
    expect(screen.getByText("信息")).toBeInTheDocument();
  });

  it("does not crash when a pending job has no events yet", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/jobs?limit=12") {
        return new Response(
          JSON.stringify([
            {
              id: 21,
              job_type: "infer_tags",
              status: "running",
              target_type: "file",
              target_id: 7
            },
            {
              id: 22,
              job_type: "embed_image",
              status: "pending",
              target_type: "file",
              target_id: 8
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/system/ai-prompt-settings") {
        return new Response(
          JSON.stringify({
            understanding_extra_prompt: ""
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/7") {
        return new Response(
          JSON.stringify({
            id: 7,
            file_name: "poster.jpg",
            abs_path: "/Volumes/media/poster.jpg",
            media_type: "image",
            status: "active",
            size_bytes: 1024,
            updated_at: "2026-04-13T12:00:00Z",
            has_preview: true,
            tag_names: [],
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
      if (url === "/api/files/8") {
        return new Response(
          JSON.stringify({
            id: 8,
            file_name: "cover.jpg",
            abs_path: "/Volumes/media/cover.jpg",
            media_type: "image",
            status: "active",
            size_bytes: 2048,
            updated_at: "2026-04-13T12:00:00Z",
            has_preview: true,
            tag_names: [],
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
      if (url === "/api/jobs/21/events") {
        return new Response(
          JSON.stringify([
            {
              id: 1,
              level: "info",
              message: "embedding started",
              created_at: "2026-04-13T12:00:00Z"
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/jobs/22/events") {
        return new Response("null", { status: 200, headers: { "Content-Type": "application/json" } });
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<TasksPage />);

    await waitFor(() => expect(screen.getAllByText("AI 打标签").length).toBeGreaterThan(0));
    await waitFor(() => expect(screen.getByText("embedding started")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /图片向量/i }));

    await waitFor(() => expect(screen.getByText("当前任务还没有事件记录。")).toBeInTheDocument());
  });

  it("saves AI prompt settings", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      if (url === "/api/jobs?limit=12") {
        return new Response(JSON.stringify([]), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url === "/api/system/ai-prompt-settings" && method === "GET") {
        return new Response(
          JSON.stringify({
            understanding_extra_prompt: ""
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/system/ai-prompt-settings" && method === "POST") {
        return new Response(init?.body ?? "{}", { status: 200, headers: { "Content-Type": "application/json" } });
      }
      throw new Error(`unexpected url: ${method} ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<TasksPage />);

    await waitFor(() => expect(screen.getByLabelText("理解提示词附加规则")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("理解提示词附加规则"), { target: { value: "只输出中文标签" } });
    fireEvent.click(screen.getByRole("button", { name: "保存提示词规则" }));
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/system/ai-prompt-settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ understanding_extra_prompt: "只输出中文标签" })
      })
    );
  });
});
