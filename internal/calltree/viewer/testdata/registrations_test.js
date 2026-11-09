(async () => {
  "use strict";
  const passed = [], errors = [];
  window.addEventListener("error", (event) => errors.push(event.message));
  document.addEventListener("securitypolicyviolation", (event) => errors.push(`CSP: ${event.violatedDirective}`));
  const check = (condition, message) => { if (!condition) throw new Error(message); };
  async function waitFor(test) {
    for (let i = 0; i < 400; i++) { if (test()) return; await new Promise((resolve) => setTimeout(resolve,25)); }
    throw new Error("Timed out waiting for registration viewer state");
  }
  const viewport = document.getElementById("viewport"), graph = document.getElementById("graph");
  const nodes = document.getElementById("graph-nodes"), toggle = document.getElementById("graph-registrations");
  const details = document.getElementById("function-details"), search = document.getElementById("function-search");
  const scope = document.getElementById("graph-scope"), mode = document.getElementById("graph-mode");
  const caption = document.getElementById("trace-caption");
  function matrix() { return new DOMMatrix(getComputedStyle(graph).transform); }
  function hover(x,y) {
    const bounds = viewport.getBoundingClientRect(), view = matrix();
    const point = { clientX: bounds.left+view.e+x*view.a, clientY:bounds.top+view.f+y*view.a };
    viewport.dispatchEvent(new PointerEvent("pointermove", { ...point, bubbles:true, pointerId:1, isPrimary:true }));
    return point;
  }
  async function change(control, value, count) {
    const previous = graph.src, scale = matrix().a;
    if (control.type === "checkbox") control.checked = value; else control.value = value;
    control.dispatchEvent(new Event("change",{bubbles:true}));
    await waitFor(() => graph.src!==previous && viewport.getAttribute("aria-busy")==="false" && nodes.children.length===count);
    check(Math.abs(matrix().a-scale)<0.01,"Registration view changes must preserve zoom");
    check(!viewport.classList.contains("is-tracing"),"Diagram replacement must discard stale traces");
  }
  function select(name) {
    search.value = name;
    search.dispatchEvent(new Event("input",{bubbles:true}));
    const buttons = [...document.querySelectorAll("#function-list button")];
    const button = buttons.find((button) => button.textContent.startsWith(name));
    check(button,"Function must be discoverable: "+name);
    button.click();
  }
  try {
    await waitFor(() => !search.disabled && nodes.children.length===4 && !document.getElementById("fit").disabled);
    const data = await (await fetch("graph.json")).json();
    check(toggle.checked && data.registrations.length===3 && data.calls.length===1,"Default view must expose registrations without inventing calls");
    check(data.registered_only.length===2,"Registration-only handlers must have distinct identities/reachability");
    check(document.querySelector(".graph-legend").textContent.includes("◇ registers route (not a call)"),"Registration needs a distinct visible legend");
    select("configure");
    check(details.querySelectorAll(".route-list li").length===3 && details.textContent.includes("nil handler"),"Configuration details must disclose all bindings, including nil handlers");
    check(details.textContent.includes("GET example.com/items/{id}") && details.textContent.includes("Go 1.22+") && details.textContent.includes("httpmuxgo121=1"),"ServeMux patterns and compatibility assumptions must be explicit");
    const image = new DOMParser().parseFromString(await (await fetch("calltree.svg")).text(),"image/svg+xml");
    const trace = JSON.parse(image.querySelector("#codebound-traces").textContent);
    const registrations = trace.routes.filter((route) => route.kind==="registration");
    check(registrations.length===2 && registrations.every((route) => Number.isInteger(route.registration)),"Only resolved handlers have independently identified tracing geometry");
    check(image.documentElement.textContent.includes("nil handler"),"Nil-handler finding must be visible in its source card");
    const route = registrations.find((route) => route.label.includes("GET example.com"));
    check(route && route.label.includes("{id}"),"Trace must retain method, host, and wildcard text");
    for (const invalid of [
      { ...route, kind:"unknown" }, { ...route, registration:-1 }, { ...route, path:"M0,0" },
    ]) {
      let rejected=false;
      try { window.CodeboundTracing.validate({ ...trace, routes:[invalid] }); } catch { rejected=true; }
      check(rejected,"Invalid registration identity/kind/spline must be rejected");
    }
    const site = trace.sites.find((site) => site.from===route.from && site.site===route.site);
    document.getElementById("fit").click();
    hover((site.bounds[0]+site.bounds[2])/2,(site.bounds[1]+site.bounds[3])/2);
    check(viewport.classList.contains("is-tracing") && caption.textContent.includes("registers GET example.com/items/{id}"),"Source-row hover must explain registration, not invocation");
    viewport.dispatchEvent(new PointerEvent("pointerleave"));
    const path = document.createElementNS("http://www.w3.org/2000/svg","path");
    path.setAttribute("d",route.path);
    const middle = path.getPointAtLength(path.getTotalLength()/2);
    const point = hover(middle.x*trace.transform[0]+trace.transform[2],middle.y*trace.transform[1]+trace.transform[3]);
    check(viewport.classList.contains("is-tracing"),"Registration connector must be hoverable");
    viewport.dispatchEvent(new MouseEvent("click",{...point,detail:1,bubbles:true}));
    check(caption.textContent.includes("Pinned"),"Registration connector must support pinning");
    document.getElementById("zoom-in").click();
    check(caption.textContent.includes("Pinned"),"Pinned registration must survive zoom");
    viewport.dispatchEvent(new KeyboardEvent("keydown",{key:"Escape",bubbles:true}));
    check(caption.hidden,"Escape must release registration tracing");
    passed.push("real ServeMux analysis, distinct registration geometry/details, source hover, pin/zoom/Escape");
    await change(toggle,false,2);
    await change(toggle,true,4);
    await change(scope,"neighbors",4);
    await change(mode,"contexts",4);
    select("list");
    await waitFor(() => nodes.children.length===2 && viewport.getAttribute("aria-busy")==="false");
    check(details.textContent.includes("Registered by") && details.textContent.includes("Registration only"),"Focused handler must show its registrar without claiming dispatch");
    await change(scope,"all",4);
    await change(mode,"calls",4);
    passed.push("registration toggle, focused handler wiring, and context-mode compatibility");
    const fetchBeforeFailure = window.fetch, lastGood = graph.src;
    window.fetch = (url, options) => String(url).startsWith("calltree.svg?") ? Promise.resolve(new Response("",{status:500})) : fetchBeforeFailure(url,options);
    toggle.checked = false;
    toggle.dispatchEvent(new Event("change",{bubbles:true}));
    await waitFor(() => document.getElementById("diagram-status").textContent.includes("previous graph"));
    check(toggle.checked && graph.src===lastGood && nodes.children.length===4,"Failed toggles must restore visibility control and last usable diagram");
    window.fetch = fetchBeforeFailure;
    check(errors.length===0,errors.join("; "));
    passed.push("registration toggle failure recovery and production CSP");
    new Image().src = `/test-report?result=${encodeURIComponent(JSON.stringify({passed}))}`;
  } catch (error) {
    new Image().src = `/test-report?result=${encodeURIComponent(JSON.stringify({passed,error:`${error}\n${error.stack || ""}`}))}`;
  }
})();
