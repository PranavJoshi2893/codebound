(() => {
  "use strict";

  const viewport = document.getElementById("viewport");
  const graph = document.getElementById("graph");
  const zoomIn = document.getElementById("zoom-in");
  const zoomOut = document.getElementById("zoom-out");
  const fit = document.getElementById("fit");
  const zoomLevel = document.getElementById("zoom-level");
  const zoomStep = 1.25;
  const minimumZoom = 0.1;

  // Zoom is relative to the fitted graph, so Fit always reads 100%.
  const view = { zoom: 1, fitScale: 1, maximumZoom: 32, x: 0, y: 0 };
  let ready = false;
  let drag = null;

  function fitGraph() {
    if (!ready || viewport.clientWidth === 0 || viewport.clientHeight === 0) return;

    const width = Math.max(1, viewport.clientWidth - 32);
    const height = Math.max(1, viewport.clientHeight - 32);
    view.fitScale = Math.min(width / graph.naturalWidth, height / graph.naturalHeight);
    // Even a very large graph can be enlarged to four times its natural size.
    view.maximumZoom = Math.max(32, 4 / view.fitScale);
    view.zoom = 1;
    view.x = (viewport.clientWidth - graph.naturalWidth * view.fitScale) / 2;
    view.y = (viewport.clientHeight - graph.naturalHeight * view.fitScale) / 2;
    renderView();
  }

  function renderView() {
    graph.style.transform = `translate(${view.x}px, ${view.y}px) scale(${view.fitScale * view.zoom})`;
    const percentage = `${Math.round(view.zoom * 100)}%`;
    if (zoomLevel.value !== percentage) zoomLevel.value = percentage;
    zoomIn.disabled = view.zoom >= view.maximumZoom;
    zoomOut.disabled = view.zoom <= minimumZoom;
  }

  function zoomAt(factor, x, y) {
    if (!ready) return;

    const nextZoom = Math.min(view.maximumZoom, Math.max(minimumZoom, view.zoom * factor));
    const ratio = nextZoom / view.zoom;
    // Keep the graph point under the cursor fixed while changing its scale.
    view.x = x - (x - view.x) * ratio;
    view.y = y - (y - view.y) * ratio;
    view.zoom = nextZoom;
    renderView();
  }

  function zoomAtCenter(factor) {
    zoomAt(factor, viewport.clientWidth / 2, viewport.clientHeight / 2);
  }

  function panBy(x, y) {
    view.x += x;
    view.y += y;
    renderView();
  }

  function handleWheel(event) {
    if (!ready) return;
    event.preventDefault();

    // Wheel deltas may be pixels, lines, or pages depending on the device.
    const unit = event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? viewport.clientHeight : 1;
    const delta = Math.max(-240, Math.min(240, event.deltaY * unit));
    const bounds = viewport.getBoundingClientRect();
    zoomAt(Math.exp(-delta * 0.002), event.clientX - bounds.left, event.clientY - bounds.top);
  }

  function startDrag(event) {
    if (!ready || event.button !== 0 || !event.isPrimary) return;
    viewport.focus({ preventScroll: true });
    viewport.setPointerCapture(event.pointerId);
    drag = { id: event.pointerId, x: event.clientX, y: event.clientY };
    viewport.classList.add("dragging");
  }

  function moveDrag(event) {
    if (drag === null || drag.id !== event.pointerId) return;
    panBy(event.clientX - drag.x, event.clientY - drag.y);
    drag.x = event.clientX;
    drag.y = event.clientY;
  }

  function endDrag(event) {
    if (drag === null || drag.id !== event.pointerId) return;
    drag = null;
    viewport.classList.remove("dragging");
    if (viewport.hasPointerCapture(event.pointerId)) {
      viewport.releasePointerCapture(event.pointerId);
    }
  }

  function handleKey(event) {
    if (!ready || event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey) return;
    if (event.target.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(event.target.tagName)) return;

    switch (event.key) {
      case "+":
      case "=":
        zoomAtCenter(zoomStep);
        break;
      case "-":
      case "_":
        zoomAtCenter(1 / zoomStep);
        break;
      case "0":
      case "Home":
        fitGraph();
        break;
      case "ArrowLeft":
        panBy(40, 0);
        break;
      case "ArrowRight":
        panBy(-40, 0);
        break;
      case "ArrowUp":
        panBy(0, 40);
        break;
      case "ArrowDown":
        panBy(0, -40);
        break;
      default:
        return;
    }
    event.preventDefault();
  }

  function loadGraph() {
    if (graph.naturalWidth === 0 || graph.naturalHeight === 0) {
      failGraph();
      return;
    }
    graph.style.width = `${graph.naturalWidth}px`;
    graph.style.height = `${graph.naturalHeight}px`;
    ready = true;
    fit.disabled = false;
    fitGraph();
  }

  function failGraph() {
    ready = false;
    if (drag !== null) endDrag({ pointerId: drag.id });
    zoomIn.disabled = true;
    zoomOut.disabled = true;
    fit.disabled = true;
    zoomLevel.value = "Load failed";
  }

  zoomIn.addEventListener("click", () => zoomAtCenter(zoomStep));
  zoomOut.addEventListener("click", () => zoomAtCenter(1 / zoomStep));
  fit.addEventListener("click", fitGraph);
  viewport.addEventListener("wheel", handleWheel, { passive: false });
  viewport.addEventListener("pointerdown", startDrag);
  viewport.addEventListener("pointermove", moveDrag);
  viewport.addEventListener("pointerup", endDrag);
  viewport.addEventListener("pointercancel", endDrag);
  viewport.addEventListener("lostpointercapture", endDrag);
  document.addEventListener("keydown", handleKey);
  window.addEventListener("resize", fitGraph);
  graph.addEventListener("load", loadGraph);
  graph.addEventListener("error", failGraph);
  if (graph.complete) loadGraph();
})();
