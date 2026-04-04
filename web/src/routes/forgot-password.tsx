import { createFileRoute } from "@tanstack/react-router";
import { ForgotPasswordScreen } from "@/components/auth/forgot-password-screen";

export const Route = createFileRoute("/forgot-password")({
  component: ForgotPasswordScreen,
});
