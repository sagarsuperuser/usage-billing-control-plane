import { useLocation } from "@tanstack/react-router";

/**
 * Bridge hook: provides URLSearchParams from the current location.
 * Drop-in replacement for next/navigation's useSearchParams().
 */
export function useSearchParamsCompat(): URLSearchParams {
  const location = useLocation();
  return new URLSearchParams(location.search as unknown as string);
}
