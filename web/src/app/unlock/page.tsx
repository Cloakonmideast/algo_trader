import { safeNext } from "@/lib/auth";

type SP = Promise<Record<string, string | string[] | undefined>>;
const first = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v);

export default async function UnlockPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const next = safeNext(first(sp.next));
  const error = first(sp.error);

  return (
    <main className="grid min-h-dvh place-items-center p-4">
      <form
        method="post"
        action="/api/unlock"
        className="w-full max-w-sm space-y-4 rounded-xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <h1 className="text-lg font-semibold">AlgoTrader Signals</h1>
        <p className="text-sm text-slate-600 dark:text-slate-400">
          Enter the passcode to unlock this device. It stays unlocked for a year.
        </p>
        <input type="hidden" name="next" value={next} />
        <input
          type="password"
          name="passcode"
          required
          autoFocus
          autoComplete="current-password"
          placeholder="Passcode"
          className="w-full rounded-md border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700"
        />
        {error === "1" && <p className="text-sm text-red-600">Wrong passcode.</p>}
        {error === "config" && (
          <p className="text-sm text-red-600">The site has no COOKIE_SECRET configured.</p>
        )}
        <button className="w-full rounded-md bg-slate-900 px-3 py-2 font-medium text-white dark:bg-slate-100 dark:text-slate-900">
          Unlock
        </button>
      </form>
    </main>
  );
}
