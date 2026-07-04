const state = {
  status: null,
  recipes: [],
  recipesTotal: 0,
  filter: "",
  stateFilter: "",
  sourceFilter: "",
  sourceSearch: "",
  sourceTypeFilter: "",
  page: 0,
  pageSize: 50,
  selected: new Set(),
  focused: null,
  builds: { active: null, queue: [], history: [] },
  workspaces: { items: [] },
  sources: { items: [] },
  logStream: null,
  componentLogStream: null,
};


const ACCENT_THEMES = {
  varanasi: ["Varanasi", "#E36A22", "#C94F13", "#FFF3EC", "#F29B57", "227, 106, 34", "242, 155, 87", "#F8F3EC", "rgba(245,196,168,.72)", "rgba(255,224,191,.72)", "rgba(238,227,210,.72)"],
  jaipur: ["Jaipur", "#C95C7A", "#9F3E5B", "#FFF0F4", "#E58AA0", "201, 92, 122", "229, 138, 160", "#FAF1F3", "rgba(243,184,200,.72)", "rgba(255,220,229,.72)", "rgba(240,226,229,.72)"],
  jodhpur: ["Jodhpur", "#4B63C7", "#34479E", "#EEF1FF", "#8092EA", "75, 99, 199", "128, 146, 234", "#F2F3FA", "rgba(190,200,242,.72)", "rgba(220,226,255,.72)", "rgba(228,230,242,.72)"],
  coorg: ["Coorg", "#6F4A35", "#4E3324", "#F5EEE9", "#A06F52", "111, 74, 53", "160, 111, 82", "#F5F0EA", "rgba(190,154,130,.72)", "rgba(236,218,204,.72)", "rgba(226,217,207,.72)"],
  kochi: ["Kochi", "#1F9F8E", "#167568", "#E9F8F5", "#58C7B7", "31, 159, 142", "88, 199, 183", "#EFF8F5", "rgba(176,232,222,.72)", "rgba(209,246,239,.72)", "rgba(225,237,233,.72)"],
  ladakh: ["Ladakh", "#3E8ED0", "#2B67A0", "#EDF6FF", "#76B7EE", "62, 142, 208", "118, 183, 238", "#F1F6FB", "rgba(186,217,245,.72)", "rgba(218,237,255,.72)", "rgba(226,233,240,.72)"],
  konark: ["Konark", "#D58A18", "#A8660F", "#FFF5E3", "#F0B24D", "213, 138, 24", "240, 178, 77", "#FAF3E6", "rgba(238,195,119,.72)", "rgba(255,232,184,.72)", "rgba(238,226,207,.72)"],
  madurai: ["Madurai", "#A04AA0", "#783278", "#FAEFFA", "#C77AC7", "160, 74, 160", "199, 122, 199", "#F8F1F8", "rgba(218,177,218,.72)", "rgba(246,218,246,.72)", "rgba(233,224,233,.72)"],
};

const THEME_STORAGE_KEY = "ignite-dashboard-theme";
const ACCENT_STORAGE_KEY = "ignite-dashboard-accent";

function applyAccent(key) {
  const theme = ACCENT_THEMES[key] || ACCENT_THEMES.varanasi;
  const [, accent, accentStrong, accentSoft, accentAlt, accentRgb, accentAltRgb, pageBg, glowOne, glowTwo, glowThree] = theme;
  const root = document.documentElement;
  root.style.setProperty("--accent", accent);
  root.style.setProperty("--accent-strong", accentStrong);
  root.style.setProperty("--accent-soft", accentSoft);
  root.style.setProperty("--accent-alt", accentAlt);
  root.style.setProperty("--accent-rgb", accentRgb);
  root.style.setProperty("--accent-alt-rgb", accentAltRgb);
  root.style.setProperty("--page-bg", pageBg);
  root.style.setProperty("--glow-one", glowOne);
  root.style.setProperty("--glow-two", glowTwo);
  root.style.setProperty("--glow-three", glowThree);
  const picker = $("#accent-picker");
  if (picker) picker.value = ACCENT_THEMES[key] ? key : "varanasi";
  const dot = $("#accent-dot");
  if (dot) dot.style.backgroundColor = accent;
}

