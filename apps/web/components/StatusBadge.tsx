import { Badge, type BadgeProps } from "@/components/ui/badge";
import { releaseStatusLabels, taskStatusLabels } from "@/lib/presentation";

function variant(status: string): BadgeProps["variant"] {
  if (["success", "published", "done"].includes(status)) return "success";
  if (["failed", "cancelled"].includes(status)) return "danger";
  if (["pending_approval", "skipped"].includes(status)) return "warning";
  if (["in_progress", "publishing", "approved", "queued", "running"].includes(status)) return "info";
  return "secondary";
}

export function TaskStatusBadge({ status, label }: { status: string; label?: string }) {
  return <Badge variant={variant(status)}>{label || taskStatusLabels[status] || status}</Badge>;
}

export function ReleaseStatusBadge({ status, label }: { status: string; label?: string }) {
  return <Badge variant={variant(status)}>{label || releaseStatusLabels[status] || status}</Badge>;
}
