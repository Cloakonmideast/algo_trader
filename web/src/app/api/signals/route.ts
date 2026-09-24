import { NextResponse, type NextRequest } from "next/server";
import { parseFilter } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { ALL_CAP, MAX_PAGE_SIZE, TooManyRowsError } from "@/lib/store/types";

const int = (v: string | null, fallback: number) => {
  const n = Number.parseInt(v ?? "", 10);
  return Number.isFinite(n) ? n : fallback;
};

export async function GET(request: NextRequest) {
  const p = request.nextUrl.searchParams;
  const filter = parseFilter(p);
  const store = getStore();

  try {
    if (p.get("all") === "1") {
      const rows = await store.listAll(filter, ALL_CAP);
      return NextResponse.json({ rows, total: rows.length });
    }
    const pageSize = Math.min(Math.max(int(p.get("pageSize"), 100), 1), MAX_PAGE_SIZE);
    const page = Math.max(int(p.get("page"), 1), 1);
    const { rows, total } = await store.list(filter, (page - 1) * pageSize, pageSize);
    return NextResponse.json({ rows, total, page, pageSize });
  } catch (e) {
    if (e instanceof TooManyRowsError) return NextResponse.json({ error: e.message }, { status: 413 });
    throw e;
  }
}
