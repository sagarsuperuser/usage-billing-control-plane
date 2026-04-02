"use client";

import { Component, type ReactNode, type ErrorInfo } from "react";

// ── Full-page boundary ────────────────────────────────────────────────────────
// Wraps the entire app in providers.tsx. Catches any unhandled render error and
// replaces the blank white screen with a recoverable error UI.

interface PageState { hasError: boolean; error: Error | null }

export class PageErrorBoundary extends Component<{ children: ReactNode }, PageState> {
  state: PageState = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error): PageState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[PageErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (!this.state.hasError) return this.props.children;
    const { error } = this.state;
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-[#f5f7fb] px-4 text-slate-900">
        <div className="w-full max-w-md rounded-2xl border border-rose-200 bg-white p-8 shadow-sm">
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-rose-500">Unexpected error</p>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Something went wrong</h1>
          <p className="mt-3 text-sm text-slate-600">
            An unexpected error occurred in the control plane. Try recovering below or reload the page.
          </p>
          {error?.message ? (
            <p className="mt-4 rounded-lg border border-rose-100 bg-rose-50 px-3 py-2 font-mono text-xs text-rose-700 break-all">
              {error.message}
            </p>
          ) : null}
          <div className="mt-6 flex flex-wrap gap-3">
            <button
              onClick={() => this.setState({ hasError: false, error: null })}
              className="inline-flex h-10 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800"
            >
              Try again
            </button>
            {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
            <a
              href="/"
              className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
            >
              Go to home
            </a>
          </div>
        </div>
      </div>
    );
  }
}

// ── Section boundary ──────────────────────────────────────────────────────────
// Drop inside any detail screen or data-heavy section. If that section crashes,
// the rest of the page stays functional.

interface SectionState { hasError: boolean }

export class SectionErrorBoundary extends Component<{ children: ReactNode }, SectionState> {
  state: SectionState = { hasError: false };

  static getDerivedStateFromError(): SectionState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[SectionErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (!this.state.hasError) return this.props.children;
    return (
      <div className="rounded-2xl border border-rose-200 bg-rose-50 p-6">
        <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-rose-500">Section error</p>
        <p className="mt-2 text-sm font-semibold text-rose-800">This section failed to render.</p>
        <p className="mt-1 text-sm text-rose-700">The rest of the page is still usable. Try recovering or reload.</p>
        <button
          onClick={() => this.setState({ hasError: false })}
          className="mt-4 inline-flex h-8 items-center rounded-lg border border-rose-200 bg-white px-3 text-xs font-medium text-rose-700 transition hover:bg-rose-100"
        >
          Try again
        </button>
      </div>
    );
  }
}
