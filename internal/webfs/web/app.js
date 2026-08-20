"use strict";
// Native frontend for the reinsurance ceding engine. Covers the full business
// loop: register treaty/policy/loss, allocate, settle bordereaux, view reports.
const api = {
  async post(path, body) {
    const r = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const txt = await r.text();
    const data = txt ? JSON.parse(txt) : null;
    if (!r.ok) throw new Error((data && (data.error || data.message)) || `HTTP ${r.status}`);
    return data;
  },
  async get(path) {
    const r = await fetch(path);
    const txt = await r.text();
    const data = txt ? JSON.parse(txt) : null;
    if (!r.ok) throw new Error((data && (data.error || data.message)) || `HTTP ${r.status}`);
    return data;
  },
};

function money(cents) {
  if (cents === undefined || cents === null) return "";
  const sign = cents < 0 ? "-" : "";
  const v = Math.abs(cents);
  return sign + (v / 100).toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
function esc(s) {
  if (s === undefined || s === null) return "";
  return String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
}
function formToObj(form) {
  const obj = {};
  for (const el of form.elements) {
    if (!el.name) continue;
    if (el.type === "number" || el.dataset.num !== undefined) {
      obj[el.name] = el.value === "" ? 0 : Number(el.value);
    } else {
      obj[el.name] = el.value;
    }
  }
  return obj;
}

// ---- tab switching ----
document.querySelectorAll(".tab").forEach(btn => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach(b => b.classList.remove("active"));
    document.querySelectorAll(".pane").forEach(p => p.classList.remove("active"));
    btn.classList.add("active");
    document.getElementById("pane-" + btn.dataset.pane).classList.add("active");
  });
});

// ---- render table ----
function renderTable(container, cols, rows, rowIdKey) {
  const c = document.getElementById(container);
  if (!rows || rows.length === 0) { c.innerHTML = '<p class="muted">暂无数据</p>'; return; }
  let html = '<table><thead><tr>';
  cols.forEach(col => { html += `<th>${esc(col.label)}</th>`; });
  html += '</tr></thead><tbody>';
  rows.forEach(row => {
    const id = row[rowIdKey];
    html += `<tr data-id="${esc(id)}" id="row-${esc(id)}">`;
    cols.forEach(col => {
      const v = row[col.key];
      html += `<td class="${col.num ? "num" : ""}">${col.num ? money(v) : esc(v)}</td>`;
    });
    html += '</tr>';
  });
  html += '</tbody></table>';
  c.innerHTML = html;
  // click-to-select rows
  c.querySelectorAll("tbody tr").forEach(tr => {
    tr.addEventListener("click", () => {
      c.querySelectorAll("tbody tr").forEach(t => t.classList.remove("sel"));
      tr.classList.add("sel");
    });
  });
}
function selectedRow(container) {
  const tr = document.querySelector(`#${container} tbody tr.sel`);
  return tr ? tr.getAttribute("data-id") : null;
}

async function loadContracts() {
  try {
    const rows = await api.get("/contracts");
    renderTable("list-contracts", [
      { key: "id", label: "ID" }, { key: "code", label: "合约号" }, { key: "type", label: "类型" },
      { key: "status", label: "状态" }, { key: "attachment_cents", label: "自留额", num: true },
      { key: "limit_cents", label: "限额", num: true }, { key: "cession_rate", label: "比例" },
      { key: "num_reinstatements", label: "恢复次数" },
    ], rows || [], "id");
  } catch (e) { console.warn(e); }
}
async function loadPolicies() {
  try {
    const rows = await api.get("/policies");
    renderTable("list-policies", [
      { key: "id", label: "ID" }, { key: "policy_no", label: "保单号" },
      { key: "contract_id", label: "合约" }, { key: "sum_insured_cents", label: "保额", num: true },
      { key: "original_premium_cents", label: "原保费", num: true }, { key: "cession_rate", label: "分出比例" },
    ], rows || [], "id");
  } catch (e) { console.warn(e); }
}
async function loadLosses() {
  try {
    const rows = await api.get("/losses");
    renderTable("list-losses", [
      { key: "id", label: "ID" }, { key: "claim_no", label: "赔案号" },
      { key: "policy_id", label: "保单" }, { key: "contract_id", label: "合约" },
      { key: "occurrence_date", label: "出险日" }, { key: "paid_amount_cents", label: "赔付", num: true },
      { key: "event_tag", label: "事件" }, { key: "status", label: "状态" },
    ], rows || [], "id");
  } catch (e) { console.warn(e); }
}
async function loadBordereaux() {
  try {
    const rows = await api.get("/bordereaux");
    renderTable("list-bord", [
      { key: "id", label: "ID" }, { key: "contract_id", label: "合约" },
      { key: "period_year", label: "年" }, { key: "period_quarter", label: "季" },
      { key: "ceded_premium_cents", label: "分出保费", num: true },
      { key: "recovered_loss_cents", label: "摊回赔款", num: true },
      { key: "ceding_commission_cents", label: "分保佣金", num: true },
      { key: "reinstatement_premium_cents", label: "恢复保费", num: true },
      { key: "net_balance_cents", label: "净额", num: true }, { key: "status", label: "状态" },
    ], rows || [], "id");
  } catch (e) { console.warn(e); }
}

