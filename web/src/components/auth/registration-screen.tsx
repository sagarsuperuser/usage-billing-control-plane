import { Link, useNavigate } from "@tanstack/react-router";
import { AuthLayout } from "@/components/auth/auth-layout";
import { useEffect, useState } from "react";
import { LoaderCircle, UserPlus } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { registerUser } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

const registrationSchema = z.object({
  display_name: z.string().min(1, "Display name is required"),
  email: z.string().email("Enter a valid email address"),
  password: z.string().min(8, "Password must be at least 8 characters"),
});

type RegistrationFields = z.infer<typeof registrationSchema>;

export function RegistrationScreen() {
  const navigate = useNavigate();
  const { isAuthenticated, apiBaseURL } = useUISession();
  const [serverError, setServerError] = useState<string | null>(null);

  useEffect(() => {
    if (isAuthenticated) {
      navigate({ to: "/control-plane", replace: true });
    }
  }, [isAuthenticated, navigate]);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegistrationFields>({
    resolver: zodResolver(registrationSchema),
  });

  const onSubmit = async (data: RegistrationFields) => {
    setServerError(null);
    try {
      await registerUser({
        runtimeBaseURL: apiBaseURL,
        email: data.email,
        password: data.password,
        display_name: data.display_name,
      });
      window.location.assign("/control-plane");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Registration failed";
      setServerError(message);
    }
  };

  if (isAuthenticated) {
    return (
      <div className="min-h-screen bg-[#f5f7fb]">
        <main className="flex min-h-screen items-center justify-center px-4">
          <div className="rounded-xl border border-stone-200 bg-white px-6 py-4 text-sm text-slate-500 shadow-sm">
            Redirecting...
          </div>
        </main>
      </div>
    );
  }

  return (
    <AuthLayout>
      <div className="w-full">
              <h2 className="text-2xl font-semibold tracking-tight text-slate-950">Create account</h2>
              <p className="mt-1.5 text-sm text-slate-500">Set up your workspace and start billing.</p>

              <form className="mt-6 grid gap-4" onSubmit={handleSubmit(onSubmit)} noValidate>
                <div className="grid gap-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-slate-500">Display name</label>
                  <input
                    type="text"
                    data-testid="register-display-name"
                    placeholder="Jane Smith"
                    autoComplete="name"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3.5 text-sm text-slate-900 outline-none ring-slate-300 transition placeholder:text-slate-400 focus:ring-2 aria-invalid:border-rose-300 aria-invalid:ring-rose-200"
                    aria-invalid={errors.display_name ? "true" : undefined}
                    {...register("display_name")}
                  />
                  {errors.display_name ? <p className="text-xs text-rose-600">{errors.display_name.message}</p> : null}
                </div>

                <div className="grid gap-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-slate-500">Email</label>
                  <input
                    type="email"
                    data-testid="register-email"
                    placeholder="you@example.com"
                    autoComplete="email"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3.5 text-sm text-slate-900 outline-none ring-slate-300 transition placeholder:text-slate-400 focus:ring-2 aria-invalid:border-rose-300 aria-invalid:ring-rose-200"
                    aria-invalid={errors.email ? "true" : undefined}
                    {...register("email")}
                  />
                  {errors.email ? <p className="text-xs text-rose-600">{errors.email.message}</p> : null}
                </div>

                <div className="grid gap-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-slate-500">Password</label>
                  <input
                    type="password"
                    data-testid="register-password"
                    placeholder="At least 8 characters"
                    autoComplete="new-password"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3.5 text-sm text-slate-900 outline-none ring-slate-300 transition placeholder:text-slate-400 focus:ring-2 aria-invalid:border-rose-300 aria-invalid:ring-rose-200"
                    aria-invalid={errors.password ? "true" : undefined}
                    {...register("password")}
                  />
                  {errors.password ? <p className="text-xs text-rose-600">{errors.password.message}</p> : null}
                </div>

                <button
                  type="submit"
                  data-testid="register-submit"
                  disabled={isSubmitting}
                  className="mt-1 inline-flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-slate-900 px-4 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {isSubmitting ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <UserPlus className="h-4 w-4" />}
                  Create account
                </button>

                {serverError ? <p className="text-xs text-rose-600">{serverError}</p> : null}
              </form>

              <p className="mt-6 text-center text-sm text-slate-500">
                Already have an account?{" "}
                <Link to="/login" className="font-medium text-slate-700 transition hover:text-slate-900">
                  Sign in
                </Link>
              </p>
            </div>
    </AuthLayout>
  );
}
