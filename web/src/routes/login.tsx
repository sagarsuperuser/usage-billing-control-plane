import { createFileRoute } from "@tanstack/react-router";
import { SessionLoginScreen } from "@/components/auth/session-login-screen";

export const Route = createFileRoute("/login")({
  component: SessionLoginScreen,
});
