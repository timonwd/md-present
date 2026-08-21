(() => {
  "use strict";

  const slides = Array.from(document.querySelectorAll(".slide"));
  const currentLabel = document.querySelector("#current");
  const fullscreenButton = document.querySelector(".fullscreen-button");
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

  document.querySelector(".deck").addEventListener("click", (event) => {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      isEditable(event.target) ||
      !(event.target instanceof Element) ||
      !event.target.closest(".slide") ||
      event.target.closest("a, button, summary, input, select, textarea, [contenteditable]:not([contenteditable='false'])")
    ) return;

    show(current + 1);
  });

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

  function updateFullscreenButton() {
    const active = Boolean(document.fullscreenElement);
    const label = active ? "Exit fullscreen" : "Enter fullscreen";
    fullscreenButton.classList.toggle("is-active", active);
    fullscreenButton.setAttribute("aria-label", label);
    fullscreenButton.setAttribute("title", label);
  }

  if (document.fullscreenEnabled) {
    fullscreenButton.addEventListener("click", async () => {
      try {
        if (document.fullscreenElement) {
          await document.exitFullscreen();
        } else {
          await document.documentElement.requestFullscreen();
        }
      } catch {
        // The browser may reject fullscreen when the user gesture is no longer valid.
      }
    });
    document.addEventListener("fullscreenchange", updateFullscreenButton);
    updateFullscreenButton();
  } else {
    fullscreenButton.hidden = true;
  }

  const session = new EventSource(`/api/session?revision=${encodeURIComponent(document.body.dataset.revision)}`);
  session.addEventListener("reload", () => window.location.reload());
})();
