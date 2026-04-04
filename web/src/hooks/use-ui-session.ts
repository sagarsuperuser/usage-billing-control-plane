import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchRuntimeConfig, fetchUISession, getConfiguredAPIBaseURL, loginUISession, logoutUISession } from "@/lib/api";
import { useSessionStore } from "@/store/use-session-store";

export function useUISession() {
  const queryClient = useQueryClient();
  const { session, setSession } = useSessionStore();

  // Try sync first (production/dev), fall back to async fetch (tests/legacy)
  const syncBaseURL = getConfiguredAPIBaseURL();

  const runtimeConfigQuery = useQuery({
    queryKey: ["runtime-config"],
    queryFn: fetchRuntimeConfig,
    staleTime: Infinity,
    enabled: !syncBaseURL,
  });

  const apiBaseURL = syncBaseURL || runtimeConfigQuery.data?.apiBaseURL || "";
  const configReady = Boolean(syncBaseURL) || runtimeConfigQuery.isSuccess;

  const sessionQuery = useQuery({
    queryKey: ["ui-session", apiBaseURL],
    queryFn: () => fetchUISession({ runtimeBaseURL: apiBaseURL }),
    enabled: configReady,
  });

  const loginMutation = useMutation({
    mutationFn: (input: { email: string; password: string; tenantID?: string; nextPath?: string }) =>
      loginUISession({
        email: input.email,
        password: input.password,
        tenantID: input.tenantID,
        nextPath: input.nextPath,
        runtimeBaseURL: apiBaseURL,
      }),
    onSuccess: (newSession) => {
      setSession(newSession);
      queryClient.setQueryData(["ui-session", apiBaseURL], newSession);
    },
  });

  const logoutMutation = useMutation({
    mutationFn: (csrfToken: string) =>
      logoutUISession({ runtimeBaseURL: apiBaseURL, csrfToken }),
    onSuccess: () => {
      setSession(null);
      queryClient.setQueryData(["ui-session", apiBaseURL], null);
      queryClient.invalidateQueries();
    },
  });

  useEffect(() => {
    if (sessionQuery.isSuccess && sessionQuery.data) {
      setSession(sessionQuery.data);
    }
  }, [sessionQuery.data, sessionQuery.isSuccess, setSession]);

  const scope = session?.scope ?? null;
  const role = scope === "tenant" ? (session?.role ?? null) : null;
  const platformRole = scope === "platform" ? (session?.platform_role ?? null) : null;
  const canWrite = role === "writer" || role === "admin";
  const isAdmin = role === "admin";
  const isPlatformAdmin = platformRole === "platform_admin";

  return {
    apiBaseURL,
    session,
    setSession,
    scope,
    role,
    platformRole,
    canWrite,
    isAdmin,
    isPlatformAdmin,
    csrfToken: session?.csrf_token ?? "",
    isAuthenticated: Boolean(session?.authenticated),
    isLoading: (!configReady) || sessionQuery.isPending,
    configError: runtimeConfigQuery.error as Error | null,
    login: loginMutation.mutateAsync,
    loggingIn: loginMutation.isPending,
    loginError: loginMutation.error as Error | null,
    logout: logoutMutation.mutateAsync,
    loggingOut: logoutMutation.isPending,
  };
}
