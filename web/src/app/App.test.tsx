import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import App from "./App";

vi.mock("../pages/DashboardPage", () => ({
  DashboardPage: () => <div>dashboard stub</div>
}));
vi.mock("../pages/VolumesPage", () => ({
  VolumesPage: () => <div>volumes stub</div>
}));
vi.mock("../features/library/LibraryRoute", () => ({
  LibraryRoute: () => <div>library stub</div>
}));
vi.mock("../pages/ReviewPage", () => ({
  ReviewPage: () => <div>review stub</div>
}));
vi.mock("../pages/TasksPage", () => ({
  TasksPage: () => <div>tasks stub</div>
}));

describe("App", () => {
  it("renders the responsive workstation shell with Chinese navigation", () => {
    render(
      <MemoryRouter initialEntries={["/library"]}>
        <App />
      </MemoryRouter>
    );

    expect(screen.getByRole("heading", { name: "影栈" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "总览" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "资源库" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "审核" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "任务" })).toBeInTheDocument();
  });
});
