"use client";

import Link from "next/link";
import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { useUISession } from "@/hooks/use-ui-session";
import { buildLoginPath } from "@/lib/session-routing";

export function LoginRedirectNotice() {
  const router = useRouter();
  const pathname = usePathname();
  const { isLoading } = useUISession();
  const loginHref = buildLoginPath(pathname || "/control-plane");

  useEffect(() => {
    if (isLoading) {
      return;
    }
    router.replace(loginHref);
  }, [isLoading, loginHref, router]);

  if (isLoading) {
    return (
      <section className="rounded-2xl border border-stone-200 bg-white px-5 py-5 text-sm text-slate-600 shadow-sm">
        <div className="flex items-center gap-2 text-slate-900">
          <LoaderCircle className="h-4 w-4 animate-spin text-slate-500" />
          Restoring your browser session
        </div>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-600">
          Verifying the existing session before redirecting. This avoids bouncing authenticated users through the login screen on reload.
        </p>
      </section>
    );
  }

  return (
    <section className="rounded-2xl border border-stone-200 bg-white px-5 py-5 text-sm text-slate-600 shadow-sm">
      <div className="flex items-center gap-2 text-slate-900">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        Redirecting to sign in
      </div>
      <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-600">
        This control plane requires an authenticated session before opening the requested surface.
      </p>
      <Link
        href={loginHref}
        prefetch={false}
        className="mt-4 inline-flex h-10 items-center rounded-xl border border-stone-200 bg-stone-50 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-slate-700 transition hover:border-stone-300 hover:bg-white"
      >
        Open sign in
      </Link>
    </section>
  );
}
