(async () => {
  "use strict";

  const passed = [];
  const errors = [];
  window.addEventListener("error", (event) => { errors.push(event.message); });
  document.addEventListener("securitypolicyviolation", (event) => { errors.push(`CSP: ${event.violatedDirective}`); });

  function check(condition, message) {
    if (!condition) throw new Error(message);
  }

  function near(actual, expected, message) {
    check(Math.abs(actual - expected) < 0.01, `${message}: ${actual} != ${expected}`);
  }

  async function waitFor(predicate) {
    for (let attempt = 0; attempt < 400; attempt++) {
      if (predicate()) return;
      await new Promise((resolve) => setTimeout(resolve, 25));
    }
    throw new Error("Timed out waiting for viewer state");
  }

  const viewport = document.getElementById("viewport");
  const graph = document.getElementById("graph");
  const zoomIn = document.getElementById("zoom-in");
  const zoomOut = document.getElementById("zoom-out");
  const fit = document.getElementById("fit");
  const level = document.getElementById("zoom-level");

  function view() {
    const matrix = new DOMMatrix(getComputedStyle(graph).transform);
    check([matrix.a, matrix.d, matrix.e, matrix.f].every(Number.isFinite), "Transform must stay finite");
    return { scale: matrix.a, x: matrix.e, y: matrix.f };
  }

  function key(value, modifiers = {}) {
    viewport.dispatchEvent(new KeyboardEvent("keydown", { key: value, bubbles: true, cancelable: true, ...modifiers }));
  }

  try {
    await waitFor(() => !fit.disabled);
    check(level.value === "100%", "Graph must start fitted at 100%");
    const initial = view();
    check(initial.scale > 0 && viewport.tabIndex === 0, "Graph must be visible and keyboard-focusable");
    passed.push("initial fit");

    zoomIn.click();
    near(view().scale, initial.scale * 1.25, "Zoom-in button");
    check(level.value === "125%", "Zoom percentage must update");
    zoomOut.click();
    near(view().scale, initial.scale, "Zoom-out button");
    passed.push("zoom buttons");

    const bounds = viewport.getBoundingClientRect();
    const point = { x: bounds.width * 0.3, y: bounds.height * 0.4 };
    const beforeWheel = view();
    const wheel = new WheelEvent("wheel", {
      deltaY: -120, clientX: bounds.left + point.x, clientY: bounds.top + point.y,
      bubbles: true, cancelable: true,
    });
    viewport.dispatchEvent(wheel);
    const afterWheel = view();
    const anchor = { x: wheel.clientX - bounds.left, y: wheel.clientY - bounds.top };
    check(wheel.defaultPrevented && afterWheel.scale > beforeWheel.scale, "Wheel must zoom instead of scrolling the page");
    near((anchor.x - afterWheel.x) / afterWheel.scale, (anchor.x - beforeWheel.x) / beforeWheel.scale, "Cursor X anchor");
    near((anchor.y - afterWheel.y) / afterWheel.scale, (anchor.y - beforeWheel.y) / beforeWheel.scale, "Cursor Y anchor");
    passed.push("cursor-centered wheel zoom");

    fit.click();
    viewport.focus();
    key("+");
    check(level.value === "125%", "Keyboard zoom-in");
    const beforeShortcut = view();
    key("+", { ctrlKey: true });
    near(view().scale, beforeShortcut.scale, "Browser shortcuts must not be intercepted");
    key("ArrowRight");
    near(view().x, beforeShortcut.x - 40, "Keyboard pan");
    key("0");
    near(view().x, initial.x, "Keyboard fit position");
    near(view().scale, initial.scale, "Keyboard fit scale");
    key("-");
    check(level.value === "80%", "Keyboard zoom-out");
    key("Home");
    check(level.value === "100%", "Home must fit the graph");
    const input = document.createElement("input");
    input.style.position = "absolute";
    document.body.appendChild(input);
    input.dispatchEvent(new KeyboardEvent("keydown", { key: "+", bubbles: true, cancelable: true }));
    check(level.value === "100%", "Typing in an input must not change graph zoom");
    input.remove();
    passed.push("keyboard navigation");

    // Synthetic pointer events cannot establish native capture. Stub only the
    // capture API; exercise the actual production pointer handlers and state.
    const captureMethods = [viewport.setPointerCapture, viewport.hasPointerCapture, viewport.releasePointerCapture];
    let captured = null;
    viewport.setPointerCapture = (id) => { captured = id; };
    viewport.hasPointerCapture = (id) => captured === id;
    viewport.releasePointerCapture = () => { captured = null; };
    function pointer(type, x, y, options = {}) {
      viewport.dispatchEvent(new PointerEvent(type, {
        pointerId: 7, isPrimary: true, button: 0, clientX: x, clientY: y, bubbles: true, ...options,
      }));
    }
    const beforeDrag = view();
    pointer("pointerdown", 100, 100);
    check(captured === 7 && viewport.classList.contains("dragging"), "Drag must capture its pointer");
    pointer("pointermove", 160, 140);
    near(view().x, beforeDrag.x + 60, "Pointer pan X");
    near(view().y, beforeDrag.y + 40, "Pointer pan Y");
    pointer("pointerup", 160, 140);
    check(captured === null && !viewport.classList.contains("dragging"), "Pointer-up must end dragging");
    const afterDrag = view();
    pointer("pointermove", 200, 200);
    near(view().x, afterDrag.x, "Movement after release must not pan");
    pointer("pointerdown", 100, 100);
    pointer("pointercancel", 100, 100);
    check(captured === null && !viewport.classList.contains("dragging"), "Cancelled pointer must end dragging");
    pointer("pointerdown", 100, 100);
    captured = null;
    pointer("lostpointercapture", 100, 100);
    check(!viewport.classList.contains("dragging"), "Lost capture must end dragging");
    pointer("pointerdown", 100, 100, { button: 2 });
    check(captured === null, "Right click must not start dragging");
    [viewport.setPointerCapture, viewport.hasPointerCapture, viewport.releasePointerCapture] = captureMethods;
    passed.push("pointer panning and cancellation");

    fit.click();
    for (let count = 0; count < 100; count++) zoomOut.click();
    check(zoomOut.disabled && level.value === "10%", "Minimum zoom must be bounded");
    for (let count = 0; count < 100; count++) zoomIn.click();
    check(zoomIn.disabled && Number.isFinite(view().scale), "Maximum zoom must be bounded");
    passed.push("zoom bounds");

    viewport.style.width = "640px";
    window.dispatchEvent(new Event("resize"));
    const resized = view();
    check(level.value === "100%", "Resize must refit the graph");
    check(graph.naturalWidth * resized.scale <= viewport.clientWidth + 0.01, "Fitted width must fit viewport");
    check(graph.naturalHeight * resized.scale <= viewport.clientHeight + 0.01, "Fitted height must fit viewport");
    near(resized.x, (viewport.clientWidth - graph.naturalWidth * resized.scale) / 2, "Fitted graph must stay centered");
    passed.push("resize refit");

    graph.src = "missing.svg";
    await waitFor(() => level.value === "Load failed");
    check(zoomIn.disabled && zoomOut.disabled && fit.disabled, "Failed image must disable navigation");
    passed.push("image failure handling");
    check(errors.length === 0, `Browser errors: ${errors.join("; ")}`);

    new Image().src = `/test-report?result=${encodeURIComponent(JSON.stringify({ passed }))}`;
  } catch (error) {
    new Image().src = `/test-report?result=${encodeURIComponent(JSON.stringify({ passed, error: `${error}\n${error.stack || ""}` }))}`;
  }
})();
