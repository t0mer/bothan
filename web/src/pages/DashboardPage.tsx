import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { gradeClasses, gradeLabel } from "../lib/grade";
import type { DashboardSummary } from "../types";

export default function DashboardPage() {
  const [data, setData] = useState<DashboardSummary | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function refresh() {
    try {
      setError(null);
      setData(await api.dashboard());
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load dashboard");
    }
  }

  useEffect(() => {
    refresh();
    const t = setInterval(refresh, 60000);
    return () => clearInterval(t);
  }, []);

  if (error) return <p className="text-sm text-red-600 dark:text-red-400">{error}</p>;
  if (!data) return <p className="text-sm text-slate-500">Loading…</p>;

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Dashboard</h1>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Total hosts" value={data.total_hosts} />
        <StatCard label="Enabled" value={data.enabled_hosts} tone="green" />
        <StatCard label="Disabled" value={data.disabled_hosts} tone="slate" />
        <StatCard label="Never scanned" value={data.never_scanned} tone="amber" />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card title="Hosts by grade">
          {data.grade_counts.length === 0 ? (
            <Empty>No scans yet.</Empty>
          ) : (
            <table className="w-full text-sm">
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {data.grade_counts.map((g) => (
                  <tr key={g.grade}>
                    <td className="py-1.5">
                      <span className={`rounded px-1.5 py-0.5 text-xs font-semibold ${gradeClasses(g.grade === "none" ? "" : g.grade)}`}>
                        {g.grade === "none" ? "n/a" : g.grade}
                      </span>
                    </td>
                    <td className="py-1.5 text-right font-medium">{g.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>

        <Card title={`Certificates expiring within ${data.cert_expiry_window_days} days`}>
          {data.certs_expiring_soon.length === 0 ? (
            <Empty>No certificates expiring soon.</Empty>
          ) : (
            <table className="w-full text-sm">
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {data.certs_expiring_soon.map((c) => (
                  <tr key={c.host_id}>
                    <td className="py-1.5 font-medium">{c.hostname}</td>
                    <td className="py-1.5 text-right">
                      <span className={c.days <= 7 ? "text-red-600 dark:text-red-400" : "text-amber-600 dark:text-amber-400"}>
                        {c.days}d ({new Date(c.not_after).toLocaleDateString()})
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      </div>

      <Card title="Recent scans">
        {data.recent_scans.length === 0 ? (
          <Empty>No scans yet.</Empty>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="text-left text-slate-500 dark:text-slate-400">
                <tr>
                  <th className="py-1.5 font-medium">Host</th>
                  <th className="py-1.5 font-medium">Grade</th>
                  <th className="py-1.5 font-medium">Status</th>
                  <th className="py-1.5 font-medium">When</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {data.recent_scans.map((s) => (
                  <tr key={s.scan_id}>
                    <td className="py-1.5 font-medium">{s.hostname}</td>
                    <td className="py-1.5">
                      <span className={`rounded px-1.5 py-0.5 text-xs font-semibold ${gradeClasses(s.grade)}`}>
                        {gradeLabel(s.grade)}
                      </span>
                    </td>
                    <td className="py-1.5 text-slate-500 dark:text-slate-400">{s.status}</td>
                    <td className="py-1.5 text-slate-500 dark:text-slate-400">
                      {new Date(s.completed_at ?? s.created_at).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}

function StatCard({ label, value, tone }: { label: string; value: number; tone?: "green" | "amber" | "slate" }) {
  const toneCls =
    tone === "green"
      ? "text-green-600 dark:text-green-400"
      : tone === "amber"
        ? "text-amber-600 dark:text-amber-400"
        : "text-slate-900 dark:text-slate-100";
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
      <div className={`text-3xl font-semibold ${toneCls}`}>{value}</div>
      <div className="mt-1 text-sm text-slate-500 dark:text-slate-400">{label}</div>
    </div>
  );
}

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
      <h2 className="mb-3 text-base font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <p className="text-sm text-slate-500 dark:text-slate-400">{children}</p>;
}
