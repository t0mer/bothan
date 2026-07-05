import { useEffect, useState } from "react";
import Modal from "./Modal";
import { api, ApiError } from "../lib/api";
import type { Host } from "../types";

const inputCls =
  "w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950";

const GRADES = ["A+", "A", "A-", "B", "C", "D", "E", "F"];

// The notification conditions a user can pick per host.
const CONDITIONS: { key: string; label: string }[] = [
  { key: "grade_below", label: "Grade drops below a threshold" },
  { key: "grade_changed", label: "Grade changes" },
  { key: "grade_downgraded", label: "Grade gets worse" },
  { key: "grade_improved", label: "Grade improves" },
  { key: "cert_expiry", label: "Certificate is expiring soon" },
  { key: "scan_failed", label: "A scan fails" },
  { key: "vuln_detected", label: "A vulnerability is detected" },
  { key: "scan_completed", label: "Every completed scan (informational)" },
];

type NotifyState = {
  selected: Set<string>;
  threshold: string; // for grade_below
  expiryDays: number; // for cert_expiry
};

export default function HostDialog({
  host,
  onClose,
  onSaved,
}: {
  host: Host | null; // null = add mode
  onClose: () => void;
  onSaved: () => void;
}) {
  const editing = host !== null;
  const [hostname, setHostname] = useState(host?.hostname ?? "");
  const [publish, setPublish] = useState(host?.publish ?? false);
  const [ignoreMismatch, setIgnoreMismatch] = useState(host?.ignore_mismatch ?? false);
  const [fromCache, setFromCache] = useState(host?.from_cache ?? false);
  const [maxAge, setMaxAge] = useState<number>(host?.max_age_hours ?? 24);
  const [notes, setNotes] = useState(host?.notes ?? "");

  const [notify, setNotify] = useState<NotifyState>({ selected: new Set(), threshold: "A", expiryDays: 30 });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load existing per-host notification rules when editing.
  useEffect(() => {
    if (!editing) return;
    (async () => {
      try {
        const rules = await api.hostRules(host!.id);
        const selected = new Set(rules.map((r) => r.condition_type));
        const below = rules.find((r) => r.condition_type === "grade_below");
        const cert = rules.find((r) => r.condition_type === "cert_expiry");
        setNotify({
          selected,
          threshold: below?.threshold_grade || "A",
          expiryDays: cert?.expiry_days || 30,
        });
      } catch {
        /* leave defaults */
      }
    })();
  }, [editing, host]);

  function toggle(key: string) {
    setNotify((n) => {
      const s = new Set(n.selected);
      s.has(key) ? s.delete(key) : s.add(key);
      return { ...n, selected: s };
    });
  }

  function buildRules() {
    return [...notify.selected].map((c) => {
      const spec: { condition_type: string; threshold_grade?: string; expiry_days?: number } = { condition_type: c };
      if (c === "grade_below") spec.threshold_grade = notify.threshold;
      if (c === "cert_expiry") spec.expiry_days = notify.expiryDays;
      return spec;
    });
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!hostname.trim()) {
      setError("hostname is required");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const payload = {
        hostname: hostname.trim(),
        publish,
        ignore_mismatch: ignoreMismatch,
        from_cache: fromCache,
        max_age_hours: fromCache ? maxAge : undefined,
        notes: notes.trim(),
      };
      const saved = editing ? await api.updateHost(host!.id, payload) : await api.createHost(payload);
      await api.setHostRules(saved.id, buildRules());
      onSaved();
      onClose();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "failed to save host");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={editing ? `Edit ${host!.hostname}` : "Add host"} onClose={onClose} size="lg">
      <form onSubmit={save} className="space-y-4">
        {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}

        <label className="block">
          <span className="mb-1 block text-sm font-medium">Hostname</span>
          <input value={hostname} onChange={(e) => setHostname(e.target.value)} placeholder="example.com" className={inputCls} autoFocus={!editing} />
        </label>

        <div className="space-y-2">
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={publish} onChange={(e) => setPublish(e.target.checked)} />
            Public scan (results published to the SSL Labs boards)
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={ignoreMismatch} onChange={(e) => setIgnoreMismatch(e.target.checked)} />
            Ignore certificate hostname mismatch
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={fromCache} onChange={(e) => setFromCache(e.target.checked)} />
            Allow cached results
            {fromCache && (
              <>
                <span className="ml-1 text-slate-500 dark:text-slate-400">max age (h):</span>
                <input type="number" min={1} value={maxAge} onChange={(e) => setMaxAge(Number(e.target.value))} className="w-20 rounded-md border border-slate-300 bg-white px-2 py-1 text-sm dark:border-slate-700 dark:bg-slate-950" />
              </>
            )}
          </label>
        </div>

        <label className="block">
          <span className="mb-1 block text-sm font-medium">Notes</span>
          <input value={notes} onChange={(e) => setNotes(e.target.value)} className={inputCls} />
        </label>

        <div className="rounded-md border border-slate-200 p-3 dark:border-slate-800">
          <p className="mb-2 text-sm font-medium">Notify me when…</p>
          <p className="mb-3 text-xs text-slate-500 dark:text-slate-400">
            Notifications go to this host's linked channels. Manage channels with the “Channels” button.
          </p>
          <div className="space-y-2">
            {CONDITIONS.map((c) => (
              <div key={c.key} className="flex flex-wrap items-center gap-2 text-sm">
                <label className="flex items-center gap-2">
                  <input type="checkbox" checked={notify.selected.has(c.key)} onChange={() => toggle(c.key)} />
                  {c.label}
                </label>
                {c.key === "grade_below" && notify.selected.has(c.key) && (
                  <select value={notify.threshold} onChange={(e) => setNotify((n) => ({ ...n, threshold: e.target.value }))} className="rounded-md border border-slate-300 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-950">
                    {GRADES.map((g) => (
                      <option key={g} value={g}>
                        below {g}
                      </option>
                    ))}
                  </select>
                )}
                {c.key === "cert_expiry" && notify.selected.has(c.key) && (
                  <span className="flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400">
                    within
                    <input type="number" min={1} value={notify.expiryDays} onChange={(e) => setNotify((n) => ({ ...n, expiryDays: Number(e.target.value) }))} className="w-16 rounded-md border border-slate-300 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-950" />
                    days
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-md border border-slate-300 px-4 py-2 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800">
            Cancel
          </button>
          <button type="submit" disabled={busy} className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white">
            {busy ? "Saving…" : editing ? "Save changes" : "Add host"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
