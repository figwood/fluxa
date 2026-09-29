import { WorkflowsView } from "@/app/workflows/WorkflowsView";
export default async function Page({ params }: { params: Promise<{ id: string }> }) { const { id } = await params; return <WorkflowsView projectId={id} />; }
