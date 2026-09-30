import "server-only";
import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import { tokenValidToday } from "@/lib/time";
import { chunk, tradeMarksFrom } from "@/lib/trades";
import type { NewTrade, Side, Signal, SignalFilter, Status, Trade, TradeFilter, TradeMarks } from "@/lib/types";
import { SIGNAL_COLUMNS, applyFilterToQuery, toSignal, type FilterableQuery, type SignalRow } from "./query";
import { TRADE_COLUMNS, applyTradeFilter, toTrade, type TradeRow } from "./trade-query";
import { MARKS_BATCH, MAX_PAGE_SIZE, TooManyRowsError, type SignalStore } from "./types";

export class SupabaseStore implements SignalStore {
  private db: SupabaseClient;

  constructor(url: string, serviceKey: string) {
    this.db = createClient(url, serviceKey, { auth: { persistSession: false, autoRefreshToken: false } });
  }

  async list(f: SignalFilter, offset: number, limit: number): Promise<{ rows: Signal[]; total: number }> {
    const base = this.db.from("signals").select(SIGNAL_COLUMNS, { count: "exact" });
    // Every PostgREST builder method returns the same builder, so it can be driven through the
    // narrow FilterableQuery interface and cast back to keep .range().
    const query = applyFilterToQuery(base as unknown as FilterableQuery, f) as unknown as typeof base;
    const { data, count, error } = await query.range(offset, offset + limit - 1);
    if (error) throw new Error(`Supabase: ${error.message}`);
    return { rows: (data as unknown as SignalRow[]).map(toSignal), total: count ?? 0 };
  }

  async listAll(f: SignalFilter, cap: number): Promise<Signal[]> {
    const rows: Signal[] = [];
    for (let offset = 0; ; offset += MAX_PAGE_SIZE) {
      const page = await this.list(f, offset, MAX_PAGE_SIZE);
      if (page.total > cap) throw new TooManyRowsError(page.total, cap);
      rows.push(...page.rows);
      if (page.rows.length < MAX_PAGE_SIZE || rows.length >= page.total) return rows;
    }
  }

  async status(): Promise<Status> {
    const [kite, engine] = await Promise.all([
      // Never select access_token: the browser must not be able to learn it.
      this.db.from("kite_session").select("created_at").eq("id", 1).maybeSingle(),
      this.db.from("engine_status").select("last_seen,instruments,feed_connected,token_valid").eq("id", 1).maybeSingle(),
    ]);
    if (kite.error) throw new Error(`Supabase: ${kite.error.message}`);
    if (engine.error) throw new Error(`Supabase: ${engine.error.message}`);

    const createdAt = (kite.data?.created_at as string | undefined) ?? null;
    const e = engine.data;
    return {
      tokenCreatedAt: createdAt,
      tokenValidToday: tokenValidToday(createdAt),
      engine: e
        ? { lastSeen: e.last_seen, instruments: e.instruments, feedConnected: e.feed_connected, tokenValid: e.token_valid }
        : null,
    };
  }

  async saveKiteSession(accessToken: string, userId: string): Promise<void> {
    const { error } = await this.db
      .from("kite_session")
      .upsert({ id: 1, access_token: accessToken, user_id: userId, created_at: new Date().toISOString() });
    if (error) throw new Error(`Supabase: ${error.message}`);
  }

  async getSignal(id: number): Promise<Signal | null> {
    const { data, error } = await this.db.from("signals").select(SIGNAL_COLUMNS).eq("id", id).maybeSingle();
    if (error) throw new Error(`Supabase: ${error.message}`);
    return data ? toSignal(data as unknown as SignalRow) : null;
  }

  async addTrade(t: NewTrade): Promise<{ trade: Trade; created: boolean }> {
    const row = {
      trade_date: t.tradeDate,
      symbol: t.symbol,
      side: t.side,
      price: t.price,
      signal_id: t.signalId,
      signal: t.signal,
    };
    if (t.signalId === null) {
      const { data, error } = await this.db.from("trades").insert(row).select(TRADE_COLUMNS).single();
      if (error) throw new Error(`Supabase: ${error.message}`);
      return { trade: toTrade(data as unknown as TradeRow), created: true };
    }

    // One buy and one sell per signal: a repeat click must not error, so skip the
    // duplicate and hand back the trade that is already there.
    const { data, error } = await this.db
      .from("trades")
      .upsert(row, { onConflict: "signal_id,side", ignoreDuplicates: true })
      .select(TRADE_COLUMNS);
    if (error) throw new Error(`Supabase: ${error.message}`);
    const inserted = data as unknown as TradeRow[];
    if (inserted.length > 0) return { trade: toTrade(inserted[0]), created: true };

    const existing = await this.db
      .from("trades")
      .select(TRADE_COLUMNS)
      .eq("signal_id", t.signalId)
      .eq("side", t.side)
      .single();
    if (existing.error) throw new Error(`Supabase: ${existing.error.message}`);
    return { trade: toTrade(existing.data as unknown as TradeRow), created: false };
  }

  async deleteTrade(id: number): Promise<void> {
    const { error } = await this.db.from("trades").delete().eq("id", id);
    if (error) throw new Error(`Supabase: ${error.message}`);
  }

  async listTrades(f: TradeFilter, cap: number): Promise<{ rows: Trade[]; total: number }> {
    const base = this.db.from("trades").select(TRADE_COLUMNS, { count: "exact" });
    const query = applyTradeFilter(base as unknown as FilterableQuery, f) as unknown as typeof base;
    const { data, count, error } = await query.range(0, cap - 1);
    if (error) throw new Error(`Supabase: ${error.message}`);
    return { rows: (data as unknown as TradeRow[]).map(toTrade), total: count ?? 0 };
  }

  async tradeMarks(signalIds: number[]): Promise<TradeMarks> {
    const rows: { id: number; signalId: number | null; side: Side }[] = [];
    for (const ids of chunk([...new Set(signalIds)], MARKS_BATCH)) {
      const { data, error } = await this.db.from("trades").select("id,signal_id,side").in("signal_id", ids);
      if (error) throw new Error(`Supabase: ${error.message}`);
      for (const r of data as { id: number; signal_id: number | null; side: Side }[]) {
        rows.push({ id: r.id, signalId: r.signal_id, side: r.side });
      }
    }
    return tradeMarksFrom(rows);
  }
}
