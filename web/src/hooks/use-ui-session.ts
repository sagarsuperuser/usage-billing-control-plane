"use client";

import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchRuntimeConfig, fetchUISession, loginUISession, logoutUISession } from "@/lib/api";
import { useSessionStore } from "@/store/use-session-store";

export function useUISession() {
  const queryClient = useQueryClient();
  const { session, setSession } = useSessionStore();

  const runtimeConfigQuery = useQuery({
    queryKey: ["runtime-config"],
    queryFn: fetchRuntimeConfig,
    staleTime: Infinity,
  });

  const apiBaseURL = runtimeConfigQuery.data?.apiBaseURL ?? "";

  const sessionQuery = useQuery({
    queryKey: ["ui-session", apiBaseURL],
    queryFn: () =>
      fetchUISession({
        runtimeBaseURL: apiBaseURL,
      }),
    enabled: runtimeConfigQuery.isSuccess,
  });

  const loginMutation = useMutation({
    mutationFn: (input: { email: string; password: string; tenantID?: string }) =>
      loginUISession({
        email: input.email,
        password: input.password,
        tenantID: input.tenantID,
        runtimeBaseURL: apiBaseURL,
      }),
    onSuccess: (session) => {
      setSession(session);
      queryClient.setQueryData(["ui-session", apiBaseURL], session);
    },
  });

  const logoutMutation = useMutation({
    mutationFn: (csrfToken: string) =>
      logoutUISession({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
      }),
    onSuccess: () => {
      setSession(null);
      queryClient.setQueryData(["ui-session", apiBaseURL], null);
      queryClient.invalidateQueries({ queryKey: ["invoice-statuses"] });
      queryClient.invalidateQueries({ queryKey: ["invoice-status-summary"] });
      queryClient.invalidateQueries({ queryKey: ["invoice-events"] });
      queryClient.invalidateQueries({ queryKey: ["invoice-explainability"] });
      queryClient.invalidateQueries({ queryKey: ["tenants"] });
      queryClient.invalidateQueries({ queryKey: ["tenant-onboarding-status"] });
      queryClient.invalidateQueries({ queryKey: ["customers"] });
      queryClient.invalidateQueries({ queryKey: ["customer-readiness"] });
    },
  });

  useEffect(() => {
    if (sessionQuery.isSuccess && sessionQuery.data) {
      setSession(sessionQuery.data);
    }
  }, [sessionQuery.data, sessionQuery.isSuccess, setSession]);

  const scope = session?.scope ?? "tenant";
  const role = scope === "tenant" ? (session?.role ?? null) : null;
  const platformRole = scope === "platform" ? (session?.platform_role ?? null) : null;
  const canWrite = role === "writer" || role === "admin";
  const isAdmin = role === "admin";
  const isPlatformAdmin = platformRole === "platform_admin";
  return {
    apiBaseURL,
    session,
    scope,
    role,
    platformRole,
    canWrite,
    isAdmin,
    isPlatformAdmin,
    csrfToken: session?.csrf_token ?? "",
    isAuthenticated: Boolean(session?.authenticated),
    isLoading: runtimeConfigQuery.isLoading || sessionQuery.isLoading,
    configError: runtimeConfigQuery.error as Error | null,
    login: loginMutation.mutateAsync,
    loggingIn: loginMutation.isPending,
    loginError: loginMutation.error as Error | null,
    logout: logoutMutation.mutateAsync,
    loggingOut: logoutMutation.isPending,
  };
}
