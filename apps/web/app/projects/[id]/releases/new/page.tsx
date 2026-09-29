import { NewReleaseView } from "@/app/releases/new/NewReleaseView";
export default async function Page({ params }: { params: Promise<{ id: string }> }) { const { id } = await params; return <NewReleaseView projectId={id} />; }
