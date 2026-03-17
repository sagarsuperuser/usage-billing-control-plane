import { NextResponse } from "next/server";

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

export const dynamic = "force-dynamic";
export const revalidate = 0;

export async function GET() {
  const apiBaseURL = trimTrailingSlash(process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ?? "");

  return NextResponse.json(
    {
      apiBaseURL,
    },
    {
      headers: {
        "Cache-Control": "no-store, max-age=0",
      },
    }
  );
}
