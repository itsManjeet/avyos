const state = {
  status: null,
  recipes: [],
  recipesTotal: 0,
  filter: "",
  stateFilter: "",
  page: 0,
  pageSize: 50,
  selected: new Set(),
  focused: null,
  builds: { active: null, queue: [], history: [] },
  logStream: null,
  componentLogStream: null,
};

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => [...document.querySelectorAll(sel)];

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${res.status} ${res.statusText}: ${text}`);
  }
  return res.json();
}

function route() {
  const hash = location.hash || "#/";
  const [path, query = ""] = hash.slice(1).split("?");
  return { path: path || "/", query: new URLSearchParams(query) };
}

function showPage(id) {
  $$(".page").forEach((page) => page.classList.toggle("active", page.id === id));
  $$(".nav-tabs a").forEach((link) => {
    link.classList.toggle("active", link.dataset.route === id.replace("page-", ""));
  });
}

async function renderRoute() {
  const r = route();
  if (r.path === "/" || r.path === "") {
    showPage("page-overview");
    await renderOverview();
    return;
  }
  if (r.path === "/components") {
    showPage("page-components");
    const incomingState = r.query.get("state");
    if (incomingState !== null) {
      state.stateFilter = incomingState;
      $("#state-filter").value = incomingState;
      state.page = 0;
    }
    await refreshRecipes();
    return;
  }
  if (r.path.startsWith("/components/")) {
    showPage("page-component");
    const id = decodeURIComponent(r.path.slice("/components/".length));
    await renderComponentPage(id);
    return;
  }
  if (r.path === "/builds") {
    showPage("page-builds");
    await refreshBuilds();
    renderBuildsPage();
    return;
  }
  location.hash = "#/";
}

async function refreshStatus() {
  const s = await api("/api/status");
  state.status = s;
  $("#c-total").textContent = s.total;
  $("#c-cached").textContent = s.cached;
  $("#c-waiting").textContent = s.waiting;
  $("#c-workspace").textContent = s.workspace;
  $("#c-broken").textContent = s.broken || 0;
  $("#arch-tag").textContent = s.arch;
}

async function refreshRecipes() {
  const params = new URLSearchParams();
  params.set("limit", state.pageSize);
  params.set("offset", state.page * state.pageSize);
  if (state.filter) params.set("q", state.filter);
  if (state.stateFilter) params.set("state", state.stateFilter);
  const data = await api(`/api/recipes?${params}`);
  state.recipes = data.items || [];
  state.recipesTotal = data.total || 0;
  renderRecipes();
}

function renderRecipes() {
  const tbody = $("#recipes-body");
  tbody.innerHTML = "";

  for (const r of state.recipes) {
    const tr = document.createElement("tr");
    tr.dataset.id = r.id;
    const checked = state.selected.has(r.id) ? "checked" : "";
    tr.innerHTML = `
      <td class="check-cell"><input type="checkbox" data-id="${escapeAttr(r.id)}" ${checked}></td>
      <td>
        <span class="row-title">${escapeHtml(r.recipe_id || r.id)}</span>
        <span class="row-sub">${escapeHtml(r.id)}</span>
      </td>
      <td>${escapeHtml(r.version || "")}</td>
      <td>${stateBadge(r.state)}</td>
      <td class="depends" title="${escapeAttr((r.depends || []).join(", "))}">${escapeHtml((r.depends || []).join(", "))}</td>
      <td><button data-open="${escapeAttr(r.id)}">Open</button></td>
    `;
    tr.addEventListener("click", (ev) => {
      if (ev.target.tagName === "INPUT" || ev.target.tagName === "BUTTON") return;
      openComponent(r.id);
    });
    tbody.appendChild(tr);
  }

  tbody.querySelectorAll("input[type=checkbox]").forEach((cb) => {
    cb.addEventListener("change", () => {
      if (cb.checked) state.selected.add(cb.dataset.id);
      else state.selected.delete(cb.dataset.id);
      updateBuildButton();
    });
  });
  tbody.querySelectorAll("button[data-open]").forEach((btn) => {
    btn.addEventListener("click", () => openComponent(btn.dataset.open));
  });

  const start = state.recipesTotal === 0 ? 0 : state.page * state.pageSize + 1;
  const end = Math.min((state.page + 1) * state.pageSize, state.recipesTotal);
  $("#components-count").textContent = `${start}-${end} of ${state.recipesTotal} components`;
  $("#page-label").textContent = `Page ${state.page + 1}`;
  $("#page-prev").disabled = state.page === 0;
  $("#page-next").disabled = end >= state.recipesTotal;
  $("#select-all").checked = state.recipes.length > 0 && state.recipes.every((r) => state.selected.has(r.id));
  updateBuildButton();
}

async function renderOverview() {
  await Promise.all([refreshStatus(), refreshBuilds()]);
  renderOverviewBuilds();
  await renderOverviewWaiting();
}

function renderOverviewBuilds() {
  const body = $("#overview-builds");
  const builds = flattenedBuilds().slice(0, 6);
  if (!builds.length) {
    body.innerHTML = `<div class="empty">No builds yet.</div>`;
    return;
  }
  body.innerHTML = builds.map(buildRow).join("");
  body.querySelectorAll("[data-build]").forEach((el) => {
    el.addEventListener("click", () => {
      location.hash = "#/builds";
      setTimeout(() => attachLogs(el.dataset.build, "#logs"), 0);
    });
  });
}

async function renderOverviewWaiting() {
  const data = await api("/api/recipes?state=waiting&limit=6&offset=0");
  const body = $("#overview-waiting");
  const items = data.items || [];
  if (!items.length) {
    body.innerHTML = `<div class="empty">No waiting components.</div>`;
    return;
  }
  body.innerHTML = items.map((r) => componentRow(r)).join("");
  body.querySelectorAll("[data-component]").forEach((el) => {
    el.addEventListener("click", () => openComponent(el.dataset.component));
  });
}

async function renderComponentPage(id) {
  state.focused = id;
  $("#component-title").textContent = id;
  $("#component-build").dataset.id = id;
  $("#component-logs").textContent = "Loading...";
  try {
    const [recipe, builds] = await Promise.all([
      api(`/api/recipes/${encodeURIComponent(id)}`),
      api(`/api/recipes/${encodeURIComponent(id)}/builds?limit=8`),
    ]);
    if (!recipe) {
      $("#component-overview").innerHTML = `<div class="empty">Component not found.</div>`;
      return;
    }
    $("#component-title").textContent = recipe.recipe_id || recipe.id;
    $("#component-state").innerHTML = stateBadge(recipe.state);
    $("#component-overview").innerHTML = definition({
      Element: recipe.element_id || recipe.id,
      Version: recipe.version,
      Hash: recipe.cache,
      Package: recipe.package_name,
      "Cache file": recipe.cache_file,
      About: recipe.about || "",
    });
    $("#component-data").innerHTML = [
      dataBlock("Depends", recipe.depends),
      dataBlock("Build-time depends", recipe.build_time_depends),
      dataBlock("Sources", recipe.sources),
      dataBlock("Backup", recipe.backup),
      recipe.integration ? `<div class="data-block"><h4>Integration</h4><pre>${escapeHtml(recipe.integration)}</pre></div>` : "",
    ].join("");
    renderComponentBuilds(builds.items || []);
  } catch (e) {
    $("#component-overview").innerHTML = `<div class="empty">${escapeHtml(e.message)}</div>`;
  }
}

function renderComponentBuilds(builds) {
  const body = $("#component-builds");
  if (!builds.length) {
    body.innerHTML = `<div class="empty">No builds in dashboard history.</div>`;
    $("#component-logs").textContent = "No build log selected.";
    $("#component-log-job").textContent = "";
    return;
  }
  body.innerHTML = builds.map(buildRow).join("");
  body.querySelectorAll("[data-build]").forEach((el) => {
    el.addEventListener("click", () => attachComponentLogs(el.dataset.build));
  });
  attachComponentLogs(builds[0].id);
}

async function refreshBuilds() {
  state.builds = await api("/api/builds");
}

function renderBuildsPage() {
  const body = $("#builds-body");
  const builds = flattenedBuilds();
  if (!builds.length) {
    body.innerHTML = `<div class="empty">No builds yet.</div>`;
    return;
  }
  body.innerHTML = builds.map(buildRow).join("");
  body.querySelectorAll("[data-build]").forEach((el) => {
    el.addEventListener("click", () => attachLogs(el.dataset.build, "#logs"));
  });
  body.querySelectorAll("[data-cancel]").forEach((btn) => {
    btn.addEventListener("click", async (ev) => {
      ev.stopPropagation();
      await fetch(`/api/builds/${encodeURIComponent(btn.dataset.cancel)}/cancel`, { method: "POST" });
      await refreshBuilds();
      renderBuildsPage();
    });
  });
}

function flattenedBuilds() {
  const items = [];
  if (state.builds.active) items.push(state.builds.active);
  items.push(...(state.builds.queue || []));
  items.push(...(state.builds.history || []));
  return items;
}

function buildRow(b) {
  const label = b.current_recipe || (b.recipes || []).join(", ");
  const cancel = b.state === "queued" || b.state === "running"
    ? `<button data-cancel="${escapeAttr(b.id)}">Cancel</button>`
    : "";
  return `
    <div class="build-item" data-build="${escapeAttr(b.id)}">
      <div>
        <span class="row-title">${escapeHtml(label || b.id)}</span>
        <span class="row-sub">${escapeHtml(b.id)} · exit ${b.exit_code ?? 0}</span>
      </div>
      <div class="header-actions local">${stateBadge(b.state)}${cancel}</div>
    </div>
  `;
}

function componentRow(r) {
  return `
    <div class="list-row clickable" data-component="${escapeAttr(r.id)}">
      <div>
        <span class="row-title">${escapeHtml(r.recipe_id || r.id)}</span>
        <span class="row-sub">${escapeHtml(r.id)} · ${escapeHtml(r.version || "")}</span>
      </div>
      ${stateBadge(r.state)}
    </div>
  `;
}

function attachComponentLogs(jobId) {
  $("#component-log-job").textContent = jobId;
  attachLogs(jobId, "#component-logs", true);
}

function attachLogs(jobId, target = "#logs", component = false) {
  const key = component ? "componentLogStream" : "logStream";
  if (state[key]) {
    state[key].close();
    state[key] = null;
  }
  const logs = $(target);
  logs.innerHTML = "";
  const stream = new EventSource(`/api/builds/${encodeURIComponent(jobId)}/logs`);
  state[key] = stream;
  stream.onmessage = (ev) => {
    try {
      appendLog(logs, JSON.parse(ev.data));
    } catch {
      appendLog(logs, ev.data);
    }
  };
  stream.addEventListener("end", (ev) => {
    const span = document.createElement("span");
    span.className = "end";
    span.textContent = `\nstream ended (${ev.data})\n`;
    logs.appendChild(span);
    logs.scrollTop = logs.scrollHeight;
    stream.close();
    refreshBuilds().then(() => {
      if (route().path === "/builds") renderBuildsPage();
    });
    refreshStatus().catch(console.error);
  });
  stream.onerror = () => stream.close();
}

function appendLog(logs, line) {
  const span = document.createElement("span");
  if (line.startsWith("==>")) span.className = "banner";
  else if (/error|fail/i.test(line)) span.className = "err";
  span.textContent = line + "\n";
  logs.appendChild(span);
  logs.scrollTop = logs.scrollHeight;
}

async function triggerBuild(recipes) {
  if (!recipes.length) return;
  const job = await api("/api/builds", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(recipes),
  });
  location.hash = "#/builds";
  setTimeout(() => attachLogs(job.id, "#logs"), 0);
}

async function triggerFetch() {
  const btn = $("#fetch-sources");
  btn.disabled = true;
  btn.textContent = "Fetching...";
  try {
    await api("/api/fetch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify([]),
    });
    await Promise.all([refreshStatus(), refreshRecipes().catch(() => {})]);
  } catch (e) {
    alert("fetch failed: " + e.message);
  } finally {
    btn.disabled = false;
    btn.textContent = "Fetch sources";
  }
}

function openComponent(id) {
  location.hash = `#/components/${encodeURIComponent(id)}`;
}

