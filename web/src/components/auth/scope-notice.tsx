"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";

export function ScopeNotice({
  title,
  body,
  actionHref,
  actionLabel,
}: {
  title: string;
  body: string;
  actionHref?: string;
  actionLabel?: string;
}) {
  return (
    <section className="rounded-2xl border border-amber-400/40 bg-[linear-gradient(135deg,rgba(245,158,11,0.14),rgba(15,23,42,0.72))] px-4 py-4 text-sm text-amber-100 backdrop-blur-xl">
      <p className="font-semibold uppercase tracking-[0.16em] text-amber-200/90">{title}</p>
      <p className="mt-2 max-w-2xl text-amber-100/90">{body}</p>
      {actionHref && actionLabel ? (
        <Link
          href={actionHref}
          className="mt-4 inline-flex h-10 items-center gap-2 rounded-xl border border-amber-300/40 bg-white/10 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-amber-50 transition hover:bg-white/15"
        >
          {actionLabel}
          <ArrowRight className="h-3.5 w-3.5" />
        </Link>
      ) : null}
    </section>
  );
}