function applyTheme(theme) {
  const safeTheme = theme === "dark" ? "dark" : "light";
  document.body.dataset.theme = safeTheme;
  const button = $("#theme-toggle");
  if (button) {
    button.textContent = safeTheme === "dark" ? "☀" : "☾";
    button.title = safeTheme === "dark" ? "Switch to light mode" : "Switch to dark mode";
    button.setAttribute("aria-label", button.title);
  }
}

function initThemeControls() {
  const storedTheme = localStorage.getItem(THEME_STORAGE_KEY) || "light";
  const storedAccent = localStorage.getItem(ACCENT_STORAGE_KEY) || "varanasi";
  applyTheme(storedTheme);
  applyAccent(storedAccent);

  const picker = $("#accent-picker");
  if (picker) {
    picker.addEventListener("change", (event) => {
      const value = event.target.value;
      localStorage.setItem(ACCENT_STORAGE_KEY, value);
      applyAccent(value);
    });
  }

  const toggle = $("#theme-toggle");
  if (toggle) {
    toggle.addEventListener("click", () => {
      const next = document.body.dataset.theme === "dark" ? "light" : "dark";
      localStorage.setItem(THEME_STORAGE_KEY, next);
      applyTheme(next);
    });
  }
}

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
  const activeRoute = id === "page-component" ? "components" : id.replace("page-", "");
  $$(".nav-tabs a, .side-nav a").forEach((link) => {
    link.classList.toggle("active", link.dataset.route === activeRoute);
  });
  closeSidebar();
}

function openSidebar() {
  document.body.classList.add("sidebar-open");
}

function closeSidebar() {
  document.body.classList.remove("sidebar-open");
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
  if (r.path === "/workspaces") {
    showPage("page-workspaces");
    await renderWorkspaces();
    return;
  }
  if (r.path === "/sources") {
    showPage("page-sources");
    await renderSources();
    return;
  }
  if (r.path === "/builds") {
    showPage("page-builds");
    await refreshBuilds();
    renderBuildsPage();
    return;
  }
  if (r.path === "/settings") {
    showPage("page-settings");
    await renderSettings();
    return;
  }
  location.hash = "#/";
}

async function refreshStatus() {
  const s = await api("/api/status");
  state.status = s;
  setText("#c-total", s.total);
  setText("#c-cached", s.cached);
  setText("#c-waiting", s.waiting);
  setText("#c-workspace", s.workspace);
  setText("#c-broken", s.broken || 0);
  setText("#c-container", s.container || 0);
  setText("#c-queue", (s.queue || 0) + (s.active ? 1 : 0));
  const archLabel = `${s.arch} · ${s.local_conf ? "local.conf.yml" : "defaults"}`;
  setText("#arch-tag", archLabel);
  setText("#sidebar-arch", archLabel);
  setText("#sidebar-queue", (s.queue || 0) + (s.active ? 1 : 0));
  setText("#sidebar-cached", s.cached);
  setText("#sidebar-waiting", s.waiting);
  setText("#sidebar-workspace", s.workspace);
  return s;
}

async function refreshRecipes() {
  const params = new URLSearchParams();
  params.set("limit", state.pageSize);
  params.set("offset", state.page * state.pageSize);
  if (state.filter) params.set("q", state.filter);
  if (state.stateFilter) params.set("state", state.stateFilter);
  if (state.sourceFilter) params.set("source", state.sourceFilter);
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
      <td>${pill(r.mode || "container")}</td>
      <td><span class="source-count">${Number(r.source_count || 0)}</span> ${sourceMiniBadges(r)}</td>
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
      updateSelectedButtons();
    });
  });
  tbody.querySelectorAll("button[data-open]").forEach((btn) => btn.addEventListener("click", () => openComponent(btn.dataset.open)));

  const start = state.recipesTotal === 0 ? 0 : state.page * state.pageSize + 1;
  const end = Math.min((state.page + 1) * state.pageSize, state.recipesTotal);
  setText("#components-count", `${start}-${end} of ${state.recipesTotal} components`);
  setText("#page-label", `Page ${state.page + 1}`);
  $("#page-prev").disabled = state.page === 0;
  $("#page-next").disabled = end >= state.recipesTotal;
  $("#select-all").checked = state.recipes.length > 0 && state.recipes.every((r) => state.selected.has(r.id));
  updateSelectedButtons();
}

