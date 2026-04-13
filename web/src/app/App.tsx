import { NavLink, Route, Routes } from "react-router-dom";
import { DashboardPage } from "../pages/DashboardPage";
import { VolumesPage } from "../pages/VolumesPage";
import { LibraryRoute } from "../features/library/LibraryRoute";
import { ReviewPage } from "../pages/ReviewPage";
import { TasksPage } from "../pages/TasksPage";

export default function App() {
  return (
    <div className="app-layout">
      <aside className="sidebar">
        <div className="brand-block">
          <h1>影栈</h1>
          <p>FrameStack</p>
        </div>
        <nav className="nav-list" aria-label="主导航">
          <NavItem to="/">总览</NavItem>
          <NavItem to="/volumes">存储卷</NavItem>
          <NavItem to="/library">资源库</NavItem>
          <NavItem to="/review">审核</NavItem>
          <NavItem to="/tasks">任务</NavItem>
        </nav>
      </aside>

      <main className="main-panel">
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/volumes" element={<VolumesPage />} />
          <Route path="/library" element={<LibraryRoute />} />
          <Route path="/review" element={<ReviewPage />} />
          <Route path="/tasks" element={<TasksPage />} />
        </Routes>
      </main>
    </div>
  );
}

function NavItem(props: { to: string; children: string }) {
  return (
    <NavLink
      to={props.to}
      end={props.to === "/"}
      className={({ isActive }) => (isActive ? "nav-item active" : "nav-item")}
    >
      {props.children}
    </NavLink>
  );
}
