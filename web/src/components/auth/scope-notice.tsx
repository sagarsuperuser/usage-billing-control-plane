"use client";

export function ScopeNotice({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-amber-400/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-100">
      <p className="font-semibold uppercase tracking-[0.16em] text-amber-200/90">{title}</p>
      <p className="mt-1 text-amber-100/90">{body}</p>
    </section>
  );
}