async function renderOverview() {
  await Promise.all([refreshStatus(), refreshBuilds(), refreshWorkspaces(), refreshSources(false)]);
  renderOverviewBuilds();
  renderOverviewSources();
  renderOverviewWorkspaces();
  await renderOverviewWaiting();
}

function renderOverviewBuilds() {
  const body = $("#overview-builds");
  const builds = flattenedBuilds().slice(0, 7);
  if (!builds.length) {
    body.innerHTML = `<div class="empty">No dashboard jobs yet. Build or fetch something to start the timeline.</div>`;
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

function renderOverviewSources() {
  const s = state.status || {};
  $("#overview-sources").innerHTML = [
    miniMetric("Git", s.git_sources || 0),
    miniMetric("Patches", s.patch_sources || 0),
    miniMetric("Remote", s.remote_sources || 0),
    miniMetric("Local", s.local_sources || 0),
  ].join("");
}

function renderOverviewWorkspaces() {
  const body = $("#overview-workspaces");
  const items = (state.workspaces.items || []).slice(0, 6);
  if (!items.length) {
    body.innerHTML = `<div class="empty">No active workspace. Create one from a component page.</div>`;
    return;
  }
  body.innerHTML = items.map(workspaceRow).join("");
  wireWorkspaceButtons(body);
}

async function renderOverviewWaiting() {
  const data = await api("/api/recipes?state=waiting&limit=7&offset=0");
  const body = $("#overview-waiting");
  const items = data.items || [];
  if (!items.length) {
    body.innerHTML = `<div class="empty">No waiting components.</div>`;
    return;
  }
  body.innerHTML = items.map(componentRow).join("");
  body.querySelectorAll("[data-component]").forEach((el) => el.addEventListener("click", () => openComponent(el.dataset.component)));
}

async function renderComponentPage(id) {
  state.focused = id;
  setText("#component-title", id);
  ["component-build", "component-fetch", "component-workspace", "component-finish"].forEach((buttonId) => {
    const node = $(`#${buttonId}`);
    if (node) node.dataset.id = id;
  });
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
    setText("#component-title", recipe.recipe_id || recipe.id);
    $("#component-state").innerHTML = `${stateBadge(recipe.state)} ${pill(recipe.mode || "container")}`;
    $("#component-overview").innerHTML = definition({
      Element: recipe.element_id || recipe.id,
      Version: recipe.version,
      Hash: recipe.cache,
      Package: recipe.package_name,
      "Cache file": recipe.cache_file,
      About: recipe.about || "",
    });
    setText("#component-workspace-path", recipe.workspace ? recipe.workspace_path : "No active workspace");
    $("#component-finish").disabled = !recipe.workspace;
    $("#component-workspace").disabled = !!recipe.workspace;
    $("#component-graph").innerHTML = [
      chipGroup("Depends", recipe.depends || []),
      chipGroup("Build depends", recipe.build_time_depends || []),
      chipGroup("Used by", recipe.dependents || []),
    ].join("");
    $("#component-sources").innerHTML = sourceCards(recipe.source_details || []);
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
    body.innerHTML = `<div class="empty">No dashboard jobs for this component.</div>`;
    $("#component-logs").textContent = "No build log selected.";
    setText("#component-log-job", "");
    return;
  }
  body.innerHTML = builds.map(buildRow).join("");
  body.querySelectorAll("[data-build]").forEach((el) => el.addEventListener("click", () => attachComponentLogs(el.dataset.build)));
  attachComponentLogs(builds[0].id);
}

async function refreshBuilds() {
  state.builds = await api("/api/builds");
}

function renderBuildsPage() {
  const body = $("#builds-body");
  const builds = flattenedBuilds();
  if (!builds.length) {
    body.innerHTML = `<div class="empty">No jobs yet.</div>`;
    return;
  }
  body.innerHTML = builds.map(buildRow).join("");
  body.querySelectorAll("[data-build]").forEach((el) => el.addEventListener("click", () => attachLogs(el.dataset.build, "#logs")));
  body.querySelectorAll("[data-cancel]").forEach((btn) => {
    btn.addEventListener("click", async (ev) => {
      ev.stopPropagation();
      await fetch(`/api/builds/${encodeURIComponent(btn.dataset.cancel)}/cancel`, { method: "POST" });
      await refreshBuilds();
      renderBuildsPage();
    });
  });
}

async function refreshWorkspaces() {
  state.workspaces = await api("/api/workspaces");
  return state.workspaces;
}

async function renderWorkspaces() {
  await refreshWorkspaces();
  setText("#workspaces-root", state.workspaces.workspace_path || "");
  const body = $("#workspaces-body");
  const items = state.workspaces.items || [];
  if (!items.length) {
    body.innerHTML = `<div class="empty">No active workspaces. Open a component and click Workspace.</div>`;
    return;
  }
  body.innerHTML = items.map(workspaceCard).join("");
  wireWorkspaceButtons(body);
}

async function refreshSources(updateState = true) {
  const params = new URLSearchParams();
  if (state.sourceSearch) params.set("q", state.sourceSearch);
  if (state.sourceTypeFilter) params.set("type", state.sourceTypeFilter);
  const data = await api(`/api/sources?${params}`);
  if (updateState) state.sources = data;
  else state.sources = data;
  return data;
}

async function renderSources() {
  await refreshSources();
  const body = $("#sources-body");
  const items = state.sources.items || [];
  setText("#sources-count", `${items.length} sources`);
  if (!items.length) {
    body.innerHTML = `<tr><td colspan="6" class="empty">No sources match this filter.</td></tr>`;
    return;
  }
  body.innerHTML = items.map((s) => `
    <tr>
      <td><span class="row-title">${escapeHtml(s.name || s.source || "")}</span><span class="row-sub">${escapeHtml(s.source || "")}</span></td>
      <td>${pill(s.type || "unknown")}</td>
      <td><a href="#/components/${encodeURIComponent(s.recipe)}">${escapeHtml(s.recipe)}</a></td>
      <td>${s.locked ? `<span class="checksum" title="${escapeAttr(s.checksum)}">${escapeHtml(shortHash(s.checksum))}</span>` : `<span class="muted">unlocked</span>`}</td>
      <td>${s.cached ? stateBadge("cached") : stateBadge(s.local_exists ? "local" : "waiting")}</td>
      <td class="depends">${escapeHtml(s.ref || s.remote || s.local_path || s.cache_path || "")}</td>
    </tr>
  `).join("");
}

async function renderSettings() {
  const s = await refreshStatus();
  $("#settings-paths").innerHTML = definition({
    Project: s.project_path,
    Cache: s.cache_path,
    Workspaces: s.workspace_path,
    Sources: s.source_path,
    Logs: s.log_path,
    "local.conf.yml": s.local_conf ? "present" : "not present",
    "Recipe watcher": s.watching ? "enabled" : "disabled",
    "Last recipe reload": s.recipe_reloaded_at ? formatTime(s.recipe_reloaded_at) : "startup",
    "Reload error": s.recipe_reload_error || "none",
  });
  $("#settings-mode").innerHTML = [
    miniMetric("Container recipes", s.container || 0),
    miniMetric("Arch", s.arch || ""),
    miniMetric("Version", s.version || ""),
  ].join("");
}

function flattenedBuilds() {
  const items = [];
  if (state.builds.active) items.push(state.builds.active);
  items.push(...(state.builds.queue || []));
  items.push(...(state.builds.history || []));
  return items;
}

function buildRow(b) {
  const label = b.current_recipe || (b.recipes || []).join(", ") || "all recipes";
  const kind = b.kind || "build";
  const cancel = b.state === "queued" || b.state === "running" ? `<button data-cancel="${escapeAttr(b.id)}">Cancel</button>` : "";
  const time = b.started_at ? formatTime(b.started_at) : formatTime(b.created_at);
  const group = b.group_id ? ` · group ${escapeHtml(b.group_id)}` : "";
  return `
    <div class="build-item" data-build="${escapeAttr(b.id)}">
      <div>
        <span class="row-title">${escapeHtml(kind)} · ${escapeHtml(label || b.id)}</span>
        <span class="row-sub">${escapeHtml(b.id)}${group} · ${escapeHtml(time)} · exit ${b.exit_code ?? 0}</span>
      </div>
      <div class="header-actions local">${stateBadge(b.state)}${cancel}</div>
    </div>
  `;
}

function componentRow(r) {
  return `
    <div class="list-row clickable" data-component="${escapeAttr(r.id)}">
      <div><span class="row-title">${escapeHtml(r.recipe_id || r.id)}</span><span class="row-sub">${escapeHtml(r.id)} · ${escapeHtml(r.version || "")}</span></div>
      <div class="header-actions local">${pill(r.mode || "container")}${stateBadge(r.state)}</div>
    </div>
  `;
}

function workspaceRow(w) {
  return `
    <div class="list-row clickable" data-component="${escapeAttr(w.id)}">
      <div><span class="row-title">${escapeHtml(w.recipe_id || w.id)}</span><span class="row-sub">${escapeHtml(w.path || "")}</span></div>
      <div class="header-actions local">${pill(w.git ? "git" : "patch")}${stateBadge(w.dirty ? "workspace" : "cached")}</div>
    </div>
  `;
}

function workspaceCard(w) {
  return `
    <article class="workspace-card">
      <div class="workspace-top">
        <div><span class="row-title">${escapeHtml(w.recipe_id || w.id)}</span><span class="row-sub">${escapeHtml(w.id)} · ${escapeHtml(w.path || "")}</span></div>
        <div class="header-actions local">${pill(w.git ? "git" : "patch")}${stateBadge(w.dirty ? "workspace" : "cached")}</div>
      </div>
      ${w.branch ? `<div class="meta-text">branch: ${escapeHtml(w.branch)}</div>` : ""}
      ${w.status ? `<pre class="status-box">${escapeHtml(w.status)}</pre>` : `<div class="empty tight">No detected source changes.</div>`}
      <div class="header-actions local">
        <button data-open-component="${escapeAttr(w.id)}">Open</button>
        <button data-build-recipe="${escapeAttr(w.id)}">Build</button>
        <button data-finish-workspace="${escapeAttr(w.id)}">Finish</button>
      </div>
    </article>
  `;
}

function wireWorkspaceButtons(root) {
  root.querySelectorAll("[data-component]").forEach((el) => el.addEventListener("click", () => openComponent(el.dataset.component)));
  root.querySelectorAll("[data-open-component]").forEach((btn) => btn.addEventListener("click", () => openComponent(btn.dataset.openComponent)));
  root.querySelectorAll("[data-build-recipe]").forEach((btn) => btn.addEventListener("click", () => triggerAction("build", [btn.dataset.buildRecipe])));
  root.querySelectorAll("[data-finish-workspace]").forEach((btn) => btn.addEventListener("click", () => finishWorkspace(btn.dataset.finishWorkspace)));
}

function attachComponentLogs(jobId) {
  setText("#component-log-job", jobId);
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
    try { appendLog(logs, JSON.parse(ev.data)); }
    catch { appendLog(logs, ev.data); }
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
      if (route().path === "/") renderOverviewBuilds();
    });
    refreshStatus().catch(console.error);
  });
  stream.onerror = () => stream.close();
}

