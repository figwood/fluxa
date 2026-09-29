import * as React from "react";
import { cn } from "@/lib/utils";

export function Alert({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700", className)} {...props} />;
}
