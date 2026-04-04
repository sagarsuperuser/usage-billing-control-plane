"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";

import { useUISession } from "@/hooks/use-ui-session";
import { buildLoginPath } from "@/lib/session-routing";

/**
 * Shown when a page requires authentication but the session is either
 * loading or missing. During loading it renders a skeleton shimmer
 * (Stripe/Linear pattern — no text, no implementation details).
 * Once loading finishes and there's no session, it silently redirects
 * to the login page.
 */
export function LoginRedirectNotice() {
  const router = useRouter();
  const pathname = usePathname();
  const { isLoading } = useUISession();
  const loginHref = buildLoginPath(pathname || "/control-plane");

  useEffect(() => {
    if (!isLoading) {
      router.replace(loginHref);
    }
  }, [isLoading, loginHref, router]);

  return (
    <div className="animate-pulse space-y-3">
      <div className="h-10 w-full rounded-lg bg-stone-100" />
      <div className="h-64 w-full rounded-lg bg-stone-100" />
    </div>
  );
}