function appendLog(logs, line) {
  const span = document.createElement("span");
  if (line.startsWith("==>")) span.className = "banner";
  else if (/error|fail|panic/i.test(line)) span.className = "err";
  span.textContent = line + "\n";
  logs.appendChild(span);
  logs.scrollTop = logs.scrollHeight;
}

async function triggerAction(action, recipes = [], opts = {}) {
  const result = await api("/api/actions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action, recipes, ...opts }),
  });
  location.hash = "#/builds";
  const firstJob = (result.jobs && result.jobs[0]) || result;
  if (firstJob && firstJob.id) setTimeout(() => attachLogs(firstJob.id, "#logs"), 0);
  return result;
}

async function triggerBuild(recipes) {
  if (!recipes.length) return;
  return triggerAction("build", recipes);
}

async function triggerFetch(recipes = [], force = false) {
  return triggerAction("fetch", recipes, { force });
}

async function finishWorkspace(id) {
  const message = $("#component-message")?.value || "";
  const push = $("#component-finish-push")?.checked || false;
  await triggerAction("workspace-finish", [id], { message, push });
}

function openComponent(id) {
  location.hash = `#/components/${encodeURIComponent(id)}`;
}

function updateSelectedButtons() {
  const size = state.selected.size;
  const build = $("#build-selected");
  build.textContent = `Build selected (${size})`;
  build.disabled = size === 0;
  const fetch = $("#selected-fetch");
  const ws = $("#selected-workspace");
  if (fetch) fetch.disabled = size === 0;
  if (ws) ws.disabled = size === 0;
}

