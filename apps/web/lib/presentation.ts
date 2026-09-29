export const taskStatusLabels: Record<string, string> = {
  todo: "待处理",
  in_progress: "处理中",
  publishing: "待发布",
  published: "已发布",
  done: "已完成",
  cancelled: "已取消"
};

export const releaseStatusLabels: Record<string, string> = {
  draft: "草稿",
  pending_approval: "待审批",
  approved: "已审批",
  queued: "等待发布",
  running: "发布中",
  publishing: "发布中",
  success: "发布成功",
  failed: "发布失败",
  cancelled: "已取消"
};

export function formatDateTime(value?: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}

export function formatFullDateTime(value?: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}
