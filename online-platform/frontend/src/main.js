/* ============ 学校智能自定义排课系统 - 前端入口 ============ */
import { createApp } from "vue/dist/vue.esm-bundler.js";
import "./legacy/css/app.css";
import api from "./api.js";
import { toast, confirmDialog, downloadFile, dayName, fmtTime } from "./util.js";

// 兼容模板表达式中直接引用的全局（Vue 渲染函数回退到 window）
window.api = api;
window.toast = toast;
window.confirmDialog = confirmDialog;
window.downloadFile = downloadFile;
window.dayName = dayName;
window.fmtTime = fmtTime;
window.AppState = { user: null };

/* 页面组件（保持原全局脚本风格，执行后写入 window.Pages） */
await import("./legacy/js/pages_auth.js");
await import("./legacy/js/pages_data.js");
await import("./legacy/js/pages_schedule.js");
await import("./legacy/js/pages_system.js");

/* 导航菜单定义（按角色过滤） */
const MENUS = [
  { key: "dashboard", title: "仪表盘", icon: "🏠", hash: "#/dashboard" },
  { group: "基础数据" },
  { key: "settings", title: "系统配置", icon: "⚙️", hash: "#/settings" },
  { key: "grades", title: "年级管理", icon: "🏫", hash: "#/grades" },
  { key: "classes", title: "班级管理", icon: "🧑‍🎓", hash: "#/classes" },
  { key: "subjects", title: "科目管理", icon: "📚", hash: "#/subjects" },
  { key: "teachers", title: "教师管理", icon: "👩‍🏫", hash: "#/teachers" },
  { key: "assignments", title: "任课关系", icon: "🔗", hash: "#/assignments" },
  { group: "排课中心" },
  { key: "rules", title: "排课规则", icon: "📐", hash: "#/rules" },
  { key: "schedule", title: "一键排课", icon: "🚀", hash: "#/schedule" },
  { key: "class-timetable", title: "班级课表", icon: "🗓️", hash: "#/class-timetable" },
  { key: "teacher-timetable", title: "教师课表", icon: "👩‍🏫", hash: "#/teacher-timetable" },
  { key: "conflicts", title: "冲突检测", icon: "🚨", hash: "#/conflicts" },
  { key: "stats", title: "课时统计", icon: "📊", hash: "#/stats" },
  { group: "数据与系统" },
  { key: "schools", title: "学校管理", icon: "🏫", hash: "#/schools" },
  { key: "export", title: "导出中心", icon: "📥", hash: "#/export" },
  { key: "logs", title: "操作日志", icon: "📜", hash: "#/logs" },
  { key: "backup", title: "备份恢复", icon: "💾", hash: "#/backup" },
  { key: "users", title: "用户管理", icon: "🔐", hash: "#/users" },
];

const PAGE_TITLES = {
  dashboard: "仪表盘", settings: "系统配置", grades: "年级管理", classes: "班级管理",
  subjects: "科目管理", teachers: "教师管理", assignments: "任课关系", rules: "排课规则",
  schedule: "一键排课", "class-timetable": "班级课表", "teacher-timetable": "教师课表",
  conflicts: "冲突检测", stats: "课时统计", export: "导出中心", logs: "操作日志",
  backup: "备份恢复", users: "用户管理", schools: "学校管理",
};

const PAGE_COMPONENTS = {
  dashboard: "DashboardPage", settings: "SettingsPage", grades: "GradesPage",
  classes: "ClassesPage", subjects: "SubjectsPage", teachers: "TeachersPage",
  assignments: "AssignmentsPage", rules: "RulesPage", schedule: "SchedulePage",
  "class-timetable": "ClassTimetablePage", "teacher-timetable": "TeacherTimetablePage",
  conflicts: "ConflictsPage", stats: "StatsPage", export: "ExportPage",
  logs: "LogsPage", backup: "BackupPage", users: "UsersPage", schools: "SchoolsPage", login: "LoginPage",
};

