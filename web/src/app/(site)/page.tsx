import { KiteFlash } from "@/components/KiteFlash";
import { LiveSignals } from "@/components/LiveSignals";
import { baseFilter, first } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { ALL_CAP } from "@/lib/store/types";
import { todayIST } from "@/lib/time";

type SP = Promise<Record<string, string | string[] | undefined>>;

export default async function DashboardPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const today = todayIST();
  const rows = await getStore().listAll(baseFilter({ from: today, to: today }), ALL_CAP);

  return (
    <>
      <KiteFlash kite={first(sp.kite)} reason={first(sp.reason)} />
      <h1 className="text-xl font-semibold">Today · {today}</h1>
      <LiveSignals date={today} initialRows={rows} />
    </>
  );
}
