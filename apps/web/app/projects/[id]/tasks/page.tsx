import { TasksView } from "@/app/tasks/TasksView";
export default async function Page({ params }: { params: Promise<{ id: string }> }) { const { id } = await params; return <TasksView projectId={id} />; }
