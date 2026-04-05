import { useEffect } from "react";

/**
 * Global keyboard shortcuts (Stripe/Linear pattern).
 * /    — focus search input
 * g c  — go to customers
 * g i  — go to invoices
 * g p  — go to payments
 * g s  — go to subscriptions
 * g m  — go to metrics
 * g d  — go to dunning
 * g h  — go to home/overview
 */
export function useKeyboardShortcuts() {
  useEffect(() => {
    let pendingG = false;
    let gTimer: ReturnType<typeof setTimeout>;

    const handler = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
      if ((e.target as HTMLElement)?.isContentEditable) return;

      if (e.key === "/" && !e.metaKey && !e.ctrlKey) {
        const input = document.querySelector<HTMLInputElement>(
          'input[placeholder*="Search"], input[placeholder*="search"]'
        );
        if (input) {
          e.preventDefault();
          input.focus();
        }
        return;
      }

      if (e.key === "g" && !e.metaKey && !e.ctrlKey && !pendingG) {
        pendingG = true;
        gTimer = setTimeout(() => { pendingG = false; }, 500);
        return;
      }

      if (pendingG) {
        pendingG = false;
        clearTimeout(gTimer);
        const routes: Record<string, string> = {
          c: "/customers",
          i: "/invoices",
          p: "/payments",
          s: "/subscriptions",
          m: "/pricing/metrics",
          d: "/dunning",
          u: "/usage-events",
          h: "/control-plane",
        };
        const target = routes[e.key];
        if (target) {
          e.preventDefault();
          window.location.assign(target);
        }
      }
    };

    document.addEventListener("keydown", handler);
    return () => {
      document.removeEventListener("keydown", handler);
      clearTimeout(gTimer);
    };
  }, []);
}
