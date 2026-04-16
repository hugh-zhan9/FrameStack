import type { AnalysisItem, FileCluster, PathHistory, ReviewAction } from "../../lib/contracts";
import type { FileItem } from "./types";

export const QUALITY_TIER_NOTE = "规格等级按分辨率、码率、帧率等技术指标粗分，不代表审美质量。";

const MEDIA_TYPE_LABELS: Record<string, string> = {
  image: "图片",
  video: "视频"
};

const QUALITY_TIER_LABELS: Record<string, string> = {
  high: "高规格",
  medium: "中规格",
  low: "基础规格"
};

const FILE_STATUS_LABELS: Record<string, string> = {
  active: "在库",
  missing: "源文件缺失",
  ignored: "已忽略",
  trashed: "已移入废纸篓"
};

const REVIEW_ACTION_LABELS: Record<string, string> = {
  keep: "保留",
  trash_candidate: "待清理",
  ignore: "忽略",
  favorite: "收藏",
  hide: "隐藏",
  deleted_to_trash: "已移入废纸篓"
};

const CLUSTER_TYPE_LABELS: Record<string, string> = {
  same_content: "同内容",
  same_person: "同人物",
  same_series: "同系列"
};

const CLUSTER_STATUS_LABELS: Record<string, string> = {
  candidate: "候选中",
  confirmed: "已确认",
  ignored: "已忽略"
};

const ANALYSIS_TYPE_LABELS: Record<string, string> = {
  quality: "质量分析",
  caption: "内容描述",
  embedding: "向量分析",
  understand: "内容理解"
};

const PATH_EVENT_LABELS: Record<string, string> = {
  discovered: "已发现",
  moved: "已移动",
  renamed: "已重命名",
  missing: "扫描缺失"
};

export function formatMediaType(value?: string) {
  return formatLabeledValue(value, MEDIA_TYPE_LABELS, "未识别类型");
}

export function formatQualityTier(value?: string) {
  return formatLabeledValue(value, QUALITY_TIER_LABELS, "未分级");
}

export function formatFileStatus(value?: string) {
  return formatLabeledValue(value, FILE_STATUS_LABELS, "未标记状态");
}

export function formatReviewAction(value?: string) {
  return formatLabeledValue(value, REVIEW_ACTION_LABELS, "未处理");
}

export function formatClusterType(value?: string) {
  return formatLabeledValue(value, CLUSTER_TYPE_LABELS, "未识别聚类");
}

export function formatClusterStatus(value?: string) {
  return formatLabeledValue(value, CLUSTER_STATUS_LABELS, "未知状态");
}

export function formatAnalysisType(value?: string) {
  return formatLabeledValue(value, ANALYSIS_TYPE_LABELS, "未识别分析");
}

export function formatPathEvent(value?: string) {
  return formatLabeledValue(value, PATH_EVENT_LABELS, "未识别事件");
}

export function formatQualityScore(score?: number) {
  if (score == null) {
    return "-";
  }
  return `${Math.round(score)} / 100`;
}

export function formatUnknown(value?: string) {
  const normalized = value?.trim();
  return normalized ? normalized : "未识别";
}

export function formatAnalysisSummary(item: AnalysisItem) {
  if (item.analysis_type === "quality") {
    const pieces = [formatQualityTier(item.quality_tier)];
    if (item.quality_score != null) {
      pieces.push(`技术评分 ${formatQualityScore(item.quality_score)}`);
    }
    return pieces.join(" · ");
  }
  return item.summary?.trim() || "无摘要";
}

export function formatClusterLabel(cluster: FileCluster) {
  return cluster.title || formatClusterType(cluster.cluster_type);
}

export function formatClusterMeta(cluster: FileCluster) {
  return `${formatClusterType(cluster.cluster_type)} / ${formatClusterStatus(cluster.status)}`;
}

export function formatReviewActionSummary(action: ReviewAction) {
  return action.note?.trim() || "无备注";
}

export function formatPathHistorySummary(entry: PathHistory) {
  return entry.abs_path?.trim() || "无路径记录";
}

export function getFileMetaBadges(file: Pick<FileItem, "media_type" | "quality_tier" | "status" | "review_action">) {
  const badges = [formatMediaType(file.media_type), formatQualityTier(file.quality_tier), formatFileStatus(file.status)];
  if (file.review_action) {
    badges.push(formatReviewAction(file.review_action));
  }
  return badges;
}

function formatLabeledValue(value: string | undefined, labels: Record<string, string>, fallback: string) {
  const normalized = value?.trim();
  if (!normalized) {
    return fallback;
  }
  return labels[normalized] ?? normalized;
}
