import { Suspense } from "react";

import { SessionLoginScreen } from "@/components/auth/session-login-screen";

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
          <main className="mx-auto flex min-h-screen max-w-[1200px] items-center justify-center px-4 py-10 md:px-8 lg:px-10">
            <div className="rounded-2xl border border-stone-200 bg-white px-6 py-5 text-sm text-slate-600 shadow-sm">
              Preparing your session
            </div>
          </main>
        </div>
      }
    >
      <SessionLoginScreen />
    </Suspense>
  );
}
