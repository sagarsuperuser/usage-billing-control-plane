"use client";

import Link from "next/link";
import { ChevronRight } from "lucide-react";

export type BreadcrumbItem = {
  href?: string;
  label: string;
};

export function AppBreadcrumbs({ items }: { items: BreadcrumbItem[] }) {
  return (
    <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.14em] text-slate-400">
      {items.map((item, index) => {
        const last = index === items.length - 1;
        return (
          <div key={`${item.label}-${index}`} className="flex items-center gap-2">
            {item.href && !last ? (
              <Link href={item.href} prefetch={false} className="transition hover:text-white">
                {item.label}
              </Link>
            ) : (
              <span className={last ? "text-slate-200" : undefined}>{item.label}</span>
            )}
            {!last ? <ChevronRight className="h-3.5 w-3.5 text-slate-600" /> : null}
          </div>
        );
      })}
    </nav>
  );
}
