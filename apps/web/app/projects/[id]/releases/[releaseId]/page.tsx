import { ReleaseDetailView } from "@/app/releases/[id]/ReleaseDetailView";
export default async function Page({ params }: { params: Promise<{ id: string; releaseId: string }> }) { const { id, releaseId } = await params; return <ReleaseDetailView projectId={id} releaseId={releaseId} />; }
