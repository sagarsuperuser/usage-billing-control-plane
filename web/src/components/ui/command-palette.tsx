import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import {
  BarChart3,
  CreditCard,
  FileText,
  LayoutDashboard,
  Package,
  Receipt,
  RefreshCw,
  Repeat,
  Search,
  Settings,
  ShieldCheck,
  Tag,
  Users,
  Zap,
} from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";

type CommandItem = {
  id: string;
  label: string;
  section: string;
  href: string;
  icon: typeof LayoutDashboard;
  keywords?: string[];
};

const commands: CommandItem[] = [
  // Revenue
  { id: "overview", label: "Overview", section: "Revenue", href: "/control-plane", icon: LayoutDashboard, keywords: ["home", "dashboard"] },
  { id: "metrics", label: "Metrics", section: "Revenue", href: "/pricing/metrics", icon: BarChart3, keywords: ["usage", "metering"] },
  { id: "plans", label: "Plans", section: "Revenue", href: "/pricing/plans", icon: Package, keywords: ["pricing", "subscription"] },
  { id: "addons", label: "Add-ons", section: "Revenue", href: "/pricing/add-ons", icon: Package, keywords: ["extra", "charge"] },
  { id: "coupons", label: "Coupons", section: "Revenue", href: "/pricing/coupons", icon: Tag, keywords: ["discount", "promo"] },
  { id: "taxes", label: "Taxes", section: "Revenue", href: "/pricing/taxes", icon: Receipt, keywords: ["tax", "vat"] },
  { id: "customers", label: "Customers", section: "Revenue", href: "/customers", icon: Users, keywords: ["account", "user"] },
  { id: "new-customer", label: "New customer", section: "Revenue", href: "/customers/new", icon: Users, keywords: ["create", "onboard"] },
  { id: "subscriptions", label: "Subscriptions", section: "Revenue", href: "/subscriptions", icon: Repeat, keywords: ["plan", "billing"] },
  { id: "new-subscription", label: "New subscription", section: "Revenue", href: "/subscriptions/new", icon: Repeat, keywords: ["create"] },
  { id: "invoices", label: "Invoices", section: "Revenue", href: "/invoices", icon: FileText, keywords: ["bill", "payment"] },
  { id: "payments", label: "Payments", section: "Revenue", href: "/payments", icon: CreditCard, keywords: ["charge", "stripe"] },

  // Operations
  { id: "access", label: "Workspace access", section: "Operations", href: "/workspace-access", icon: Settings, keywords: ["members", "service accounts", "team"] },
  { id: "usage", label: "Usage events", section: "Operations", href: "/usage-events", icon: Zap, keywords: ["events", "metering"] },
  { id: "dunning", label: "Dunning", section: "Operations", href: "/dunning", icon: ShieldCheck, keywords: ["collections", "retry", "failed"] },
  { id: "replay", label: "Replay", section: "Operations", href: "/replay-operations", icon: RefreshCw, keywords: ["reprocess", "jobs"] },
  { id: "explainability", label: "Explainability", section: "Operations", href: "/invoice-explainability", icon: FileText, keywords: ["breakdown", "fees"] },
];

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  // Cmd+K / Ctrl+K to toggle
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setOpen((prev) => !prev);
        setQuery("");
        setSelectedIndex(0);
      }
      if (e.key === "Escape" && open) {
        setOpen(false);
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open]);

  // Focus input when opened
  useEffect(() => {
    if (open) {
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter((cmd) => {
      const haystack = `${cmd.label} ${cmd.section} ${(cmd.keywords || []).join(" ")}`.toLowerCase();
      return q.split(/\s+/).every((word) => haystack.includes(word));
    });
  }, [query]);

  // Reset selection when filter changes
  useEffect(() => {
    setSelectedIndex(0);
  }, [filtered.length]);

  const go = (href: string) => {
    setOpen(false);
    setQuery("");
    navigate({ to: href });
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1));
      scrollIntoView(Math.min(selectedIndex + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((i) => Math.max(i - 1, 0));
      scrollIntoView(Math.max(selectedIndex - 1, 0));
    } else if (e.key === "Enter" && filtered[selectedIndex]) {
      e.preventDefault();
      go(filtered[selectedIndex].href);
    }
  };

  const scrollIntoView = (index: number) => {
    const el = listRef.current?.children[index] as HTMLElement | undefined;
    el?.scrollIntoView({ block: "nearest" });
  };

  // Group by section
  const sections = useMemo(() => {
    const map = new Map<string, CommandItem[]>();
    for (const cmd of filtered) {
      const list = map.get(cmd.section) || [];
      list.push(cmd);
      map.set(cmd.section, list);
    }
    return Array.from(map.entries());
  }, [filtered]);

  let flatIndex = 0;

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.12 }}
        >
          {/* Backdrop */}
          <div className="absolute inset-0 bg-black/40" onClick={() => setOpen(false)} />

          {/* Dialog */}
          <motion.div
            className="relative w-full max-w-lg overflow-hidden rounded-xl border border-border bg-surface shadow-2xl"
            initial={{ opacity: 0, scale: 0.96, y: -8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: -8 }}
            transition={{ duration: 0.15, ease: "easeOut" }}
          >
            {/* Search input */}
            <div className="flex items-center gap-3 border-b border-border-light px-4 py-3">
              <Search className="h-4 w-4 shrink-0 text-text-faint" />
              <input
                ref={inputRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={onKeyDown}
                placeholder="Search pages..."
                className="w-full bg-transparent text-sm text-text-primary outline-none placeholder:text-text-faint"
              />
              <kbd className="hidden shrink-0 rounded border border-border bg-surface-secondary px-1.5 py-0.5 text-[10px] font-medium text-text-faint sm:inline">
                ESC
              </kbd>
            </div>

            {/* Results */}
            <div ref={listRef} className="max-h-[320px] overflow-y-auto p-2">
              {filtered.length === 0 ? (
                <p className="px-3 py-6 text-center text-xs text-text-faint">No results found.</p>
              ) : (
                sections.map(([section, items]) => (
                  <div key={section}>
                    <p className="px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-text-faint">{section}</p>
                    {items.map((cmd) => {
                      const idx = flatIndex++;
                      const Icon = cmd.icon;
                      return (
                        <button
                          key={cmd.id}
                          type="button"
                          onClick={() => go(cmd.href)}
                          onMouseEnter={() => setSelectedIndex(idx)}
                          className={`flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm transition ${
                            idx === selectedIndex ? "bg-slate-900 text-white" : "text-text-secondary hover:bg-surface-secondary"
                          }`}
                        >
                          <Icon className={`h-4 w-4 shrink-0 ${idx === selectedIndex ? "text-slate-300" : "text-text-faint"}`} />
                          {cmd.label}
                        </button>
                      );
                    })}
                  </div>
                ))
              )}
            </div>

            {/* Footer */}
            <div className="flex items-center justify-between border-t border-border-light px-4 py-2">
              <div className="flex items-center gap-2 text-[10px] text-text-faint">
                <kbd className="rounded border border-border bg-surface-secondary px-1 py-0.5 font-mono">↑↓</kbd>
                navigate
                <kbd className="rounded border border-border bg-surface-secondary px-1 py-0.5 font-mono">↵</kbd>
                open
              </div>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
