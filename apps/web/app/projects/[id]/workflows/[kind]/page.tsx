import { WorkflowEditorView } from "@/app/workflows/[kind]/WorkflowEditorView";
import type { WorkflowKind } from "@/lib/types";
export default async function Page({ params }: { params: Promise<{ id: string; kind: WorkflowKind }> }) { const { id, kind } = await params; return <WorkflowEditorView projectId={id} workflowKind={kind} />; }