function sourceMiniBadges(r) {
  const parts = [];
  if (Number(r.git_source_count || 0) > 0) parts.push(pill(`git ${r.git_source_count}`));
  if (Number(r.patch_count || 0) > 0) parts.push(pill(`patch ${r.patch_count}`));
  return parts.join(" ");
}

function sourceCards(items) {
  if (!items.length) return `<div class="empty">No sources.</div>`;
  return items.map((s) => `
    <article class="source-card">
      <div class="source-card-head"><strong>${escapeHtml(s.name || s.source)}</strong>${pill(s.type || "unknown")}</div>
      <div class="row-sub">${escapeHtml(s.source || "")}</div>
      <div class="source-meta">
        ${s.ref ? `<span>ref: <code>${escapeHtml(s.ref)}</code></span>` : ""}
        ${s.locked ? `<span>lock: <code title="${escapeAttr(s.checksum)}">${escapeHtml(shortHash(s.checksum))}</code></span>` : `<span>lock: missing</span>`}
        <span>cache: ${s.cached ? "yes" : "no"}</span>
      </div>
    </article>
  `).join("");
}

function definition(values) {
  return Object.entries(values)
    .filter(([, value]) => value !== "" && value != null)
    .map(([key, value]) => `<dt>${escapeHtml(key)}</dt><dd>${escapeHtml(value)}</dd>`)
    .join("");
}

