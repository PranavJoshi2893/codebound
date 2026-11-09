(() => {
  "use strict";

  const viewport = document.getElementById("viewport");
  const graph = document.getElementById("graph");
  const nodeLayer = document.getElementById("graph-nodes");
  const mode = document.getElementById("graph-mode");
  const scope = document.getElementById("graph-scope");
  const status = document.getElementById("diagram-status");
  const contextKey = document.getElementById("context-key");
  const camera = new window.CodeboundCamera(viewport, graph, nodeLayer);
  const buttons = new Map();
  let selected = 0;
  let selectedName = "root";
  let controller;
  let requestNumber = 0;
  let pendingKey;
  let shown;
  let imageURL;

  function describe(options) {
    const content = options.contexts ? "Calls + context · Blue labels show context passed on the call arrow" : "Call flow";
    return options.focus !== null ? `${content} · Focus: ${options.name} · Direct links only; other relationships are hidden. +N links = hidden links.` :
      `${content} · Start at ROOT. Inside a block read ↓, then follow its labeled exits. Shaded groups share a package.`;
  }

  function setStatus(text) {
    status.textContent = status.title = text;
  }

  function renderNodes(nodes, restoreFocus) {
    const fragment = document.createDocumentFragment();
    buttons.clear();
    for (const node of nodes) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "graph-node-hit";
      button.dataset.functionId = node.id;
      button.dataset.functionName = node.label.split("\n")[0];
      button.setAttribute("aria-label", `Select ${node.label}`);
      button.setAttribute("aria-pressed", String(node.id === selected));
      button.title = node.label;
      const [left, top, right, bottom] = node.bounds;
      Object.assign(button.style, { left: `${left}px`, top: `${top}px`, width: `${right - left}px`, height: `${bottom - top}px` });
      buttons.set(node.id, button);
      fragment.append(button);
    }
    nodeLayer.replaceChildren(fragment);
    if (restoreFocus) buttons.get(selected)?.focus({ preventScroll: true });
  }

  function readNodes(svg) {
    // Parse only the inert metadata, not a second DOM copy of the entire graph.
    // SVG content is NEVER inserted into the page: it stays an isolated image.
    const start = svg.lastIndexOf('<metadata id="codebound-nodes">');
    if (start < 0) return [];
    const end = svg.indexOf("</metadata>", start);
    const parsed = new DOMParser().parseFromString(svg.slice(start, end + 11), "application/xml");
    if (parsed.querySelector("parsererror") || parsed.documentElement.localName !== "metadata") throw new Error("Invalid graph metadata");
    const nodes = JSON.parse(parsed.documentElement.textContent);
    if (!Array.isArray(nodes) || !nodes.every((node) => Number.isInteger(node.id) && node.id >= 0 &&
      typeof node.label === "string" && Array.isArray(node.bounds) && node.bounds.length === 4 &&
      node.bounds.every((value) => Number.isFinite(value) && value >= 0) &&
      node.bounds[2] > node.bounds[0] && node.bounds[3] > node.bounds[1])) throw new Error("Invalid node coordinates");
    return nodes;
  }

  async function requestGraph() {
    const options = { contexts: mode.value === "contexts", focus: scope.value === "neighbors" ? selected : null, name: selectedName };
    const key = `${options.contexts}:${options.focus}`;
    if (key === pendingKey) return;
    controller?.abort();
    controller = new AbortController();
    const number = ++requestNumber;
    if (shown?.key === key) {
      pendingKey = null;
      setStatus(describe(shown));
      viewport.setAttribute("aria-busy", "false");
      return;
    }
    pendingKey = key;
    const restoreFocus = nodeLayer.contains(document.activeElement);
    const query = new URLSearchParams({ mode: options.contexts ? "contexts" : "calls" });
    if (options.focus !== null) query.set("focus", options.focus);
    setStatus("Laying out the graph…");
    viewport.setAttribute("aria-busy", "true");
    let nextURL;
    try {
      const response = await fetch(`calltree.svg?${query}`, { signal: controller.signal });
      if (!response.ok) throw new Error("Could not render graph");
      const svg = await response.text();
      if (number !== requestNumber) return;
      const nodes = readNodes(svg);
      nextURL = URL.createObjectURL(new Blob([svg], { type: "image/svg+xml" }));
      const decoded = new Image();
      decoded.src = nextURL;
      await decoded.decode();
      if (number !== requestNumber) return;
      const previousURL = imageURL;
      graph.src = imageURL = nextURL;
      nextURL = null;
      camera.setImage(nodes, decoded.naturalWidth, decoded.naturalHeight);
      renderNodes(nodes, restoreFocus);
      if (previousURL) URL.revokeObjectURL(previousURL);
      shown = { ...options, key };
      contextKey.hidden = !options.contexts;
      setStatus(describe(options));
    } catch (error) {
      if (error.name === "AbortError" || number !== requestNumber) return;
      if (shown) {
        mode.value = shown.contexts ? "contexts" : "calls";
        scope.value = shown.focus === null ? "all" : "neighbors";
        setStatus("Could not update the diagram. The previous graph is still shown; try again.");
      } else {
        camera.fail();
        setStatus("Diagram could not load. The function explorer is still available; change the view to retry.");
      }
    } finally {
      if (nextURL) URL.revokeObjectURL(nextURL);
      if (number === requestNumber) {
        pendingKey = null;
        viewport.setAttribute("aria-busy", "false");
      }
    }
  }

  function selectFunction(id, name) {
    const previous = selected;
    selected = camera.selected = id;
    selectedName = name;
    buttons.get(previous)?.setAttribute("aria-pressed", "false");
    buttons.get(id)?.setAttribute("aria-pressed", "true");
    if (scope.value === "neighbors" && previous !== id) requestGraph();
    else camera.reveal(id);
  }

  nodeLayer.addEventListener("click", (event) => {
    const button = event.target.closest(".graph-node-hit");
    if (!button) return;
    const id = Number(button.dataset.functionId);
    selectFunction(id, button.dataset.functionName);
    document.dispatchEvent(new CustomEvent("codebound:graph-select", { detail: { id } }));
  });
  mode.addEventListener("change", requestGraph);
  scope.addEventListener("change", requestGraph);
  document.addEventListener("codebound:function-selected", (event) => selectFunction(event.detail.id, event.detail.name));
  graph.addEventListener("error", () => {
    shown = null;
    camera.fail();
    buttons.clear();
    nodeLayer.replaceChildren();
    setStatus("Diagram could not load. The function explorer is still available.");
  });
  requestGraph();
})();
