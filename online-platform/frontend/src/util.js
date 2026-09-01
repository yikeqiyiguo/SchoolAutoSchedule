/* ============ 通用工具：Toast、格式化、确认弹窗 ============ */

/* ---------- Toast ---------- */
export function toast(message, type = "success", duration = 2800) {
  let wrap = document.querySelector(".toast-wrap");
  if (!wrap) {
    wrap = document.createElement("div");
    wrap.className = "toast-wrap";
    document.body.appendChild(wrap);
  }
  const icons = { success: "✅", error: "❌", warning: "⚠️" };
  const el = document.createElement("div");
  el.className = `toast ${type}`;
  el.innerHTML = `<span class="t-ic">${icons[type] || "ℹ️"}</span><span>${escapeHtml(message)}</span>`;
  wrap.appendChild(el);
  setTimeout(() => { el.style.opacity = "0"; el.style.transition = "opacity .3s"; setTimeout(() => el.remove(), 320); }, duration);
}

/* ---------- 格式化 ---------- */
export function escapeHtml(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}

const DAY_NAMES = { 1: "周一", 2: "周二", 3: "周三", 4: "周四", 5: "周五", 6: "周六", 7: "周日" };
export function dayName(d) { return DAY_NAMES[d] || `周${d}`; }

export function fmtTime() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/* ---------- 下载文件 ---------- */
export function downloadFile(url, fallbackName) {
  const a = document.createElement("a");
  a.href = url;
  a.download = fallbackName || "";
  document.body.appendChild(a);
  a.click();
  a.remove();
}

/* ---------- 通用确认弹窗 ---------- */
export function confirmDialog(message, title = "确认操作") {
  return new Promise((resolve) => {
    const mask = document.createElement("div");
    mask.className = "modal-mask";
    mask.innerHTML = `
      <div class="modal" style="max-width:400px">
        <div class="modal-head"><h4>${escapeHtml(title)}</h4></div>
        <div class="modal-body" style="font-size:14px">${escapeHtml(message)}</div>
        <div class="modal-foot">
          <button class="btn" data-act="cancel">取消</button>
          <button class="btn btn-primary" data-act="ok">确定</button>
        </div>
      </div>`;
    document.body.appendChild(mask);
    mask.addEventListener("click", (e) => {
      const act = e.target.getAttribute("data-act");
      if (act === "cancel" || e.target === mask) { mask.remove(); resolve(false); }
      else if (act === "ok") { mask.remove(); resolve(true); }
    });
  });
}
