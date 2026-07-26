export const initializeCodeCopy = () => {
  document.querySelectorAll("[data-copy-code]").forEach((button) => {
    button.addEventListener("click", async () => {
      const code = button.closest(".code-block")?.querySelector("code")?.textContent || "";
      await navigator.clipboard.writeText(code);

      const originalLabel = button.textContent;
      button.textContent = "Copied";
      setTimeout(() => {
        button.textContent = originalLabel;
      }, 1200);
    });
  });
};
