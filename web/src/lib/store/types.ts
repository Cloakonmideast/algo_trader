import type { Signal, SignalFilter, Status } from "@/lib/types";

export const EXPORT_CAP = 100_000;
export const ALL_CAP = 5_000;
export const MAX_PAGE_SIZE = 1_000;

export class TooManyRowsError extends Error {
  constructor(
    public readonly total: number,
    public readonly cap: number,
  ) {
    super(`${total.toLocaleString("en-US")} signals match — narrow the range to ${cap.toLocaleString("en-US")} or fewer.`);
    this.name = "TooManyRowsError";
  }
}

export interface SignalStore {
  list(f: SignalFilter, offset: number, limit: number): Promise<{ rows: Signal[]; total: number }>;
  listAll(f: SignalFilter, cap: number): Promise<Signal[]>;
  status(): Promise<Status>;
  saveKiteSession(accessToken: string, userId: string): Promise<void>;
}
