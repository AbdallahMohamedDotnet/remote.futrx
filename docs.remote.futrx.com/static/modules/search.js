const escapeHtml = (value) => String(value)
  .replaceAll("&", "&amp;")
  .replaceAll("<", "&lt;")
  .replaceAll(">", "&gt;")
  .replaceAll('"', "&quot;");

class DocumentationSearch {
  #activeResult = 0;
  #dialog;
  #index;
  #input;
  #results;

  constructor() {
    this.#dialog = document.querySelector("[data-search-dialog]");
    this.#input = document.querySelector("[data-search-input]");
    this.#results = document.querySelector("[data-search-results]");
  }

  initialize() {
    if (!this.#dialog || !this.#input || !this.#results) return;

    document.querySelector("[data-search-open]")?.addEventListener("click", () => this.#open());
    document.querySelector("[data-search-close]")?.addEventListener("click", () => this.#close());
    this.#dialog.addEventListener("click", (event) => {
      if (event.target === this.#dialog) this.#close();
    });
    this.#input.addEventListener("input", () => this.#renderResults());
    this.#input.addEventListener("keydown", (event) => this.#handleResultNavigation(event));
    document.addEventListener("keydown", (event) => this.#handleShortcut(event));
  }

  async #loadIndex() {
    if (!this.#index) {
      this.#index = await fetch("/search-index.json").then((response) => response.json());
    }
    return this.#index;
  }

  async #open() {
    if (!this.#dialog.open) this.#dialog.showModal();
    await this.#loadIndex();
    this.#input.focus();
  }

  #close() {
    this.#dialog.close();
  }

  #handleShortcut(event) {
    if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== "k") return;
    event.preventDefault();
    this.#open();
  }

  #handleResultNavigation(event) {
    const items = [...this.#results.querySelectorAll(".search-result")];
    if (!items.length) return;

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const direction = event.key === "ArrowDown" ? 1 : -1;
      this.#activeResult = (this.#activeResult + direction + items.length) % items.length;
      items.forEach((item, index) => item.classList.toggle("is-active", index === this.#activeResult));
      items[this.#activeResult].scrollIntoView({ block: "nearest" });
    }

    if (event.key === "Enter") {
      event.preventDefault();
      items[this.#activeResult].click();
    }
  }

  async #renderResults() {
    const rawQuery = this.#input.value.trim();
    const query = rawQuery.toLowerCase();

    if (!query) {
      this.#results.innerHTML = "<p>Start typing to search all documentation.</p>";
      return;
    }

    const terms = query.split(/\s+/).filter(Boolean);
    const matches = (await this.#loadIndex())
      .map((entry) => ({ ...entry, score: this.#score(entry, terms) }))
      .filter((entry) => entry.score >= terms.length)
      .sort((left, right) => right.score - left.score)
      .slice(0, 8);

    this.#activeResult = 0;
    this.#results.innerHTML = matches.length
      ? matches.map((entry, index) => this.#resultTemplate(entry, index)).join("")
      : `<div class="search-empty"><strong>No results for “${escapeHtml(rawQuery)}”</strong><span>Try a broader word or phrase.</span></div>`;
  }

  #score(entry, terms) {
    const title = entry.title.toLowerCase();
    const headings = entry.headings.join(" ").toLowerCase();
    const content = entry.content.toLowerCase();

    return terms.reduce((total, term) => (
      total
      + (title.includes(term) ? 12 : 0)
      + (headings.includes(term) ? 5 : 0)
      + (content.includes(term) ? 1 : 0)
    ), 0);
  }

  #resultTemplate(entry, index) {
    const activeClass = index === 0 ? " is-active" : "";
    return `<a class="search-result${activeClass}" href="${escapeHtml(entry.url)}"><strong>${escapeHtml(entry.title)}</strong><span>${escapeHtml(entry.description)}</span></a>`;
  }
}

export const initializeSearch = () => {
  new DocumentationSearch().initialize();
};
