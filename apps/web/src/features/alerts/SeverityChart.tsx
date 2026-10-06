import { Bar, BarChart, CartesianGrid, LabelList, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { ALERT_SEVERITIES, type AlertSeverity } from "@/lib/apiTypes";
import type { SeverityCounts } from "@/features/alerts/summary";

const label = (s: AlertSeverity) => s.charAt(0).toUpperCase() + s.slice(1);

const TICK = { fill: "var(--chart-text)", fontSize: 12 };

function ChartTooltip({ active, payload }: { active?: boolean; payload?: { payload?: { name: string; count: number } }[] }) {
  const row = payload?.[0]?.payload;
  if (!active || !row) return null;
  return (
    <div className="rounded-md border border-border bg-background px-3 py-2 text-sm shadow-sm">
      <p className="font-semibold">{row.count}</p>
      <p className="text-muted-foreground">{row.name}</p>
    </div>
  );
}

/**
 * SeverityChart is a single-series column chart of alert counts by severity
 * (always all four, in severity order). The chart is a visual aid only; the
 * same numbers are always present in the table below it, which is what
 * assistive technology reads.
 */
export function SeverityChart({ counts, truncated }: { counts: SeverityCounts; truncated: boolean }) {
  const data = ALERT_SEVERITIES.map((s) => ({ name: label(s), count: counts[s] }));
  const total = data.reduce((sum, d) => sum + d.count, 0);

  return (
    <figure className="space-y-3">
      <div aria-hidden="true" className="h-56 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 20, right: 8, bottom: 0, left: 0 }} accessibilityLayer={false}>
            <CartesianGrid vertical={false} stroke="var(--chart-grid)" strokeWidth={1} />
            <XAxis dataKey="name" tick={TICK} tickLine={false} axisLine={{ stroke: "var(--chart-axis)" }} />
            <YAxis allowDecimals={false} tick={TICK} tickLine={false} axisLine={false} width={32} />
            <Tooltip content={<ChartTooltip />} cursor={{ fill: "var(--chart-grid)", fillOpacity: 0.4 }} />
            <Bar
              dataKey="count"
              fill="var(--chart-bar)"
              maxBarSize={24}
              radius={[4, 4, 0, 0]}
              activeBar={{ fill: "var(--chart-bar-hover)" }}
              isAnimationActive={false}
            >
              <LabelList dataKey="count" position="top" fill="var(--chart-text)" fontSize={12} />
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>
      <figcaption className="text-sm text-muted-foreground">
        {total === 0 ? "No open alerts. " : ""}
        {truncated ? "Based on the 200 most recent open alerts." : ""}
      </figcaption>
      <table className="w-full max-w-sm text-left text-sm">
        <caption className="sr-only">Open alerts by severity</caption>
        <thead>
          <tr className="border-b border-border">
            <th scope="col" className="py-1 pr-4 font-medium">Severity</th>
            <th scope="col" className="py-1 text-right font-medium">Open alerts</th>
          </tr>
        </thead>
        <tbody>
          {data.map((d) => (
            <tr key={d.name} className="border-b border-border last:border-0">
              <th scope="row" className="py-1 pr-4 font-normal">{d.name}</th>
              <td className="py-1 text-right tabular-nums">{d.count}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
