"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const links = [
  { href: "/control-plane", label: "Overview" },
  { href: "/payment-operations", label: "Payment Ops" },
  { href: "/invoice-explainability", label: "Invoice Explainability" },
];

export function ControlPlaneNav() {
  const pathname = usePathname();

  return (
    <nav className="flex flex-wrap items-center gap-2 rounded-2xl border border-white/10 bg-slate-950/60 p-2">
      {links.map((link) => {
        const active = pathname === link.href;
        return (
          <Link
            key={link.href}
            href={link.href}
            className={`rounded-xl px-3 py-2 text-xs font-semibold uppercase tracking-[0.14em] transition ${
              active
                ? "bg-cyan-400/20 text-cyan-100"
                : "text-slate-300 hover:bg-white/10 hover:text-white"
            }`}
          >
            {link.label}
          </Link>
        );
      })}
    </nav>
  );
}
