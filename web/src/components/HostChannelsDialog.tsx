import { useEffect, useState } from "react";
import Modal from "./Modal";
import { api } from "../lib/api";
import type { Channel, Host } from "../types";

export default function HostChannelsDialog({ host, onClose }: { host: Host; onClose: () => void }) {
  const [all, setAll] = useState<Channel[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const [channels, linked] = await Promise.all([api.listChannels(), api.hostChannels(host.id)]);
        setAll(channels);
        setSelected(new Set(linked.map((c) => c.id)));
      } catch (e) {
        setError(e instanceof Error ? e.message : "failed to load channels");
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
      await api.setHostChannels(host.id, [...selected]);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to save");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal title={`Channels for ${host.hostname}`} onClose={onClose}>
      {error && <p className="mb-2 text-sm text-red-600 dark:text-red-400">{error}</p>}
      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : all.length === 0 ? (
        <p className="text-sm text-slate-500">No channels defined yet. Create one on the Channels page.</p>
      ) : (
        <div className="space-y-2">
          {all.map((c) => (
            <label key={c.id} className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={selected.has(c.id)} onChange={() => toggle(c.id)} />
              <span className="font-medium">{c.name}</span>
              <span className="text-xs text-slate-500 dark:text-slate-400">{c.type}</span>
              {c.needs_credentials && <span className="text-xs text-amber-600">(needs credentials)</span>}
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
