import { useEffect, useState } from "react";
import { createVolume, deleteVolume, fetchSystemStatus, fetchVolumes, pickDirectory, scanVolume } from "../lib/api";
import type { SystemStatus, VolumeItem } from "../lib/contracts";

export function VolumesPage() {
  const [volumes, setVolumes] = useState<VolumeItem[]>([]);
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [displayName, setDisplayName] = useState("");
  const [mountPath, setMountPath] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [pendingAction, setPendingAction] = useState<{ type: "scan" | "delete"; volumeId: number } | null>(null);

  async function refresh() {
    setLoading(true);
    setError(null);
    try {
      const [items, status] = await Promise.all([fetchVolumes(), fetchSystemStatus()]);
      setVolumes(items);
      setSystemStatus(status);
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载卷失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (databaseDisabled) {
      setError("当前服务未启用数据库，卷管理暂不可用。");
      return;
    }
    setError(null);
    setNotice(null);
    try {
      await createVolume({ display_name: displayName, mount_path: mountPath });
      setDisplayName("");
      setMountPath("");
      await refresh();
      setNotice("卷已添加。");
    } catch (err) {
      setError(err instanceof Error ? err.message : "添加卷失败");
    }
  }

  async function handleScan(volumeId: number) {
    if (databaseDisabled) {
      setError("当前服务未启用数据库，无法触发扫描。");
      return;
    }
    if (pendingAction) {
      return;
    }
    setPendingAction({ type: "scan", volumeId });
    setError(null);
    setNotice(null);
    try {
      await scanVolume(volumeId);
      await refresh();
      setNotice("已提交卷扫描任务。");
    } catch (err) {
      setError(err instanceof Error ? err.message : "触发扫描失败");
    } finally {
      setPendingAction(null);
    }
  }

  async function handlePickDirectory() {
    if (databaseDisabled) {
      setError("当前服务未启用数据库，卷管理暂不可用。");
      return;
    }
    try {
      const path = await pickDirectory();
      setMountPath(path);
      setDisplayName((current) => current || path.split("/").filter(Boolean).at(-1) || "");
    } catch (err) {
      setError(err instanceof Error ? err.message : "选择目录失败");
    }
  }

  async function handleDelete(volume: VolumeItem) {
    if (databaseDisabled) {
      setError("当前服务未启用数据库，卷管理暂不可用。");
      return;
    }
    if (pendingAction) {
      return;
    }
    const confirmed = window.confirm(`删除卷“${volume.display_name}”后，会移除该卷的索引记录，但不会删除磁盘上的原文件。是否继续？`);
    if (!confirmed) {
      return;
    }
    setPendingAction({ type: "delete", volumeId: volume.id });
    setError(null);
    setNotice(null);
    try {
      await deleteVolume(volume.id);
      setVolumes((current) => current.filter((item) => item.id !== volume.id));
      setNotice("卷已删除，磁盘原文件未受影响。");
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除卷失败");
    } finally {
      setPendingAction(null);
    }
  }

  const databaseDisabled = systemStatus?.checks?.some((check) => check.name === "database" && check.status === "disabled");

  return (
    <section className="page-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">Volumes</p>
          <h2>存储卷</h2>
          <p className="page-subtitle">录入、查看和扫描本地卷。</p>
        </div>
      </header>

      <form className="volume-form-react" onSubmit={handleSubmit}>
        <input
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          placeholder="显示名称"
          disabled={databaseDisabled}
        />
        <input
          value={mountPath}
          onChange={(e) => setMountPath(e.target.value)}
          placeholder="/Volumes/media"
          disabled={databaseDisabled}
        />
        <button type="button" onClick={() => void handlePickDirectory()} disabled={databaseDisabled}>
          选择文件夹
        </button>
        <button type="submit" disabled={databaseDisabled}>
          添加卷
        </button>
      </form>

      {databaseDisabled ? <div className="empty-state">当前服务未启用数据库，卷管理暂不可用。</div> : null}
      {error ? <div className="empty-state">{error}</div> : null}
      {notice ? <div className="empty-state">{notice}</div> : null}
      {loading ? <div className="empty-state">正在加载卷…</div> : null}

      <div className="summary-list">
        {volumes.map((volume) => (
          <article key={volume.id} className="summary-item">
            <strong>{volume.display_name}</strong>
            <span>{volume.mount_path}</span>
            <small>{volume.is_online ? "在线" : "离线"}</small>
            <button
              type="button"
              onClick={() => void handleScan(volume.id)}
              disabled={databaseDisabled || pendingAction !== null}
            >
              {pendingAction?.type === "scan" && pendingAction.volumeId === volume.id ? "扫描中…" : "扫描"}
            </button>
            <button
              type="button"
              className="secondary-button"
              onClick={() => void handleDelete(volume)}
              disabled={databaseDisabled || pendingAction !== null}
            >
              {pendingAction?.type === "delete" && pendingAction.volumeId === volume.id ? "删除中…" : "删除卷"}
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}
