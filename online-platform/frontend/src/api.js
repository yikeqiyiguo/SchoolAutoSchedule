/* ============ 请求封装 ============ */
import { toast } from "./util.js";

const api = {
  get: (url) => request("GET", url),
  post: (url, data) => request("POST", url, data),
  put: (url, data) => request("PUT", url, data),
  del: (url, data) => request("DELETE", url, data),
};

async function request(method, url, data) {
  const opt = { method, credentials: "same-origin" };
  if (data !== undefined) {
    opt.headers = { "Content-Type": "application/json" };
    opt.body = JSON.stringify(data);
  }
  const res = await fetch(url, opt);
  if (res.status === 401) {
    location.hash = "#/login";
    throw new Error("未登录或登录已过期");
  }
  if (res.status === 403) {
    const j = await res.json().catch(() => ({}));
    toast(j.message || "无权限执行此操作", "error");
    throw new Error(j.message || "无权限");
  }
  if (!res.ok) {
    let msg = `请求失败(${res.status})`;
    try { msg = (await res.json()).message || msg; } catch (e) {}
    toast(msg, "error");
    throw new Error(msg);
  }
  const json = await res.json();
  if (json && json.success === false) {
    toast(json.message || "操作失败", "error");
    throw new Error(json.message || "操作失败");
  }
  return json;
}

export default api;
