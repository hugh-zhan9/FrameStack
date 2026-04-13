import { fetchClusterSummary, fetchSystemStatus, fetchTaskSummary } from "../lib/api";
import { useAsync } from "../lib/useAsync";

export function DashboardPage() {
  const system = useAsync(fetchSystemStatus, []);
  const tasks = useAsync(fetchTaskSummary, []);
  const clusters = useAsync(fetchClusterSummary, []);

  return (
    <section className="page-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">Dashboard</p>
          <h2>总览</h2>
          <p className="page-subtitle">系统状态、后台任务和聚类候选总览。</p>
        </div>
      </header>

      <div className="stats-grid-react">
        <SummaryCard
          title="系统状态"
          value={system.data?.status ?? (system.loading ? "加载中" : "未知")}
          detail={system.error ?? summarizeChecks(system.data?.checks ?? [])}
        />
        <SummaryCard
          title="运行中任务"
          value={String(tasks.data?.running ?? 0)}
          detail={`pending ${tasks.data?.pending ?? 0} / failed ${tasks.data?.failed ?? 0}`}
        />
        <SummaryCard
          title="候选聚类"
          value={String((clusters.data ?? []).reduce((sum, item) => sum + item.cluster_count, 0))}
          detail="same_content / same_series / same_person"
        />
      </div>

      <div className="summary-section">
        <h3>候选摘要</h3>
        <div className="summary-list">
          {(clusters.data ?? []).map((item) => (
            <article key={`${item.cluster_type}:${item.status}`} className="summary-item">
              <strong>{item.cluster_type}</strong>
              <span>{item.cluster_count} 组</span>
              <small>{item.member_count} 个成员</small>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function SummaryCard(props: { title: string; value: string; detail: string }) {
  return (
    <article className="summary-card">
      <span>{props.title}</span>
      <strong>{props.value}</strong>
      <small>{props.detail}</small>
    </article>
  );
}

function summarizeChecks(checks: { name: string; status: string }[]) {
  if (checks.length === 0) {
    return "暂无依赖状态";
  }
  return checks.map((check) => `${check.name}:${check.status}`).join(" · ");
}
