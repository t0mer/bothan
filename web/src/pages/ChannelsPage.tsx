import { useEffect, useState } from "react";
import { api, ApiError } from "../lib/api";
import type { Channel } from "../types";

const TYPES = [
  { value: "shoutrrr", label: "Shoutrrr (Telegram, Slack, Discord, SMTP…)" },
  { value: "whatsapp_greenapi", label: "GreenAPI (WhatsApp cloud)" },
  { value: "whatsapp_multidevice", label: "WhatsApp Web (self-hosted)" },
];

const inputCls =
  "w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950";

export default function ChannelsPage() {
  const [channels, setChannels] = useState<Channel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function refresh() {
    try {
      setError(null);
      setChannels(await api.listChannels());
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load channels");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function guard(fn: () => Promise<unknown>, ok?: string) {
    try {
      setError(null);
      setNotice(null);
      await fn();
      if (ok) setNotice(ok);
      refresh();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "action failed");
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Channels</h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Notification destinations. Credentials are encrypted at rest and never shown again.
        </p>
      </div>

      <AddChannelForm onAdded={refresh} onError={setError} onNotice={setNotice} />

      {error && <Banner tone="error">{error}</Banner>}
      {notice && <Banner tone="ok">{notice}</Banner>}

      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : channels.length === 0 ? (
        <p className="rounded-md border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500 dark:border-slate-700">
          No channels yet. Add one above.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
          <table className="w-full text-sm">
            <thead className="bg-slate-100 text-left text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              <tr>
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">Type</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {channels.map((c) => (
                <tr key={c.id} className="bg-white dark:bg-slate-900">
                  <td className="px-4 py-2 font-medium">{c.name}</td>
                  <td className="px-4 py-2 text-slate-500 dark:text-slate-400">{c.type}</td>
                  <td className="px-4 py-2">
                    <span
                      className={
                        c.needs_credentials
                          ? "rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700 dark:bg-amber-950 dark:text-amber-300"
                          : c.enabled
                            ? "rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-300"
                            : "rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                      }
                    >
                      {c.needs_credentials ? "Needs credentials" : c.enabled ? "Enabled" : "Disabled"}
                    </span>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex justify-end gap-2">
                      <button
                        type="button"
                        onClick={() => guard(() => api.testChannel(c.id), "Test message sent.")}
                        className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                      >
                        Test
                      </button>
                      <button
                        type="button"
                        onClick={() => guard(() => api.updateChannel(c.id, { name: c.name, type: c.type, enabled: !c.enabled }))}
                        className="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                      >
                        {c.enabled ? "Disable" : "Enable"}
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          if (confirm(`Delete channel "${c.name}"?`)) guard(() => api.deleteChannel(c.id));
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

function Banner({ tone, children }: { tone: "ok" | "error"; children: React.ReactNode }) {
  const cls =
    tone === "ok"
      ? "border-green-200 bg-green-50 text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-300"
      : "border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300";
  return <div className={`rounded-md border px-4 py-2 text-sm ${cls}`}>{children}</div>;
}

type FieldMap = Record<string, string>;

function AddChannelForm({
  onAdded,
  onError,
  onNotice,
}: {
  onAdded: () => void;
  onError: (m: string) => void;
  onNotice: (m: string) => void;
}) {
  const [name, setName] = useState("");
  const [type, setType] = useState("shoutrrr");
  const [fields, setFields] = useState<FieldMap>({});
  const [busy, setBusy] = useState(false);

  function set(k: string, v: string) {
    setFields((f) => ({ ...f, [k]: v }));
  }

  function config(): FieldMap {
    switch (type) {
      case "shoutrrr":
        return { url: fields.url ?? "" };
      case "whatsapp_greenapi":
        return {
          instance_id: fields.instance_id ?? "",
          token: fields.token ?? "",
          phone: fields.phone ?? "",
          api_url: fields.api_url ?? "",
        };
      case "whatsapp_multidevice":
        return {
          base_url: fields.base_url ?? "",
          phone: fields.phone ?? "",
          username: fields.username ?? "",
          password: fields.password ?? "",
        };
      default:
        return {};
    }
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await api.createChannel({ name: name.trim(), type, config: config() });
      setName("");
      setFields({});
      onAdded();
      onNotice("Channel created.");
    } catch (e) {
      onError(e instanceof ApiError ? e.message : "failed to create channel");
    } finally {
      setBusy(false);
    }
  }

  async function test() {
    onError("");
    try {
      await api.testChannelConfig({ type, config: config() });
      onNotice("Test message sent.");
    } catch (e) {
      onError(e instanceof ApiError ? e.message : "test failed");
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3 rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
      <div className="grid gap-3 sm:grid-cols-2">
        <label>
          <span className="mb-1 block text-sm font-medium">Name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} className={inputCls} placeholder="Ops Slack" />
        </label>
        <label>
          <span className="mb-1 block text-sm font-medium">Provider</span>
          <select value={type} onChange={(e) => { setType(e.target.value); setFields({}); }} className={inputCls}>
            {TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>
        </label>
      </div>

      {type === "shoutrrr" && (
        <label className="block">
          <span className="mb-1 block text-sm font-medium">Shoutrrr URL</span>
          <input value={fields.url ?? ""} onChange={(e) => set("url", e.target.value)} className={inputCls} placeholder="slack://token@channel" />
        </label>
      )}
      {type === "whatsapp_greenapi" && (
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Instance ID" v={fields.instance_id} on={(v) => set("instance_id", v)} />
          <Field label="Token" v={fields.token} on={(v) => set("token", v)} secret />
          <Field label="Recipient phone" v={fields.phone} on={(v) => set("phone", v)} placeholder="972501234567" />
          <Field label="API URL (optional)" v={fields.api_url} on={(v) => set("api_url", v)} placeholder="https://api.green-api.com" />
        </div>
      )}
      {type === "whatsapp_multidevice" && (
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Base URL" v={fields.base_url} on={(v) => set("base_url", v)} placeholder="http://localhost:3000" />
          <Field label="Recipient phone" v={fields.phone} on={(v) => set("phone", v)} />
          <Field label="Username (optional)" v={fields.username} on={(v) => set("username", v)} />
          <Field label="Password (optional)" v={fields.password} on={(v) => set("password", v)} secret />
        </div>
      )}

      <div className="flex gap-2">
        <button
          type="submit"
          disabled={busy}
          className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        >
          {busy ? "Saving…" : "Add channel"}
        </button>
        <button
          type="button"
          onClick={test}
          className="rounded-md border border-slate-300 px-4 py-2 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
        >
          Send test
        </button>
      </div>
    </form>
  );
}

function Field({
  label,
  v,
  on,
  secret,
  placeholder,
}: {
  label: string;
  v?: string;
  on: (v: string) => void;
  secret?: boolean;
  placeholder?: string;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium">{label}</span>
      <input
        type={secret ? "password" : "text"}
        value={v ?? ""}
        onChange={(e) => on(e.target.value)}
        placeholder={placeholder}
        className={inputCls}
      />
    </label>
  );
}
