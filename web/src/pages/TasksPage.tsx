import { useEffect, useState } from "react";
import { fetchJobEvents, fetchJobs } from "../lib/api";
import type { JobEvent, JobItem } from "../lib/contracts";

export function TasksPage() {
  const [jobs, setJobs] = useState<JobItem[]>([]);
  const [selectedJobId, setSelectedJobId] = useState<number | null>(null);
  const [events, setEvents] = useState<JobEvent[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchJobs()
      .then((items) => {
        if (cancelled) {
          return;
        }
        setJobs(items);
        setSelectedJobId(items[0]?.id ?? null);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "加载任务失败");
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (selectedJobId == null) {
      setEvents([]);
      return;
    }
    let cancelled = false;
    fetchJobEvents(selectedJobId)
      .then((items) => {
        if (!cancelled) {
          setEvents(items);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "加载任务事件失败");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selectedJobId]);

  return (
    <section className="page-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">Tasks</p>
          <h2>任务</h2>
          <p className="page-subtitle">查看最近任务和执行事件。</p>
        </div>
      </header>

      <div className="review-layout">
        <aside className="review-list">
          {jobs.map((job) => (
            <button
              key={job.id}
              type="button"
              className={job.id === selectedJobId ? "review-item active" : "review-item"}
              onClick={() => setSelectedJobId(job.id)}
            >
              <strong>{job.job_type}</strong>
              <span>{job.status}</span>
              <small>
                {job.target_type || "-"} / {job.target_id ?? "-"}
              </small>
            </button>
          ))}
        </aside>
        <section className="detail-panel">
          <h3>任务事件</h3>
          {error ? <p>{error}</p> : null}
          <ul className="detail-list">
            {events.map((event) => (
              <li key={event.id}>
                <strong>{event.level}</strong>
                <span>{event.message}</span>
                <small>{event.created_at}</small>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </section>
  );
}
