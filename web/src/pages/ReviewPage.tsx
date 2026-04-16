import { useEffect, useMemo, useState } from "react";
import { applyClusterReviewAction, applyFileReviewAction, fetchClusterDetail, fetchClusters, updateClusterStatus } from "../lib/api";
import type { ClusterDetail, ClusterItem, ClusterMember } from "../lib/contracts";

const CLUSTER_TYPE_LABELS: Record<string, string> = {
  same_content: "同内容聚类",
  same_series: "同系列聚类",
  same_person: "同人物聚类"
};

const CLUSTER_STATUS_LABELS: Record<string, string> = {
  candidate: "候选中",
  confirmed: "已确认",
  ignored: "已忽略"
};

export function ReviewPage() {
  const [clusterType, setClusterType] = useState("same_content");
  const [clusters, setClusters] = useState<ClusterItem[]>([]);
  const [selectedClusterId, setSelectedClusterId] = useState<number | null>(null);
  const [detail, setDetail] = useState<ClusterDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [actionPending, setActionPending] = useState(false);
  const [queueOpen, setQueueOpen] = useState(false);
  const [selectedKeepFileId, setSelectedKeepFileId] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    setNotice(null);
    fetchClusters(clusterType, "candidate")
      .then((items) => {
        if (cancelled) {
          return;
        }
        const nextClusters = Array.isArray(items) ? items : [];
        setClusters(nextClusters);
        setSelectedClusterId((current) => {
          if (current && nextClusters.some((cluster) => cluster.id === current)) {
            return current;
          }
          return nextClusters[0]?.id ?? null;
        });
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
          setSelectedKeepFileId(selectInitialKeepFileId(item));
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

  const selectedIndex = selectedClusterId == null ? -1 : clusters.findIndex((cluster) => cluster.id === selectedClusterId);
  const hasPrevious = selectedIndex > 0;
  const hasNext = selectedIndex >= 0 && selectedIndex < clusters.length - 1;

  const summaryItems = useMemo(() => {
    if (!detail) {
      return [];
    }
    return [
      { label: "成员数", value: `${detail.member_count}` },
      { label: "强候选", value: `${detail.strong_member_count ?? 0}` },
      { label: "人物向量", value: `${detail.person_visual_count ?? 0}` },
      { label: "最高得分", value: detail.top_member_score?.toFixed(2) ?? "-" }
    ];
  }, [detail]);

  const isSameContent = detail?.cluster_type === "same_content";
  async function runClusterAction(action: "confirm" | "ignore" | "keep" | "favorite" | "trash_candidate" | "next" | "previous") {
    if (selectedClusterId == null || actionPending) {
      return;
    }

    setActionPending(true);
    setError(null);
    try {
      if (action === "confirm") {
        await updateClusterStatus(selectedClusterId, "confirmed");
        setNotice("已确认当前聚类，已切换到下一组。");
        resolveCurrentCluster();
        return;
      }
      if (action === "ignore") {
        await updateClusterStatus(selectedClusterId, "ignored");
        setNotice("已忽略当前聚类，已切换到下一组。");
        resolveCurrentCluster();
        return;
      }
      if (action === "next") {
        moveToAdjacentCluster(1);
        return;
      }
      if (action === "previous") {
        moveToAdjacentCluster(-1);
        return;
      }
      if (action === "keep" && isSameContent && detail) {
        const members = Array.isArray(detail.members) ? detail.members : [];
        const selectedMember =
          members.find((member) => member.file_id === selectedKeepFileId) ??
          members.find((member) => member.role === "best_quality") ??
          members[0] ??
          null;
        if (!selectedMember) {
          setNotice("当前分组没有可处理的成员。");
          return;
        }
        await applyFileReviewAction(selectedMember.file_id, { action_type: "keep" });
        const cleanupTargets = members.filter((member) => member.file_id !== selectedMember.file_id);
        for (const member of cleanupTargets) {
          await applyFileReviewAction(member.file_id, { action_type: "trash_candidate" });
        }
        await updateClusterStatus(selectedClusterId, "confirmed");
        setNotice("已保留当前选择，并确认当前分组，已切换到下一组。");
        resolveCurrentCluster();
        return;
      }

      await applyClusterReviewAction(selectedClusterId, { action_type: action });
      setNotice(noticeForAction(action));
      const refreshed = await fetchClusterDetail(selectedClusterId);
      setDetail(refreshed);
    } catch (err) {
      setError(err instanceof Error ? err.message : "聚类审核操作失败");
    } finally {
      setActionPending(false);
    }
  }

  function moveToAdjacentCluster(direction: -1 | 1) {
    if (selectedIndex < 0) {
      return;
    }
    const next = clusters[selectedIndex + direction];
    if (next) {
      setSelectedClusterId(next.id);
    }
  }

  function resolveCurrentCluster() {
    if (selectedClusterId == null) {
      return;
    }
    setClusters((current) => {
      const index = current.findIndex((cluster) => cluster.id === selectedClusterId);
      if (index < 0) {
        return current;
      }
      const nextClusters = current.filter((cluster) => cluster.id !== selectedClusterId);
      const nextSelection = nextClusters[index] ?? nextClusters[index - 1] ?? null;
      setSelectedClusterId(nextSelection?.id ?? null);
      setDetail(null);
      setSelectedKeepFileId(null);
      return nextClusters;
    });
  }

  function selectCluster(clusterId: number) {
    setSelectedClusterId(clusterId);
    setQueueOpen(false);
    setNotice(null);
    setError(null);
    setSelectedKeepFileId(null);
  }

  return (
    <section className="page-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">Review</p>
          <h2>审核</h2>
          <p className="page-subtitle">聚类详情优先的审核工作台。</p>
        </div>
      </header>

      <div className="review-workbench">
        <section className="review-focus-shell">
          <div className="review-focus-header">
            <div className="review-focus-title">
              <p className="eyebrow">Focus</p>
              <h3>审核主视图</h3>
              <p className="page-subtitle">
                {detail
                  ? `${formatClusterType(detail.cluster_type)} / ${formatClusterStatus(detail.status)}`
                  : "选择一个候选聚类开始审核"}
              </p>
            </div>
            <div className="review-toolbar">
              <select value={clusterType} onChange={(e) => setClusterType(e.target.value)} className="library-search">
                <option value="same_content">同内容聚类</option>
                <option value="same_series">同系列聚类</option>
                <option value="same_person">同人物聚类</option>
              </select>
              <button
                type="button"
                className="secondary-button"
                onClick={() => setQueueOpen((current) => !current)}
              >
                候选队列（{clusters.length}）
              </button>
            </div>
          </div>

          {queueOpen ? (
            <aside className="review-queue-drawer">
              <div className="review-queue-header">
                <strong>候选队列</strong>
                <span>{clusters.length} 组待查</span>
              </div>
              <div className="review-queue-list">
                {clusters.map((cluster, index) => (
                  <button
                    key={cluster.id}
                    type="button"
                    className={cluster.id === selectedClusterId ? "review-item active" : "review-item"}
                    onClick={() => selectCluster(cluster.id)}
                  >
                    <strong>{cluster.title}</strong>
                    <span>
                      第 {index + 1} 组 / {cluster.member_count} 个成员
                    </span>
                    {cluster.cluster_type === "same_person" ? (
                      <small>
                        人物向量 {cluster.person_visual_count ?? 0} · 最高得分{" "}
                        {cluster.top_member_score?.toFixed(2) ?? "-"}
                      </small>
                    ) : null}
                  </button>
                ))}
              </div>
            </aside>
          ) : null}

          {error ? <div className="detail-notice detail-notice-error">{error}</div> : null}
          {notice ? <div className="detail-notice">{notice}</div> : null}

          {detail ? (
            <div className="review-detail-workbench">
              <section className="review-stage">
                <div className="review-stage-header">
                  <div className="review-stage-title">
                    <strong>{detail.title}</strong>
                    <span className="detail-queue-position">当前第 {selectedIndex + 1} 组 / 共 {clusters.length} 组</span>
                  </div>
                </div>

                <div className="review-member-grid review-member-grid-hero">
                  {(Array.isArray(detail.members) ? detail.members : []).map((member) => (
                    <MemberCard
                      key={member.file_id}
                      member={member}
                      selectable={isSameContent}
                      selected={isSameContent && member.file_id === selectedKeepFileId}
                      onSelect={
                        isSameContent
                          ? () => {
                              setSelectedKeepFileId(member.file_id);
                              setNotice(null);
                            }
                          : undefined
                      }
                    />
                  ))}
                </div>
              </section>

              <aside className="review-sidecar">
                <div className="review-sidecar-card">
                  <h4>聚类摘要</h4>
                  <div className="detail-grid">
                    {summaryItems.map((item) => (
                      <DetailStat key={item.label} label={item.label} value={item.value} />
                    ))}
                  </div>
                </div>

                <div className="review-sidecar-card">
                  <h4>分组判断</h4>
                  <div className="detail-actions review-actions review-actions-compact">
                    <button type="button" className="primary-button" onClick={() => runClusterAction("confirm")} disabled={actionPending}>
                      确认分组
                    </button>
                    <button type="button" className="secondary-button" onClick={() => runClusterAction("ignore")} disabled={actionPending}>
                      否决分组
                    </button>
                  </div>
                </div>

                {isSameContent ? (
                  <div className="review-sidecar-card">
                    <h4>同内容处置</h4>
                  <div className="detail-actions review-actions review-actions-compact">
                    <button type="button" className="secondary-button" onClick={() => runClusterAction("keep")} disabled={actionPending}>
                        保留当前选择，其余标记清理候选
                    </button>
                  </div>
                </div>
                ) : null}

                <div className="review-sidecar-card">
                  <h4>队列导航</h4>
                  <div className="detail-actions review-actions review-actions-compact">
                    <button type="button" className="secondary-button" onClick={() => runClusterAction("previous")} disabled={actionPending || !hasPrevious}>
                      上一组
                    </button>
                    <button type="button" className="secondary-button" onClick={() => runClusterAction("next")} disabled={actionPending || !hasNext}>
                      下一组
                    </button>
                  </div>
                </div>
              </aside>
            </div>
          ) : (
            <div className="empty-state">暂无聚类详情。</div>
          )}
        </section>
      </div>
    </section>
  );
}

function selectInitialKeepFileId(detail: ClusterDetail, preferredFileId?: number | null) {
  const members = Array.isArray(detail.members) ? detail.members : [];
  if (preferredFileId && members.some((member) => member.file_id === preferredFileId)) {
    return preferredFileId;
  }
  return members.find((member) => member.role === "best_quality")?.file_id ?? members[0]?.file_id ?? null;
}

function MemberCard(props: { member: ClusterMember; selectable?: boolean; selected?: boolean; onSelect?: () => void }) {
  const { member, selectable = false, selected = false, onSelect } = props;
  const roleLabel = labelForMemberRole(member.role);
  const recommended = selectable ? selected : member.role === "best_quality";
  const semanticLabel = recommended ? "推荐保留" : roleLabel;
  return (
    <article
      className={recommended ? "review-member-card review-member-card-hero review-member-card-recommended" : "review-member-card review-member-card-hero"}
      onClick={selectable ? onSelect : undefined}
      role={selectable ? "button" : undefined}
      tabIndex={selectable ? 0 : undefined}
      onKeyDown={
        selectable
          ? (event) => {
              if ((event.key === "Enter" || event.key === " ") && onSelect) {
                event.preventDefault();
                onSelect();
              }
            }
          : undefined
      }
      aria-label={selectable ? `${member.file_name}-preview` : undefined}
    >
      {recommended ? <span className="review-member-badge">推荐保留</span> : null}
      <img
        src={`/api/files/${member.file_id}/preview`}
        alt={`${member.file_name}-preview`}
        className="review-member-preview"
      />
      <strong>{member.file_name}</strong>
      <span>
        {member.media_type} / {semanticLabel}
      </span>
      <span>{member.quality_tier || "unknown"}</span>
      <span>{member.score != null ? `score ${member.score.toFixed(2)}` : "score -"}</span>
      <span>
        {(member.embedding_provider || "unknown") + " / " + (member.embedding_model || "unknown")}
      </span>
    </article>
  );
}

function labelForMemberRole(role: string) {
  switch (role) {
    case "best_quality":
      return "推荐保留";
    case "duplicate_candidate":
      return "重复候选";
    case "series_focus":
      return "审核焦点";
    default:
      return role;
  }
}

function formatClusterType(value?: string) {
  const normalized = value?.trim();
  if (!normalized) {
    return "未识别聚类";
  }
  return CLUSTER_TYPE_LABELS[normalized] ?? normalized;
}

function formatClusterStatus(value?: string) {
  const normalized = value?.trim();
  if (!normalized) {
    return "未知状态";
  }
  return CLUSTER_STATUS_LABELS[normalized] ?? normalized;
}

function noticeForAction(action: "keep" | "favorite" | "trash_candidate") {
  switch (action) {
    case "keep":
      return "已将当前聚类标记为保留。";
    case "favorite":
      return "已将当前聚类标记为收藏。";
    case "trash_candidate":
      return "已将当前聚类标记为待删候选。";
  }
}

function DetailStat(props: { label: string; value: string }) {
  return (
    <div className="detail-stat">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
