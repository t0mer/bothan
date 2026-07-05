import { useEffect, useState } from "react";
import { api, ApiError } from "../lib/api";
import type { Host } from "../types";

export default function HostsPage() {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  async function refresh() {
    try {
      setError(null);
      setHosts(await api.listHosts());
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load hosts");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Hosts</h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            {hosts.length} monitored {hosts.length === 1 ? "host" : "hosts"}
          </p>
        </div>
      </div>

      <AddHostForm onAdded={refresh} />

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
          {error}
        </div>
      )}

      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : hosts.length === 0 ? (
        <p className="rounded-md border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500 dark:border-slate-700">
          No hosts yet. Add one above to start monitoring.
        </p>
      ) : (
        <HostsTable hosts={hosts} onChanged={refresh} onError={setError} />
      )}
    </div>
  );
}

function AddHostForm({ onAdded }: { onAdded: () => void }) {
  const [hostname, setHostname] = useState("");
  const [publish, setPublish] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!hostname.trim()) return;
    setBusy(true);
    setErr(null);
    try {
      await api.createHost({ hostname: hostname.trim(), publish });
      setHostname("");
      setPublish(false);
      onAdded();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "failed to add host");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form
      onSubmit={submit}
      className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <label className="flex-1">
          <span className="mb-1 block text-sm font-medium">Hostname</span>
          <input
            value={hostname}
            onChange={(e) => setHostname(e.target.value)}
            placeholder="example.com"
            className="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950"
          />
        </label>
        <label className="flex items-center gap-2 pb-2 text-sm">
          <input type="checkbox" checked={publish} onChange={(e) => setPublish(e.target.checked)} />
          Public scan
        </label>
        <button
          type="submit"
          disabled={busy}
          className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        >
          {busy ? "Adding…" : "Add host"}
        </button>
      </div>
      {err && <p className="mt-2 text-sm text-red-600 dark:text-red-400">{err}</p>}
    </form>
  );
}

function HostsTable({
  hosts,
  onChanged,
  onError,
}: {
  hosts: Host[];
  onChanged: () => void;
  onError: (msg: string) => void;
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
    <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
      <table className="w-full text-sm">
        <thead className="bg-slate-100 text-left text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          <tr>
            <th className="px-4 py-2 font-medium">Hostname</th>
            <th className="px-4 py-2 font-medium">Grade</th>
            <th className="px-4 py-2 font-medium">Visibility</th>
            <th className="px-4 py-2 font-medium">Status</th>
            <th className="px-4 py-2 text-right font-medium">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
          {hosts.map((h) => (
            <tr key={h.id} className="bg-white dark:bg-slate-900">
              <td className="px-4 py-2 font-medium">{h.hostname}</td>
              <td className="px-4 py-2">
                <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                  n/a
                </span>
              </td>
              <td className="px-4 py-2 text-slate-500 dark:text-slate-400">
                {h.publish ? "Public" : "Private"}
              </td>
              <td className="px-4 py-2">
                <span
                  className={
                    h.enabled
                      ? "rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-300"
                      : "rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                  }
                >
                  {h.enabled ? "Enabled" : "Disabled"}
                </span>
              </td>
              <td className="px-4 py-2">
                <div className="flex justify-end gap-2">
                  <button
                    type="button"
                    onClick={() => guard(() => (h.enabled ? api.disableHost(h.id) : api.enableHost(h.id)))}
                    className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                  >
                    {h.enabled ? "Disable" : "Enable"}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      if (confirm(`Delete ${h.hostname}? This removes its scan history.`)) {
                        guard(() => api.deleteHost(h.id));
                      }
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
  );
}
