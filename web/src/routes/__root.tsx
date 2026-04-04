import { createRootRoute, Outlet } from "@tanstack/react-router";

import { AppProviders } from "@/app/providers";
import { SidebarLayout } from "@/components/layout/sidebar";

export const Route = createRootRoute({
  component: function RootLayout() {
    return (
      <AppProviders>
        <SidebarLayout>
          <Outlet />
        </SidebarLayout>
      </AppProviders>
    );
  },
});
