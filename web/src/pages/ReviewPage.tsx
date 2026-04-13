import { useEffect, useState } from "react";
import { fetchClusterDetail, fetchClusters } from "../lib/api";
import type { ClusterDetail, ClusterItem } from "../lib/contracts";

export function ReviewPage() {
  const [clusterType, setClusterType] = useState("same_content");
  const [clusters, setClusters] = useState<ClusterItem[]>([]);
  const [selectedClusterId, setSelectedClusterId] = useState<number | null>(null);
  const [detail, setDetail] = useState<ClusterDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    fetchClusters(clusterType)
      .then((items) => {
        if (cancelled) {
          return;
        }
        setClusters(items);
        setSelectedClusterId(items[0]?.id ?? null);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "加载聚类失败");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [clusterType]);

  useEffect(() => {
    if (selectedClusterId == null) {
      setDetail(null);
      return;
    }
    let cancelled = false;
    fetchClusterDetail(selectedClusterId)
      .then((item) => {
        if (!cancelled) {
          setDetail(item);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "加载聚类详情失败");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selectedClusterId]);

  return (
    <section className="page-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">Review</p>
          <h2>审核</h2>
          <p className="page-subtitle">聚类候选审核和连续处理入口。</p>
        </div>
      </header>

      <div className="review-layout">
        <aside className="review-list">
          <select value={clusterType} onChange={(e) => setClusterType(e.target.value)} className="library-search">
            <option value="same_content">same_content</option>
            <option value="same_series">same_series</option>
            <option value="same_person">same_person</option>
          </select>
          {clusters.map((cluster) => (
            <button
              key={cluster.id}
              type="button"
              className={cluster.id === selectedClusterId ? "review-item active" : "review-item"}
              onClick={() => setSelectedClusterId(cluster.id)}
            >
              <strong>{cluster.title}</strong>
              <span>{cluster.member_count} 个成员</span>
              {cluster.cluster_type === "same_person" ? (
                <small>
                  person visual {cluster.person_visual_count ?? 0} · top score{" "}
                  {cluster.top_member_score?.toFixed(2) ?? "-"}
                </small>
              ) : null}
            </button>
          ))}
        </aside>

        <section className="detail-panel">
          <h3>聚类详情</h3>
          {error ? <p>{error}</p> : null}
          {detail ? (
            <div className="detail-stack">
              <strong>{detail.title}</strong>
              <span>
                {detail.cluster_type} / {detail.status}
              </span>
              <div className="detail-grid">
                <DetailStat label="成员数" value={`${detail.member_count}`} />
                <DetailStat label="强候选" value={`${detail.strong_member_count ?? 0}`} />
                <DetailStat label="person visual" value={`${detail.person_visual_count ?? 0}`} />
                <DetailStat label="top score" value={detail.top_member_score?.toFixed(2) ?? "-"} />
              </div>
              <div className="detail-section">
                <h4>成员</h4>
                <div className="review-member-grid">
                  {detail.members.map((member) => (
                    <article key={member.file_id} className="review-member-card">
                      <strong>{member.file_name}</strong>
                      <span>
                        {member.media_type} / {member.role}
                      </span>
                      <span>{member.quality_tier || "unknown"}</span>
                      <span>{member.score != null ? `score ${member.score.toFixed(2)}` : "score -"}</span>
                      <span>
                        {(member.embedding_provider || "unknown") + " / " + (member.embedding_model || "unknown")}
                      </span>
                    </article>
                  ))}
                </div>
              </div>
            </div>
          ) : (
            <p>暂无聚类详情。</p>
          )}
        </section>
      </div>
    </section>
  );
}

function DetailStat(props: { label: string; value: string }) {
  return (
    <div className="detail-stat">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
