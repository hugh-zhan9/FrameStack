import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { VolumesPage } from "./VolumesPage";

describe("VolumesPage", () => {
  it("picks a directory and fills mount path", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "/api/volumes" && (!init?.method || init.method === "GET")) {
        return new Response(JSON.stringify([]), {
          status: 200,
          headers: { "Content-Type": "application/json" }
        });
      }
      if (url === "/api/system/pick-directory" && init?.method === "POST") {
        return new Response(JSON.stringify({ path: "/Volumes/media" }), {
          status: 200,
          headers: { "Content-Type": "application/json" }
        });
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<VolumesPage />);

    fireEvent.click(await screen.findByRole("button", { name: "选择文件夹" }));

    await waitFor(() => {
      expect(screen.getByPlaceholderText("/Volumes/media")).toHaveValue("/Volumes/media");
    });
  });

  it("disables volume creation when database is disabled", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "/api/volumes" && (!init?.method || init.method === "GET")) {
        return new Response(JSON.stringify([]), {
          status: 200,
          headers: { "Content-Type": "application/json" }
        });
      }
      if (url === "/api/system-status") {
        return new Response(
          JSON.stringify({
            status: "ready",
            checks: [{ name: "database", status: "disabled", message: "database disabled" }]
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`unexpected url: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);

    render(<VolumesPage />);

    expect(await screen.findByText("当前服务未启用数据库，卷管理暂不可用。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "选择文件夹" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "添加卷" })).toBeDisabled();
  });
});
