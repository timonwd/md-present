(() => {
  "use strict";

  const slides = Array.from(document.querySelectorAll(".slide"));
  const currentLabel = document.querySelector("#current");
  const fullscreenButton = document.querySelector(".fullscreen-button");
  const colorScheme = window.matchMedia("(prefers-color-scheme: dark)");
  let current = 0;
  let diagramRender = Promise.resolve();

  const diagrams = Array.from(document.querySelectorAll("pre > code.language-mermaid")).map((code, index) => {
    const figure = document.createElement("figure");
    figure.className = "mermaid-diagram";
    figure.setAttribute("aria-busy", "true");
    code.parentElement.replaceWith(figure);
    return { figure, source: code.textContent, index };
  });

  function showDiagramError(diagram, error) {
    const message = document.createElement("div");
    message.className = "mermaid-error";
    message.setAttribute("role", "alert");

    const title = document.createElement("strong");
    title.textContent = "Could not render Mermaid diagram.";
    message.append(title);

    const detail = error instanceof Error ? error.message.trim() : String(error || "The diagram syntax is invalid.").trim();
    if (detail) {
      const explanation = document.createElement("span");
      explanation.textContent = detail;
      message.append(explanation);
    }

    const source = document.createElement("pre");
    const code = document.createElement("code");
    code.textContent = diagram.source;
    source.append(code);
    diagram.figure.replaceChildren(message, source);
    diagram.figure.removeAttribute("aria-busy");
  }

  function safeDiagramImage(svg, fallbackLabel) {
    const parsed = new DOMParser().parseFromString(svg, "image/svg+xml");
    const root = parsed.documentElement;
    if (root.localName !== "svg" || parsed.querySelector("parsererror")) {
      throw new Error("Mermaid returned an invalid diagram.");
    }

    root.querySelectorAll("script").forEach((script) => script.remove());
    [root, ...root.querySelectorAll("*")].forEach((element) => {
      Array.from(element.attributes).forEach((attribute) => {
        const name = attribute.name.toLowerCase();
        const value = attribute.value.trim().toLowerCase();
        if (name.startsWith("on") || ((name === "href" || name === "xlink:href") && /^(?:javascript|vbscript|data):/.test(value))) {
          element.removeAttribute(attribute.name);
        }
      });
    });

    const image = document.createElement("img");
    image.className = "mermaid-diagram__image";
    image.alt = root.querySelector("title")?.textContent?.trim() || fallbackLabel;

    const viewBox = root.getAttribute("viewBox")?.trim().split(/[ ,]+/).map(Number);
    if (viewBox?.length === 4 && viewBox.every(Number.isFinite) && viewBox[2] > 0 && viewBox[3] > 0) {
      image.width = Math.ceil(viewBox[2]);
      image.height = Math.ceil(viewBox[3]);
    }

    const serialized = new XMLSerializer().serializeToString(root);
    image.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(serialized)}`;
    return image;
  }

  async function renderDiagrams(theme) {
    if (diagrams.length === 0) return;
    if (!globalThis.mermaid) {
      diagrams.forEach((diagram) => showDiagramError(diagram, new Error("The bundled Mermaid renderer did not load.")));
      return;
    }

    globalThis.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      suppressErrorRendering: true,
      theme,
    });

    for (const diagram of diagrams) {
      diagram.figure.setAttribute("aria-busy", "true");
      const id = `mermaid-diagram-${diagram.index + 1}`;
      try {
        const { svg } = await globalThis.mermaid.render(id, diagram.source);
        const image = safeDiagramImage(svg, `Mermaid diagram ${diagram.index + 1}`);
        image.addEventListener("error", () => {
          if (diagram.figure.contains(image)) showDiagramError(diagram, new Error("The rendered diagram could not be displayed."));
        }, { once: true });
        diagram.figure.replaceChildren(image);
        diagram.figure.removeAttribute("aria-busy");
      } catch (error) {
        showDiagramError(diagram, error);
      } finally {
        document.getElementById(`d${id}`)?.remove();
      }
    }
  }

  function scheduleDiagramRender() {
    const theme = colorScheme.matches ? "dark" : "default";
    diagramRender = diagramRender.catch(() => {}).then(() => renderDiagrams(theme));
  }

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
  scheduleDiagramRender();
  colorScheme.addEventListener("change", scheduleDiagramRender);

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
