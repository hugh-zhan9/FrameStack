import { render, screen, waitFor } from "@testing-library/react";
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

    await waitFor(() => expect(screen.getByText("infer_tags")).toBeInTheDocument());
    expect(screen.getByText("running")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("embedding started")).toBeInTheDocument());
  });
});
