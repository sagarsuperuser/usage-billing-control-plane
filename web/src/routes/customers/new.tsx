import { createFileRoute } from "@tanstack/react-router";
import { CustomerOnboardingScreen } from "@/components/onboarding/customer-onboarding-screen";

export const Route = createFileRoute("/customers/new")({
  component: CustomerOnboardingScreen,
});
