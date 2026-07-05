import type { Host, HostInput, Settings, SettingsPatch } from "../types";

const BASE = "/api/v1";

/** ApiError carries the server's error envelope message. */
export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return undefined as T;

  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const err = data?.error ?? {};
    throw new ApiError(res.status, err.code ?? "error", err.message ?? `request failed (${res.status})`);
  }
  return data as T;
}

export const api = {
  listHosts: () => req<Host[]>("GET", "/hosts"),
  createHost: (h: HostInput) => req<Host>("POST", "/hosts", h),
  updateHost: (id: number, h: HostInput) => req<Host>("PUT", `/hosts/${id}`, h),
  deleteHost: (id: number) => req<void>("DELETE", `/hosts/${id}`),
  enableHost: (id: number) => req<Host>("POST", `/hosts/${id}/enable`),
  disableHost: (id: number) => req<Host>("POST", `/hosts/${id}/disable`),

  getSettings: () => req<Settings>("GET", "/settings"),
  updateSettings: (patch: SettingsPatch) => req<Settings>("PUT", "/settings", patch),
};
