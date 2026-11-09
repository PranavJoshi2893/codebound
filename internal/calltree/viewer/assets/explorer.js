(() => {
  "use strict";

  const search = document.getElementById("function-search");
  const list = document.getElementById("function-list");
  const status = document.getElementById("search-status");
  const details = document.getElementById("function-details");
  const rootButton = document.getElementById("show-root");
  const overview = document.getElementById("overview");
  let data;
  let incoming;
  let outgoing;
  let registeredBy;
  let registers;
  let selected = 0;
  let searchText;

  // Source names, paths, and analysis findings are always text, never HTML.
  function element(tag, text, className) {
    const node = document.createElement(tag);
    if (text !== undefined) node.textContent = text;
    if (className) node.className = className;
    return node;
  }

  function origins(ids = [], unresolved = false) {
    if (ids.length === 0) return "unknown origin";
    const label = ids.join(" or ");
    return (ids.length > 1 || unresolved ? "possibly " : "") + label + (unresolved ? " or an unknown origin" : "");
  }

  function bindingText(binding) {
    return `${binding.parameter} receives ${origins(binding.ids, binding.unresolved)}`;
  }

  function functionButton(id, showPackage = false) {
    const fn = data.functions[id];
    const button = element("button", fn.name + (id === 0 ? " (root)" : ""), "function-button");
    button.type = "button";
    button.dataset.functionId = id;
    if (showPackage) button.append(element("small", fn.package));
    return button;
  }

  function paged(items, renderItem, noun, tag = "div", className) {
    const wrapper = element("div");
    const content = element(tag, undefined, className);
    const progress = element("p", undefined, "muted");
    const more = element("button");
    more.type = "button";
    more.dataset.moreItems = noun;
    let count = 0;
    function appendBatch() {
      const fragment = document.createDocumentFragment();
      const end = Math.min(count + 50, items.length);
      while (count < end) fragment.append(renderItem(items[count++]));
      content.append(fragment);
      progress.textContent = `Showing ${count} of ${items.length} ${noun}.`;
      progress.hidden = items.length <= 50;
      more.hidden = count === items.length;
      more.textContent = `Show next ${Math.min(50, items.length - count)} (${items.length - count} more ${noun})`;
    }
    more.addEventListener("click", appendBatch);
    appendBatch();
    wrapper.append(content, progress, more);
    return wrapper;
  }

  function showMatches() {
    const query = search.value.trim().toLowerCase();
    let count = 0;
    const fragment = document.createDocumentFragment();
    searchText.forEach((text, id) => {
      if (!text.includes(query)) return;
      count++;
      // Keep the DOM small for large graphs; refine the search to see more.
      if (count > 50) return;
      const button = functionButton(id, true);
      button.setAttribute("aria-pressed", String(id === selected));
      fragment.append(button);
    });
    list.replaceChildren(fragment);
    status.textContent = count === 0 ? "No matching functions. Try a name, package, or file." :
      count > 50 ? `${count} matches. Showing the first 50; narrow your search for more.` :
        `${count} ${count === 1 ? "function" : "functions"} found.`;
  }

  function callSection(title, calls, direction) {
    details.append(element("h3", title));
    if (calls.length === 0) {
      details.append(element("p", direction === "from" ?
        "No callers shown inside this tree. This does not rule out callers outside it." :
        "No further calls shown in this analyzed tree. External or unsupported calls may be omitted.", "muted"));
      return;
    }
    details.append(paged(calls, (call) => {
      const item = element("li");
      item.append(functionButton(call[direction], true));
      const notes = [];
      if (call.path) {
        notes.push(call.path.http_dispatch ?
          "Possible HTTP request dispatch through hidden calls; not a direct invocation or route registration" :
          "Indirect path through hidden calls; not a direct invocation");
        notes.push("Execution mode and source row describe path entry; context bindings describe path exit.");
        for (const [label, id] of [["Entry", call.path.entry], ["Exit", call.path.exit]]) {
          const hop = data.evidence?.edges[id];
          if (hop) notes.push(`${label}: ${data.evidence.nodes[hop.from].id} → ${data.evidence.nodes[hop.to].id} · ${hop.location || "no source position"} · ${hop.instruction || "no instruction"}`);
        }
      }
      if (call.through_interface) notes.push("Possible target through an interface");
      if (call.dynamic) notes.push("Possible target of a function-value callback invocation");
      if (call.execution === "goroutine") notes.push("Launched in a goroutine (concurrently)");
      else if (call.execution === "deferred") notes.push("Deferred until the caller returns");
      else if (call.execution) notes.push(`Execution: ${call.execution}`);
      for (const binding of call.contexts || []) notes.push(bindingText(binding));
      if (call.violation) notes.push(`Architecture warning (naming heuristic): ${call.violation}`);
      for (const note of notes) item.append(element("p", note, "muted"));
      return item;
    }, "call findings", "ul", "call-list"));
  }

  function operationText(operation) {
    const context = origins(operation.ids, operation.unresolved);
    const parent = origins(operation.parent_ids, operation.parent_unresolved);
    switch (operation.kind) {
      case "create": return `Creates ${context} using ${operation.target}.`;
      case "derive": return `Creates child context ${context} from ${parent} using ${operation.target}.`;
      case "detach": return `Creates ${context} from ${parent}, keeping values but detaching cancellation and deadlines.`;
      case "forward": return `Passes ${context} to ${operation.target}.`;
      case "cancel": return `Calls cancel/stop for ${context}. This is source evidence, not proof the call runs.`;
      case "observe": return `Receives from the Done channel for ${context}. This can observe cancellation but does not prove the function stops.`;
      case "read": return `Reads ${operation.target} on ${context}. This alone does not prove cancellation is handled.`;
      default: return `${operation.kind}: ${operation.target} on ${context}.`;
    }
  }

  function registrationSection(title, routes, direction) {
    if (!routes.length) return;
    details.append(element("h3", title));
    details.append(element("p", "Route configuration only — registration does not invoke the handler or pass request context.", "muted"));
    if (routes.some((route) => route.framework === "net/http")) {
      details.append(element("p", "ServeMux patterns use Go 1.22+ semantics: GET also matches HEAD; ALL means no method restriction. Legacy GODEBUG=httpmuxgo121=1 changes pattern semantics; runtime settings are not inferred here.", "muted"));
    }
    details.append(paged(routes, (route) => {
      const item = element("li");
      item.append(element("strong", `Registers ${route.label}`));
      const target = route[direction];
      if (target >= 0) item.append(functionButton(target, true));
      if (route.issue) item.append(element("p", route.issue, "muted"));
      const evidence = element("details");
      evidence.append(element("summary", "Registration evidence"));
      evidence.append(element("p", `${route.framework} · ${route.location}${route.site ? ` · ${route.site}` : ""}`));
      evidence.append(element("p", `Declared pattern: ${route.declared_pattern || "unknown"}`));
      evidence.append(element("p", `Router allocation/context: ${route.mux}`));
      for (const origin of route.provenance || []) evidence.append(element("p", `Prefix ${origin.pattern || "unknown"} · ${origin.location}`));
      item.append(evidence);
      return item;
    }, "route registrations", "ul", "route-list"));
  }

  function contextSection(info) {
    details.append(element("h3", "How this function uses context"));
    if (!info || (!(info.parameters || []).length && !(info.operations || []).length)) {
      details.append(element("p", "No context actions were identified here. This is not proof that no context is used.", "muted"));
      return;
    }
    const bindings = info.bindings || [];
    (info.parameters || []).forEach((parameter, index) => {
      const binding = bindings[index] || { parameter, unresolved: true };
      details.append(element("p", bindingText(binding) + "."));
    });
    const operations = info.operations || [];
    if (operations.length) details.append(element("p", "Source findings, not a runtime timeline:", "muted"));
    if (operations.length) details.append(paged(operations, (operation) => {
      const evidence = element("div", undefined, "evidence");
      evidence.append(element("p", operationText(operation)));
      if (operation.detail) evidence.append(element("p", operation.detail, "muted"));
      if (operation.location) evidence.append(element("p", operation.location, "muted"));
      if (operation.execution || operation.scope) {
        evidence.append(element("p", [operation.execution, operation.scope].filter(Boolean).join(" · "), "muted"));
      }
      const raw = element("details");
      raw.append(element("summary", "Technical evidence"));
      raw.append(element("p", `Action: ${operation.kind} · API: ${operation.target}`));
      raw.append(element("p", `Local source: ${operation.context}`));
      if (operation.parent) raw.append(element("p", `Parent source: ${operation.parent}`));
      evidence.append(raw);
      return evidence;
    }, "context findings"));
  }

  function selectFunction(id, focus = false) {
    selected = id;
    const fn = data.functions[id];
    const heading = element("h2", fn.name + (id === 0 ? " · Root function" : ""));
    heading.tabIndex = -1;
    details.replaceChildren(heading);
    details.append(element("p", fn.package, "muted"));
    details.append(element("p", `${fn.file}:${fn.line}`, "muted"));
    if (fn.layer) details.append(element("p", `Detected architecture layer: ${fn.layer}`, "muted"));
    if ((data.registered_only || []).includes(id)) details.append(element("p", "Registration only: shown for route configuration; no invocation edge is recorded in this call tree. Registration does not prove request dispatch.", "muted"));
    callSection("Called by — who can reach this function", incoming[id], "from");
    callSection("Calls — where to go next", outgoing[id], "to");
    registrationSection("Registers routes", registers[id], "to");
    registrationSection("Registered by", registeredBy[id], "from");
    contextSection(fn.context);
    for (const button of list.children) {
      button.setAttribute("aria-pressed", String(Number(button.dataset.functionId) === id));
    }
    details.setAttribute("aria-busy", "false");
    document.dispatchEvent(new CustomEvent("codebound:function-selected", { detail: { id, name: fn.name } }));
    if (focus) {
      heading.focus({ preventScroll: true });
      heading.scrollIntoView({ block: "nearest" });
    }
  }

  async function loadDetails() {
    try {
      const response = await fetch("graph.json");
      if (!response.ok) throw new Error("Graph details unavailable");
      data = await response.json();
      if (!data.functions.length) throw new Error("No function details supplied");
      incoming = data.functions.map(() => []);
      outgoing = data.functions.map(() => []);
      registeredBy = data.functions.map(() => []);
      registers = data.functions.map(() => []);
      for (const route of data.registrations || []) {
        registers[route.from].push(route);
        if (route.to >= 0) registeredBy[route.to].push(route);
      }
      const routeCount = (data.registrations || []).length;
      delete data.registrations;
      const registrationStatus = document.getElementById("registration-status");
      if (data.registration_warnings?.length) {
        registrationStatus.hidden = false;
        registrationStatus.textContent = data.registration_warnings.join(" ");
      }
      const seen = new Set();
      const relationships = new Set();
      let warnings = 0;
      for (const call of data.calls) {
        relationships.add(`${call.from}:${call.to}`);
        if (call.violation) warnings++;
        // Normalize repeated call evidence once, not on every selection.
        const key = JSON.stringify(call);
        if (seen.has(key)) continue;
        seen.add(key);
        incoming[call.to].push(call);
        outgoing[call.from].push(call);
      }
      // Adjacency lists retain the unique findings; release the redundant raw
      // array and identical repeated call objects after normalization.
      delete data.calls;
      searchText = data.functions.map((fn) => `${fn.name} ${fn.package} ${fn.file}`.toLowerCase());
      overview.textContent = `Root: ${data.functions[0].name} · ${data.functions.length} functions · ${relationships.size} caller–callee relationships` +
        (routeCount ? ` · ${routeCount} route registrations (not calls)` : "") +
        (warnings ? ` · ${warnings} call findings with architecture warnings` : "") + ". Possible paths, not a runtime trace.";
      overview.title = overview.textContent;
      search.disabled = false;
      rootButton.disabled = false;
      search.addEventListener("input", showMatches);
      document.querySelector("aside").addEventListener("click", (event) => {
        const button = event.target.closest(".function-button");
        if (button) selectFunction(Number(button.dataset.functionId), true);
      });
      rootButton.addEventListener("click", () => {
        search.value = "";
        showMatches();
        selectFunction(0, true);
      });
      showMatches();
      selectFunction(0);
      document.addEventListener("codebound:graph-select", (event) => {
        if (!Number.isInteger(event.detail.id) || !data.functions[event.detail.id]) return;
        selectFunction(event.detail.id);
        details.querySelector("h2").scrollIntoView({ block: "nearest" });
      });
    } catch {
      status.textContent = "Function details unavailable. You can still use the diagram and reading guide.";
      details.setAttribute("aria-busy", "false");
    }
  }

  loadDetails();
})();
