import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/reset-password")({
  component: lazyRouteComponent(() => import("@/components/auth/reset-password-screen"), "ResetPasswordScreen"),
});
