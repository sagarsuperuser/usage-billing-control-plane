import { type ReactNode } from "react";

/**
 * Shared layout for all auth screens (login, register, forgot/reset password).
 * Left: gradient brand panel. Right: form content.
 * Ensures consistent branding across all auth flows.
 */
export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-[#f5f7fb]">
      <main className="grid min-h-screen xl:grid-cols-[minmax(0,1fr)_480px]">

        {/* Left — brand panel */}
        <div className="hidden xl:flex flex-col justify-between overflow-hidden relative bg-[#0a0a0f] px-14 py-12">
          {/* Gradient mesh */}
          <div className="absolute inset-0 opacity-30">
            <div className="absolute -left-20 -top-20 h-[500px] w-[500px] rounded-full bg-blue-600/30 blur-[120px]" />
            <div className="absolute -bottom-20 -right-20 h-[400px] w-[400px] rounded-full bg-indigo-600/20 blur-[100px]" />
            <div className="absolute left-1/2 top-1/2 h-[300px] w-[300px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-violet-600/15 blur-[80px]" />
          </div>

          <div className="relative flex items-center gap-3">
            <BrandLogo size={32} />
            <span className="text-sm font-semibold text-white tracking-wide">Alpha</span>
          </div>

          <div className="relative">
            <h1 className="text-5xl font-bold leading-[1.1] tracking-tight text-white">
              Billing<br />infrastructure<br />
              <span className="bg-gradient-to-r from-blue-400 to-violet-400 bg-clip-text text-transparent">
                that scales.
              </span>
            </h1>
            <p className="mt-6 max-w-sm text-[15px] leading-7 text-slate-400">
              Usage-based pricing, automated invoicing, and payment collection — built for operators who need full control.
            </p>
          </div>

          <p className="relative text-[11px] text-slate-600 tracking-wider uppercase">Staging environment</p>
        </div>

        {/* Right — form content */}
        <div className="flex flex-col items-center justify-center px-6 py-12 sm:px-10">
          <div className="w-full max-w-[400px]">
            <div className="mb-8 xl:hidden flex items-center gap-2.5">
              <BrandLogo size={28} />
              <span className="text-sm font-semibold text-slate-900">Alpha</span>
            </div>
            {children}
          </div>
        </div>
      </main>
    </div>
  );
}

export function BrandLogo({ size = 32 }: { size?: number }) {
  return (
    <div
      className="flex items-center justify-center rounded-xl bg-gradient-to-br from-blue-600 to-violet-600"
      style={{ width: size, height: size }}
    >
      <svg width={size * 0.5} height={size * 0.5} viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="2" y="9" width="3" height="7" rx="1" fill="white" fillOpacity="0.5" />
        <rect x="7" y="5" width="3" height="11" rx="1" fill="white" fillOpacity="0.75" />
        <rect x="12" y="2" width="3" height="14" rx="1" fill="white" />
      </svg>
    </div>
  );
}
