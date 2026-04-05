import { createRootRoute, Outlet, useLocation } from "@tanstack/react-router";
import { AnimatePresence, motion } from "framer-motion";

import { AppProviders } from "@/app/providers";
import { SidebarLayout } from "@/components/layout/sidebar";
import { CommandPalette } from "@/components/ui/command-palette";
import { useKeyboardShortcuts } from "@/hooks/use-keyboard-shortcuts";

export const Route = createRootRoute({
  component: function RootLayout() {
    useKeyboardShortcuts();
    return (
      <AppProviders>
        <CommandPalette />
        <SidebarLayout>
          <PageTransition />
        </SidebarLayout>
      </AppProviders>
    );
  },
});

function PageTransition() {
  const { pathname } = useLocation();

  return (
    <AnimatePresence mode="wait" initial={false}>
      <motion.div
        key={pathname}
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.15, ease: "easeOut" }}
      >
        <Outlet />
      </motion.div>
    </AnimatePresence>
  );
}
