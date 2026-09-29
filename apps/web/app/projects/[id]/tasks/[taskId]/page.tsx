import { TaskDetailView } from "@/app/tasks/[id]/TaskDetailView";

export default async function Page({ params }: { params: Promise<{ id: string; taskId: string }> }) {
  const { id, taskId } = await params;
  return <TaskDetailView projectId={id} taskId={taskId} />;
}
