"use client";

import { FormEvent, useState } from "react";
import { LoaderCircle, LogIn } from "lucide-react";

import { useUISession } from "@/hooks/use-ui-session";

export function SessionLoginCard() {
  const { apiBaseURL, setAPIBaseURL, login, loggingIn, loginError } = useUISession();
  const [apiKey, setAPIKey] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  const onSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setErrorMessage("");
    try {
      await login(apiKey);
      setAPIKey("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Login failed";
      setErrorMessage(message);
    }
  };

  return (
    <section className="rounded-2xl border border-amber-400/30 bg-amber-500/10 p-4 text-sm text-amber-100">
      <p className="mb-3 text-sm font-medium text-amber-100">
        Sign in with a control-plane API key to start this session.
      </p>
      <form className="grid gap-3 md:grid-cols-3" onSubmit={onSubmit}>
        <div className="grid gap-2">
          <label className="text-xs font-medium uppercase tracking-wider text-amber-200">API Key</label>
          <input
            type="password"
            data-testid="session-login-api-key"
            value={apiKey}
            onChange={(event) => setAPIKey(event.target.value)}
            placeholder="reader/writer/admin key"
            className="h-10 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
          />
        </div>
        <div className="grid gap-2">
          <label className="text-xs font-medium uppercase tracking-wider text-amber-200">API Base URL</label>
          <input
            type="text"
            data-testid="session-login-api-base-url"
            value={apiBaseURL}
            onChange={(event) => setAPIBaseURL(event.target.value)}
            placeholder="Optional, default same-origin"
            className="h-10 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
          />
        </div>
        <div className="flex items-end">
          <button
            type="submit"
            data-testid="session-login-submit"
            disabled={!apiKey.trim() || loggingIn}
            className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/10 px-4 text-sm text-emerald-100 transition hover:bg-emerald-500/20 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingIn ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
            Sign In
          </button>
        </div>
      </form>
      {(errorMessage || loginError?.message) && (
        <p className="mt-3 text-xs text-rose-200">{errorMessage || loginError?.message}</p>
      )}
    </section>
  );
}
