"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import symbols from "@/lib/nse-symbols.json";
import { lookupSymbol, manualTradeBody } from "@/lib/trades";
import type { Side, Trade } from "@/lib/types";

const input = "rounded-md border border-slate-300 bg-transparent px-2 py-1.5 text-sm dark:border-slate-700";
const MAX_HINTS = 5;

export function AddTradeForm({ today }: { today: string }) {
  const router = useRouter();
  const [symbol, setSymbol] = useState("");
  const [side, setSide] = useState<Side>("buy");
  const [price, setPrice] = useState("");
  const [date, setDate] = useState(today);
  const [dateTouched, setDateTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hint, setHint] = useState<[string, string][] | null>(null);
  const [added, setAdded] = useState<string | null>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setHint(null);
    setAdded(null);
    const p = price.trim() === "" ? null : Number(price);
    if (p !== null && !Number.isFinite(p)) {
      setError("Price must be a number.");
      return;
    }

    // A company name typed without picking a suggestion must not become a made-up symbol.
    const found = lookupSymbol(symbol, symbols);
    if (found.kind === "company") {
      setHint(found.matches);
      return;
    }
    if (found.kind === "unknown" && !window.confirm(`${found.symbol} isn't in NSE's list of listed stocks. Add it anyway?`)) {
      return;
    }

    setBusy(true);
    try {
      const res = await fetch("/api/trades", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(manualTradeBody({ symbol, side, price: p, date, dateTouched })),
      });
      const body = (await res.json().catch(() => ({}))) as { trade?: Trade; error?: string };
      if (!res.ok || !body.trade) throw new Error(body.error ?? `Couldn't save (HTTP ${res.status}).`);
      const t = body.trade;
      setAdded(`Added ${t.symbol} ${t.side} on ${t.tradeDate}.`);
      setSymbol("");
      setPrice("");
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  function pick(s: string) {
    setSymbol(s);
    setHint(null);
  }

  return (
    <form
      onSubmit={submit}
      className="space-y-3 rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900"
    >
      <h2 className="text-sm font-medium">Add a trade</h2>
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-xs text-slate-500">
          Stock
          <input
            list="nse-symbols"
            value={symbol}
            onChange={(e) => setSymbol(e.target.value)}
            required
            autoComplete="off"
            placeholder="Symbol or company"
            className={`${input} w-56`}
          />
        </label>
        <datalist id="nse-symbols">
          {symbols.map(([s, name]) => (
            <option key={s} value={s} label={name} />
          ))}
        </datalist>
        <fieldset className="flex gap-1">
          <legend className="sr-only">Side</legend>
          {(["buy", "sell"] as const).map((s) => (
            <label
              key={s}
              className={`cursor-pointer rounded-md border px-3 py-1.5 text-sm font-medium ${
                side === s
                  ? s === "buy"
                    ? "border-emerald-600 bg-emerald-600 text-white"
                    : "border-rose-600 bg-rose-600 text-white"
                  : "border-slate-300 text-slate-600 dark:border-slate-700 dark:text-slate-300"
              }`}
            >
              <input type="radio" name="side" value={s} checked={side === s} onChange={() => setSide(s)} className="sr-only" />
              {s === "buy" ? "Buy" : "Sell"}
            </label>
          ))}
        </fieldset>
        <label className="flex flex-col gap-1 text-xs text-slate-500">
          Price (optional)
          <input
            type="number"
            step="any"
            min="0.01"
            inputMode="decimal"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            className={`${input} w-32`}
          />
        </label>
        <label className="flex flex-col gap-1 text-xs text-slate-500">
          Date
          <input
            type="date"
            value={date}
            max={today}
            required
            onChange={(e) => {
              setDate(e.target.value);
              setDateTouched(true);
            }}
            className={input}
          />
        </label>
        <button
          disabled={busy}
          className="rounded-md bg-slate-900 px-4 py-1.5 text-sm font-medium text-white disabled:opacity-60 dark:bg-slate-100 dark:text-slate-900"
        >
          {busy ? "Adding…" : "Add"}
        </button>
      </div>
      {hint && (
        <div className="text-sm text-amber-700 dark:text-amber-400">
          {hint.length === 1 ? "Did you mean " : `"${symbol.trim()}" matches ${hint.length} companies. Pick one: `}
          {hint.slice(0, MAX_HINTS).map(([s, name], i) => (
            <span key={s}>
              {i > 0 && ", "}
              <button type="button" onClick={() => pick(s)} className="font-medium underline">
                {s}
              </button>{" "}
              ({name})
            </span>
          ))}
          {hint.length > MAX_HINTS ? `, or keep typing (${hint.length - MAX_HINTS} more).` : hint.length === 1 ? "?" : "."}
        </div>
      )}
      {added && <p className="text-sm text-emerald-700 dark:text-emerald-400">{added}</p>}
      {error && <p className="text-sm text-red-600">{error}</p>}
    </form>
  );
}
