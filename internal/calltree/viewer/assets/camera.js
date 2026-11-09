// Camera coordinates are SVG image pixels. 100% always means natural size,
// independent of graph complexity, viewport dimensions, and the active view.
window.CodeboundCamera = class {
  constructor(viewport, image, overlay) {
    this.viewport = viewport;
    this.image = image;
    this.overlay = overlay;
    this.level = document.getElementById("zoom-level");
    this.zoomIn = document.getElementById("zoom-in");
    this.zoomOut = document.getElementById("zoom-out");
    this.fitButton = document.getElementById("fit");
    this.actualButton = document.getElementById("actual-size");
    this.scale = 1;
    this.x = this.y = 0;
    this.width = viewport.clientWidth;
    this.height = viewport.clientHeight;
    this.ready = false;
    this.fitted = false;
    this.nodes = new Map();
    this.selected = 0;
    this.drag = null;

    this.zoomIn.addEventListener("click", () => this.zoomAtCenter(1.25));
    this.zoomOut.addEventListener("click", () => this.zoomAtCenter(0.8));
    this.fitButton.addEventListener("click", () => this.fit());
    this.actualButton.addEventListener("click", () => this.actual());
    viewport.addEventListener("wheel", (event) => this.wheel(event), { passive: false });
    viewport.addEventListener("pointerdown", (event) => this.startDrag(event));
    viewport.addEventListener("pointermove", (event) => this.moveDrag(event));
    for (const type of ["pointerup", "pointercancel", "lostpointercapture"]) {
      viewport.addEventListener(type, (event) => this.endDrag(event));
    }
    document.addEventListener("keydown", (event) => this.key(event));
    this.observer = new ResizeObserver(() => this.resize());
    this.observer.observe(viewport);
  }

  nodeCenter(id) {
    const bounds = this.nodes.get(id);
    return bounds ? { x: (bounds[0] + bounds[2]) / 2, y: (bounds[1] + bounds[3]) / 2 } : null;
  }

  setImage(nodes, width, height) {
    const previous = this.ready && this.nodeCenter(this.selected);
    const anchor = previous && { x: this.x + previous.x * this.scale, y: this.y + previous.y * this.scale };
    const first = !this.ready;
    this.nodes = new Map(nodes.map((node) => [node.id, node.bounds]));
    this.imageWidth = width;
    this.imageHeight = height;
    this.ready = true;
    this.width = this.viewport.clientWidth;
    this.height = this.viewport.clientHeight;
    this.image.style.width = this.overlay.style.width = `${width}px`;
    this.image.style.height = this.overlay.style.height = `${height}px`;
    this.fitButton.disabled = this.actualButton.disabled = false;
    // Fit is a one-shot action for this graph. A replacement must not change
    // the user's absolute zoom just because its bounding box is different.
    this.fitted = false;
    if (anchor && this.nodeCenter(this.selected)) {
      const next = this.nodeCenter(this.selected);
      this.x = anchor.x - next.x * this.scale;
      this.y = anchor.y - next.y * this.scale;
      this.render();
    } else {
      if (first) this.scale = 1;
      this.center();
    }
  }

  center() {
    const node = this.nodeCenter(this.selected);
    const width = this.imageWidth * this.scale;
    const height = this.imageHeight * this.scale;
    this.x = (this.width - width) / 2;
    this.y = (this.height - height) / 2;
    if (node && width > this.width - 32) {
      // The call map runs left-to-right; start at its readable root rather than
      // spending half the canvas on empty space to the left of that card.
      this.x = this.selected === 0 ? 24 - this.nodes.get(0)[0] * this.scale : this.width / 2 - node.x * this.scale;
    }
    if (node && height > this.height - 32) {
      this.y = this.selected === 0 ? 24 - this.nodes.get(0)[1] * this.scale : this.height / 2 - node.y * this.scale;
    }
    this.render();
  }

  fitScale() {
    return Math.min(Math.max(1, this.width - 32) / this.imageWidth, Math.max(1, this.height - 32) / this.imageHeight);
  }

  fit() {
    if (!this.ready) return;
    this.scale = Math.min(1, this.fitScale());
    this.fitted = true;
    this.x = (this.width - this.imageWidth * this.scale) / 2;
    this.y = (this.height - this.imageHeight * this.scale) / 2;
    this.render();
  }

  actual() {
    if (!this.ready) return;
    this.scale = 1;
    this.fitted = false;
    this.center();
  }

  reveal(id) {
    const node = this.nodeCenter(id);
    if (!this.ready || !node) return;
    const x = this.x + node.x * this.scale;
    const y = this.y + node.y * this.scale;
    if (x < 24 || x > this.width - 24 || y < 24 || y > this.height - 24) {
      this.pan(this.width / 2 - x, this.height / 2 - y);
    }
  }

  fail() {
    this.ready = false;
    if (this.drag) this.endDrag({ pointerId: this.drag.id });
    this.zoomIn.disabled = this.zoomOut.disabled = this.fitButton.disabled = this.actualButton.disabled = true;
    this.level.value = "Load failed";
  }

  resize() {
    const width = this.viewport.clientWidth;
    const height = this.viewport.clientHeight;
    if (width === this.width && height === this.height) return;
    this.x += (width - this.width) / 2;
    this.y += (height - this.height) / 2;
    this.width = width;
    this.height = height;
    if (!this.ready) return;
    if (this.fitted) this.fit();
    else this.render();
  }

  render() {
    const transform = `translate(${this.x}px, ${this.y}px) scale(${this.scale})`;
    this.image.style.transform = this.overlay.style.transform = transform;
    const percent = this.scale * 100;
    const percentage = `${percent < 1 ? Number(percent.toPrecision(2)) : Math.round(percent)}%`;
    if (this.level.value !== percentage) this.level.value = percentage;
    this.zoomIn.disabled = this.scale >= 4;
    this.zoomOut.disabled = this.scale <= Math.min(0.1, this.fitScale());
  }

  zoomAt(factor, x, y) {
    if (!this.ready) return;
    const scale = Math.min(4, Math.max(Math.min(0.1, this.fitScale()), this.scale * factor));
    const ratio = scale / this.scale;
    this.x = x - (x - this.x) * ratio;
    this.y = y - (y - this.y) * ratio;
    this.scale = scale;
    this.fitted = false;
    this.render();
  }

  zoomAtCenter(factor) {
    this.zoomAt(factor, this.width / 2, this.height / 2);
  }

  pan(x, y) {
    this.x += x;
    this.y += y;
    this.fitted = false;
    this.render();
  }

  wheel(event) {
    if (!this.ready) return;
    event.preventDefault();
    const unit = event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? this.height : 1;
    const delta = Math.max(-240, Math.min(240, event.deltaY * unit));
    const bounds = this.viewport.getBoundingClientRect();
    this.zoomAt(Math.exp(-delta * 0.002), event.clientX - bounds.left, event.clientY - bounds.top);
  }

  startDrag(event) {
    if (!this.ready || event.button !== 0 || !event.isPrimary || event.target.closest(".graph-node-hit")) return;
    this.viewport.focus({ preventScroll: true });
    this.viewport.setPointerCapture(event.pointerId);
    this.drag = { id: event.pointerId, x: event.clientX, y: event.clientY };
    this.viewport.classList.add("dragging");
  }

  moveDrag(event) {
    if (!this.drag || this.drag.id !== event.pointerId) return;
    this.pan(event.clientX - this.drag.x, event.clientY - this.drag.y);
    this.drag.x = event.clientX;
    this.drag.y = event.clientY;
  }

  endDrag(event) {
    if (!this.drag || this.drag.id !== event.pointerId) return;
    this.drag = null;
    this.viewport.classList.remove("dragging");
    if (this.viewport.hasPointerCapture(event.pointerId)) this.viewport.releasePointerCapture(event.pointerId);
  }

  key(event) {
    if (!this.ready || event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.target.isContentEditable) return;
    if (!this.viewport.contains(event.target) && !event.target.closest(".controls")) return;
    if (["INPUT", "TEXTAREA", "SELECT"].includes(event.target.tagName)) return;
    switch (event.key) {
      case "+": case "=": this.zoomAtCenter(1.25); break;
      case "-": case "_": this.zoomAtCenter(0.8); break;
      case "0": case "Home": this.fit(); break;
      case "1": this.actual(); break;
      case "ArrowLeft": this.pan(40, 0); break;
      case "ArrowRight": this.pan(-40, 0); break;
      case "ArrowUp": this.pan(0, 40); break;
      case "ArrowDown": this.pan(0, -40); break;
      default: return;
    }
    event.preventDefault();
  }
};
