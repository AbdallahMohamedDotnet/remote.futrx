const mermaidUrl = "https://cdn.jsdelivr.net/npm/mermaid@11.16.0/dist/mermaid.esm.min.mjs";

const darkThemeVariables = {
  background: "#121317",
  primaryColor: "#202126",
  primaryTextColor: "#e4e4e7",
  primaryBorderColor: "#8ab4ff",
  lineColor: "#9b9ba3",
  secondaryColor: "#191a1f",
  tertiaryColor: "#0f1014",
  clusterBkg: "#191a1f",
  clusterBorder: "rgba(255, 255, 255, 0.12)",
  edgeLabelBackground: "#121317",
  fontFamily: "-apple-system, BlinkMacSystemFont, Segoe UI, system-ui, sans-serif",
};

const lightThemeVariables = {
  background: "#ffffff",
  primaryColor: "#f6f8fa",
  primaryTextColor: "#24292f",
  primaryBorderColor: "#0969da",
  lineColor: "#57606a",
  secondaryColor: "#ffffff",
  tertiaryColor: "#f6f8fa",
  clusterBkg: "#f6f8fa",
  clusterBorder: "rgba(31, 35, 40, 0.16)",
  edgeLabelBackground: "#ffffff",
  fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
};

const showFallback = () => {
  document.querySelectorAll(".mermaid-shell").forEach((item) => {
    item.classList.add("is-fallback");
  });
};

export const initializeDiagrams = async () => {
  const diagrams = document.querySelectorAll(".mermaid");
  if (!diagrams.length) return;

  try {
    const { default: mermaid } = await import(mermaidUrl);
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "base",
      themeVariables: document.documentElement.dataset.theme === "light"
        ? lightThemeVariables
        : darkThemeVariables,
    });
    await mermaid.run({ nodes: diagrams });
  } catch {
    showFallback();
  }
};
