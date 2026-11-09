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

  async function waitFor(predicate, message = "Timed out waiting for viewer state") {
    for (let attempt = 0; attempt < 400; attempt++) {
      if (predicate()) return;
      await new Promise((resolve) => setTimeout(resolve, 25));
    }
    throw new Error(message);
  }

  const viewport = document.getElementById("viewport");
  const graph = document.getElementById("graph");
  const zoomIn = document.getElementById("zoom-in");
  const zoomOut = document.getElementById("zoom-out");
  const fit = document.getElementById("fit");
  const actual = document.getElementById("actual-size");
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
    check(level.value === "100%", "Graph must start at readable natural size, not a graph-relative fit percentage");
    const initial = view();
    check(initial.scale > 0 && viewport.tabIndex === 0, "Graph must be visible and keyboard-focusable");
    check(document.documentElement.scrollWidth <= window.innerWidth + 1, "Initial layout must fit the screen without horizontal page scrolling");
    const initialBounds = viewport.getBoundingClientRect();
    check(initialBounds.right <= window.innerWidth + 0.5 && initialBounds.bottom <= window.innerHeight + 0.5, "The actual graph canvas must stay inside the screen, not hide an overflowing grid column");
    const rootBounds = document.querySelector('#graph-nodes [data-function-id="0"]').getBoundingClientRect();
    check(rootBounds.left >= initialBounds.left && rootBounds.right <= initialBounds.right, "The readable root must be visible inside the canvas on narrow screens");
    near(initial.scale, 1, "100% must mean natural size");
    passed.push("readable initial size");

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

    actual.click();
    viewport.focus();
    key("+");
    check(level.value === "125%", "Keyboard zoom-in");
    const beforeShortcut = view();
    key("+", { ctrlKey: true });
    near(view().scale, beforeShortcut.scale, "Browser shortcuts must not be intercepted");
    key("ArrowRight");
    near(view().x, beforeShortcut.x - 40, "Keyboard pan");
    key("0");
    const fitted = view();
    check(fitted.scale <= 1 && graph.naturalWidth * fitted.scale <= viewport.clientWidth, "Fit must show the whole graph without enlarging small graphs");
    key("-");
    near(view().scale, fitted.scale * 0.8, "Keyboard zoom-out");
    key("Home");
    near(view().scale, fitted.scale, "Home must fit the graph");
    key("1");
    check(level.value === "100%", "1 must restore readable natural size");
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

    const beforeResize = view();
    const oldWidth = viewport.clientWidth;
    const oldHeight = viewport.clientHeight;
    const centerX = (oldWidth / 2 - beforeResize.x) / beforeResize.scale;
    const centerY = (oldHeight / 2 - beforeResize.y) / beforeResize.scale;
    viewport.style.width = "640px";
    // ResizeObserver delivery is asynchronous and is not guaranteed to finish
    // within a fixed number of animation frames in headless browsers. Wait for
    // the observable camera result; broken resize handling still times out.
    await waitFor(() => {
      const current = view();
      return Math.abs((viewport.clientWidth / 2 - current.x) / current.scale - centerX) < 0.01
        && Math.abs((viewport.clientHeight / 2 - current.y) / current.scale - centerY) < 0.01;
    }, "Resize must preserve the graph point at the viewport center");
    const resized = view();
    near(resized.scale, beforeResize.scale, "Resize must preserve user zoom rather than jumping to a new fit");
    near((viewport.clientWidth / 2 - resized.x) / resized.scale, (oldWidth / 2 - beforeResize.x) / beforeResize.scale, "Resize must preserve the graph point at the horizontal center");
    near((viewport.clientHeight / 2 - resized.y) / resized.scale, (oldHeight / 2 - beforeResize.y) / beforeResize.scale, "Resize must preserve the graph point at the vertical center");
    fit.click();
    check(graph.naturalWidth * view().scale <= viewport.clientWidth + 0.01, "Explicit fit must fit viewport width");
    check(graph.naturalHeight * view().scale <= viewport.clientHeight + 0.01, "Explicit fit must fit viewport height");
    actual.click();
    passed.push("stable resize and explicit fit/readable size");

    const search = document.getElementById("function-search");
    const matches = document.getElementById("function-list");
    const details = document.getElementById("function-details");
    await waitFor(() => !search.disabled);
    check(document.getElementById("reading-guide").open, "Reading guide must be open for first-time readers");
    check(document.getElementById("overview").textContent.includes("3 functions"), "Overview must summarize the analyzed tree");
    check(details.querySelector("h2").textContent.includes("Root function"), "Explorer must start at the selected root");
    check(details.querySelectorAll(".call-list .function-button").length === 1, "Identical repeated call evidence must be normalized once before rendering");
    check(details.textContent.includes("Launched in a goroutine") && details.textContent.includes("interface"), "Calls must explain concurrency and interface dispatch");
    passed.push("guided root overview");

    function find(query) {
      search.value = query;
      search.dispatchEvent(new Event("input", { bubbles: true }));
    }
    find("WORKER.GO");
    check(matches.children.length === 1, "Search must match file paths, case-insensitively");
    matches.firstElementChild.click();
    check(details.querySelector("h2").textContent === "Worker.Run", "Choosing a search result must show its details");
    check(matches.firstElementChild.getAttribute("aria-pressed") === "true", "Selected search result must be identified accessibly");
    check(details.textContent.includes("ctx receives possibly C1 or I1 or an unknown origin"), "Details must preserve multiple and unresolved context origins");
    check(details.textContent.includes("does not prove the function stops"), "Context observations must not overstate runtime behavior");
    check(details.textContent.includes("detaching cancellation and deadlines"), "Detached contexts need a plain-language explanation");
    check(details.textContent.includes("Deferred until the caller returns"), "Deferred calls must be explained");
    check(details.textContent.includes("Architecture warning (naming heuristic)"), "Architecture warnings must retain their heuristic caveat");
    check(details.textContent.includes("worker.go:6") && details.textContent.includes("Technical evidence"), "Experts must retain source locations and API evidence");
    check(details.querySelectorAll(".evidence").length === 50, "Large context finding sets must not all be mounted at once");
    const moreEvidence = details.querySelector('[data-more-items="context findings"]');
    check(!moreEvidence.hidden && details.textContent.includes("Showing 50 of 62"), "Paged findings must disclose the full evidence count");
    moreEvidence.click();
    check(details.querySelectorAll(".evidence").length === 62 && moreEvidence.hidden, "All evidence must remain reachable without truncation");
    const beforePanelKey = view();
    details.querySelector("h2").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, cancelable: true }));
    near(view().y, beforePanelKey.y, "Reading-panel arrow keys must not pan the graph");
    passed.push("search and plain-language context details");

    const leafButton = [...details.querySelectorAll("button")].find((button) => button.textContent.includes("not markup"));
    leafButton.click();
    check(details.querySelector("h2").textContent === "<script>not markup</script>", "Calls must navigate to their target function");
    check(details.querySelector("script") === null, "Source names must be rendered as text, not executable markup");
    check(details.textContent.includes("No further calls shown") && details.textContent.includes("No context actions were identified"), "Leaf functions need clear, conservative empty states");
    find("does-not-exist");
    check(matches.children.length === 0 && document.getElementById("search-status").textContent.includes("No matching"), "Empty searches must explain how to recover");
    document.getElementById("show-root").click();
    check(search.value === "" && matches.children.length === 3, "Back to root must clear the search");
    check(details.querySelector("h2").textContent.includes("Root function"), "Back to root must restore the starting point");
    passed.push("function navigation, safe text, and empty states");

    const nodeLayer = document.getElementById("graph-nodes");
    const mode = document.getElementById("graph-mode");
    const scope = document.getElementById("graph-scope");
    const diagramStatus = document.getElementById("diagram-status");
    await waitFor(() => nodeLayer.children.length === 3);
    check(performance.getEntriesByType("resource").filter((entry) => entry.name.includes("/calltree.svg")).length === 1, "Initial graph and click metadata must use a single network request");
    check(mode.value === "calls" && document.getElementById("context-key").hidden, "The default diagram must be a simple call flow, without context clutter");
    nodeLayer.querySelector('[data-function-id="1"]').click();
    check(details.querySelector("h2").textContent === "Worker.Run", "Clicking a graph function must update text details");
    check(nodeLayer.querySelector('[data-function-id="1"]').getAttribute("aria-pressed") === "true", "Selected graph nodes must have an accessible visual highlight");
    zoomIn.click();
    check(nodeLayer.style.transform === graph.style.transform, "Clickable nodes must follow image zoom/pan exactly");
    const hit = nodeLayer.querySelector('[data-function-id="1"]');
    const overlayStyle = getComputedStyle(hit);
    const markerStyle = getComputedStyle(hit, "::after");
    check(overlayStyle.borderTopWidth === "0px" && overlayStyle.borderRightWidth === "0px", "Click targets must not draw another border over the SVG box");
    check(markerStyle.opacity === "1" && parseFloat(markerStyle.top) <= -6 && parseFloat(markerStyle.left) <= -6, "Selection markers must remain outside the original node border");
    check(overlayStyle.color === "rgb(180, 83, 9)", "Selected markers must use the amber selection color");
    check(markerStyle.pointerEvents === "none", "Selection markers must not intercept adjacent graph interactions");
    // Exercise the real hover selectors/cascade without a browser automation
    // dependency: a temporary class has the same specificity as :hover.
    const hoverRules = [];
    for (const sheet of document.styleSheets) {
      for (const rule of sheet.cssRules) {
        if (!rule.selectorText?.includes(".graph-node-hit") || !rule.selectorText.includes(":hover")) continue;
        hoverRules.push([rule, rule.selectorText]);
        rule.selectorText = rule.selectorText.replaceAll(":hover", ".test-hover");
      }
    }
    try {
      hit.classList.add("test-hover");
      check(getComputedStyle(hit).color === "rgb(180, 83, 9)" && getComputedStyle(hit).borderTopWidth === "0px", "Hovering a selected node must keep amber markers without introducing another box border");
    } finally {
      hit.classList.remove("test-hover");
      for (const [rule, selector] of hoverRules) rule.selectorText = selector;
    }
    hit.focus({ preventScroll: true });
    check(document.activeElement === hit, "Graph nodes must remain keyboard-focusable");
    if (hit.matches(":focus-visible")) {
      check(getComputedStyle(hit).outlineStyle === "none", "Keyboard focus must not stack another outline on the SVG border");
      check(parseFloat(getComputedStyle(hit, "::after").top) <= -9, "Keyboard focus needs its own visibly separated marker state");
    }
    const imageBounds = graph.getBoundingClientRect();
    const hitBounds = hit.getBoundingClientRect();
    check(hitBounds.width > 0 && hitBounds.left >= imageBounds.left && hitBounds.right <= imageBounds.right + 1, "Graphviz hit regions must line up with the rendered image");
    passed.push("clickable graph nodes and synchronized highlights");
    passed.push("single SVG border with separate selection/focus markers");

    async function changeView(control, value, count) {
      const previousSource = graph.src;
      const previousScale = view().scale;
      const previousBounds = { width: viewport.clientWidth, height: viewport.clientHeight };
      control.value = value;
      control.dispatchEvent(new Event("change", { bubbles: true }));
      await waitFor(() => graph.src !== previousSource && graph.complete && viewport.getAttribute("aria-busy") === "false" && nodeLayer.children.length === count);
      near(view().scale, previousScale, "Changing graph content must preserve readable zoom");
      near(viewport.clientWidth, previousBounds.width, "Changing graph content must not resize the canvas width");
      near(viewport.clientHeight, previousBounds.height, "Status and legend changes must not resize the canvas height");
    }
    await changeView(mode, "contexts", 3);
    check(!document.getElementById("context-key").hidden && diagramStatus.textContent.includes("Calls + context"), "Context mode must reveal its distinct link legend");
    await changeView(scope, "neighbors", 3);
    check(diagramStatus.textContent.includes("Focus: Worker.Run") && diagramStatus.textContent.includes("relationships are hidden"), "Focused views must name the center and disclose omitted relationships");
    const previousFocusSource = graph.src;
    nodeLayer.querySelector('[data-function-id="2"]').click();
    await waitFor(() => graph.src !== previousFocusSource && graph.complete && nodeLayer.children.length === 2);
    check(nodeLayer.querySelector('[data-function-id="0"]') === null, "Following a leaf in focused mode must hide the unrelated original root");
    check(details.querySelector("h2").textContent.includes("not markup"), "Graph navigation must preserve exact function identities");
    await changeView(scope, "all", 3);
    await changeView(mode, "calls", 3);
    check(document.getElementById("context-key").hidden, "Returning to simple mode must hide context-specific legend entries");
    fit.click();
    await changeView(mode, "contexts", 3);
    await changeView(mode, "calls", 3);
    actual.click();
    passed.push("context toggle and graph-first focused navigation");

    {
      const fetchBeforeRace = window.fetch;
      const stableSource = graph.src;
      let releaseStaleResponse;
      window.fetch = (url, options) => String(url).startsWith("calltree.svg?") ? new Promise((resolve) => {
        releaseStaleResponse = () => resolve(new Response("stale non-SVG response"));
      }) : fetchBeforeRace(url, options);
      mode.value = "contexts";
      mode.dispatchEvent(new Event("change", { bubbles: true }));
      mode.value = "calls";
      mode.dispatchEvent(new Event("change", { bubbles: true }));
      releaseStaleResponse();
      await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      check(graph.src === stableSource && mode.value === "calls" && document.getElementById("context-key").hidden, "An obsolete response must never replace the latest selected view");
      check(viewport.getAttribute("aria-busy") === "false", "Returning to the shown view must cancel stale loading state");
      window.fetch = fetchBeforeRace;
      passed.push("latest-request-wins graph updates");
    }

    const originalFetch = window.fetch;
    const lastGoodSource = graph.src;
    window.fetch = (url, options) => String(url).startsWith("calltree.svg?") ? Promise.resolve(new Response("", { status: 500 })) : originalFetch(url, options);
    mode.value = "contexts";
    mode.dispatchEvent(new Event("change", { bubbles: true }));
    await waitFor(() => diagramStatus.textContent.includes("previous graph is still shown"));
    check(graph.src === lastGoodSource && nodeLayer.children.length === 3 && mode.value === "calls", "Failed layouts must retain the last usable graph and restore its mode");
    window.fetch = originalFetch;
    document.getElementById("show-root").click();
    passed.push("graph layout failure recovery");

    graph.src = "missing.svg";
    await waitFor(() => level.value === "Load failed");
    check(zoomIn.disabled && zoomOut.disabled && fit.disabled, "Failed image must disable navigation");
    check(!search.disabled && details.textContent.includes("Root function"), "Text exploration must remain available when the diagram fails");
    passed.push("image failure handling");
    check(errors.length === 0, `Browser errors: ${errors.join("; ")}`);

    new Image().src = `/test-report?result=${encodeURIComponent(JSON.stringify({ passed }))}`;
  } catch (error) {
    new Image().src = `/test-report?result=${encodeURIComponent(JSON.stringify({ passed, error: `${error}\n${error.stack || ""}` }))}`;
  }
})();
