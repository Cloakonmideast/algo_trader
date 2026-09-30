import { NextResponse, type NextRequest } from "next/server";
import { isSameOrigin } from "@/lib/origin";
import { getStore } from "@/lib/store";
import { todayIST } from "@/lib/time";
import { manualTrade, parseTradeInput, tradeFromSignal } from "@/lib/trades";
import type { NewTrade } from "@/lib/types";

const fail = (status: number, error: string) => NextResponse.json({ error }, { status });

export async function POST(request: NextRequest) {
  if (!isSameOrigin(request.headers.get("origin"), request.headers.get("host"))) {
    return fail(403, "Cross-site request refused.");
  }
  const body = await request.json().catch(() => null);
  const parsed = parseTradeInput(body, todayIST());
  if (!parsed.ok) return fail(400, parsed.error);

  try {
    const store = getStore();
    const input = parsed.value;
    let row: NewTrade;
    if (input.kind === "signal") {
      // The browser only names the signal; its stock, price and day come from the database.
      const signal = await store.getSignal(input.signalId);
      if (!signal) return fail(404, "That signal no longer exists.");
      row = tradeFromSignal(signal, input.side);
    } else {
      row = manualTrade(input);
    }
    const { trade, created } = await store.addTrade(row);
    return NextResponse.json({ trade }, { status: created ? 201 : 200 });
  } catch (e) {
    return fail(500, e instanceof Error ? e.message : String(e));
  }
}
