export const initializeTheme = () => {
  const root = document.documentElement;
  const toggle = document.querySelector("[data-theme-toggle]");

  toggle?.addEventListener("click", () => {
    const theme = root.dataset.theme === "dark" ? "light" : "dark";
    root.dataset.theme = theme;

    try {
      localStorage.setItem("docs-theme", theme);
    } catch {}
  });
};
