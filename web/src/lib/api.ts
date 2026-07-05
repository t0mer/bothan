import type { Channel, Host, HostInput, Rule, Scan, Schedule, Settings, SettingsPatch } from "../types";

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

  scanHost: (id: number) => req<Scan>("POST", `/hosts/${id}/scan`),
  hostScans: (id: number) => req<Scan[]>("GET", `/hosts/${id}/scans`),
  getScan: (id: number) => req<Scan>("GET", `/scans/${id}`),

  listSchedules: () => req<Schedule[]>("GET", "/schedules"),
  createSchedule: (s: { name: string; spec: string; enabled?: boolean }) =>
    req<Schedule>("POST", "/schedules", s),
  updateSchedule: (id: number, s: { name: string; spec: string; enabled?: boolean }) =>
    req<Schedule>("PUT", `/schedules/${id}`, s),
  deleteSchedule: (id: number) => req<void>("DELETE", `/schedules/${id}`),
  hostSchedules: (id: number) => req<Schedule[]>("GET", `/hosts/${id}/schedules`),
  setHostSchedules: (id: number, ids: number[]) =>
    req<Schedule[]>("PUT", `/hosts/${id}/schedules`, { ids }),

  listChannels: () => req<Channel[]>("GET", "/channels"),
  createChannel: (c: unknown) => req<Channel>("POST", "/channels", c),
  updateChannel: (id: number, c: unknown) => req<Channel>("PUT", `/channels/${id}`, c),
  deleteChannel: (id: number) => req<void>("DELETE", `/channels/${id}`),
  testChannel: (id: number) => req<{ sent: boolean }>("POST", `/channels/${id}/test`),
  testChannelConfig: (c: unknown) => req<{ sent: boolean }>("POST", "/channels/test", c),
  hostChannels: (id: number) => req<Channel[]>("GET", `/hosts/${id}/channels`),
  setHostChannels: (id: number, ids: number[]) =>
    req<Channel[]>("PUT", `/hosts/${id}/channels`, { ids }),

  listRules: () => req<Rule[]>("GET", "/rules"),
  createRule: (r: unknown) => req<Rule>("POST", "/rules", r),
  updateRule: (id: number, r: unknown) => req<Rule>("PUT", `/rules/${id}`, r),
  deleteRule: (id: number) => req<void>("DELETE", `/rules/${id}`),

  getSettings: () => req<Settings>("GET", "/settings"),
  updateSettings: (patch: SettingsPatch) => req<Settings>("PUT", "/settings", patch),
};
