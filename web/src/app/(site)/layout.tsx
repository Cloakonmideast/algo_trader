import { Header } from "@/components/Header";

// Every page reads live data; never prerender at build time.
export const dynamic = "force-dynamic";

export default function SiteLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Header />
      <main className="mx-auto max-w-6xl space-y-6 px-4 py-6">{children}</main>
    </>
  );
}
