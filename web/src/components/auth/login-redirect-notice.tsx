import { useEffect } from "react";
import { useLocation, useNavigate } from "@tanstack/react-router";

import { useUISession } from "@/hooks/use-ui-session";
import { buildLoginPath } from "@/lib/session-routing";

/**
 * Silent redirect — no visible output. Waits for session check to
 * complete, then redirects to login if not authenticated.
 * Screens render their own skeleton during the loading phase.
 */
export function LoginRedirectNotice() {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { isLoading, isAuthenticated } = useUISession();
  const loginHref = buildLoginPath(pathname || "/control-plane");

  useEffect(() => {
    if (isLoading) return;
    if (isAuthenticated) return;
    // Small delay to prevent race condition during session hydration.
    // TanStack Router SPA renders before the async session fetch settles.
    const timer = setTimeout(() => {
      navigate({ to: loginHref, replace: true });
    }, 200);
    return () => clearTimeout(timer);
  }, [isLoading, isAuthenticated, loginHref, navigate]);

  return null;
}
