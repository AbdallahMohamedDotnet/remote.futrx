export const initializeNavigation = () => {
  const toggle = document.querySelector("[data-nav-toggle]");

  const setOpen = (open) => {
    document.body.classList.toggle("nav-open", open);
    toggle?.setAttribute("aria-expanded", String(open));
  };

  toggle?.addEventListener("click", () => {
    setOpen(!document.body.classList.contains("nav-open"));
  });

  document.querySelectorAll("[data-nav-close]").forEach((button) => {
    button.addEventListener("click", () => setOpen(false));
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") setOpen(false);
  });
};
