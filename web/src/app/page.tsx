"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import { useUISession } from "@/hooks/use-ui-session";
import { getDefaultLandingPath } from "@/lib/session-routing";

export default function HomePage() {
  const router = useRouter();
  const { session, isAuthenticated, isLoading } = useUISession();

  useEffect(() => {
    if (isLoading) {
      return;
    }
    router.replace(isAuthenticated ? getDefaultLandingPath(session) : "/login");
  }, [isAuthenticated, isLoading, router, session]);

  return null;
}
