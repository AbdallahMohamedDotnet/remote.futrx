package httphandlers

import (
	"html/template"
	"net/http"
	"regexp"
)

var devPreviewHostPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*--\d{4,5}\.dev\.`)

type BrowserInspectorHandler struct{}

func NewBrowserInspectorHandler() *BrowserInspectorHandler {
	return &BrowserInspectorHandler{}
}

func (h *BrowserInspectorHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/__remote_inspector", h.Handle)
}

func (h *BrowserInspectorHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !devPreviewHostPattern.MatchString(r.Host) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if err := browserInspectorTemplate.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var browserInspectorTemplate = template.Must(template.New("browser-inspector").Parse(`<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <style>
    html, body { margin: 0; height: 100%; overflow: hidden; background: #fff; }
    #app { position: fixed; inset: 0; width: 100%; height: 100%; border: 0; background: #fff; }
    #remote-highlight {
      position: fixed;
      z-index: 2147483645;
      pointer-events: none;
      border: 2px solid #7ba7ff;
      background: rgba(123, 167, 255, 0.14);
      box-shadow: 0 0 0 1px rgba(11, 13, 17, 0.8), 0 8px 30px rgba(0, 0, 0, 0.22);
      display: none;
    }
    #remote-tooltip {
      position: fixed;
      z-index: 2147483646;
      pointer-events: none;
      max-width: min(420px, calc(100vw - 24px));
      padding: 7px 9px;
      border-radius: 6px;
      background: rgba(11, 13, 17, 0.94);
      color: #f4f7fb;
      font: 12px/1.35 ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      box-shadow: 0 10px 30px rgba(0, 0, 0, 0.32);
      display: none;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
  </style>
