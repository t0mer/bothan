import { useEffect, useState } from "react";
import { api, ApiError } from "../lib/api";
import { gradeClasses, gradeLabel } from "../lib/grade";
import HostSchedulesDialog from "../components/HostSchedulesDialog";
import HostChannelsDialog from "../components/HostChannelsDialog";
import HostHistoryDialog from "../components/HostHistoryDialog";
import HostDialog from "../components/HostDialog";
import type { Host } from "../types";

export default function HostsPage() {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [scanning, setScanning] = useState<Set<number>>(new Set());
  const [linkHost, setLinkHost] = useState<Host | null>(null);
  const [channelHost, setChannelHost] = useState<Host | null>(null);
  const [historyHost, setHistoryHost] = useState<Host | null>(null);
  const [dialog, setDialog] = useState<{ open: boolean; host: Host | null }>({ open: false, host: null });

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
    const t = setInterval(refresh, 15000); // live refresh while scans run
    return () => clearInterval(t);
  }, []);

  async function scan(id: number) {
    setScanning((s) => new Set(s).add(id));
    try {
      await api.scanHost(id);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "failed to start scan");
    } finally {
      setScanning((s) => {
        const n = new Set(s);
        n.delete(id);
        return n;
      });
      refresh();
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Hosts</h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            {hosts.length} monitored {hosts.length === 1 ? "host" : "hosts"}
          </p>
        </div>
        <button
          type="button"
          onClick={() => setDialog({ open: true, host: null })}
          className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        >
          Add host
        </button>
      </div>

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
          {error}
        </div>
      )}

      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : hosts.length === 0 ? (
        <p className="rounded-md border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500 dark:border-slate-700">
          No hosts yet. Click “Add host” to start monitoring.
        </p>
      ) : (
        <HostsTable
          hosts={hosts}
          scanning={scanning}
          onScan={scan}
          onEdit={(h) => setDialog({ open: true, host: h })}
          onSchedules={setLinkHost}
          onChannels={setChannelHost}
          onHistory={setHistoryHost}
          onChanged={refresh}
          onError={setError}
        />
      )}

      {dialog.open && (
        <HostDialog host={dialog.host} onClose={() => setDialog({ open: false, host: null })} onSaved={refresh} />
      )}
      {linkHost && <HostSchedulesDialog host={linkHost} onClose={() => setLinkHost(null)} />}
      {channelHost && <HostChannelsDialog host={channelHost} onClose={() => setChannelHost(null)} />}
      {historyHost && <HostHistoryDialog host={historyHost} onClose={() => setHistoryHost(null)} />}
    </div>
  );
}

function statusBadge(status?: string): { label: string; cls: string } | null {
  switch (status) {
    case "running":
    case "pending":
      return { label: "scanning…", cls: "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300" };
    case "error":
      return { label: "scan error", cls: "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300" };
    default:
      return null;
  }
}

function HostsTable({
  hosts,
  scanning,
  onScan,
  onEdit,
  onSchedules,
  onChannels,
  onHistory,
  onChanged,
  onError,
}: {
  hosts: Host[];
  scanning: Set<number>;
  onScan: (id: number) => void;
  onEdit: (h: Host) => void;
  onSchedules: (h: Host) => void;
  onChannels: (h: Host) => void;
  onHistory: (h: Host) => void;
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
          {hosts.map((h) => {
            const sb = statusBadge(h.last_scan_status);
            const isScanning = scanning.has(h.id) || h.last_scan_status === "running" || h.last_scan_status === "pending";
            return (
              <tr key={h.id} className="bg-white dark:bg-slate-900">
                <td className="px-4 py-2 font-medium">
                  <button
                    type="button"
                    onClick={() => onHistory(h)}
                    className="text-slate-900 underline decoration-dotted underline-offset-4 hover:decoration-solid dark:text-slate-100"
                    title="View scan history and compare"
                  >
                    {h.hostname}
                  </button>
                </td>
                <td className="px-4 py-2">
                  <span className={`rounded px-1.5 py-0.5 text-xs font-semibold ${gradeClasses(h.latest_grade)}`}>
                    {gradeLabel(h.latest_grade)}
                  </span>
                </td>
                <td className="px-4 py-2 text-slate-500 dark:text-slate-400">
                  {h.publish ? "Public" : "Private"}
                </td>
                <td className="px-4 py-2">
                  <div className="flex items-center gap-2">
                    <span
                      className={
                        h.enabled
                          ? "rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-300"
                          : "rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                      }
                    >
                      {h.enabled ? "Enabled" : "Disabled"}
                    </span>
                    {sb && <span className={`rounded px-1.5 py-0.5 text-xs ${sb.cls}`}>{sb.label}</span>}
                  </div>
                </td>
                <td className="px-4 py-2">
                  <div className="flex justify-end gap-2">
                    <button
                      type="button"
                      disabled={isScanning}
                      onClick={() => onScan(h.id)}
                      className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:hover:bg-slate-800"
                    >
                      {isScanning ? "Scanning…" : "Scan now"}
                    </button>
                    <button
                      type="button"
                      onClick={() => onEdit(h)}
                      className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      onClick={() => onSchedules(h)}
                      className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                    >
                      Schedules
                    </button>
                    <button
                      type="button"
                      onClick={() => onChannels(h)}
                      className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                    >
                      Channels
                    </button>
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
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
