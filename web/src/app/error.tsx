"use client"; // Error boundaries must be Client Components

import { useEffect } from "react";

export default function ErrorPage({ error, retry }: { error: Error & { digest?: string }; retry: () => void }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  // In production Next.js replaces server error messages with a generic one; the digest links to the server log.
  const dev = process.env.NODE_ENV !== "production";

  return (
    <main className="mx-auto max-w-xl space-y-4 p-6">
      <h1 className="text-xl font-semibold">Couldn&apos;t load signals</h1>
      <p className="text-sm text-slate-600 dark:text-slate-400">
        The site couldn&apos;t read from Supabase. Usually <code>SUPABASE_URL</code> or <code>SUPABASE_SERVICE_KEY</code> is
        missing or wrong, the tables haven&apos;t been created yet (run <code>supabase/migrations/001_init.sql</code>, and{" "}
        <code>002_trades.sql</code> for the Trades tab), or
        Supabase is temporarily unreachable.
      </p>
      {dev ? (
        <pre className="whitespace-pre-wrap rounded-md bg-rose-50 p-3 text-xs text-rose-900 dark:bg-rose-950 dark:text-rose-200">
          {error.message}
        </pre>
      ) : (
        error.digest && (
          <p className="text-xs text-slate-500">
            Error reference <code>{error.digest}</code>. Look it up in Vercel → your project → Logs.
          </p>
        )
      )}
      <button
        type="button"
        onClick={() => retry()}
        className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white dark:bg-slate-100 dark:text-slate-900"
      >
        Try again
      </button>
    </main>
  );
}