function dataBlock(title, items) {
  if (!items || !items.length) return "";
  return `<div class="data-block"><h4>${escapeHtml(title)}</h4><ul>${items.map((i) => `<li>${escapeHtml(i)}</li>`).join("")}</ul></div>`;
}

function chipGroup(title, items) {
  const chips = (items || []).length ? items.map((item) => `<a class="chip" href="#/components/${encodeURIComponent(item)}">${escapeHtml(item)}</a>`).join("") : `<span class="muted">none</span>`;
  return `<div><h4>${escapeHtml(title)}</h4><div class="chips">${chips}</div></div>`;
}

function miniMetric(label, value) {
  return `<div class="mini-metric"><span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong></div>`;
}

function stateBadge(value) {
  const stateName = value || "unknown";
  return `<span class="state-badge ${escapeAttr(stateName)}">${escapeHtml(stateName)}</span>`;
}

function pill(value) {
  const raw = value || "unknown";
  const name = String(raw).split(" ")[0];
  return `<span class="pill ${escapeAttr(name)}">${escapeHtml(raw)}</span>`;
}

function shortHash(value) {
  const text = String(value || "");
  return text.length > 14 ? `${text.slice(0, 12)}…` : text;
}

function formatTime(epoch) {
  if (!epoch) return "not started";
  return new Date(epoch * 1000).toLocaleString();
}

