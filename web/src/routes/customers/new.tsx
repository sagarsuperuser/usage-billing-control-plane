import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/customers/new")({
  component: lazyRouteComponent(() => import("@/components/onboarding/customer-onboarding-screen"), "CustomerOnboardingScreen"),
});
