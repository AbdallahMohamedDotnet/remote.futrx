export const initializeTableOfContents = () => {
  const links = [...document.querySelectorAll(".toc a")];
  if (!links.length || !("IntersectionObserver" in window)) return;

  const headings = links
    .map((link) => document.getElementById(decodeURIComponent(link.hash.slice(1))))
    .filter(Boolean);

  const observer = new IntersectionObserver((entries) => {
    const visible = entries.find((entry) => entry.isIntersecting);
    if (!visible) return;

    links.forEach((link) => {
      link.classList.toggle("is-active", link.hash === `#${visible.target.id}`);
    });
  }, { rootMargin: "-18% 0px -72%", threshold: 0 });

  headings.forEach((heading) => observer.observe(heading));
};
