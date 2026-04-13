import { useEffect, useState } from "react";
import { createVolume, fetchSystemStatus, fetchVolumes, pickDirectory, scanVolume } from "../lib/api";
import type { SystemStatus, VolumeItem } from "../lib/contracts";

export function VolumesPage() {
  const [volumes, setVolumes] = useState<VolumeItem[]>([]);
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [displayName, setDisplayName] = useState("");
  const [mountPath, setMountPath] = useState("");
  const [error, setError] = useState<string | null>(null);

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
    try {
      await createVolume({ display_name: displayName, mount_path: mountPath });
      setDisplayName("");
      setMountPath("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "添加卷失败");
    }
  }

  async function handleScan(volumeId: number) {
    if (databaseDisabled) {
      setError("当前服务未启用数据库，无法触发扫描。");
      return;
    }
    try {
      await scanVolume(volumeId);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "触发扫描失败");
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
      {loading ? <div className="empty-state">正在加载卷…</div> : null}

      <div className="summary-list">
        {volumes.map((volume) => (
          <article key={volume.id} className="summary-item">
            <strong>{volume.display_name}</strong>
            <span>{volume.mount_path}</span>
            <small>{volume.is_online ? "在线" : "离线"}</small>
            <button type="button" onClick={() => void handleScan(volume.id)} disabled={databaseDisabled}>
              扫描
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}
