"use client";

import { useState } from "react";
import type { Side, Trade, TradeMarks } from "@/lib/types";

const LABEL = { buy: ["Buy", "Bought ✓"], sell: ["Sell", "Sold ✓"] } as const;
const STYLE = {
  buy: {
    off: "border-emerald-600 text-emerald-700 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-950",
    on: "border-emerald-600 bg-emerald-600 text-white",
  },
  sell: {
    off: "border-rose-600 text-rose-700 hover:bg-rose-50 dark:text-rose-400 dark:hover:bg-rose-950",
    on: "border-rose-600 bg-rose-600 text-white",
  },
} as const;

/**
 * Records a buy or sell of this signal's stock, or removes it when clicked again.
 * State starts from the marks the page loaded; later changes are kept here, so the
 * Dashboard's 30-second refresh doesn't reset them.
 */
export function TradeButtons({ signalId, initial }: { signalId: number; initial?: TradeMarks[number] }) {
  const [marks, setMarks] = useState<TradeMarks[number]>(initial ?? {});
  const [busy, setBusy] = useState<Side | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function toggle(side: Side) {
    setBusy(side);
    setError(null);
    try {
      const tradeId = marks[side];
      if (tradeId === undefined) {
        const res = await fetch("/api/trades", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ signalId, side }),
        });
        const body = (await res.json().catch(() => ({}))) as { trade?: Trade; error?: string };
        if (!res.ok || !body.trade) throw new Error(body.error ?? `Couldn't save (HTTP ${res.status}).`);
        const id = body.trade.id;
        setMarks((m) => ({ ...m, [side]: id }));
      } else {
        const res = await fetch(`/api/trades/${tradeId}`, { method: "DELETE" });
        if (!res.ok) {
          const body = (await res.json().catch(() => ({}))) as { error?: string };
          throw new Error(body.error ?? `Couldn't remove (HTTP ${res.status}).`);
        }
        setMarks((m) => {
          const next = { ...m };
          delete next[side];
          return next;
        });
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="flex flex-col items-start gap-1">
      <div className="flex gap-1">
        {(["buy", "sell"] as const).map((side) => {
          const on = marks[side] !== undefined;
          return (
            <button
              key={side}
              type="button"
              // Ignore the second click of a double-click: if the save finished in
              // between, it would otherwise remove the trade just recorded.
              onClick={(e) => {
                if (e.detail > 1) return;
                toggle(side);
              }}
              disabled={busy !== null}
              aria-pressed={on}
              title={on ? "Recorded. Click to remove it." : `Record a ${side} of this stock`}
              className={`w-[4.75rem] rounded border px-2 py-0.5 text-xs font-medium disabled:opacity-60 ${STYLE[side][on ? "on" : "off"]}`}
            >
              {busy === side ? "…" : LABEL[side][on ? 1 : 0]}
            </button>
          );
        })}
      </div>
      {error && <span className="max-w-[12rem] text-xs text-red-600">{error}</span>}
    </div>
  );
}
