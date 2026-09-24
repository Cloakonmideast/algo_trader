import "server-only";
import { SupabaseStore } from "./supabase";
import type { SignalStore } from "./types";

// One client per server instance.
const g = globalThis as unknown as { __signalStore?: SignalStore };

export function getStore(): SignalStore {
  if (g.__signalStore) return g.__signalStore;
  const url = process.env.SUPABASE_URL;
  const key = process.env.SUPABASE_SERVICE_KEY;
  if (!url || !key) {
    throw new Error(
      "SUPABASE_URL and SUPABASE_SERVICE_KEY are not set. Add them to web/.env.local (local) or the Vercel project's Environment Variables.",
    );
  }
  g.__signalStore = new SupabaseStore(url, key);
  return g.__signalStore;
}
