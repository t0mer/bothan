import { useState } from "react";
import Modal from "./Modal";
import { api, ApiError } from "../lib/api";
import type { Channel } from "../types";

const inputCls =
  "w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950";

const TYPES = [
  { value: "shoutrrr", label: "Shoutrrr (Telegram, Slack, Discord, SMTP…)" },
  { value: "whatsapp_greenapi", label: "GreenAPI (WhatsApp cloud)" },
  { value: "whatsapp_multidevice", label: "WhatsApp Web (self-hosted)" },
];

type Fields = Record<string, string>;

export default function ChannelEditDialog({
  channel,
  onClose,
  onSaved,
}: {
  channel: Channel;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(channel.name);
  const [type, setType] = useState(channel.type);
  const [enabled, setEnabled] = useState(channel.enabled);
  const [fields, setFields] = useState<Fields>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const set = (k: string, v: string) => setFields((f) => ({ ...f, [k]: v }));

  // Build config only if the user actually entered something (else keep current).
  function config(): Fields | null {
    const has = Object.values(fields).some((v) => v.trim() !== "");
    if (!has) return null;
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
        return null;
    }
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const cfg = config();
      const body: Record<string, unknown> = { name: name.trim(), type, enabled };
      if (cfg) body.config = cfg;
      await api.updateChannel(channel.id, body);
      onSaved();
      onClose();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "failed to save channel");
    } finally {
      setBusy(false);
    }
  }

  async function test() {
    setError(null);
    setNotice(null);
    try {
      const cfg = config();
      if (cfg) {
        await api.testChannelConfig({ type, config: cfg });
      } else {
        await api.testChannel(channel.id); // test the stored config
      }
      setNotice("Test message sent.");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "test failed");
    }
  }

  return (
    <Modal title={`Edit ${channel.name}`} onClose={onClose} size="lg">
      <form onSubmit={save} className="space-y-3">
        {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
        {notice && <p className="text-sm text-green-600 dark:text-green-400">{notice}</p>}

        <div className="grid gap-3 sm:grid-cols-2">
          <label>
            <span className="mb-1 block text-sm font-medium">Name</span>
            <input value={name} onChange={(e) => setName(e.target.value)} className={inputCls} />
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

        <p className="text-xs text-slate-500 dark:text-slate-400">
          Leave credential fields blank to keep the current credentials. Enter them to replace, or if you
          changed the provider.
        </p>

        {type === "shoutrrr" && (
          <Field label="Shoutrrr URL" v={fields.url} on={(v) => set("url", v)} placeholder="slack://token@channel" />
        )}
        {type === "whatsapp_greenapi" && (
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Instance ID" v={fields.instance_id} on={(v) => set("instance_id", v)} />
            <Field label="Token" v={fields.token} on={(v) => set("token", v)} secret />
            <Field label="Recipient phone" v={fields.phone} on={(v) => set("phone", v)} />
            <Field label="API URL (optional)" v={fields.api_url} on={(v) => set("api_url", v)} />
          </div>
        )}
        {type === "whatsapp_multidevice" && (
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Base URL" v={fields.base_url} on={(v) => set("base_url", v)} />
            <Field label="Recipient phone" v={fields.phone} on={(v) => set("phone", v)} />
            <Field label="Username (optional)" v={fields.username} on={(v) => set("username", v)} />
            <Field label="Password (optional)" v={fields.password} on={(v) => set("password", v)} secret />
          </div>
        )}

        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Enabled
        </label>

        <div className="flex justify-end gap-2">
          <button type="button" onClick={test} className="rounded-md border border-slate-300 px-4 py-2 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800">
            Send test
          </button>
          <button type="submit" disabled={busy} className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white">
            {busy ? "Saving…" : "Save changes"}
          </button>
        </div>
      </form>
    </Modal>
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
