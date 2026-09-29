import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import { Inbox } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";

export function EmptyState({ title, description, action, href, onAction, icon: Icon = Inbox }: { title: string; description: string; action?: string; href?: string; onAction?: () => void; icon?: LucideIcon }) {
  return (
    <div className="grid justify-items-center gap-3 px-6 py-12 text-center">
      <div className="grid h-11 w-11 place-items-center rounded-full bg-muted text-muted-foreground"><Icon className="h-5 w-5" /></div>
      <div><div className="font-medium">{title}</div><p className="mt-1 max-w-md text-sm text-muted-foreground">{description}</p></div>
      {action && href ? <Link href={href} className={buttonVariants({ size: "sm" })}>{action}</Link> : action && onAction ? <Button size="sm" onClick={onAction}>{action}</Button> : null}
    </div>
  );
}
