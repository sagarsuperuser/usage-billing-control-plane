import { createFileRoute } from "@tanstack/react-router";
import { RegistrationScreen } from "@/components/auth/registration-screen";

export const Route = createFileRoute("/register")({
  component: RegistrationScreen,
});
