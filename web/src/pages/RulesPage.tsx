import { useEffect, useState } from "react";
import { api, ApiError } from "../lib/api";
import type { Host, Rule } from "../types";

const CONDITIONS = [
  { value: "grade_below", label: "Grade below threshold" },
  { value: "grade_changed", label: "Grade changed" },
  { value: "grade_downgraded", label: "Grade downgraded" },
  { value: "grade_improved", label: "Grade improved" },
  { value: "cert_expiry", label: "Certificate expiring" },
  { value: "scan_failed", label: "Scan failed" },
  { value: "vuln_detected", label: "Vulnerability detected" },
  { value: "scan_completed", label: "Scan completed (info)" },
];

const GRADES = ["A+", "A", "A-", "B", "C", "D", "E", "F"];

const inputCls =
  "w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950";

export default function RulesPage() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  async function refresh() {
    try {
      setError(null);
      const [r, h] = await Promise.all([api.listRules(), api.listHosts()]);
      setRules(r);
      setHosts(h);
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load rules");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  function hostName(id?: number | null): string {
    if (!id) return "All hosts";
    return hosts.find((h) => h.id === id)?.hostname ?? `host ${id}`;
  }

  async function guard(fn: () => Promise<unknown>) {
    try {
      setError(null);
      await fn();
      refresh();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "action failed");
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Rules</h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          When a rule matches after a scan, a message is sent to the host's channels.
        </p>
      </div>

      <AddRuleForm hosts={hosts} onAdded={refresh} onError={setError} />

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
          {error}
        </div>
      )}

      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : rules.length === 0 ? (
        <p className="rounded-md border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500 dark:border-slate-700">
          No rules yet. Add one above.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
          <table className="w-full text-sm">
            <thead className="bg-slate-100 text-left text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              <tr>
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">Condition</th>
                <th className="px-4 py-2 font-medium">Scope</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {rules.map((rule) => (
                <tr key={rule.id} className="bg-white dark:bg-slate-900">
                  <td className="px-4 py-2 font-medium">{rule.name}</td>
                  <td className="px-4 py-2 text-slate-500 dark:text-slate-400">
                    {rule.condition_type}
                    {rule.threshold_grade ? ` < ${rule.threshold_grade}` : ""}
                    {rule.expiry_days ? ` (${rule.expiry_days}d)` : ""}
                  </td>
                  <td className="px-4 py-2 text-slate-500 dark:text-slate-400">{hostName(rule.host_id)}</td>
                  <td className="px-4 py-2">
                    <span
                      className={
                        rule.enabled
                          ? "rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-300"
                          : "rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                      }
                    >
                      {rule.enabled ? "Enabled" : "Disabled"}
                    </span>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex justify-end gap-2">
                      <button
                        type="button"
                        onClick={() =>
                          guard(() =>
                            api.updateRule(rule.id, {
                              host_id: rule.host_id ?? null,
                              name: rule.name,
                              condition_type: rule.condition_type,
                              threshold_grade: rule.threshold_grade,
                              expiry_days: rule.expiry_days,
                              enabled: !rule.enabled,
                            }),
                          )
                        }
                        className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                      >
                        {rule.enabled ? "Disable" : "Enable"}
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          if (confirm(`Delete rule "${rule.name}"?`)) guard(() => api.deleteRule(rule.id));
                        }}
                        className="rounded border border-red-300 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950"
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function AddRuleForm({ hosts, onAdded, onError }: { hosts: Host[]; onAdded: () => void; onError: (m: string) => void }) {
  const [name, setName] = useState("");
  const [cond, setCond] = useState("grade_below");
  const [grade, setGrade] = useState("A");
  const [expiry, setExpiry] = useState(30);
  const [hostId, setHostId] = useState<string>("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const body: Record<string, unknown> = {
        name: name.trim(),
        condition_type: cond,
        host_id: hostId ? Number(hostId) : null,
      };
      if (cond === "grade_below") body.threshold_grade = grade;
      if (cond === "cert_expiry") body.expiry_days = expiry;
      await api.createRule(body);
      setName("");
      onAdded();
    } catch (e) {
      onError(e instanceof ApiError ? e.message : "failed to create rule");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3 rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
      <div className="grid gap-3 sm:grid-cols-2">
        <label>
          <span className="mb-1 block text-sm font-medium">Name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} className={inputCls} placeholder="Alert on downgrade" />
        </label>
        <label>
          <span className="mb-1 block text-sm font-medium">Condition</span>
          <select value={cond} onChange={(e) => setCond(e.target.value)} className={inputCls}>
            {CONDITIONS.map((c) => (
              <option key={c.value} value={c.value}>
                {c.label}
              </option>
            ))}
          </select>
        </label>
        {cond === "grade_below" && (
          <label>
            <span className="mb-1 block text-sm font-medium">Threshold grade</span>
            <select value={grade} onChange={(e) => setGrade(e.target.value)} className={inputCls}>
              {GRADES.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </select>
          </label>
        )}
        {cond === "cert_expiry" && (
          <label>
            <span className="mb-1 block text-sm font-medium">Days before expiry</span>
            <input type="number" min={1} value={expiry} onChange={(e) => setExpiry(Number(e.target.value))} className={inputCls} />
          </label>
        )}
        <label>
          <span className="mb-1 block text-sm font-medium">Scope</span>
          <select value={hostId} onChange={(e) => setHostId(e.target.value)} className={inputCls}>
            <option value="">All hosts (global)</option>
            {hosts.map((h) => (
              <option key={h.id} value={h.id}>
                {h.hostname}
              </option>
            ))}
          </select>
        </label>
      </div>
      <button
        type="submit"
        disabled={busy}
        className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
      >
        {busy ? "Saving…" : "Add rule"}
      </button>
    </form>
  );
}