// ---- forms ----
document.getElementById("form-contract").addEventListener("submit", async e => {
  e.preventDefault();
  const body = formToObj(e.target);
  try { await api.post("/contracts", body); await loadContracts(); }
  catch (err) { alert("登记失败: " + err.message); }
});
document.getElementById("form-policy").addEventListener("submit", async e => {
  e.preventDefault();
  const body = formToObj(e.target);
  try { await api.post("/policies", body); await loadPolicies(); }
  catch (err) { alert("登记失败: " + err.message); }
});
document.getElementById("form-loss").addEventListener("submit", async e => {
  e.preventDefault();
  const body = formToObj(e.target);
  try { await api.post("/losses", body); await loadLosses(); }
  catch (err) { alert("登记失败: " + err.message); }
});
document.getElementById("form-bord").addEventListener("submit", async e => {
  e.preventDefault();
  const body = formToObj(e.target);
  try { await api.post("/bordereaux", body); await loadBordereaux(); }
  catch (err) { alert("失败: " + err.message); }
});

document.getElementById("btn-allocate").addEventListener("click", async () => {
  const id = selectedRow("list-losses");
  if (!id) { alert("请先选中一个赔案"); return; }
  try { await api.post(`/losses/${id}/allocate`, {}); await loadLosses(); alert("摊回完成"); }
  catch (err) { alert("摊回失败: " + err.message); }
});
document.getElementById("btn-event").addEventListener("click", async () => {
  const tag = document.getElementById("event-tag").value;
  const cid = document.getElementById("event-contract").value;
  if (!tag || !cid) { alert("请填写巨灾事件标签和合约ID"); return; }
  try { await api.post(`/contracts/${cid}/events/${encodeURIComponent(tag)}/allocate`, {}); await loadLosses(); alert("巨灾事件摊回完成"); }
  catch (err) { alert("摊回失败: " + err.message); }
});
document.getElementById("btn-settle").addEventListener("click", async () => {
  const id = selectedRow("list-bord");
  if (!id) { alert("请先选中一个账单"); return; }
  try { await api.post(`/bordereaux/${id}/settle`, {}); await loadBordereaux(); alert("结算完成"); }
  catch (err) { alert("结算失败: " + err.message); }
});
document.getElementById("btn-reopen").addEventListener("click", async () => {
  const id = selectedRow("list-bord");
  if (!id) { alert("请先选中一个账单"); return; }
  try { await api.post(`/bordereaux/${id}/reopen`, {}); await loadBordereaux(); alert("重开完成"); }
  catch (err) { alert("重开失败: " + err.message); }
});

document.getElementById("btn-report-contract").addEventListener("click", async () => {
  const id = document.getElementById("report-contract").value;
  if (!id) { alert("请填写合约ID"); return; }
  try { const r = await api.get(`/reports/contracts/${id}`); document.getElementById("report-out").textContent = JSON.stringify(r, null, 2); }
  catch (err) { alert("失败: " + err.message); }
});
document.getElementById("btn-report-bord").addEventListener("click", async () => {
  try { const r = await api.get("/reports/bordereaux"); document.getElementById("report-out").textContent = JSON.stringify(r, null, 2); }
  catch (err) { alert("失败: " + err.message); }
});
document.getElementById("btn-report-losses").addEventListener("click", async () => {
  try { const r = await api.get("/reports/losses"); document.getElementById("report-out").textContent = JSON.stringify(r, null, 2); }
  catch (err) { alert("失败: " + err.message); }
});

loadContracts(); loadPolicies(); loadLosses(); loadBordereaux();
