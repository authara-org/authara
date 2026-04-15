export type AppTheme = "light" | "dark" | "system";

const THEME_KEY = "app_theme";

export function getStoredTheme(): AppTheme {
  const value = localStorage.getItem(THEME_KEY);
  if (value === "light" || value === "dark" || value === "system") {
    return value;
  }
  return "system";
}

export function applyTheme(theme: AppTheme): void {
  const systemDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  const isDark = theme === "dark" || (theme === "system" && systemDark);

  document.documentElement.classList.toggle("dark", isDark);
}

export function setTheme(theme: AppTheme): void {
  localStorage.setItem(THEME_KEY, theme);
  applyTheme(theme);
}

export function initTheme(): void {
  // apply on load
  applyTheme(getStoredTheme());

  // react to system changes
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      if (getStoredTheme() === "system") {
        applyTheme("system");
      }
    });

  // sync across tabs
  window.addEventListener("storage", (e) => {
    if (e.key === THEME_KEY) {
      applyTheme(getStoredTheme());
    }
  });
}
