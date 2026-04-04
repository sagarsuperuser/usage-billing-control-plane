"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";

import { useUISession } from "@/hooks/use-ui-session";
import { buildLoginPath } from "@/lib/session-routing";

/**
 * Silent redirect — no visible output. Waits for session check to
 * complete, then redirects to login if not authenticated.
 * Screens render their own skeleton during the loading phase.
 */
export function LoginRedirectNotice() {
  const router = useRouter();
  const pathname = usePathname();
  const { isLoading, isAuthenticated } = useUISession();
  const loginHref = buildLoginPath(pathname || "/control-plane");

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace(loginHref);
    }
  }, [isLoading, isAuthenticated, loginHref, router]);

  return null;
}
