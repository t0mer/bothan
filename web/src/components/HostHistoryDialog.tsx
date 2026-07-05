import { useEffect, useState } from "react";
import Modal from "./Modal";
import { api } from "../lib/api";
import { gradeClasses, gradeLabel } from "../lib/grade";
import type { Host, Scan, ScanDiff } from "../types";

export default function HostHistoryDialog({ host, onClose }: { host: Host; onClose: () => void }) {
  const [scans, setScans] = useState<Scan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [picked, setPicked] = useState<number[]>([]);
  const [diff, setDiff] = useState<ScanDiff | null>(null);

  useEffect(() => {
    (async () => {
      try {
        setScans(await api.hostScans(host.id));
      } catch (e) {
        setError(e instanceof Error ? e.message : "failed to load scans");
      } finally {
        setLoading(false);
      }
    })();
  }, [host.id]);

  function togglePick(id: number) {
    setDiff(null);
    setPicked((p) => {
      if (p.includes(id)) return p.filter((x) => x !== id);
      if (p.length >= 2) return [p[1], id];
      return [...p, id];
    });
  }

  async function compare() {
    if (picked.length !== 2) return;
    try {
      setError(null);
      // Compare older → newer for readable direction.
      const [a, b] = [...picked].sort((x, y) => x - y);
      setDiff(await api.compareScans(a, b));
    } catch (e) {
      setError(e instanceof Error ? e.message : "compare failed");
    }
  }

  return (
    <Modal title={`Scan history — ${host.hostname}`} onClose={onClose}>
      {error && <p className="mb-2 text-sm text-red-600 dark:text-red-400">{error}</p>}
      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : scans.length === 0 ? (
        <p className="text-sm text-slate-500">No scans yet. Trigger one from the Hosts page.</p>
      ) : (
        <>
          <p className="mb-2 text-xs text-slate-500 dark:text-slate-400">Select two scans to compare.</p>
          <div className="max-h-64 overflow-y-auto rounded-md border border-slate-200 dark:border-slate-800">
            <table className="w-full text-sm">
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {scans.map((s) => (
                  <tr key={s.id} className="bg-white dark:bg-slate-900">
                    <td className="px-3 py-1.5">
                      <input type="checkbox" checked={picked.includes(s.id)} onChange={() => togglePick(s.id)} />
                    </td>
                    <td className="px-3 py-1.5 text-xs text-slate-500 dark:text-slate-400">
                      {new Date(s.created_at).toLocaleString()}
                    </td>
                    <td className="px-3 py-1.5">
                      <span className={`rounded px-1.5 py-0.5 text-xs font-semibold ${gradeClasses(s.overall_grade)}`}>
                        {gradeLabel(s.overall_grade)}
                      </span>
                    </td>
                    <td className="px-3 py-1.5 text-xs text-slate-500 dark:text-slate-400">{s.status}</td>
                    <td className="px-3 py-1.5 text-xs text-slate-500 dark:text-slate-400">{s.trigger}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="mt-3 flex justify-end">
            <button
              type="button"
              onClick={compare}
              disabled={picked.length !== 2}
              className="rounded-md bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-40 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
            >
              Compare selected
            </button>
          </div>
          {diff && <DiffView diff={diff} />}
        </>
      )}
    </Modal>
  );
}

function DiffView({ diff }: { diff: ScanDiff }) {
  return (
    <div className="mt-4 space-y-3 border-t border-slate-200 pt-3 dark:border-slate-800">
      <div className="flex items-center gap-2 text-sm">
        <span>Overall grade:</span>
        <span className={`rounded px-1.5 py-0.5 text-xs font-semibold ${gradeClasses(diff.from.grade)}`}>
          {gradeLabel(diff.from.grade)}
        </span>
        <span>→</span>
        <span className={`rounded px-1.5 py-0.5 text-xs font-semibold ${gradeClasses(diff.to.grade)}`}>
          {gradeLabel(diff.to.grade)}
        </span>
        {diff.overall_grade_changed && <span className="text-xs text-amber-600 dark:text-amber-400">changed</span>}
      </div>
      <div className="space-y-2">
        {diff.endpoints.map((e) => (
          <div key={e.ip_address} className="rounded-md border border-slate-200 p-2 text-sm dark:border-slate-800">
            <div className="flex items-center gap-2">
              <span className="font-mono text-xs">{e.ip_address}</span>
              <ChangeBadge change={e.change} />
              {e.grade_changed && (
                <span className="text-xs">
                  {gradeLabel(e.from_grade)} → {gradeLabel(e.to_grade)}
                </span>
              )}
            </div>
            {e.cert_changed && <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">certificate changed</p>}
            <DiffList label="Protocols added" items={e.protocols_added} tone="green" />
            <DiffList label="Protocols removed" items={e.protocols_removed} tone="red" />
            <DiffList label="Vulnerabilities added" items={e.vulns_added} tone="red" />
            <DiffList label="Vulnerabilities removed" items={e.vulns_removed} tone="green" />
          </div>
        ))}
      </div>
    </div>
  );
}

function ChangeBadge({ change }: { change: string }) {
  const map: Record<string, string> = {
    added: "bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300",
    removed: "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300",
    changed: "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300",
    unchanged: "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400",
  };
  return <span className={`rounded px-1.5 py-0.5 text-xs ${map[change] ?? map.unchanged}`}>{change}</span>;
}

function DiffList({ label, items, tone }: { label: string; items?: string[]; tone: "green" | "red" }) {
  if (!items || items.length === 0) return null;
  const cls = tone === "green" ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400";
  return (
    <p className={`mt-1 text-xs ${cls}`}>
      {label}: {items.join(", ")}
    </p>
  );
}
