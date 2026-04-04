import { createFileRoute } from "@tanstack/react-router";
import { ResetPasswordScreen } from "@/components/auth/reset-password-screen";

export const Route = createFileRoute("/reset-password")({
  component: ResetPasswordScreen,
});
