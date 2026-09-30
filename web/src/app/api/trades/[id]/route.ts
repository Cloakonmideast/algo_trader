import { NextResponse, type NextRequest } from "next/server";
import { isSameOrigin } from "@/lib/origin";
import { getStore } from "@/lib/store";

export async function DELETE(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  if (!isSameOrigin(request.headers.get("origin"), request.headers.get("host"))) {
    return NextResponse.json({ error: "Cross-site request refused." }, { status: 403 });
  }
  const { id } = await params;
  const tradeId = /^\d+$/.test(id) ? Number(id) : NaN;
  if (!Number.isSafeInteger(tradeId) || tradeId <= 0) {
    return NextResponse.json({ error: "Unknown trade." }, { status: 400 });
  }
  try {
    await getStore().deleteTrade(tradeId); // deleting a missing id is not an error
  } catch (e) {
    return NextResponse.json({ error: e instanceof Error ? e.message : String(e) }, { status: 500 });
  }
  return new NextResponse(null, { status: 204 });
}
