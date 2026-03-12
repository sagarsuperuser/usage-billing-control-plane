"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchUISession, loginUISession, logoutUISession } from "@/lib/api";
import { useSessionStore } from "@/store/use-session-store";

export function useUISession() {
  const queryClient = useQueryClient();
  const { apiBaseURL, setAPIBaseURL } = useSessionStore();

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
      queryClient.setQueryData(["ui-session", apiBaseURL], null);
      queryClient.invalidateQueries({ queryKey: ["invoice-statuses"] });
      queryClient.invalidateQueries({ queryKey: ["invoice-status-summary"] });
      queryClient.invalidateQueries({ queryKey: ["invoice-events"] });
      queryClient.invalidateQueries({ queryKey: ["invoice-explainability"] });
    },
  });

  const session = loginMutation.data ?? sessionQuery.data ?? null;
  return {
    apiBaseURL,
    setAPIBaseURL,
    session,
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
