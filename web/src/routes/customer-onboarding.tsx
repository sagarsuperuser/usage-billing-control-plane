import { createFileRoute } from "@tanstack/react-router";
import { CustomerOnboardingScreen } from "@/components/onboarding/customer-onboarding-screen";

export const Route = createFileRoute("/customer-onboarding")({
  component: CustomerOnboardingScreen,
});
