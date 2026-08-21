(() => {
  "use strict";

  const slides = Array.from(document.querySelectorAll(".slide"));
  const currentLabel = document.querySelector("#current");
  let current = 0;

  function hashIndex() {
    const match = window.location.hash.match(/^#(?:slide-)?(\d+)$/);
    if (!match) return 0;
    const index = Number(match[1]) - 1;
    return Number.isInteger(index) && index >= 0 && index < slides.length ? index : 0;
  }

  function show(index, updateHash = true) {
    current = Math.max(0, Math.min(index, slides.length - 1));
    slides.forEach((slide, slideIndex) => {
      const active = slideIndex === current;
      slide.classList.toggle("is-active", active);
      slide.setAttribute("aria-hidden", String(!active));
    });
    currentLabel.textContent = String(current + 1);
    if (updateHash) history.replaceState(null, "", `#${current + 1}`);
  }

  function isEditable(target) {
    return target instanceof Element && Boolean(target.closest("input, textarea, select, [contenteditable]:not([contenteditable='false'])"));
  }

  document.addEventListener("keydown", (event) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || isEditable(event.target)) return;

    const next = ["ArrowRight", "ArrowDown", "PageDown", " "];
    const previous = ["ArrowLeft", "ArrowUp", "PageUp"];
    if (next.includes(event.key)) {
      event.preventDefault();
      show(current + 1);
    } else if (previous.includes(event.key)) {
      event.preventDefault();
      show(current - 1);
    } else if (event.key === "Home") {
      event.preventDefault();
      show(0);
    } else if (event.key === "End") {
      event.preventDefault();
      show(slides.length - 1);
    }
  });

  window.addEventListener("hashchange", () => show(hashIndex()));
  show(hashIndex());

  const session = new EventSource(`/api/session?revision=${encodeURIComponent(document.body.dataset.revision)}`);
  session.addEventListener("reload", () => window.location.reload());
})();
