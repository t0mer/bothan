import { useEffect, useState } from "react";
import { api, ApiError } from "../lib/api";
import type { Schedule } from "../types";

export default function SchedulesPage() {
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  async function refresh() {
    try {
      setError(null);
      setSchedules(await api.listSchedules());
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load schedules");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Schedules</h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Reusable cron schedules. Attach them to hosts to scan automatically.
        </p>
      </div>

      <AddScheduleForm onAdded={refresh} onError={setError} />

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
          {error}
        </div>
      )}

      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : schedules.length === 0 ? (
        <p className="rounded-md border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500 dark:border-slate-700">
          No schedules yet. Add one above.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
          <table className="w-full text-sm">
            <thead className="bg-slate-100 text-left text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              <tr>
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">Spec</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {schedules.map((s) => (
                <ScheduleRow key={s.id} schedule={s} onChanged={refresh} onError={setError} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

const inputCls =
  "w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950";

function AddScheduleForm({ onAdded, onError }: { onAdded: () => void; onError: (m: string) => void }) {
  const [name, setName] = useState("");
  const [spec, setSpec] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || !spec.trim()) return;
    setBusy(true);
    try {
      await api.createSchedule({ name: name.trim(), spec: spec.trim() });
      setName("");
      setSpec("");
      onAdded();
    } catch (e) {
      onError(e instanceof ApiError ? e.message : "failed to add schedule");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <label className="flex-1">
          <span className="mb-1 block text-sm font-medium">Name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Nightly" className={inputCls} />
        </label>
        <label className="flex-1">
          <span className="mb-1 block text-sm font-medium">Spec</span>
          <input value={spec} onChange={(e) => setSpec(e.target.value)} placeholder="Daily, @weekly, or 0 3 * * *" className={inputCls} />
        </label>
        <button
          type="submit"
          disabled={busy}
          className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        >
          {busy ? "Adding…" : "Add"}
        </button>
      </div>
      <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
        Friendly text (Everyday, Hourly, Weekly, Monthly) or standard cron (<code>min hour dom mon dow</code>) / <code>@daily</code>.
      </p>
    </form>
  );
}

function ScheduleRow({
  schedule,
  onChanged,
  onError,
}: {
  schedule: Schedule;
  onChanged: () => void;
  onError: (m: string) => void;
}) {
  async function guard(fn: () => Promise<unknown>) {
    try {
      await fn();
      onChanged();
    } catch (e) {
      onError(e instanceof Error ? e.message : "action failed");
    }
  }

  return (
    <tr className="bg-white dark:bg-slate-900">
      <td className="px-4 py-2 font-medium">{schedule.name}</td>
      <td className="px-4 py-2 font-mono text-xs">{schedule.spec}</td>
      <td className="px-4 py-2">
        <span
          className={
            schedule.enabled
              ? "rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-300"
              : "rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400"
          }
        >
          {schedule.enabled ? "Enabled" : "Disabled"}
        </span>
      </td>
      <td className="px-4 py-2">
        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={() => guard(() => api.updateSchedule(schedule.id, { name: schedule.name, spec: schedule.spec, enabled: !schedule.enabled }))}
            className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
          >
            {schedule.enabled ? "Disable" : "Enable"}
          </button>
          <button
            type="button"
            onClick={() => {
              if (confirm(`Delete schedule "${schedule.name}"?`)) guard(() => api.deleteSchedule(schedule.id));
            }}
            className="rounded border border-red-300 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950"
          >
            Delete
          </button>
        </div>
      </td>
    </tr>
  );
}
