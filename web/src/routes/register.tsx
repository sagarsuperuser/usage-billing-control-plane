import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/register")({
  component: lazyRouteComponent(() => import("@/components/auth/registration-screen"), "RegistrationScreen"),
});
