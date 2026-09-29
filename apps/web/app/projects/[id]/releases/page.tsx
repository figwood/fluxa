import { ReleasesView } from "@/app/releases/ReleasesView";
export default async function Page({ params }: { params: Promise<{ id: string }> }) { const { id } = await params; return <ReleasesView projectId={id} />; }
