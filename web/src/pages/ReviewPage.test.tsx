import { render, screen, waitFor } from "@testing-library/react";
import { ReviewPage } from "./ReviewPage";

describe("ReviewPage", () => {
  it("loads cluster detail and shows evidence summary", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/clusters?")) {
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

    await waitFor(() => expect(screen.getByText("人物候选组 A")).toBeInTheDocument());
    expect(screen.getAllByText(/person visual/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/top score/i).length).toBeGreaterThan(0);
  });
});