function setText(sel, value) {
  const node = $(sel);
  if (node) node.textContent = value == null ? "" : String(value);
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
  const sidebarToggle = $("#sidebar-toggle");
  const sidebarClose = $("#sidebar-close");
  const sidebarBackdrop = $("#sidebar-backdrop");
  if (sidebarToggle) sidebarToggle.addEventListener("click", openSidebar);
  if (sidebarClose) sidebarClose.addEventListener("click", closeSidebar);
  if (sidebarBackdrop) sidebarBackdrop.addEventListener("click", closeSidebar);
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") closeSidebar();
  });

  $("#overview-refresh").addEventListener("click", renderOverview);
  $("#overview-fetch").addEventListener("click", () => triggerFetch());
  $$("[data-route-link]").forEach((btn) => btn.addEventListener("click", () => { location.hash = btn.dataset.routeLink; }));
  $$("[data-action]").forEach((btn) => btn.addEventListener("click", () => {
    if (btn.dataset.action === "fetch") triggerFetch();
    if (btn.dataset.action === "status") triggerAction("status", []);
  }));

  $("#filter").addEventListener("input", (e) => { state.filter = e.target.value; state.page = 0; refreshRecipes().catch(console.error); });
  $("#state-filter").addEventListener("change", (e) => { state.stateFilter = e.target.value; state.page = 0; refreshRecipes().catch(console.error); });
  $("#source-filter").addEventListener("change", (e) => { state.sourceFilter = e.target.value; state.page = 0; refreshRecipes().catch(console.error); });
  $("#page-size").addEventListener("change", (e) => { state.pageSize = Number(e.target.value); state.page = 0; refreshRecipes().catch(console.error); });
  $("#page-prev").addEventListener("click", () => { state.page = Math.max(0, state.page - 1); refreshRecipes().catch(console.error); });
  $("#page-next").addEventListener("click", () => { state.page += 1; refreshRecipes().catch(console.error); });
  $("#select-all").addEventListener("change", (e) => {
    for (const r of state.recipes) {
      if (e.target.checked) state.selected.add(r.id);
      else state.selected.delete(r.id);
    }
    renderRecipes();
  });
  $("#build-selected").addEventListener("click", () => triggerBuild([...state.selected]).catch((e) => alert(e.message)));
  $("#selected-fetch").addEventListener("click", () => triggerFetch([...state.selected]).catch((e) => alert(e.message)));
  $("#selected-workspace").addEventListener("click", () => triggerAction("workspace", [...state.selected]).catch((e) => alert(e.message)));
  $("#fetch-sources").addEventListener("click", () => triggerFetch());

  $("#component-build").addEventListener("click", () => { const id = $("#component-build").dataset.id; if (id) triggerBuild([id]).catch((e) => alert(e.message)); });
  $("#component-fetch").addEventListener("click", () => { const id = $("#component-fetch").dataset.id; if (id) triggerFetch([id]).catch((e) => alert(e.message)); });
  $("#component-workspace").addEventListener("click", () => { const id = $("#component-workspace").dataset.id; if (id) triggerAction("workspace", [id]).catch((e) => alert(e.message)); });
  $("#component-finish").addEventListener("click", () => { const id = $("#component-finish").dataset.id; if (id) finishWorkspace(id).catch((e) => alert(e.message)); });
  $("#component-log-refresh").addEventListener("click", () => { if (state.focused) renderComponentPage(state.focused); });

  $("#refresh-workspaces").addEventListener("click", renderWorkspaces);
  $("#source-search").addEventListener("input", (e) => { state.sourceSearch = e.target.value; renderSources().catch(console.error); });
  $("#source-type-filter").addEventListener("change", (e) => { state.sourceTypeFilter = e.target.value; renderSources().catch(console.error); });
  $("#refresh-sources").addEventListener("click", renderSources);
  $("#sources-fetch-all").addEventListener("click", () => triggerFetch());

  $("#refresh-builds").addEventListener("click", async () => { await refreshBuilds(); renderBuildsPage(); });
  $("#logs-clear").addEventListener("click", () => { $("#logs").innerHTML = ""; });
  $("#refresh-settings").addEventListener("click", renderSettings);
}

async function init() {
  initThemeControls();
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
