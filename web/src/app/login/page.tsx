import { Suspense } from "react";

import { SessionLoginScreen } from "@/components/auth/session-login-screen";

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
          <main className="relative mx-auto flex min-h-screen max-w-[1200px] items-center justify-center px-4 py-10 md:px-8 lg:px-10">
            <div className="rounded-3xl border border-white/10 bg-slate-900/70 px-6 py-5 text-sm text-slate-300 backdrop-blur-xl">
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
