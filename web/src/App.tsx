import { useEffect, useState } from "react";
import { currentTheme, toggleTheme, type Theme } from "./lib/theme";
import HostsPage from "./pages/HostsPage";

export default function App() {
  const [theme, setThemeState] = useState<Theme>(currentTheme());

  useEffect(() => {
    document.title = "Bothan";
  }, []);

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <header className="border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
          <div className="flex items-center gap-2">
            <span className="text-lg font-semibold tracking-tight">Bothan</span>
            <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400">
              SSL/TLS monitor
            </span>
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
        <HostsPage />
      </main>
    </div>
  );
}
