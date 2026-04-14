import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ReviewPage } from "./ReviewPage";

describe("ReviewPage", () => {
  it("loads cluster detail and shows evidence summary", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/clusters?limit=12&cluster_type=same_content&status=candidate") {
        return new Response(JSON.stringify([]), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url === "/api/clusters?limit=12&cluster_type=same_person&status=candidate") {
        return new Response(
          JSON.stringify([
            {
              id: 11,
              cluster_type: "same_person",
              title: "人物候选组 A",
              status: "candidate",
              member_count: 3,
              strong_member_count: 2,
              top_member_score: 0.94,
              person_visual_count: 2,
              generic_visual_count: 1,
              top_evidence_type: "person_visual"
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/clusters/11") {
        return new Response(
          JSON.stringify({
            id: 11,
            cluster_type: "same_person",
            title: "人物候选组 A",
            status: "candidate",
            member_count: 3,
            strong_member_count: 2,
            top_member_score: 0.94,
            person_visual_count: 2,
            generic_visual_count: 1,
            top_evidence_type: "person_visual",
            members: [
              {
                file_id: 7,
                file_name: "a.jpg",
                abs_path: "/Volumes/media/a.jpg",
                media_type: "image",
                role: "member",
                score: 0.94,
                quality_tier: "high",
                embedding_type: "person_visual",
                embedding_provider: "semantic",
                embedding_model: "vlm-hash-v1",
                embedding_vector_count: 1
              }
            ]
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<ReviewPage />);
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "same_person" } });

    await waitFor(() => expect(screen.getByText("人物候选组 A")).toBeInTheDocument());
    expect(screen.getByText("审核主视图")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "候选队列（1）" })).toBeInTheDocument();
    expect(screen.getAllByText(/person visual/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/top score/i).length).toBeGreaterThan(0);
    expect(screen.getByText("当前第 1 组 / 共 1 组")).toBeInTheDocument();
    expect(screen.getByAltText("a.jpg-preview")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "确认分组" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "否决分组" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保留推荐版本" })).not.toBeInTheDocument();
  });

  it("does not crash when cluster APIs return null arrays", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/clusters?limit=12&cluster_type=same_content&status=candidate") {
        return new Response(JSON.stringify([]), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url === "/api/clusters?limit=12&cluster_type=same_person&status=candidate") {
        return new Response("null", { status: 200, headers: { "Content-Type": "application/json" } });
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<ReviewPage />);
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "same_person" } });

    await waitFor(() => expect(screen.getByText("暂无聚类详情。")).toBeInTheDocument());
  });

  it("does not crash when cluster detail members is null", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/clusters?limit=12&cluster_type=same_content&status=candidate") {
        return new Response(JSON.stringify([]), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url === "/api/clusters?limit=12&cluster_type=same_person&status=candidate") {
        return new Response(
          JSON.stringify([
            {
              id: 22,
              cluster_type: "same_person",
              title: "人物候选组 B",
              status: "candidate",
              member_count: 0
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/clusters/22") {
        return new Response(
          JSON.stringify({
            id: 22,
            cluster_type: "same_person",
            title: "人物候选组 B",
            status: "candidate",
            member_count: 0,
            members: null
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<ReviewPage />);
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "same_person" } });

    await waitFor(() => expect(screen.getByText("人物候选组 B")).toBeInTheDocument());
    expect(screen.getByText("审核主视图")).toBeInTheDocument();
  });

  it("supports review actions and advances to the next cluster", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      if (url === "/api/clusters?limit=12&cluster_type=same_content&status=candidate") {
        return new Response(
          JSON.stringify([
            {
              id: 11,
              cluster_type: "same_content",
              title: "重复候选 A",
              status: "candidate",
              member_count: 2
            },
            {
              id: 12,
              cluster_type: "same_content",
              title: "重复候选 B",
              status: "candidate",
              member_count: 3
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/clusters/11") {
        return new Response(
          JSON.stringify({
            id: 11,
            cluster_type: "same_content",
            title: "重复候选 A",
            status: "candidate",
            member_count: 2,
            members: []
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/clusters/12") {
        return new Response(
          JSON.stringify({
            id: 12,
            cluster_type: "same_content",
            title: "重复候选 B",
            status: "candidate",
            member_count: 3,
            members: []
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/clusters/11/review-actions" && method === "POST") {
        return new Response(null, { status: 200 });
      }
      if (url === "/api/clusters/11/status" && method === "POST") {
        return new Response(null, { status: 200 });
      }
      throw new Error(`unexpected url: ${method} ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<ReviewPage />);

    await waitFor(() => expect(screen.getByText("重复候选 A")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "确认分组" }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/clusters/11/status", expect.objectContaining({ method: "POST" }))
    );
    await waitFor(() => expect(screen.getByText("已确认当前聚类，已切换到下一组。")).toBeInTheDocument());
    await waitFor(() => expect(screen.getAllByText("重复候选 B").length).toBeGreaterThan(0));
  });

  it("shows same_content-only disposition actions", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      if (url === "/api/clusters?limit=12&cluster_type=same_content&status=candidate") {
        return new Response(
          JSON.stringify([
            {
              id: 31,
              cluster_type: "same_content",
              title: "重复候选 C",
              status: "candidate",
              member_count: 1
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/clusters/31") {
        return new Response(
          JSON.stringify({
            id: 31,
            cluster_type: "same_content",
            title: "重复候选 C",
            status: "candidate",
            member_count: 2,
            members: [
              {
                file_id: 99,
                file_name: "best-version.mp4",
                abs_path: "/Volumes/media/best-version.mp4",
                media_type: "video",
                role: "best_quality",
                score: 0.91,
                quality_tier: "high",
                embedding_type: "video_frame_visual",
                embedding_provider: "semantic",
                embedding_model: "vlm-hash-v1",
                embedding_vector_count: 3
              },
              {
                file_id: 100,
                file_name: "duplicate-version.mp4",
                abs_path: "/Volumes/media/duplicate-version.mp4",
                media_type: "video",
                role: "duplicate_candidate",
                score: 0.66,
                quality_tier: "medium",
                embedding_type: "video_frame_visual",
                embedding_provider: "semantic",
                embedding_model: "vlm-hash-v1",
                embedding_vector_count: 3
              }
            ]
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/files/99/review-actions" && method === "POST") {
        return new Response(null, { status: 200 });
      }
      if (url === "/api/files/100/review-actions" && method === "POST") {
        return new Response(null, { status: 200 });
      }
      if (url === "/api/clusters/31/status" && method === "POST") {
        return new Response(null, { status: 200 });
      }
      throw new Error(`unexpected url: ${method} ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<ReviewPage />);

    await waitFor(() => expect(screen.getByText("重复候选 C")).toBeInTheDocument());
    expect(screen.getAllByText("best-version.mp4").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/推荐保留/).length).toBeGreaterThan(0);
    await waitFor(() => expect(screen.getByRole("button", { name: "保留当前选择，其余标记清理候选" })).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "其余标记清理候选" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "duplicate-version.mp4-preview" }));
    fireEvent.click(screen.getByRole("button", { name: "保留当前选择，其余标记清理候选" }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files/100/review-actions", expect.objectContaining({ method: "POST" }))
    );
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/files/99/review-actions", expect.objectContaining({ method: "POST" }))
    );
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith("/api/clusters/31/status", expect.objectContaining({ method: "POST" }))
    );
    await waitFor(() => expect(screen.getByText("已保留当前选择，并确认当前分组，已切换到下一组。")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("暂无聚类详情。")).toBeInTheDocument());
  });

  it("toggles the queue drawer and allows switching clusters from the queue", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/clusters?limit=12&cluster_type=same_content&status=candidate") {
        return new Response(JSON.stringify([]), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url === "/api/clusters?limit=12&cluster_type=same_series&status=candidate") {
        return new Response(
          JSON.stringify([
            {
              id: 41,
              cluster_type: "same_series",
              title: "系列 A",
              status: "candidate",
              member_count: 2
            },
            {
              id: 42,
              cluster_type: "same_series",
              title: "系列 B",
              status: "candidate",
              member_count: 4
            }
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/clusters/41") {
        return new Response(
          JSON.stringify({
            id: 41,
            cluster_type: "same_series",
            title: "系列 A",
            status: "candidate",
            member_count: 2,
            members: []
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url === "/api/clusters/42") {
        return new Response(
          JSON.stringify({
            id: 42,
            cluster_type: "same_series",
            title: "系列 B",
            status: "candidate",
            member_count: 4,
            members: []
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<ReviewPage />);
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "same_series" } });

    await waitFor(() => expect(screen.getAllByText("系列 A").length).toBeGreaterThan(0));
    fireEvent.click(screen.getByRole("button", { name: "候选队列（2）" }));
    await waitFor(() => expect(screen.getByText("候选队列")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /系列 B/ }));
    await waitFor(() => expect(screen.getAllByText("系列 B").length).toBeGreaterThan(0));
  });
});
