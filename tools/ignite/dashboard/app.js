const state = {
  recipes: [],
  filter: "",
  states: new Set(["cached", "waiting", "workspace", "unknown"]),
  selected: new Set(),
  focused: null,
  logStream: null,
  activeLogJob: null,
};

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => document.querySelectorAll(sel);

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${res.status} ${res.statusText}: ${text}`);
  }
  return res.json();
}

async function refreshStatus() {
  try {
    const s = await api("/api/status");
    $("#c-total").textContent = s.total;
    $("#c-cached").textContent = s.cached;
    $("#c-waiting").textContent = s.waiting;
    $("#c-workspace").textContent = s.workspace;
    $("#arch-tag").textContent = s.arch;
  } catch (e) {
    console.error(e);
  }
}

async function refreshRecipes() {
  try {
    const data = await api("/api/recipes");
    state.recipes = data;
    renderRecipes();
  } catch (e) {
    console.error(e);
  }
}

function renderRecipes() {
  const tbody = $("#recipes-body");
  const filter = state.filter.toLowerCase();
  const filtered = state.recipes.filter((r) => {
    if (!state.states.has(r.state)) return false;
    if (!filter) return true;
    return (
      r.id.toLowerCase().includes(filter) ||
      (r.version || "").toLowerCase().includes(filter) ||
      (r.about || "").toLowerCase().includes(filter)
    );
  });

  tbody.innerHTML = "";
  for (const r of filtered) {
    const tr = document.createElement("tr");
    if (state.focused === r.id) tr.classList.add("selected");
    tr.dataset.id = r.id;
    const checked = state.selected.has(r.id) ? "checked" : "";
    tr.innerHTML = `
      <td><input type="checkbox" data-id="${escapeAttr(r.id)}" ${checked} /></td>
      <td><strong>${escapeHtml(r.recipe_id || r.id)}</strong><br><span style="color:var(--muted);font-size:11px">${escapeHtml(r.id)}</span></td>
      <td>${escapeHtml(r.version || "")}</td>
      <td><span class="state-badge ${r.state}">${r.state}</span></td>
      <td class="depends" title="${escapeAttr((r.depends || []).join(", "))}">${escapeHtml((r.depends || []).join(", "))}</td>
    `;
    tr.addEventListener("click", (ev) => {
      if (ev.target.tagName === "INPUT") return;
      showDetail(r.id);
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
}

function updateBuildButton() {
  const btn = $("#build-selected");
  btn.textContent = `Build selected (${state.selected.size})`;
  btn.disabled = state.selected.size === 0;
}

async function showDetail(id) {
  state.focused = id;
  renderRecipes();
  const body = $("#detail-body");
  body.classList.remove("empty");
  body.innerHTML = "Loading…";
  $("#detail-build").disabled = false;
  $("#detail-build").dataset.id = id;
  try {
    const r = await api(`/api/recipes/${encodeURIComponent(id)}`);
    if (!r) {
      body.textContent = "not found";
      return;
    }
    body.innerHTML = `
      <dl>
        <dt>id</dt><dd>${escapeHtml(r.recipe_id)}</dd>
        <dt>element</dt><dd>${escapeHtml(r.element_id || r.id)}</dd>
        <dt>version</dt><dd>${escapeHtml(r.version)}</dd>
        <dt>state</dt><dd><span class="state-badge ${r.state}">${r.state}</span></dd>
        <dt>hash</dt><dd>${escapeHtml(r.cache || "")}</dd>
        <dt>package</dt><dd>${escapeHtml(r.package_name || "")}</dd>
        <dt>cache file</dt><dd>${escapeHtml(r.cache_file || "")}</dd>
      </dl>
      ${r.about ? `<h3>About</h3><p>${escapeHtml(r.about)}</p>` : ""}
      ${listSection("Depends", r.depends)}
      ${listSection("Build-time depends", r.build_time_depends)}
      ${listSection("Sources", r.sources)}
      ${listSection("Backup", r.backup)}
      ${r.integration ? `<h3>Integration</h3><pre>${escapeHtml(r.integration)}</pre>` : ""}
    `;
  } catch (e) {
    body.textContent = "error: " + e.message;
  }
}

function listSection(title, items) {
  if (!items || !items.length) return "";
  return `<h3>${title}</h3><ul>${items
    .map((i) => `<li>${escapeHtml(i)}</li>`)
    .join("")}</ul>`;
}

async function refreshBuilds() {
  try {
    const data = await api("/api/builds");
    renderBuilds(data);
  } catch (e) {
    console.error(e);
  }
}

function renderBuilds(data) {
  const body = $("#builds-body");
  const items = [];
  if (data.active) items.push(data.active);
  items.push(...data.queue);
  items.push(...data.history);

  if (!items.length) {
    body.innerHTML = `<div class="detail empty">no builds yet</div>`;
    return;
  }
  body.innerHTML = "";
  for (const b of items) {
    const el = document.createElement("div");
    el.className = "build-item" + (state.activeLogJob === b.id ? " active" : "");
    el.innerHTML = `
      <div class="meta">
        <span class="id">${escapeHtml(b.id)}</span>
        <span class="rs" title="${escapeAttr(b.recipes.join(", "))}">${escapeHtml(
      b.current_recipe || b.recipes.join(", ")
    )}</span>
      </div>
      <div style="display:flex;gap:6px;align-items:center">
        <span class="state-badge ${b.state}">${b.state}</span>
        ${
          b.state === "queued" || b.state === "running"
            ? `<button class="link" data-cancel="${escapeAttr(b.id)}">cancel</button>`
            : ""
        }
      </div>
    `;
    el.addEventListener("click", (ev) => {
      if (ev.target.dataset.cancel) return;
      attachLogs(b.id);
    });
    body.appendChild(el);
  }

  body.querySelectorAll("button[data-cancel]").forEach((btn) => {
    btn.addEventListener("click", async (ev) => {
      ev.stopPropagation();
      try {
        await fetch(`/api/builds/${encodeURIComponent(btn.dataset.cancel)}/cancel`, {
          method: "POST",
        });
        refreshBuilds();
      } catch (e) {
        console.error(e);
      }
    });
  });
}

function attachLogs(jobId) {
  if (state.logStream) {
    state.logStream.close();
    state.logStream = null;
  }
  state.activeLogJob = jobId;
  $("#logs-job").textContent = jobId;
  const logs = $("#logs");
  logs.innerHTML = "";

  const stream = new EventSource(`/api/builds/${encodeURIComponent(jobId)}/logs`);
  state.logStream = stream;
  stream.onmessage = (ev) => {
    try {
      const line = JSON.parse(ev.data);
      appendLog(line);
    } catch {
      appendLog(ev.data);
    }
  };
  stream.addEventListener("end", (ev) => {
    const span = document.createElement("span");
    span.className = "end";
    span.textContent = `\n— stream ended (${ev.data}) —\n`;
    logs.appendChild(span);
    logs.scrollTop = logs.scrollHeight;
    stream.close();
    refreshBuilds();
    refreshStatus();
    refreshRecipes();
  });
  stream.onerror = () => {
    stream.close();
  };
  refreshBuilds();
}

function appendLog(line) {
  const logs = $("#logs");
  const span = document.createElement("span");
  if (line.startsWith("==>")) span.className = "banner";
  else if (/error|fail/i.test(line)) span.className = "err";
  span.textContent = line + "\n";
  logs.appendChild(span);
  const nearBottom =
    logs.scrollHeight - logs.scrollTop - logs.clientHeight < 80;
  if (nearBottom) logs.scrollTop = logs.scrollHeight;
}

async function triggerBuild(recipes) {
  if (!recipes.length) return;
  try {
    const job = await api("/api/builds", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(recipes),
    });
    attachLogs(job.id);
  } catch (e) {
    alert("build failed to start: " + e.message);
  }
}

async function triggerFetch() {
  const btn = $("#fetch-sources");
  btn.disabled = true;
  btn.textContent = "Fetching…";
  try {
    await api("/api/fetch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify([]),
    });
    alert("fetch complete");
  } catch (e) {
    alert("fetch failed: " + e.message);
  } finally {
    btn.disabled = false;
    btn.textContent = "Fetch sources";
  }
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
  $("#filter").addEventListener("input", (e) => {
    state.filter = e.target.value;
    renderRecipes();
  });

  $$(".filter-states input").forEach((cb) => {
    cb.addEventListener("change", () => {
      if (cb.checked) state.states.add(cb.value);
      else state.states.delete(cb.value);
      renderRecipes();
    });
  });

  $("#select-all").addEventListener("change", (e) => {
    const tbody = $("#recipes-body");
    tbody.querySelectorAll("input[type=checkbox]").forEach((cb) => {
      cb.checked = e.target.checked;
      if (e.target.checked) state.selected.add(cb.dataset.id);
      else state.selected.delete(cb.dataset.id);
    });
    updateBuildButton();
  });

  $("#build-selected").addEventListener("click", () => {
    triggerBuild([...state.selected]);
  });

  $("#detail-build").addEventListener("click", () => {
    const id = $("#detail-build").dataset.id;
    if (id) triggerBuild([id]);
  });

  $("#fetch-sources").addEventListener("click", triggerFetch);
  $("#refresh-builds").addEventListener("click", refreshBuilds);
  $("#logs-clear").addEventListener("click", () => {
    $("#logs").innerHTML = "";
  });
}

async function init() {
  wireUI();
  await Promise.all([refreshStatus(), refreshRecipes(), refreshBuilds()]);
  setInterval(refreshStatus, 15000);
  setInterval(refreshBuilds, 3000);
}

init();
