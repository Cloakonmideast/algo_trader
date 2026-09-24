import "server-only";
import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import { tokenValidToday } from "@/lib/time";
import type { Signal, SignalFilter, Status } from "@/lib/types";
import { SIGNAL_COLUMNS, applyFilterToQuery, toSignal, type FilterableQuery, type SignalRow } from "./query";
import { MAX_PAGE_SIZE, TooManyRowsError, type SignalStore } from "./types";

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
}
