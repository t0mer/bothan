import { useEffect, useState } from "react";
import { api, ApiError } from "../lib/api";
import type { Settings, SettingsPatch } from "../types";

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function load() {
    try {
      setError(null);
      setSettings(await api.getSettings());
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load settings");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function save(patch: SettingsPatch) {
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      setSettings(await api.updateSettings(patch));
      setNotice("Settings saved.");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "failed to save settings");
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <p className="text-sm text-slate-500">Loading…</p>;
  if (!settings) return <p className="text-sm text-red-600">{error ?? "no settings"}</p>;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Settings</h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Stored in the database and applied at runtime.
        </p>
      </div>

      {error && <Banner tone="error">{error}</Banner>}
      {notice && <Banner tone="ok">{notice}</Banner>}

      <SSLLabsSection settings={settings} onSave={save} saving={saving} />
      <LoggingSection settings={settings} onSave={save} saving={saving} />
      <MetricsSection settings={settings} onSave={save} saving={saving} />
      <ServerSection settings={settings} onSave={save} saving={saving} />
      <BootstrapSection settings={settings} />
    </div>
  );
}

function Card({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
      <h2 className="text-base font-semibold">{title}</h2>
      {description && <p className="mb-3 text-sm text-slate-500 dark:text-slate-400">{description}</p>}
      <div className="space-y-3">{children}</div>
    </section>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-slate-500 dark:text-slate-400">{hint}</span>}
    </label>
  );
}

const inputCls =
  "w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-950";

function SaveButton({ saving }: { saving: boolean }) {
  return (
    <button
      type="submit"
      disabled={saving}
      className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
    >
      {saving ? "Saving…" : "Save"}
    </button>
  );
}

function Banner({ tone, children }: { tone: "ok" | "error"; children: React.ReactNode }) {
  const cls =
    tone === "ok"
      ? "border-green-200 bg-green-50 text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-300"
      : "border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300";
  return <div className={`rounded-md border px-4 py-2 text-sm ${cls}`}>{children}</div>;
}

type SectionProps = {
  settings: Settings;
  onSave: (p: SettingsPatch) => void;
  saving: boolean;
};

function SSLLabsSection({ settings, onSave, saving }: SectionProps) {
  const [f, setF] = useState(settings.ssllabs);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSave({ ssllabs: f });
      }}
    >
      <Card title="SSL Labs" description="Qualys SSL Labs assessment settings.">
        <Field label="API version" hint="v4 requires a registered email; v3 is legacy.">
          <select className={inputCls} value={f.api_version} onChange={(e) => setF({ ...f, api_version: e.target.value })}>
            <option value="v4">v4</option>
            <option value="v3">v3</option>
          </select>
        </Field>
        <Field label="Registered email" hint="Required for API v4.">
          <input className={inputCls} value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} placeholder="you@example.com" />
        </Field>
        <div className="grid gap-3 sm:grid-cols-3">
          <Field label="Poll interval" hint="e.g. 10s">
            <input className={inputCls} value={f.poll_interval} onChange={(e) => setF({ ...f, poll_interval: e.target.value })} />
          </Field>
          <Field label="Scan timeout" hint="e.g. 20m">
            <input className={inputCls} value={f.scan_timeout} onChange={(e) => setF({ ...f, scan_timeout: e.target.value })} />
          </Field>
          <Field label="Max workers">
            <input type="number" min={1} className={inputCls} value={f.max_workers} onChange={(e) => setF({ ...f, max_workers: Number(e.target.value) })} />
          </Field>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={f.default_publish} onChange={(e) => setF({ ...f, default_publish: e.target.checked })} />
          Publish new hosts publicly by default
        </label>
        <SaveButton saving={saving} />
      </Card>
    </form>
  );
}

function LoggingSection({ settings, onSave, saving }: SectionProps) {
  const [f, setF] = useState(settings.log);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSave({ log: f });
      }}
    >
      <Card title="Logging">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Level" hint="Applied immediately.">
            <select className={inputCls} value={f.level} onChange={(e) => setF({ ...f, level: e.target.value })}>
              {["debug", "info", "warn", "error"].map((l) => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Format" hint="Restart required to change format.">
            <select className={inputCls} value={f.format} onChange={(e) => setF({ ...f, format: e.target.value })}>
              <option value="json">json</option>
              <option value="text">text</option>
            </select>
          </Field>
        </div>
        <SaveButton saving={saving} />
      </Card>
    </form>
  );
}

function MetricsSection({ settings, onSave, saving }: SectionProps) {
  const [enabled, setEnabled] = useState(settings.metrics.enabled);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSave({ metrics: { enabled } });
      }}
    >
      <Card title="Metrics" description="Restart required for changes to take effect.">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Expose Prometheus metrics at /metrics
        </label>
        <SaveButton saving={saving} />
      </Card>
    </form>
  );
}

function ServerSection({ settings, onSave, saving }: SectionProps) {
  const [f, setF] = useState({
    host: settings.server.host,
    port: settings.server.port,
    base_path: settings.server.base_path,
  });
  const overridden = settings.server.env_overridden;
  const isOver = (field: string) => overridden.includes(field);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSave({ server: f });
      }}
    >
      <Card title="Server" description="Restart required. Fields pinned by environment variables are shown read-only.">
        <div className="grid gap-3 sm:grid-cols-3">
          <Field label="Host" hint={isOver("host") ? "Overridden by environment" : undefined}>
            <input className={inputCls} disabled={isOver("host")} value={f.host} onChange={(e) => setF({ ...f, host: e.target.value })} />
          </Field>
          <Field label="Port" hint={isOver("port") ? "Overridden by environment" : undefined}>
            <input type="number" className={inputCls} disabled={isOver("port")} value={f.port} onChange={(e) => setF({ ...f, port: Number(e.target.value) })} />
          </Field>
          <Field label="Base path" hint="For reverse-proxy sub-paths.">
            <input className={inputCls} value={f.base_path} onChange={(e) => setF({ ...f, base_path: e.target.value })} />
          </Field>
        </div>
        <SaveButton saving={saving} />
      </Card>
    </form>
  );
}

function BootstrapSection({ settings }: { settings: Settings }) {
  return (
    <Card title="Bootstrap" description="Provided via environment/flags — not editable here.">
      <dl className="grid gap-2 text-sm sm:grid-cols-2">
        <div>
          <dt className="text-slate-500 dark:text-slate-400">Database path</dt>
          <dd className="font-mono">{settings.bootstrap.database_path}</dd>
        </div>
        <div>
          <dt className="text-slate-500 dark:text-slate-400">Encryption key</dt>
          <dd>
            {settings.bootstrap.encryption_key_set ? (
              <span className="rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-300">
                configured
              </span>
            ) : (
              <span className="rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700 dark:bg-amber-950 dark:text-amber-300">
                not set (set BOTHAN_CRYPTO_ENCRYPTION_KEY before adding channels)
              </span>
            )}
          </dd>
        </div>
      </dl>
    </Card>
  );
}
