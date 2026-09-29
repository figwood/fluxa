import ProjectDetail from "../ProjectDetail";
import { ServicesView } from "@/app/services/ServicesView";
import DeliveryPanel from "./DeliveryPanel";
export default async function Page({ params }: { params: Promise<{ id: string }> }) { const { id } = await params; return <div className="grid gap-6"><ProjectDetail projectId={id} section="repositories" /><ServicesView projectId={id} /><DeliveryPanel projectId={id} /></div>; }
