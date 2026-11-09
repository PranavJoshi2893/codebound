// Trace geometry is inert data. A viewport-sized canvas reinforces the isolated
// SVG image without inserting Graphviz markup or allocating a graph-sized bitmap.
window.CodeboundTracing = class {
  constructor(viewport, image, nodes, camera) {
    this.viewport = viewport;
    this.image = image;
    this.camera = camera;
    this.canvas = document.getElementById("graph-trace");
    this.context = this.canvas.getContext("2d");
    this.hitContext = document.createElement("canvas").getContext("2d");
    this.caption = document.getElementById("trace-caption");
    this.reset();
    viewport.addEventListener("codebound:camera", () => this.draw());
    viewport.addEventListener("pointermove", (event) => {
      if (this.pinned) return;
      const routes = event.buttons || viewport.classList.contains("dragging") ? [] : this.hit(event);
      this.show(routes);
    });
    viewport.addEventListener("pointerleave", () => { if (!this.pinned) this.show([]); });
    viewport.addEventListener("pointerdown", (event) => { this.press = { x: event.clientX, y: event.clientY }; });
    viewport.addEventListener("click", (event) => {
      if (!event.detail || event.target.closest(".graph-node-hit")) return;
      if (this.press && Math.hypot(event.clientX - this.press.x, event.clientY - this.press.y) > 4) return;
      const routes = this.hit(event);
      this.pinned = routes.length > 0 && !(this.pinned && this.same(routes, this.active));
      this.show(this.pinned ? routes : []);
    });
    viewport.addEventListener("keydown", (event) => {
      if (event.key !== "Escape") return;
      this.pinned = false;
      this.show([]);
    });
    nodes.addEventListener("focusin", (event) => {
      const button = event.target.closest(".graph-node-hit");
      if (!button || !button.matches(":focus-visible")) return;
      this.pinned = false;
      this.show(this.routes.filter((route) => route.from === Number(button.dataset.functionId)));
    });
    nodes.addEventListener("focusout", () => { if (!this.pinned) this.show([]); });
  }

  static validate(data) {
    if (data === null) return { transform: [1, 1, 0, 0], sites: [], routes: [] };
    const finite = (values, count) => Array.isArray(values) && values.length === count && values.every(Number.isFinite);
    const id = (value) => Number.isInteger(value) && value >= 0;
    const path = (value) => {
      if (typeof value !== "string" || !/^[MCmc0-9eE.,+\s-]+$/.test(value)) return false;
      const coordinates = value.replace(/[MCmc]/g, " ").split(/[\s,]+/).filter(Boolean);
      return coordinates.length >= 8 && (coordinates.length - 2) % 6 === 0 && coordinates.every((coordinate) => Number.isFinite(Number(coordinate)));
    };
    if (!data || !finite(data.transform, 4) || data.transform[0] <= 0 || data.transform[1] <= 0 ||
      !Array.isArray(data.sites) || !Array.isArray(data.routes) ||
      !data.sites.every((site) => id(site.from) && typeof site.site === "string" && typeof site.label === "string" &&
        finite(site.bounds, 4) && site.bounds.every((value) => value >= 0) && site.bounds[2] > site.bounds[0] && site.bounds[3] > site.bounds[1]) ||
      !data.routes.every((route) => id(route.from) && id(route.to) && typeof route.site === "string" &&
        (route.kind === undefined || route.kind === "" || route.kind === "registration" && id(route.registration) && typeof route.label === "string") &&
        path(route.path) &&
        /^#[0-9a-fA-F]{6}$/.test(route.color) && finite(route.origin, 3) && route.origin[2] >= 0 &&
        (!route.head || Array.isArray(route.head) && route.head.every((point) => finite(point, 2))) &&
        (!route.dash || Array.isArray(route.dash) && route.dash.every((value) => Number.isFinite(value) && value >= 0)))) {
      throw new Error("Invalid graph trace geometry");
    }
    return data;
  }

  reset() {
    this.routes = this.sites = this.active = [];
    this.nodes = new Map();
    this.transform = [1, 1, 0, 0];
    this.pinned = false;
    this.shownPinned = null;
    this.show([]);
  }

  setGraph(data, nodes) {
    this.reset();
    this.transform = data.transform;
    this.nodes = new Map(nodes.map((node) => [node.id, node]));
    this.routes = data.routes.map((route) => ({ ...route, geometry: new Path2D(route.path) }));
    const sources = new Set(this.routes.map((route) => `${route.from}:${route.site}`));
    this.sites = data.sites.filter((site) => sources.has(`${site.from}:${site.site}`));
  }

  same(first, second) {
    return first.length === second.length && first.every((route, index) => route === second[index]);
  }

  hit(event) {
    const bounds = this.viewport.getBoundingClientRect();
    const x = (event.clientX - bounds.left - this.camera.x) / this.camera.scale;
    const y = (event.clientY - bounds.top - this.camera.y) / this.camera.scale;
    const contains = (box) => x >= box[0] && x <= box[2] && y >= box[1] && y <= box[3];
    const site = this.sites.find((source) => contains(source.bounds));
    if (site) return this.routes.filter((route) => route.from === site.from && route.site === site.site);
    // A route must not steal hover from unrelated text inside a function.
    for (const node of this.nodes.values()) if (contains(node.bounds)) return [];
    const [sx, sy, tx, ty] = this.transform;
    this.hitContext.lineWidth = 12 / (this.camera.scale * Math.max(sx, sy));
    for (let index = this.routes.length - 1; index >= 0; index--) {
      const route = this.routes[index];
      const [cx, cy] = route.origin;
      if (this.hitContext.isPointInStroke(route.geometry, (x - tx) / sx, (y - ty) / sy) ||
        Math.hypot(x - (cx * sx + tx), y - (cy * sy + ty)) < 8 / this.camera.scale) return [route];
    }
    return [];
  }

  show(routes) {
    if (this.same(routes, this.active) && this.shownPinned === this.pinned) return;
    this.active = routes;
    this.shownPinned = this.pinned;
    this.viewport.classList.toggle("is-tracing", routes.length > 0);
    this.caption.hidden = !routes.length;
    if (routes.length) {
      const route = routes[0];
      const name = (id) => this.nodes.get(id)?.label.split("\n")[0] || "function";
      const source = this.sites.find((site) => site.from === route.from && site.site === route.site);
      const targets = [...new Set(routes.map((item) => item.kind === "registration" ? `${item.label} → ${name(item.to)}` : `→ ${name(item.to)}`))].join("; ");
      this.caption.textContent = `${name(route.from)} · ${source?.label || "source row unavailable"} ${targets}${this.pinned ? " · Pinned · Esc to clear" : ""}`;
    }
    this.draw();
  }

  draw() {
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    const width = this.viewport.clientWidth;
    const height = this.viewport.clientHeight;
    if (this.canvas.width !== Math.ceil(width * ratio)) this.canvas.width = Math.ceil(width * ratio);
    if (this.canvas.height !== Math.ceil(height * ratio)) this.canvas.height = Math.ceil(height * ratio);
    Object.assign(this.canvas.style, { width: `${width}px`, height: `${height}px` });
    const context = this.context;
    context.resetTransform();
    context.clearRect(0, 0, this.canvas.width, this.canvas.height);
    if (!this.active.length || !this.image.complete || !this.image.naturalWidth) return;
    context.scale(ratio, ratio);
    context.translate(this.camera.x, this.camera.y);
    context.scale(this.camera.scale, this.camera.scale);
    const restore = (box) => {
      const [left, top, right, bottom] = box;
      context.drawImage(this.image, left, top, right - left, bottom - top, left, top, right - left, bottom - top);
    };
    const functions = new Set(this.active.flatMap((route) => [route.from, route.to]));
    for (const id of functions) {
      const node = this.nodes.get(id);
      if (node) restore(node.bounds);
    }
    for (const site of this.sites) {
      if (!this.active.some((route) => route.from === site.from && route.site === site.site)) continue;
      const [left, top, right, bottom] = site.bounds;
      context.fillStyle = "#7c3aed22";
      context.fillRect(left, top, right - left, bottom - top);
      context.fillStyle = "#7c3aed";
      context.fillRect(left, top, 3, bottom - top);
    }
    context.transform(this.transform[0], 0, 0, this.transform[1], this.transform[2], this.transform[3]);
    context.lineJoin = context.lineCap = "round";
    for (const route of this.active) {
      context.setLineDash([]);
      context.strokeStyle = "white";
      context.lineWidth = 6;
      context.stroke(route.geometry);
      context.strokeStyle = context.fillStyle = route.color;
      context.lineWidth = 2.5;
      context.setLineDash(route.dash || []);
      context.stroke(route.geometry);
      context.setLineDash([]);
      if (route.head?.length) {
        context.beginPath();
        route.head.forEach(([x, y], index) => index ? context.lineTo(x, y) : context.moveTo(x, y));
        context.closePath();
        context.fillStyle = route.kind === "registration" ? "white" : route.color;
        context.fill();
        if (route.kind === "registration") { context.lineWidth = 1; context.stroke(); }
      }
      const [x, y, radius] = route.origin;
      context.beginPath();
      context.arc(x, y, radius, 0, Math.PI * 2);
      context.fillStyle = route.site ? route.color : "white";
      context.fill();
      context.lineWidth = 1;
      context.stroke();
    }
  }
};
