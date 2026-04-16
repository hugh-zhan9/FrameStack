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
      if (url === "/api/system-status") {
        return new Response(JSON.stringify({ status: "ready", checks: [] }), {
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

  it("deletes a volume from the list", async () => {
    const confirmMock = vi.fn(() => true);
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      if (url === "/api/volumes" && method === "GET") {
        return new Response(
          JSON.stringify([
            { id: 7, display_name: "Media", mount_path: "/Volumes/media", is_online: true },
            { id: 8, display_name: "Archive", mount_path: "/Volumes/archive", is_online: false }
          ]),
          {
            status: 200,
            headers: { "Content-Type": "application/json" }
          }
        );
      }
      if (url === "/api/system-status") {
        return new Response(JSON.stringify({ status: "ready", checks: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" }
        });
      }
      if (url === "/api/volumes/7" && method === "DELETE") {
        return new Response(null, { status: 204 });
      }
      throw new Error(`unexpected url: ${method} ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("confirm", confirmMock);

    render(<VolumesPage />);

    expect(await screen.findByText("Media")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "删除卷" })[0]);

    await waitFor(() => expect(confirmMock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/volumes/7", { method: "DELETE" }));
    await waitFor(() => expect(screen.queryByText("Media")).not.toBeInTheDocument());
    expect(screen.getByText("卷已删除，磁盘原文件未受影响。")).toBeInTheDocument();
  });
});
