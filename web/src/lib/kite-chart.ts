/** The stock's NSE chart on Kite web. Opens in the viewer's own Kite session (they log in there if needed). */
export function kiteChartUrl(symbol: string, token: number): string {
  return `https://kite.zerodha.com/chart/ext/ciq/NSE/${encodeURIComponent(symbol)}/${token}`;
}
