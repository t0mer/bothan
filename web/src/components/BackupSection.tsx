import { useState } from "react";
import { api, ApiError } from "../lib/api";
import type { ImportResult } from "../types";

const inputCls =
  "w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950";

export default function BackupSection() {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
      <h2 className="text-base font-semibold">Backup / Migrate</h2>
      <p className="mb-3 text-sm text-slate-500 dark:text-slate-400">
        Export the configuration (hosts, schedules, channels, rules, and links) and import it into
        another instance. Scan history is not included.
      </p>
      <div className="grid gap-6 lg:grid-cols-2">
        <ExportPanel />
        <ImportPanel />
      </div>
    </section>
  );
}

function ExportPanel() {
  const [secret, setSecret] = useState("none");
  const [passphrase, setPassphrase] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function download() {
    setBusy(true);
    setError(null);
    try {
      const data = await api.exportConfig(secret, passphrase || undefined);
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "bothan-config.json";
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "export failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold">Export</h3>
      <label className="block">
        <span className="mb-1 block text-sm font-medium">Secrets</span>
        <select value={secret} onChange={(e) => setSecret(e.target.value)} className={inputCls}>
          <option value="none">None — channels need re-entry on import</option>
          <option value="instance_key">Include (same key) — portable with the same encryption key</option>
          <option value="passphrase">Include (passphrase) — portable across different keys</option>
        </select>
      </label>
      {secret === "passphrase" && (
        <label className="block">
          <span className="mb-1 block text-sm font-medium">Passphrase</span>
          <input type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} className={inputCls} />
        </label>
      )}
      {secret !== "none" && (
        <p className="text-xs text-amber-600 dark:text-amber-400">
          ⚠ This bundle contains channel credentials. Handle it securely.
        </p>
      )}
      {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
      <button
        type="button"
        onClick={download}
        disabled={busy || (secret === "passphrase" && !passphrase)}
        className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
      >
        {busy ? "Exporting…" : "Download bundle"}
      </button>
    </div>
  );
}

function ImportPanel() {
  const [bundleData, setBundleData] = useState<unknown | null>(null);
  const [filename, setFilename] = useState("");
  const [mode, setMode] = useState("merge");
  const [passphrase, setPassphrase] = useState("");
  const [preview, setPreview] = useState<ImportResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setError(null);
    setPreview(null);
    setDone(null);
    setFilename(file.name);
    try {
      setBundleData(JSON.parse(await file.text()));
    } catch {
      setError("invalid JSON file");
      setBundleData(null);
    }
  }

  async function run(dryRun: boolean) {
    if (!bundleData) return;
    setBusy(true);
    setError(null);
    setDone(null);
    try {
      const res = await api.importConfig(bundleData, { mode, dry_run: dryRun, passphrase: passphrase || undefined });
      if (dryRun) {
        setPreview(res);
      } else {
        setPreview(null);
        setDone("Import applied.");
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "import failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold">Import</h3>
      <input type="file" accept="application/json" onChange={onFile} className="block text-sm" />
      {filename && <p className="text-xs text-slate-500 dark:text-slate-400">{filename}</p>}

      <div className="grid grid-cols-2 gap-3">
        <label className="block">
          <span className="mb-1 block text-sm font-medium">Mode</span>
          <select value={mode} onChange={(e) => setMode(e.target.value)} className={inputCls}>
            <option value="merge">Merge (upsert)</option>
            <option value="replace">Replace (wipe schedules/channels/rules)</option>
          </select>
        </label>
        <label className="block">
          <span className="mb-1 block text-sm font-medium">Passphrase (if any)</span>
          <input type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} className={inputCls} />
        </label>
      </div>

      {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
      {done && <p className="text-sm text-green-600 dark:text-green-400">{done}</p>}

      {preview && (
        <div className="rounded-md border border-slate-200 p-2 text-xs dark:border-slate-800">
          <p className="mb-1 font-medium">Dry-run preview:</p>
          <ul className="space-y-0.5 text-slate-600 dark:text-slate-300">
            <li>Hosts: +{preview.report.hosts_created} / ~{preview.report.hosts_updated}</li>
            <li>Schedules: +{preview.report.schedules_created} / ~{preview.report.schedules_updated}</li>
            <li>Channels: +{preview.report.channels_created} / ~{preview.report.channels_updated} (needs credentials: {preview.report.channels_needing_credentials})</li>
            <li>Rules: +{preview.report.rules_created} / ~{preview.report.rules_updated}</li>
            <li>Links created: {preview.report.links_created}</li>
            {preview.report.removed > 0 && <li className="text-amber-600 dark:text-amber-400">Removed (replace): {preview.report.removed}</li>}
          </ul>
        </div>
      )}

      <div className="flex gap-2">
        <button
          type="button"
          onClick={() => run(true)}
          disabled={busy || !bundleData}
          className="rounded-md border border-slate-300 px-4 py-2 text-sm hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:hover:bg-slate-800"
        >
          Dry run
        </button>
        <button
          type="button"
          onClick={() => {
            if (confirm(`Apply import in ${mode} mode?`)) run(false);
          }}
          disabled={busy || !bundleData}
          className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        >
          Apply import
        </button>
      </div>
    </div>
  );
}
