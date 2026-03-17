"use client";

import Link from "next/link";
import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { buildLoginPath } from "@/lib/session-routing";

export function LoginRedirectNotice() {
  const router = useRouter();
  const pathname = usePathname();
  const loginHref = buildLoginPath(pathname || "/control-plane");

  useEffect(() => {
    router.replace(loginHref);
  }, [loginHref, router]);

  return (
    <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 text-sm text-slate-300 backdrop-blur-xl">
      <div className="flex items-center gap-2 text-slate-100">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        Redirecting to sign in
      </div>
      <p className="mt-3 max-w-2xl text-sm text-slate-300">
        This control plane requires an authenticated session before opening the requested surface.
      </p>
      <Link
        href={loginHref}
        className="mt-4 inline-flex h-10 items-center rounded-xl border border-white/10 bg-white/5 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-slate-100 transition hover:bg-white/10"
      >
        Open sign in
      </Link>
    </section>
  );
}
