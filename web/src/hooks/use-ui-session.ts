"use client";

import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchUISession, loginUISession, logoutUISession } from "@/lib/api";
import { useSessionStore } from "@/store/use-session-store";

export function useUISession() {
  const queryClient = useQueryClient();
  const { apiBaseURL, setAPIBaseURL, session, setSession } = useSessionStore();

  const sessionQuery = useQuery({
    queryKey: ["ui-session", apiBaseURL],
    queryFn: () =>
      fetchUISession({
        runtimeBaseURL: apiBaseURL,
      }),
  });

  const loginMutation = useMutation({
    mutationFn: (apiKey: string) =>
      loginUISession({
        apiKey,
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
    },
  });

  useEffect(() => {
    if (sessionQuery.isSuccess && sessionQuery.data) {
      setSession(sessionQuery.data);
    }
  }, [sessionQuery.data, sessionQuery.isSuccess, setSession]);

  const role = session?.role ?? null;
  const canWrite = role === "writer" || role === "admin";
  const isAdmin = role === "admin";
  return {
    apiBaseURL,
    setAPIBaseURL,
    session,
    role,
    canWrite,
    isAdmin,
    csrfToken: session?.csrf_token ?? "",
    isAuthenticated: Boolean(session?.authenticated),
    isLoading: sessionQuery.isLoading,
    login: loginMutation.mutateAsync,
    loggingIn: loginMutation.isPending,
    loginError: loginMutation.error as Error | null,
    logout: logoutMutation.mutateAsync,
    loggingOut: logoutMutation.isPending,
  };
}