const Root = {
  data() {
    return {
      page: "dashboard", sidebarOpen: false, showPwdModal: false,
      pwdForm: { old_password: "", new_password: "" },
      schools: [], schoolId: "",
    };
  },
  computed: {
    user() { return window.AppState.user; },
    menus() {
      // 权限已统一：所有登录用户可见全部菜单
      return MENUS;
    },
    pageTitle() { return PAGE_TITLES[this.page] || ""; },
    pageComponent() { return PAGE_COMPONENTS[this.page] || "DashboardPage"; },
  },
  methods: {
    nav(hash) { location.hash = hash; this.sidebarOpen = false; },
    async loadSchools() {
      try {
        const r = await api.get("/api/schools");
        this.schools = r.data.list || [];
        this.schoolId = r.data.current ? r.data.current.id : "";
      } catch (e) {}
    },
    async switchSchool() {
      if (!this.schoolId) return;
      try {
        const r = await api.post(`/api/schools/${this.schoolId}/switch`);
        toast(r.message || "已切换学校");
        setTimeout(() => location.reload(), 400);
      } catch (e) {
        this.loadSchools();
      }
    },
    logout() {
      api.post("/api/auth/logout").catch(() => {});
      window.AppState.user = null;
      location.hash = "#/login";
    },
    openPwd() { this.pwdForm = { old_password: "", new_password: "" }; this.showPwdModal = true; },
    async savePwd() {
      if (!this.pwdForm.old_password || !this.pwdForm.new_password) return toast("请填写完整", "warning");
      try {
        const r = await api.post("/api/auth/change-password", this.pwdForm);
        toast(r.message);
        this.showPwdModal = false;
      } catch (e) {}
    },
  },
  template: `
  <div>
    <template v-if="page==='login'">
      <component :is="'LoginPage'"></component>
    </template>
    <template v-else>
      <div class="layout">
        <div class="sidebar-mask" :class="{show: sidebarOpen}" @click="sidebarOpen=false"></div>
        <aside class="sidebar" :class="{open: sidebarOpen}">
          <div class="sidebar-logo">
            <div class="logo-ic">📅</div>
            <div><div class="logo-t">智能排课系统</div><div class="logo-s">School Auto Schedule</div></div>
          </div>
          <div style="flex:1;overflow-y:auto">
            <template v-for="m in menus" :key="m.key || m.group">
              <div v-if="m.group" class="nav-group">{{ m.group }}</div>
              <div v-else class="nav-item" :class="{active: page===m.key}" @click="nav(m.hash)">
                <span class="nav-ic">{{ m.icon }}</span>{{ m.title }}
              </div>
            </template>
          </div>
          <div class="nav-foot">Web 线上版 · v1.0</div>
        </aside>
        <div class="main">
          <header class="topbar">
            <div style="display:flex;align-items:center;gap:14px">
              <div class="page-title">{{ pageTitle }}</div>
              <select class="school-switch" v-if="schools.length" v-model="schoolId" @change="switchSchool" title="切换学校（各学校数据相互独立）">
                <option v-for="s in schools" :key="s.id" :value="s.id">🏫 {{ s.name }}{{ s.current ? ' ·当前' : '' }}</option>
              </select>
            </div>
            <div class="topbar-right">
              <div class="user-chip">
                <div class="avatar">{{ (user?.real_name || user?.username || '?').charAt(0) }}</div>
                <div>
                  <div style="font-weight:600;font-size:13.5px">{{ user?.real_name || user?.username }}</div>
                  <div style="font-size:11.5px;color:var(--text-2)">{{ user?.role_name }}</div>
                </div>
              </div>
              <button class="btn btn-sm" @click="openPwd">修改密码</button>
              <button class="btn btn-sm btn-danger" @click="logout">退出</button>
            </div>
          </header>
          <main class="content">
            <component :is="pageComponent"></component>
          </main>
        </div>
      </div>

      <div class="modal-mask" v-if="showPwdModal">
        <div class="modal" style="max-width:400px">
          <div class="modal-head"><h4>修改密码</h4></div>
          <div class="modal-body">
            <div class="form-item"><label>原密码</label><input type="password" v-model="pwdForm.old_password"></div>
            <div class="form-item mt-8"><label>新密码（至少6位）</label><input type="password" v-model="pwdForm.new_password"></div>
          </div>
          <div class="modal-foot">
            <button class="btn" @click="showPwdModal=false">取消</button>
            <button class="btn btn-primary" @click="savePwd">确定</button>
          </div>
        </div>
      </div>
    </template>
  </div>`,
};

/* 注册全部页面组件（含 ImportModal 全局弹窗） */
const sasApp = createApp(Root);
Object.keys(window.Pages).forEach((name) => {
  sasApp.component(name, window.Pages[name]);
});
sasApp.component("import-modal", window.Pages.ImportModal);
const vm = sasApp.mount("#app");

function parseHash() {
  const h = location.hash.replace(/^#\/?/, "");
  return h.split("?")[0] || "dashboard";
}

async function doRoute() {
  let page = parseHash();
  if (page === "login") { vm.page = "login"; return; }
  if (!window.AppState.user) {
    try {
      const r = await api.get("/api/auth/me");
      window.AppState.user = r.data;
    } catch (e) {
      vm.page = "login";
      return;
    }
  }
  // 无论从登录页进入还是会话恢复，都确保学校列表已加载（顶栏切换器依赖）
  if (!vm.schools.length) {
    vm.loadSchools();
  }
  const menu = MENUS.find((m) => m.key === page);
  if (!menu) {
    // 权限已统一：登录用户可访问任意功能页，仅校验页面是否存在
    page = "dashboard";
  }
  vm.page = page;
}

window.addEventListener("hashchange", doRoute);
doRoute();
