import { useEffect, useState } from "react";
import { currentTheme, toggleTheme, type Theme } from "./lib/theme";
import HostsPage from "./pages/HostsPage";
import SettingsPage from "./pages/SettingsPage";

type Page = "hosts" | "settings";

export default function App() {
  const [theme, setThemeState] = useState<Theme>(currentTheme());
  const [page, setPage] = useState<Page>("hosts");

  useEffect(() => {
    document.title = "Bothan";
  }, []);

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
        <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <span className="text-lg font-semibold tracking-tight">Bothan</span>
              <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                SSL/TLS monitor
              </span>
            </div>
            <nav className="flex items-center gap-1">
              {navItem("hosts", "Hosts")}
              {navItem("settings", "Settings")}
            </nav>
          </div>
          <button
            type="button"
            onClick={() => setThemeState(toggleTheme())}
            className="rounded-md border border-slate-200 px-2.5 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
            aria-label="Toggle theme"
          >
            {theme === "dark" ? "☀️ Light" : "🌙 Dark"}
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-4 py-6">
        {page === "hosts" ? <HostsPage /> : <SettingsPage />}
      </main>
    </div>
  );
}
