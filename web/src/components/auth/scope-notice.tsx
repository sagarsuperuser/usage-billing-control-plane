
import { Link } from "@tanstack/react-router";
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
    <section className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
      <p className="font-semibold uppercase tracking-[0.16em] text-amber-700">{title}</p>
      <p className="mt-2 max-w-2xl text-amber-800">{body}</p>
      {actionHref && actionLabel ? (
        <Link
          to={actionHref}
         
          className="mt-4 inline-flex h-10 items-center gap-2 rounded-xl border border-amber-200 bg-white px-4 text-xs font-semibold uppercase tracking-[0.14em] text-amber-800 transition hover:bg-amber-100"
        >
          {actionLabel}
          <ArrowRight className="h-3.5 w-3.5" />
        </Link>
      ) : null}
    </section>
  );
}
