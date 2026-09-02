(() => {
  "use strict";

  const deck = document.querySelector(".deck");
  const slides = Array.from(document.querySelectorAll(".slide"));
  const currentLabel = document.querySelector("#current");
  const overviewButton = document.querySelector(".overview-button");
  const fullscreenButton = document.querySelector(".fullscreen-button");
  const editorButton = document.querySelector(".editor-button");
  const editor = document.querySelector(".editor");
  const editorCloseButton = document.querySelector(".editor__close");
  const editorResizeHandle = document.querySelector(".editor__resize-handle");
  const editorSource = document.querySelector(".editor__source");
  const editorSaveButton = document.querySelector(".editor__save");
  const editorStatus = document.querySelector(".editor__status");
  const editorPreviousButton = document.querySelector(".editor__previous");
  const editorNextButton = document.querySelector(".editor__next");
  const editorSlideNumber = document.querySelector(".editor__slide-number");
  const overflowWarning = document.querySelector(".overflow-warning");
  const overflowWarningText = document.querySelector(".overflow-warning__text");
  const colorScheme = window.matchMedia("(prefers-color-scheme: dark)");
  let current = 0;
  let overview = false;
  let overviewIndex = 0;
  let diagramRender = Promise.resolve();
  let overflowMeasurement = 0;
  let lastOverflowReport = "";
  let overflowingSlides = [];
  let overflowWarningTimer;
  let overflowTransitionTimer;
  let overflowWarningExpanded = false;
  let loadWarningActive = false;
  let loadWarningShown = false;
  let editorRevision = "";
  let editorSlide = 0;
  let editorDirty = false;
  let editorSaveInProgress = false;
  let editorExpectedDeckRevision = 0;
  let editorOriginalHTML = "";
  let editorPreviewTimer;
  let editorPreviewGeneration = 0;
  const observedMedia = new WeakSet();
const overflowWarningDuration = 5000;
const updateNotice = document.querySelector(".update-notice");
const updateNoticeText = updateNotice.querySelector(".update-notice__text");

async function checkForUpdate() {
  try {
    const response = await fetch("/api/update");
    if (response.status === 204) {
      window.setTimeout(checkForUpdate, 500);
      return;
    }
    if (!response.ok) return;
    const update = await response.json();
    if (!update.available) return;
    updateNoticeText.textContent = `md-present ${update.version} is available — update with Homebrew`;
    updateNotice.hidden = false;
  } catch {
    // The update notice is optional; keep presenting if the local server closes.
  }
}

checkForUpdate();
  const overflowTransitionDuration = 180;
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

  function collectDiagrams() {
    return Array.from(document.querySelectorAll("pre > code.language-mermaid")).map((code, index) => {
      const figure = document.createElement("figure");
      figure.className = "mermaid-diagram";
      figure.setAttribute("aria-busy", "true");
      code.parentElement.replaceWith(figure);
      return { figure, source: code.textContent, index };
    });
  }
  let diagrams = collectDiagrams();
  const mermaidConfigurationDirective = /^\s*%%\{\s*(?:config|init)\s*:/im;

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
      themeVariables: {
        primaryColor: getComputedStyle(document.documentElement).getPropertyValue("--theme-mermaid-node").trim() || getComputedStyle(document.documentElement).getPropertyValue("--accent").trim(),
        primaryTextColor: getComputedStyle(document.documentElement).getPropertyValue("--theme-mermaid-label").trim() || getComputedStyle(document.documentElement).getPropertyValue("--ink").trim(),
        primaryBorderColor: getComputedStyle(document.documentElement).getPropertyValue("--line").trim(),
        lineColor: getComputedStyle(document.documentElement).getPropertyValue("--theme-mermaid-edge").trim() || getComputedStyle(document.documentElement).getPropertyValue("--muted").trim(),
        secondaryColor: getComputedStyle(document.documentElement).getPropertyValue("--code").trim(),
        tertiaryColor: getComputedStyle(document.documentElement).getPropertyValue("--stage").trim(),
      },
      htmlLabels: false,
    });

    for (const diagram of diagrams) {
      diagram.figure.setAttribute("aria-busy", "true");
      const id = `mermaid-diagram-${diagram.index + 1}`;
      try {
        if (mermaidConfigurationDirective.test(diagram.source)) {
          throw new Error("Mermaid configuration directives are not supported.");
        }
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
    diagramRender.then(scheduleOverflowMeasurement, scheduleOverflowMeasurement);
  }

  function overflowMessage(slideNumbers) {
    if (slideNumbers.length === 1) {
      return `Slide ${slideNumbers[0]} exceeds the regular slide size. Scroll to view all content.`;
    }
    if (slideNumbers.length <= 5) {
      return `Slides ${slideNumbers.join(", ")} exceed the regular slide size. Scroll to view all content.`;
    }
    return `${slideNumbers.length} slides exceed the regular slide size. Scroll to view all content.`;
  }

  function activeSlideOverflows() {
    return overflowingSlides.includes(current + 1);
  }

  function showCollapsedOverflowIndicator() {
    overflowWarningExpanded = false;
    loadWarningActive = false;
    if (!activeSlideOverflows()) {
      overflowWarning.hidden = true;
      return;
    }
    overflowWarning.hidden = false;
    overflowWarning.classList.add("is-collapsed");
    overflowWarning.setAttribute("aria-label", `Show overflow warning for slide ${current + 1}`);
  }

  function collapseOverflowWarning() {
    clearTimeout(overflowWarningTimer);
    clearTimeout(overflowTransitionTimer);
    overflowWarningExpanded = false;
    loadWarningActive = false;
    overflowWarning.classList.add("is-dismissing");

    const finish = () => {
      if (overflowWarningExpanded) return;
      if (activeSlideOverflows()) {
        showCollapsedOverflowIndicator();
      } else {
        overflowWarning.hidden = true;
      }
      overflowWarning.classList.remove("is-dismissing");
    };
    if (reducedMotion.matches) {
      finish();
    } else {
      overflowTransitionTimer = setTimeout(finish, overflowTransitionDuration);
    }
  }

  function showOverflowWarning(slideNumbers, global = false) {
    clearTimeout(overflowTransitionTimer);
    overflowWarningExpanded = true;
    loadWarningActive = global;
    overflowWarningText.textContent = overflowMessage(slideNumbers);
    overflowWarning.classList.remove("is-collapsed", "is-dismissing");
    overflowWarning.removeAttribute("aria-label");
    overflowWarning.hidden = false;
    clearTimeout(overflowWarningTimer);
    overflowWarningTimer = setTimeout(collapseOverflowWarning, overflowWarningDuration);
  }

  function updateOverflowWarningForActiveSlide() {
    if (overflowWarningExpanded && loadWarningActive) return;
    if (overflowWarningExpanded) {
      collapseOverflowWarning();
      return;
    }
    if (!activeSlideOverflows()) {
      overflowWarning.hidden = true;
      return;
    }
    showCollapsedOverflowIndicator();
  }

  function measureSlide(slide) {
    const hidden = !slide.classList.contains("is-active");
    const scroller = slide.querySelector(".slide__scroller");
    slide.classList.remove("has-overflow");
    if (hidden) slide.classList.add("is-measuring");
    const overflow = scroller.scrollHeight > scroller.clientHeight + 1 || scroller.scrollWidth > scroller.clientWidth + 1;
    const size = { width: slide.clientWidth, height: slide.clientHeight };
    if (hidden) slide.classList.remove("is-measuring");
    slide.classList.toggle("has-overflow", overflow);
    if (overflow) {
      scroller.tabIndex = 0;
    } else {
      scroller.removeAttribute("tabindex");
    }
    return { overflow, size };
  }

  async function measureOverflow(generation) {
    await (document.fonts?.ready || Promise.resolve());
    await diagramRender.catch(() => {});
    document.querySelectorAll("img, video").forEach((media) => {
      if (observedMedia.has(media)) return;
      observedMedia.add(media);
      if (media instanceof HTMLImageElement && !media.complete) {
        media.addEventListener("load", scheduleOverflowMeasurement, { once: true });
        media.addEventListener("error", scheduleOverflowMeasurement, { once: true });
      }
      if (media instanceof HTMLVideoElement) {
        prepareVideoPoster(media);
        media.addEventListener("loadedmetadata", scheduleOverflowMeasurement, { once: true });
        media.addEventListener("error", scheduleOverflowMeasurement, { once: true });
      }
    });
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    if (generation !== overflowMeasurement) return;

    const overflowing = [];
    let stageSize = { width: 0, height: 0 };
    slides.forEach((slide, index) => {
      const measurement = measureSlide(slide);
      stageSize = measurement.size;
      if (measurement.overflow) overflowing.push(index + 1);
    });

    if (overflowing.length === 0) {
      clearTimeout(overflowWarningTimer);
      clearTimeout(overflowTransitionTimer);
      overflowWarningExpanded = false;
      loadWarningActive = false;
      overflowWarning.classList.remove("is-collapsed", "is-dismissing");
      overflowWarning.hidden = true;
      overflowWarningText.textContent = "";
      overflowingSlides = [];
      lastOverflowReport = "";
      return;
    }

    const changed = overflowing.join(",") !== overflowingSlides.join(",");
    overflowingSlides = overflowing;
    if (!loadWarningShown) {
      loadWarningShown = true;
      showOverflowWarning(overflowing, true);
    } else if (changed) {
      updateOverflowWarningForActiveSlide();
    }
    const report = {
      revision: Number(document.body.dataset.revision),
      slides: overflowing,
      stageWidth: stageSize.width,
      stageHeight: stageSize.height,
    };
    const fingerprint = JSON.stringify({ revision: report.revision, slides: report.slides });
    if (fingerprint === lastOverflowReport) return;
    lastOverflowReport = fingerprint;
    fetch("/api/overflow", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(report),
    }).catch(() => {});
  }

  function prepareVideoPoster(video) {
    const captureFirstFrame = () => {
      if (!video.videoWidth || !video.videoHeight) return;

      const canvas = document.createElement("canvas");
      const maximumDimension = 1920;
      const scale = Math.min(1, maximumDimension / Math.max(video.videoWidth, video.videoHeight));
      canvas.width = Math.max(1, Math.round(video.videoWidth * scale));
      canvas.height = Math.max(1, Math.round(video.videoHeight * scale));
      const context = canvas.getContext("2d");
      if (!context) return;

      try {
        context.drawImage(video, 0, 0, canvas.width, canvas.height);
        video.poster = canvas.toDataURL("image/jpeg", 0.85);
      } catch {
        // Cross-origin videos without CORS permission cannot become data URLs.
        // The seek still leaves their first decoded frame visible in the player.
      }
    };

    const seekToFirstFrame = () => {
      const firstFrame = Number.isFinite(video.duration) ? Math.min(0.001, video.duration) : 0.001;
      if (firstFrame <= 0) return;

      video.addEventListener("seeked", captureFirstFrame, { once: true });
      try {
        video.currentTime = firstFrame;
      } catch {
        video.removeEventListener("seeked", captureFirstFrame);
      }
    };

    if (video.readyState >= HTMLMediaElement.HAVE_METADATA) {
      seekToFirstFrame();
    } else {
      video.addEventListener("loadedmetadata", seekToFirstFrame, { once: true });
    }
  }

  function scheduleOverflowMeasurement() {
    if (overview) return;
    const generation = ++overflowMeasurement;
    requestAnimationFrame(() => measureOverflow(generation));
  }

  function hashIndex() {
    const match = window.location.hash.match(/^#(?:slide-)?(\d+)$/);
    if (!match) return 0;
    const index = Number(match[1]) - 1;
    return Number.isInteger(index) && index >= 0 && index < slides.length ? index : 0;
  }

  function show(index, updateHash = true) {
    const next = Math.max(0, Math.min(index, slides.length - 1));
    const changed = next !== current;
    if (changed) slides[current].querySelector(".slide__scroller").scrollTop = 0;
    current = next;
    slides.forEach((slide, slideIndex) => {
      const active = slideIndex === current;
      slide.classList.toggle("is-active", active);
      slide.setAttribute("aria-hidden", String(!overview && !active));
      if (active) {
        slide.setAttribute("aria-current", "page");
      } else {
        slide.removeAttribute("aria-current");
      }
    });
    currentLabel.textContent = String(current + 1);
    if (updateHash) history.replaceState(null, "", `#${current + 1}`);
    if (changed) updateOverflowWarningForActiveSlide();
    if (changed && editor && !editor.hidden && !editorDirty) loadEditorSlide();
  }

  function focusOverviewSlide(index) {
    overviewIndex = Math.max(0, Math.min(index, slides.length - 1));
    slides.forEach((slide, slideIndex) => {
      const selected = slideIndex === overviewIndex;
      slide.tabIndex = selected ? 0 : -1;
      slide.setAttribute("aria-selected", String(selected));
    });
    slides[overviewIndex].focus({ preventScroll: true });
    slides[overviewIndex].scrollIntoView({ block: "nearest", inline: "nearest" });
  }

  function overviewColumnCount() {
    const firstTop = slides[0]?.offsetTop;
    if (firstTop === undefined) return 1;
    const count = slides.findIndex((slide) => Math.abs(slide.offsetTop - firstTop) > 1);
    return count === -1 ? slides.length : Math.max(1, count);
  }

  function toggleOverview() {
    overview = !overview;
    document.body.classList.toggle("is-overview", overview);
    deck.classList.toggle("is-overview", overview);
    const overviewLabel = overview ? "Close slide overview" : "Open slide overview";
    overviewButton.classList.toggle("is-active", overview);
    overviewButton.setAttribute("aria-expanded", String(overview));
    overviewButton.setAttribute("aria-label", overviewLabel);
    overviewButton.setAttribute("title", overviewLabel);

    if (overview) {
      overflowMeasurement += 1;
      deck.setAttribute("role", "grid");
      deck.setAttribute("aria-label", "Slide overview");
      deck.removeAttribute("aria-live");
      slides.forEach((slide) => {
        slide.setAttribute("role", "gridcell");
        slide.setAttribute("aria-hidden", "false");
      });
      requestAnimationFrame(() => focusOverviewSlide(current));
      return;
    }

    deck.removeAttribute("role");
    deck.removeAttribute("aria-label");
    deck.setAttribute("aria-live", "polite");
    slides.forEach((slide, index) => {
      slide.removeAttribute("role");
      slide.removeAttribute("aria-selected");
      slide.removeAttribute("tabindex");
      slide.setAttribute("aria-hidden", String(index !== current));
    });
    scheduleOverflowMeasurement();
  }

  function isEditable(target) {
    return target instanceof Element && Boolean(target.closest("input, textarea, select, [contenteditable]:not([contenteditable='false'])"));
  }

  function isMediaControl(target) {
    return target instanceof Element && Boolean(target.closest("video, audio"));
  }

  function setEditorStatus(message, error = false) {
    if (!editorStatus) return;
    editorStatus.textContent = message;
    editorStatus.classList.toggle("is-error", error);
    editorStatus.hidden = !message;
  }

  function updateEditorCancelLabel() {
    if (!editorCloseButton) return;
    editorCloseButton.textContent = editorDirty ? "Discard" : "Cancel";
  }

  function updateEditorSaveState() {
    if (!editorSaveButton) return;
    editorSaveButton.disabled = !editorDirty || editorSaveInProgress;
  }

  function updateEditorNavigation() {
    if (!editorPreviousButton || !editorNextButton) return;
    editorPreviousButton.disabled = current === 0;
    editorNextButton.disabled = current === slides.length - 1;
    editorSlideNumber.textContent = `${current + 1} / ${slides.length}`;
  }

  function discardEditorChanges() {
    clearTimeout(editorPreviewTimer);
    editorPreviewGeneration += 1;
    if (editorDirty) updateActiveSlide(editorOriginalHTML);
    editorDirty = false;
    updateEditorCancelLabel();
    updateEditorSaveState();
    setEditorStatus("");
  }

  function navigateEditor(direction) {
    if (editorDirty && !window.confirm("Discard unsaved changes to this slide?")) return;
    if (editorDirty) discardEditorChanges();
    show(current + direction);
    updateEditorNavigation();
  }

  function updateActiveSlide(html) {
    slides[current].querySelector(".slide__content").innerHTML = html;
    diagrams = collectDiagrams();
    scheduleDiagramRender();
    scheduleOverflowMeasurement();
  }

  async function loadEditorSlide() {
    if (!editorSource) return;
    setEditorStatus("Loading…");
    try {
      const slide = current + 1;
      const response = await fetch(`/api/source?slide=${encodeURIComponent(slide)}`, { cache: "no-store" });
      if (!response.ok) throw new Error("Could not load the Markdown source.");
      const source = await response.json();
      editorOriginalHTML = slides[current].querySelector(".slide__content").innerHTML;
      editorSource.value = source.source;
      editorRevision = source.revision;
      editorSlide = source.slide;
      editorDirty = false;
      updateEditorCancelLabel();
      updateEditorSaveState();
      setEditorStatus("");
      updateEditorNavigation();
    } catch (error) {
      setEditorStatus(error instanceof Error ? error.message : "Could not load the Markdown source.", true);
    }
  }

  function scheduleEditorPreview() {
    clearTimeout(editorPreviewTimer);
    const generation = ++editorPreviewGeneration;
    editorPreviewTimer = setTimeout(async () => {
      try {
        const response = await fetch("/api/source/preview", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ source: editorSource.value, revision: editorRevision, slide: editorSlide }),
        });
        if (!response.ok) throw new Error(await response.text() || "Could not preview the Markdown source.");
        const preview = await response.json();
        if (generation !== editorPreviewGeneration || editorSlide !== current + 1) return;
        updateActiveSlide(preview.html);
        setEditorStatus("Previewing unsaved changes.");
      } catch (error) {
        if (generation === editorPreviewGeneration) setEditorStatus(error instanceof Error ? error.message : "Could not preview the Markdown source.", true);
      }
    }, 250);
  }

  async function openEditor() {
    if (!editor || !editorSource) return;
    editor.hidden = false;
    editorButton.setAttribute("aria-expanded", "true");
    await loadEditorSlide();
    editorSource.focus();
  }

  function closeEditor() {
    if (!editor || editor.hidden) return;
    if (editorDirty && !window.confirm("Discard unsaved changes to this slide?")) return;
    discardEditorChanges();
    editor.hidden = true;
    editorButton.setAttribute("aria-expanded", "false");
    editorButton.focus();
  }

  async function saveEditor() {
    if (!editorSource || !editorSaveButton || !editorDirty) return;
    clearTimeout(editorPreviewTimer);
    editorPreviewGeneration += 1;
    editorSaveInProgress = true;
    updateEditorSaveState();
    setEditorStatus("Saving…");
    try {
      const response = await fetch("/api/source", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ source: editorSource.value, revision: editorRevision, slide: editorSlide }),
      });
      if (!response.ok) {
        const detail = await response.text();
        throw new Error(detail || "Could not save the Markdown source.");
      }
      const saved = await response.json();
      editorRevision = saved.revision;
      editorDirty = false;
	updateEditorCancelLabel();
      editorExpectedDeckRevision = saved.deckRevision;
      document.body.dataset.revision = String(saved.deckRevision);
      updateActiveSlide(saved.html);
      editorOriginalHTML = saved.html;
      setEditorStatus("Saved.");
    } catch (error) {
      setEditorStatus(error instanceof Error ? error.message : "Could not save the Markdown source.", true);
    } finally {
      editorSaveInProgress = false;
      updateEditorSaveState();
    }
  }

  function closePresentationTab() {
    // Browsers ignore window.close() for tabs that were not opened by script.
    window.close();
  }

  function scrollActiveSlide(direction, key) {
    const slide = slides[current];
    if (!slide.classList.contains("has-overflow")) return false;
    const scroller = slide.querySelector(".slide__scroller");
    const maximum = scroller.scrollHeight - scroller.clientHeight;
    if (direction > 0 && scroller.scrollTop >= maximum - 1) return false;
    if (direction < 0 && scroller.scrollTop <= 1) return false;
    const pageDistance = Math.max(1, scroller.clientHeight * 0.8);
    const arrowDistance = Math.max(40, scroller.clientHeight * 0.12);
    scroller.scrollBy({ top: direction * (key.startsWith("Arrow") ? arrowDistance : pageDistance) });
    return true;
  }

  overflowWarning.addEventListener("click", () => {
    if (overflowWarningExpanded) {
      collapseOverflowWarning();
    } else if (activeSlideOverflows()) {
      showOverflowWarning([current + 1]);
    }
  });

  overviewButton.addEventListener("click", toggleOverview);

  if (editorButton) {
    editorButton.addEventListener("click", () => editor?.hidden ? openEditor() : closeEditor());
    editorCloseButton.addEventListener("click", closeEditor);
    editorPreviousButton.addEventListener("click", () => navigateEditor(-1));
    editorNextButton.addEventListener("click", () => navigateEditor(1));
    editorSource.addEventListener("input", () => {
      editorDirty = true;
      updateEditorCancelLabel();
      updateEditorSaveState();
      setEditorStatus("Unsaved changes.");
      scheduleEditorPreview();
    });
    editorSaveButton.addEventListener("click", saveEditor);
    editorSource.addEventListener("keydown", (event) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
        saveEditor();
      }
      if (event.key === "Escape" && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey) {
        event.preventDefault();
        closeEditor();
      }
    });
    editorResizeHandle.addEventListener("pointerdown", (event) => {
      event.preventDefault();
      editorResizeHandle.setPointerCapture(event.pointerId);
      const resize = (moveEvent) => {
        const width = Math.max(320, Math.min(window.innerWidth * 0.85, window.innerWidth - moveEvent.clientX));
        editor.style.setProperty("--editor-width", `${width}px`);
      };
      editorResizeHandle.addEventListener("pointermove", resize);
      editorResizeHandle.addEventListener("pointerup", () => editorResizeHandle.removeEventListener("pointermove", resize), { once: true });
      editorResizeHandle.addEventListener("pointercancel", () => editorResizeHandle.removeEventListener("pointermove", resize), { once: true });
    });
  }

  deck.addEventListener("click", (event) => {
    if (editor && !editor.hidden) return;
    const clickedSlide = event.target instanceof Element ? event.target.closest(".slide") : null;
    if (overview && clickedSlide) {
      event.preventDefault();
      show(slides.indexOf(clickedSlide));
      toggleOverview();
      return;
    }
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      isEditable(event.target) ||
      isMediaControl(event.target) ||
      !(event.target instanceof Element) ||
      !event.target.closest(".slide") ||
      event.target.closest("a, button, summary, input, select, textarea, [contenteditable]:not([contenteditable='false'])")
    ) return;

    show(current + 1);
  });

  document.addEventListener("keydown", (event) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || isEditable(event.target) || isMediaControl(event.target)) return;

    if (editor && !editor.hidden) return;

    if (event.key.toLowerCase() === "o") {
      event.preventDefault();
      toggleOverview();
      return;
    }

    if (overview) {
      const columns = overviewColumnCount();
      let next = overviewIndex;
      if (event.key === "ArrowRight") next += 1;
      else if (event.key === "ArrowLeft") next -= 1;
      else if (event.key === "ArrowDown") next += columns;
      else if (event.key === "ArrowUp") next -= columns;
      else if (event.key === "Home") next = 0;
      else if (event.key === "End") next = slides.length - 1;
      else if (event.key === "Enter") {
        event.preventDefault();
        show(overviewIndex);
        toggleOverview();
        return;
      } else if (event.key === "Escape") {
        event.preventDefault();
        toggleOverview();
        return;
      } else {
        return;
      }
      event.preventDefault();
      focusOverviewSlide(next);
      return;
    }

    const scrollNext = ["ArrowDown", "PageDown", " "];
    const scrollPrevious = ["ArrowUp", "PageUp"];
    if (event.key === "ArrowRight") {
      event.preventDefault();
      show(current + 1);
    } else if (scrollNext.includes(event.key)) {
      event.preventDefault();
      if (!scrollActiveSlide(1, event.key)) show(current + 1);
    } else if (event.key === "ArrowLeft") {
      event.preventDefault();
      show(current - 1);
    } else if (scrollPrevious.includes(event.key)) {
      event.preventDefault();
      if (!scrollActiveSlide(-1, event.key)) show(current - 1);
    } else if (event.key === "Home") {
      event.preventDefault();
      show(0);
    } else if (event.key === "End") {
      event.preventDefault();
      show(slides.length - 1);
    } else if (event.key === "Escape") {
      event.preventDefault();
      closePresentationTab();
    }
  });

  window.addEventListener("hashchange", () => show(hashIndex()));
  window.addEventListener("resize", scheduleOverflowMeasurement);
  show(hashIndex());
  scheduleDiagramRender();
  scheduleOverflowMeasurement();
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
    document.addEventListener("fullscreenchange", () => {
      updateFullscreenButton();
      scheduleOverflowMeasurement();
    });
    updateFullscreenButton();
  } else {
    fullscreenButton.hidden = true;
  }

  const session = new EventSource(`/api/session?revision=${encodeURIComponent(document.body.dataset.revision)}`);
  session.addEventListener("reload", (event) => {
    const revision = Number(event.data);
    if (editorSaveInProgress || revision === editorExpectedDeckRevision) return;
    if (editorDirty) {
      setEditorStatus("The file changed outside the editor. Save will report a conflict.", true);
      return;
    }
    window.location.reload();
  });
})();
