import { useEffect, useState } from "react";
import { api } from "./lib/api";
import { currentTheme, toggleTheme, type Theme } from "./lib/theme";
import DashboardPage from "./pages/DashboardPage";
import HostsPage from "./pages/HostsPage";
import SchedulesPage from "./pages/SchedulesPage";
import ChannelsPage from "./pages/ChannelsPage";
import RulesPage from "./pages/RulesPage";
import SettingsPage from "./pages/SettingsPage";
import TokensPage from "./pages/TokensPage";
import LoginPage from "./pages/LoginPage";

type Page = "dashboard" | "hosts" | "schedules" | "channels" | "rules" | "tokens" | "settings";

type AuthState = { loading: boolean; enabled: boolean; authed: boolean; username?: string };

export default function App() {
  const [theme, setThemeState] = useState<Theme>(currentTheme());
  const [page, setPage] = useState<Page>("dashboard");
  const [auth, setAuth] = useState<AuthState>({ loading: true, enabled: false, authed: false });

  async function refreshAuth() {
    try {
      const me = await api.me();
      setAuth({ loading: false, enabled: me.auth_enabled, authed: !me.auth_enabled || !!me.username, username: me.username });
    } catch {
      // A 401 means auth is on but we're not signed in.
      setAuth({ loading: false, enabled: true, authed: false });
    }
  }

  useEffect(() => {
    document.title = "Bothan";
    refreshAuth();
  }, []);

  if (auth.loading) {
    return <div className="flex min-h-screen items-center justify-center text-sm text-slate-500">Loading…</div>;
  }
  if (auth.enabled && !auth.authed) {
    return <LoginPage onLogin={refreshAuth} />;
  }

  async function logout() {
    await api.logout().catch(() => {});
    refreshAuth();
  }

  const navItem = (id: Page, label: string) => (
    <button
      type="button"
      onClick={() => setPage(id)}
      className={
        "rounded-md px-3 py-1.5 text-sm font-medium " +
        (page === id
          ? "bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900"
          : "text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800")
      }
    >
      {label}
    </button>
  );

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <header className="border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-2 px-4 py-3">
          <div className="flex flex-wrap items-center gap-4">
            <div className="flex items-center gap-2">
              <span className="text-lg font-semibold tracking-tight">Bothan</span>
              <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                SSL/TLS monitor
              </span>
            </div>
            <nav className="flex flex-wrap items-center gap-1">
              {navItem("dashboard", "Dashboard")}
              {navItem("hosts", "Hosts")}
              {navItem("schedules", "Schedules")}
              {navItem("channels", "Channels")}
              {navItem("rules", "Rules")}
              {auth.enabled && navItem("tokens", "Tokens")}
              {navItem("settings", "Settings")}
            </nav>
          </div>
          <div className="flex items-center gap-2">
            {auth.enabled && auth.username && (
              <>
                <span className="text-sm text-slate-500 dark:text-slate-400">{auth.username}</span>
                <button
                  type="button"
                  onClick={logout}
                  className="rounded-md border border-slate-200 px-2.5 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                >
                  Logout
                </button>
              </>
            )}
            <button
              type="button"
              onClick={() => setThemeState(toggleTheme())}
              className="rounded-md border border-slate-200 px-2.5 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
              aria-label="Toggle theme"
            >
              {theme === "dark" ? "☀️ Light" : "🌙 Dark"}
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-4 py-6">
        {page === "dashboard" && <DashboardPage />}
        {page === "hosts" && <HostsPage />}
        {page === "schedules" && <SchedulesPage />}
        {page === "channels" && <ChannelsPage />}
        {page === "rules" && <RulesPage />}
        {page === "tokens" && <TokensPage />}
        {page === "settings" && <SettingsPage />}
      </main>
    </div>
  );
}