</head>
<body>
  <iframe id="app" title="Inspectable app preview"></iframe>
  <div id="remote-highlight"></div>
  <div id="remote-tooltip"></div>
  <script>
    (() => {
      const params = new URLSearchParams(location.search);
      const appFrame = document.getElementById("app");
      const highlight = document.getElementById("remote-highlight");
      const tooltip = document.getElementById("remote-tooltip");
      const parentOrigin = safeOrigin(params.get("parent"));
      let inspectEnabled = false;
      let currentElement = null;
      let cleanupFrameListeners = null;

      appFrame.src = sanitizeTarget(params.get("target") || "/");

      window.addEventListener("message", (event) => {
        if (parentOrigin !== "*" && event.origin !== parentOrigin) return;
        const data = event.data || {};
        if (data.type === "remote-inspector:set-enabled") {
          setInspectEnabled(Boolean(data.enabled));
        }
      });

      appFrame.addEventListener("load", () => {
        bindFrame();
        send("remote-inspector:ready", {
          url: frameHref(),
          title: frameTitle(),
        });
      });

      send("remote-inspector:ready", { url: location.href, title: "" });

      function setInspectEnabled(enabled) {
        inspectEnabled = enabled;
        document.body.style.cursor = enabled ? "crosshair" : "";
        if (!enabled) {
          currentElement = null;
          hideHighlight();
        }
      }

      function bindFrame() {
        if (cleanupFrameListeners) cleanupFrameListeners();
        let doc;
        try {
          doc = appFrame.contentDocument;
          if (!doc) throw new Error("missing frame document");
        } catch (error) {
          send("remote-inspector:error", { message: "Could not access preview DOM: " + error.message });
          return;
        }

        const onMove = (event) => {
          if (!inspectEnabled) return;
          const target = selectableElement(event.target);
          if (!target) return;
          currentElement = target;
          showHighlight(target);
        };
        const onLeave = () => {
          if (!inspectEnabled) return;
          currentElement = null;
          hideHighlight();
        };
        const onClick = (event) => {
          if (!inspectEnabled) return;
          event.preventDefault();
          event.stopPropagation();
          if (typeof event.stopImmediatePropagation === "function") event.stopImmediatePropagation();
          const target = selectableElement(event.target) || currentElement;
          if (!target) return;
          currentElement = target;
          showHighlight(target);
          send("remote-inspector:element-selected", describeElement(target));
        };

        doc.addEventListener("mousemove", onMove, true);
        doc.addEventListener("mouseover", onMove, true);
        doc.addEventListener("mouseleave", onLeave, true);
        doc.addEventListener("click", onClick, true);
        cleanupFrameListeners = () => {
          doc.removeEventListener("mousemove", onMove, true);
          doc.removeEventListener("mouseover", onMove, true);
          doc.removeEventListener("mouseleave", onLeave, true);
          doc.removeEventListener("click", onClick, true);
        };
      }

      function selectableElement(node) {
        if (!node) return null;
        if (node.nodeType === Node.TEXT_NODE) return node.parentElement;
        if (node.nodeType !== Node.ELEMENT_NODE) return null;
        return node;
      }

      function showHighlight(element) {
        const rect = element.getBoundingClientRect();
        const frameRect = appFrame.getBoundingClientRect();
        if (!rect.width && !rect.height) return;
        highlight.style.display = "block";
        highlight.style.left = Math.max(0, frameRect.left + rect.left) + "px";
        highlight.style.top = Math.max(0, frameRect.top + rect.top) + "px";
        highlight.style.width = Math.max(0, rect.width) + "px";
        highlight.style.height = Math.max(0, rect.height) + "px";

        const label = elementLabel(element);
        tooltip.textContent = label + " - click to add to chat";
        tooltip.style.display = "block";
        const tipTop = Math.max(8, frameRect.top + rect.top - 34);
        const tipLeft = Math.min(window.innerWidth - 24, Math.max(8, frameRect.left + rect.left));
        tooltip.style.left = tipLeft + "px";
        tooltip.style.top = tipTop + "px";
      }

      function hideHighlight() {
        highlight.style.display = "none";
        tooltip.style.display = "none";
      }

      function describeElement(element) {
        const rect = element.getBoundingClientRect();
        const win = appFrame.contentWindow;
        const doc = appFrame.contentDocument;
        const styles = win.getComputedStyle(element);
        return {
          url: frameHref(),
          title: frameTitle(),
          selector: selectorFor(element),
          tag: element.tagName.toLowerCase(),
          id: element.id || "",
          classes: Array.from(element.classList || []),
          role: element.getAttribute("role") || "",
          ariaLabel: element.getAttribute("aria-label") || "",
          text: truncate(normalizeText(element.innerText || element.textContent || ""), 500),
          html: truncate(normalizeText(element.outerHTML || ""), 900),
          rect: {
            x: Math.round(rect.x),
            y: Math.round(rect.y),
            width: Math.round(rect.width),
            height: Math.round(rect.height),
          },
          viewport: {
            width: win.innerWidth,
            height: win.innerHeight,
          },
          styles: {
            display: styles.display,
            position: styles.position,
            zIndex: styles.zIndex,
            margin: styles.margin,
            padding: styles.padding,
            color: styles.color,
            backgroundColor: styles.backgroundColor,
            fontSize: styles.fontSize,
            fontWeight: styles.fontWeight,
            lineHeight: styles.lineHeight,
            flex: styles.flex,
            gridArea: styles.gridArea,
            overflow: styles.overflow,
          },
          parents: parentTrail(element, doc),
        };
      }

      function parentTrail(element, doc) {
        const out = [];
        let node = element.parentElement;
        while (node && node !== doc.body && out.length < 5) {
          out.push(elementLabel(node));
          node = node.parentElement;
        }
        return out;
      }

      function elementLabel(element) {
        let label = element.tagName.toLowerCase();
        if (element.id) label += "#" + element.id;
        const classes = Array.from(element.classList || []).slice(0, 4);
        if (classes.length) label += "." + classes.join(".");
        const role = element.getAttribute("role");
        if (role) label += "[role=" + role + "]";
        return label;
      }

      function selectorFor(element) {
        if (element.id) return element.tagName.toLowerCase() + "#" + cssEscape(element.id);
        const doc = element.ownerDocument;
        const parts = [];
        let node = element;
        while (node && node.nodeType === Node.ELEMENT_NODE && node !== doc.body && parts.length < 8) {
          let part = node.tagName.toLowerCase();
          const testClasses = Array.from(node.classList || []).filter(Boolean).slice(0, 2);
          if (testClasses.length) part += "." + testClasses.map(cssEscape).join(".");
          const parent = node.parentElement;
          if (parent) {
            const siblings = Array.from(parent.children).filter((child) => child.tagName === node.tagName);
            if (siblings.length > 1) part += ":nth-of-type(" + (siblings.indexOf(node) + 1) + ")";
          }
          parts.unshift(part);
          node = parent;
        }
        return parts.join(" > ");
      }

      function frameHref() {
        try {
          return appFrame.contentWindow.location.href;
        } catch {
          return appFrame.src;
        }
      }

      function frameTitle() {
        try {
          return appFrame.contentDocument.title || "";
        } catch {
          return "";
        }
      }

      function sanitizeTarget(value) {
        try {
          const url = new URL(value, location.origin);
          if (url.origin !== location.origin) return "/";
          return url.pathname + url.search + url.hash;
        } catch {
          return "/";
        }
      }

      function safeOrigin(value) {
        if (!value) return "*";
        try {
          return new URL(value).origin;
        } catch {
          return "*";
        }
      }

      function send(type, payload) {
        window.parent.postMessage({ type, payload }, parentOrigin);
      }

      function normalizeText(value) {
        return String(value).replace(/\s+/g, " ").trim();
      }

      function truncate(value, max) {
        value = String(value || "");
        return value.length > max ? value.slice(0, max - 1) + "…" : value;
      }

      function cssEscape(value) {
        if (window.CSS && typeof window.CSS.escape === "function") return window.CSS.escape(value);
        return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
      }
    })();
  </script>
</body>
</html>`))
