import { useEffect, useState } from "react";
import { api, ApiError } from "../lib/api";
import type { ApiToken } from "../types";

const inputCls =
  "w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950";

export default function TokensPage() {
  const [tokens, setTokens] = useState<ApiToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState("read");
  const [created, setCreated] = useState<string | null>(null);

  async function refresh() {
    try {
      setError(null);
      setTokens(await api.listTokens());
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load tokens");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    try {
      setError(null);
      const res = await api.createToken({ name: name.trim(), scopes });
      setCreated(res.plaintext);
      setName("");
      refresh();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "failed to create token");
    }
  }

  async function del(id: number) {
    if (!confirm("Delete this token? Clients using it will stop working.")) return;
    try {
      await api.deleteToken(id);
      refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to delete token");
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">API Tokens</h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Bearer tokens for the REST API. The token value is shown only once at creation.
        </p>
      </div>

      {created && (
        <div className="rounded-md border border-green-200 bg-green-50 p-3 text-sm dark:border-green-900 dark:bg-green-950">
          <p className="mb-1 font-medium text-green-700 dark:text-green-300">New token — copy it now, it won't be shown again:</p>
          <code className="block break-all rounded bg-white px-2 py-1 font-mono text-xs dark:bg-slate-900">{created}</code>
          <button type="button" onClick={() => setCreated(null)} className="mt-2 text-xs text-green-700 underline dark:text-green-300">
            Dismiss
          </button>
        </div>
      )}

      <form onSubmit={create} className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <label className="flex-1">
            <span className="mb-1 block text-sm font-medium">Name</span>
            <input value={name} onChange={(e) => setName(e.target.value)} className={inputCls} placeholder="CI pipeline" />
          </label>
          <label>
            <span className="mb-1 block text-sm font-medium">Scopes</span>
            <select value={scopes} onChange={(e) => setScopes(e.target.value)} className={inputCls}>
              <option value="read">read</option>
              <option value="read,write">read, write</option>
              <option value="admin">admin</option>
            </select>
          </label>
          <button
            type="submit"
            className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
          >
            Create token
          </button>
        </div>
      </form>

      {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}

      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : tokens.length === 0 ? (
        <p className="rounded-md border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500 dark:border-slate-700">
          No tokens yet.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
          <table className="w-full text-sm">
            <thead className="bg-slate-100 text-left text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              <tr>
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">Scopes</th>
                <th className="px-4 py-2 font-medium">Last used</th>
                <th className="px-4 py-2 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {tokens.map((t) => (
                <tr key={t.id} className="bg-white dark:bg-slate-900">
                  <td className="px-4 py-2 font-medium">{t.name}</td>
                  <td className="px-4 py-2 font-mono text-xs">{t.scopes}</td>
                  <td className="px-4 py-2 text-slate-500 dark:text-slate-400">
                    {t.last_used_at ? new Date(t.last_used_at).toLocaleString() : "never"}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <button
                      type="button"
                      onClick={() => del(t.id)}
                      className="rounded border border-red-300 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950"
                    >
                      Delete
                    </button>
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
