import { useEffect, useState } from "react";
import Modal from "./Modal";
import { api } from "../lib/api";
import type { Host, Schedule } from "../types";

export default function HostSchedulesDialog({
  host,
  onClose,
}: {
  host: Host;
  onClose: () => void;
}) {
  const [all, setAll] = useState<Schedule[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const [schedules, linked] = await Promise.all([api.listSchedules(), api.hostSchedules(host.id)]);
        setAll(schedules);
        setSelected(new Set(linked.map((s) => s.id)));
      } catch (e) {
        setError(e instanceof Error ? e.message : "failed to load schedules");
      } finally {
        setLoading(false);
      }
    })();
  }, [host.id]);

  function toggle(id: number) {
    setSelected((s) => {
      const n = new Set(s);
      n.has(id) ? n.delete(id) : n.add(id);
      return n;
    });
  }

  async function save() {
    setSaving(true);
    setError(null);
    try {
      await api.setHostSchedules(host.id, [...selected]);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to save");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal title={`Schedules for ${host.hostname}`} onClose={onClose}>
      {error && <p className="mb-2 text-sm text-red-600 dark:text-red-400">{error}</p>}
      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : all.length === 0 ? (
        <p className="text-sm text-slate-500">No schedules defined yet. Create one on the Schedules page.</p>
      ) : (
        <div className="space-y-2">
          {all.map((s) => (
            <label key={s.id} className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={selected.has(s.id)} onChange={() => toggle(s.id)} />
              <span className="font-medium">{s.name}</span>
              <span className="font-mono text-xs text-slate-500 dark:text-slate-400">{s.spec}</span>
              {!s.enabled && <span className="text-xs text-slate-400">(disabled)</span>}
            </label>
          ))}
        </div>
      )}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" onClick={onClose} className="rounded-md border border-slate-300 px-3 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800">
          Cancel
        </button>
        <button
          type="button"
          onClick={save}
          disabled={saving || loading}
          className="rounded-md bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        >
          {saving ? "Saving…" : "Save"}
        </button>
      </div>
    </Modal>
  );
}
