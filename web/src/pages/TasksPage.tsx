import { useEffect, useState } from "react";
import { fetchAIPromptSettings, fetchFileDetail, fetchJobEvents, fetchJobs, updateAIPromptSettings } from "../lib/api";
import type { AIPromptSettings, FileDetail } from "../lib/contracts";
import type { JobEvent, JobItem } from "../lib/contracts";

export function TasksPage() {
  const [jobs, setJobs] = useState<JobItem[]>([]);
  const [selectedJobId, setSelectedJobId] = useState<number | null>(null);
  const [events, setEvents] = useState<JobEvent[]>([]);
  const [jobFiles, setJobFiles] = useState<Record<number, FileDetail>>({});
  const [promptSettings, setPromptSettings] = useState<AIPromptSettings>({ understanding_extra_prompt: "" });
  const [promptPending, setPromptPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchJobs()
      .then((items) => {
        if (cancelled) {
          return;
        }
        const nextJobs = Array.isArray(items) ? items : [];
        setJobs(nextJobs);
        setSelectedJobId(nextJobs[0]?.id ?? null);
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
    let cancelled = false;
    fetchAIPromptSettings()
      .then((item) => {
        if (!cancelled) {
          setPromptSettings(item);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "加载提示词设置失败");
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const fileIDs = Array.from(
      new Set(
        jobs
          .filter((job) => job.target_type === "file" && typeof job.target_id === "number")
          .map((job) => job.target_id as number)
      )
    ).filter((fileID) => jobFiles[fileID] == null);
    if (fileIDs.length === 0) {
      return;
    }

    let cancelled = false;
    Promise.all(
      fileIDs.map(async (fileID) => {
        try {
          const detail = await fetchFileDetail(fileID);
          return [fileID, detail] as const;
        } catch {
          return null;
        }
      })
    ).then((items) => {
      if (cancelled) {
        return;
      }
      const nextEntries = items.filter((item): item is readonly [number, FileDetail] => item !== null);
      if (nextEntries.length === 0) {
        return;
      }
      setJobFiles((current) => {
        const next = { ...current };
        for (const [fileID, detail] of nextEntries) {
          next[fileID] = detail;
        }
        return next;
      });
    });

    return () => {
      cancelled = true;
    };
  }, [jobs, jobFiles]);

  useEffect(() => {
    if (selectedJobId == null) {
      setEvents([]);
      return;
    }
    let cancelled = false;
    fetchJobEvents(selectedJobId)
      .then((items) => {
        if (!cancelled) {
          setEvents(Array.isArray(items) ? items : []);
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

  const selectedJob = jobs.find((job) => job.id === selectedJobId) ?? null;
  const selectedFile = selectedJob?.target_type === "file" && selectedJob.target_id ? jobFiles[selectedJob.target_id] : null;

  async function handleSavePromptSettings() {
    setPromptPending(true);
    setError(null);
    try {
      const next = await updateAIPromptSettings(promptSettings);
      setPromptSettings(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存提示词设置失败");
    } finally {
      setPromptPending(false);
    }
  }

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
              className={job.id === selectedJobId ? "review-item task-job-card active" : "review-item task-job-card"}
              onClick={() => setSelectedJobId(job.id)}
              aria-label={`${formatJobType(job.job_type)} ${job.target_type === "file" && job.target_id ? (jobFiles[job.target_id]?.file_name ?? `文件 ${job.target_id}`) : formatJobTarget(job)}`}
            >
              {job.target_type === "file" && job.target_id ? (
                <img
                  src={`/api/files/${job.target_id}/preview`}
                  alt={`${jobFiles[job.target_id]?.file_name ?? `file-${job.target_id}`} 预览`}
                  loading="lazy"
                  decoding="async"
                  className="review-member-preview task-job-preview"
                />
              ) : null}
              <strong>{formatJobType(job.job_type)}</strong>
              <span>{formatJobStatus(job.status)}</span>
              <small>{job.target_type === "file" && job.target_id ? (jobFiles[job.target_id]?.file_name ?? `文件 #${job.target_id}`) : formatJobTarget(job)}</small>
            </button>
          ))}
        </aside>
        <section className="detail-panel">
          <h3>任务事件</h3>
          {error ? <p>{error}</p> : null}
          <div className="detail-section">
            <h4>AI 提示词</h4>
            <textarea
              value={promptSettings.understanding_extra_prompt}
              onChange={(event) =>
                setPromptSettings((current) => ({
                  ...current,
                  understanding_extra_prompt: event.target.value
                }))
              }
              aria-label="理解提示词附加规则"
              className="task-prompt-textarea"
              placeholder="例如：优先输出更明确的中文行为标签，不要使用模糊敏感标签。"
            />
            <button type="button" className="secondary-button" onClick={handleSavePromptSettings} disabled={promptPending}>
              {promptPending ? "保存中…" : "保存提示词规则"}
            </button>
          </div>
          {selectedJob ? (
            <div className="detail-notice task-events-header">
              <strong>{formatJobType(selectedJob.job_type)}</strong>
              <span>{formatJobStatus(selectedJob.status)}</span>
              <small>{selectedFile ? `${selectedFile.file_name} · 文件 #${selectedFile.id}` : formatJobTarget(selectedJob)}</small>
            </div>
          ) : null}
          {events.length ? (
            <ul className="detail-list">
              {events.map((event) => (
                <li key={event.id}>
                  <strong>{formatEventLevel(event.level)}</strong>
                  <span>{event.message}</span>
                  <small>{event.created_at}</small>
                </li>
              ))}
            </ul>
          ) : (
            <p>当前任务还没有事件记录。</p>
          )}
        </section>
      </div>
    </section>
  );
}

function formatJobType(jobType: string) {
  const mapping: Record<string, string> = {
    scan_volume: "扫描存储卷",
    extract_image_features: "提取图片特征",
    extract_video_features: "提取视频特征",
    hash_file: "计算文件哈希",
    recompute_search_doc: "重建搜索文档",
    infer_tags: "AI 打标签",
    infer_quality: "质量分析",
    embed_image: "图片向量",
    embed_video_frames: "视频向量",
    embed_person_image: "人物向量",
    embed_person_video_frames: "人物视频向量",
    cluster_same_content: "同内容聚类",
    cluster_same_series: "同系列聚类",
    cluster_same_person: "同人聚类",
  };
  return mapping[jobType] ?? jobType;
}

function formatJobStatus(status: string) {
  const mapping: Record<string, string> = {
    pending: "排队中",
    leased: "已领取",
    running: "执行中",
    succeeded: "已完成",
    failed: "失败",
    dead: "已终止",
  };
  return mapping[status] ?? status;
}

function formatJobTarget(job: JobItem) {
  if (job.target_type === "file") {
    return `文件 #${job.target_id ?? "-"}`;
  }
  if (job.target_type === "volume") {
    return `存储卷 #${job.target_id ?? "-"}`;
  }
  return `${job.target_type || "未知目标"} / ${job.target_id ?? "-"}`;
}

function formatEventLevel(level: string) {
  const mapping: Record<string, string> = {
    info: "信息",
    warning: "警告",
    error: "错误",
  };
  return mapping[level] ?? level;
}
