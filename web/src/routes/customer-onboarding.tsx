import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/customer-onboarding")({
  component: lazyRouteComponent(() => import("@/components/onboarding/customer-onboarding-screen"), "CustomerOnboardingScreen"),
});