function updateBuildButton() {
  const btn = $("#build-selected");
  btn.textContent = `Build selected (${state.selected.size})`;
  btn.disabled = state.selected.size === 0;
}

function definition(values) {
  return `${
    Object.entries(values)
      .filter(([, value]) => value !== "")
      .map(([key, value]) => `<dt>${escapeHtml(key)}</dt><dd>${escapeHtml(value)}</dd>`)
      .join("")
  }`;
}

function dataBlock(title, items) {
  if (!items || !items.length) return "";
  return `<div class="data-block"><h4>${escapeHtml(title)}</h4><ul>${items.map((i) => `<li>${escapeHtml(i)}</li>`).join("")}</ul></div>`;
}

function stateBadge(value) {
  const stateName = value || "unknown";
  return `<span class="state-badge ${escapeAttr(stateName)}">${escapeHtml(stateName)}</span>`;
}

function escapeHtml(s) {
  return (s == null ? "" : String(s))
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function escapeAttr(s) {
  return escapeHtml(s).replaceAll('"', "&quot;");
}

function wireUI() {
  window.addEventListener("hashchange", renderRoute);

  $("#overview-refresh").addEventListener("click", renderOverview);
  $("#filter").addEventListener("input", (e) => {
    state.filter = e.target.value;
    state.page = 0;
    refreshRecipes().catch(console.error);
  });
  $("#state-filter").addEventListener("change", (e) => {
    state.stateFilter = e.target.value;
    state.page = 0;
    refreshRecipes().catch(console.error);
  });
  $("#page-size").addEventListener("change", (e) => {
    state.pageSize = Number(e.target.value);
    state.page = 0;
    refreshRecipes().catch(console.error);
  });
  $("#page-prev").addEventListener("click", () => {
    state.page = Math.max(0, state.page - 1);
    refreshRecipes().catch(console.error);
  });
  $("#page-next").addEventListener("click", () => {
    state.page += 1;
    refreshRecipes().catch(console.error);
  });
  $("#select-all").addEventListener("change", (e) => {
    for (const r of state.recipes) {
      if (e.target.checked) state.selected.add(r.id);
      else state.selected.delete(r.id);
    }
    renderRecipes();
  });
  $("#build-selected").addEventListener("click", () => triggerBuild([...state.selected]).catch((e) => alert(e.message)));
  $("#fetch-sources").addEventListener("click", triggerFetch);
  $("#component-build").addEventListener("click", () => {
    const id = $("#component-build").dataset.id;
    if (id) triggerBuild([id]).catch((e) => alert(e.message));
  });
  $("#component-log-refresh").addEventListener("click", () => {
    if (state.focused) renderComponentPage(state.focused);
  });
  $("#refresh-builds").addEventListener("click", async () => {
    await refreshBuilds();
    renderBuildsPage();
  });
  $("#logs-clear").addEventListener("click", () => {
    $("#logs").innerHTML = "";
  });
}

async function init() {
  wireUI();
  await renderRoute();
  setInterval(() => refreshStatus().catch(console.error), 15000);
  setInterval(async () => {
    await refreshBuilds().catch(console.error);
    if (route().path === "/builds") renderBuildsPage();
    if (route().path === "/") renderOverviewBuilds();
  }, 3500);
}

init().catch((e) => {
  console.error(e);
  document.body.insertAdjacentHTML("beforeend", `<pre class="empty">${escapeHtml(e.message)}</pre>`);
});
