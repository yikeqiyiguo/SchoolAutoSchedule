/* ============ 操作日志 / 备份恢复 / 账号权限 / 学校管理 ============ */

window.Pages = window.Pages || {};

/* ---------------- 操作日志 ---------------- */
window.Pages.LogsPage = {
  template: `
  <div class="card">
    <div class="card-head"><h3>操作日志</h3>
      <div class="flex">
        <select v-model="filter.action" style="width:130px" @change="load">
          <option value="">全部类型</option>
          <option v-for="(n, k) in actionNames" :key="k" :value="k">{{ n }}</option>
        </select>
        <input v-model="filter.keyword" placeholder="搜索操作人/内容" style="width:200px" @keyup.enter="load">
        <button class="btn btn-sm" @click="load">搜索</button>
      </div>
    </div>
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th style="width:70px">ID</th><th style="width:110px">操作人</th><th style="width:90px">类型</th><th>操作内容</th><th style="width:160px">时间</th></tr></thead>
        <tbody>
          <tr v-for="l in logs" :key="l.id">
            <td>{{ l.id }}</td><td>{{ l.username }}</td>
            <td><span class="tag tag-blue">{{ l.action_name }}</span></td>
            <td>{{ l.detail }}</td><td class="muted">{{ l.created_at }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="flex mt-16" style="justify-content:flex-end">
      <button class="btn btn-sm" :disabled="page<=1" @click="go(page-1)">上一页</button>
      <span class="muted">第 {{ page }} / {{ pages }} 页</span>
      <button class="btn btn-sm" :disabled="page>=pages" @click="go(page+1)">下一页</button>
    </div>
  </div>`,
  data() { return { logs: [], actionNames: {}, filter: { action: "", keyword: "" }, page: 1, pages: 1 }; },
  mounted() { this.loadMeta(); this.load(); },
  methods: {
    async loadMeta() { const r = await api.get("/api/logs/actions"); this.actionNames = r.data; },
    async load() {
      const q = new URLSearchParams({ page: this.page, per_page: 20 });
      if (this.filter.action) q.set("action", this.filter.action);
      if (this.filter.keyword) q.set("keyword", this.filter.keyword);
      const r = await api.get(`/api/logs?${q.toString()}`);
      this.logs = r.data; this.page = r.page; this.pages = r.pages;
    },
    go(p) { if (p < 1) return; this.page = p; this.load(); },
  },
};

/* ---------------- 备份恢复 ---------------- */
window.Pages.BackupPage = {
  template: `
  <div class="card">
    <div class="card-head"><h3>数据备份与恢复 <span class="sub">支持一键手动备份、定时自动备份、备份文件恢复</span></h3>
      <button class="btn btn-primary btn-sm" :disabled="busy" @click="create">📦 立即手动备份</button>
    </div>
    <div class="section-tip">备份文件保存在系统 backups 目录。恢复操作会用所选备份覆盖当前数据库（恢复前会自动备份当前数据）。</div>
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th style="width:70px">ID</th><th>备份文件</th><th style="width:100px">大小</th><th style="width:90px">类型</th><th style="width:160px">时间</th><th style="width:200px">操作</th></tr></thead>
        <tbody>
          <tr v-for="b in backups" :key="b.id">
            <td>{{ b.id }}</td><td>{{ b.filename }}</td>
            <td>{{ b.file_size_text }}</td>
            <td><span class="tag" :class="b.backup_type==='auto' ? 'tag-purple' : 'tag-green'">{{ b.backup_type==='auto' ? '自动' : '手动' }}</span></td>
            <td class="muted">{{ b.created_at }}</td>
            <td class="ops">
              <button class="btn btn-sm" @click="download(b)">⬇ 下载</button>
              <button class="btn btn-sm btn-warning" @click="restore(b)">🔄 恢复</button>
              <button class="btn btn-sm btn-danger" @click="remove(b)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>`,
  data() { return { backups: [], busy: false }; },
  mounted() { this.load(); },
  methods: {
    async load() { const r = await api.get("/api/backup"); this.backups = r.data; },
    async create() {
      this.busy = true;
      try { const r = await api.post("/api/backup/create"); toast(r.message); this.load(); } catch (e) {} finally { this.busy = false; }
    },
    download(b) { downloadFile(`/api/backup/download/${b.id}`, b.filename); },
    async restore(b) {
      const ok = await confirmDialog(`确定用备份「${b.filename}」恢复数据吗？当前数据将被覆盖（恢复前系统会自动备份）。`);
      if (!ok) return;
      try { const r = await api.post(`/api/backup/restore/${b.id}`); toast(r.message); setTimeout(() => location.reload(), 1200); } catch (e) {}
    },
    async remove(b) {
      const ok = await confirmDialog(`确定删除备份「${b.filename}」吗？`);
      if (!ok) return;
      try { await api.del(`/api/backup/delete/${b.id}`); toast("删除成功"); this.load(); } catch (e) {}
    },
  },
};

/* ---------------- 账号权限管理 ---------------- */
window.Pages.UsersPage = {
  template: `
  <div class="card">
    <div class="card-head"><h3>用户管理 <span class="sub">所有登录账号均拥有完整功能权限</span></h3>
      <button class="btn btn-primary btn-sm" @click="openEdit()">+ 新增账号</button>
    </div>
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th>用户名</th><th>姓名</th><th style="width:90px">状态</th><th style="width:170px">创建时间</th><th style="width:240px">操作</th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td style="font-weight:600">{{ u.username }}</td><td>{{ u.real_name }}</td>
            <td><span class="tag" :class="u.enabled ? 'tag-green' : 'tag-gray'">{{ u.enabled ? '启用' : '禁用' }}</span></td>
            <td class="muted">{{ u.created_at }}</td>
            <td class="ops">
              <button class="btn btn-sm" @click="openEdit(u)">编辑</button>
              <button class="btn btn-sm" @click="resetPwd(u)">重置密码</button>
              <button class="btn btn-sm" :class="u.enabled ? 'btn-warning' : 'btn-success'" @click="toggle(u)">{{ u.enabled ? '禁用' : '启用' }}</button>
              <button class="btn btn-sm btn-danger" @click="remove(u)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="modal-mask" v-if="showEdit">
      <div class="modal" style="max-width:460px">
        <div class="modal-head"><h4>{{ editForm.id ? '编辑账号' : '新增账号' }}</h4></div>
        <div class="modal-body">
          <div class="form-grid">
            <div class="form-item"><label>用户名 <span class="req">*</span></label><input v-model="editForm.username" :disabled="!!editForm.id"></div>
            <div class="form-item"><label>姓名</label><input v-model="editForm.real_name"></div>
            <div class="form-item"><label>{{ editForm.id ? '重置密码（留空不修改）' : '密码（默认123456）' }}</label><input type="password" v-model="editForm.password"></div>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>`,
  data() { return { users: [], showEdit: false, editForm: {} }; },
  mounted() { this.load(); },
  methods: {
    async load() { const r = await api.get("/api/auth/users"); this.users = r.data; },
    openEdit(u) {
      this.editForm = u ? { id: u.id, username: u.username, real_name: u.real_name || "" } : { username: "", real_name: "", password: "" };
      this.showEdit = true;
    },
    async save() {
      if (!this.editForm.username) return toast("请填写用户名", "warning");
      try {
        const payload = { real_name: this.editForm.real_name || "", password: this.editForm.password || "" };
        if (this.editForm.id) await api.put(`/api/auth/users/${this.editForm.id}`, payload);
        else await api.post("/api/auth/users", { username: this.editForm.username, ...payload });
        this.showEdit = false; toast("保存成功"); this.load();
      } catch (e) {}
    },
    async resetPwd(u) {
      const pwd = prompt(`为账号「${u.username}」设置新密码（至少6位）：`, "123456");
      if (!pwd) return;
      try { await api.post(`/api/auth/reset-password/${u.id}`, { password: pwd }); toast("密码已重置"); } catch (e) {}
    },
    async toggle(u) {
      if (u.id === window.AppState.user?.id && u.enabled) return toast("不能禁用当前登录账号", "warning");
      try { await api.put(`/api/auth/users/${u.id}`, { enabled: !u.enabled }); toast("已更新"); this.load(); } catch (e) {}
    },
    async remove(u) {
      const ok = await confirmDialog(`确定删除账号「${u.username}」吗？`);
      if (!ok) return;
      try { await api.del(`/api/auth/users/${u.id}`); toast("删除成功"); this.load(); } catch (e) {}
    },
  },
};

/* ---------------- 学校管理（多学校独立课表） ---------------- */
window.Pages.SchoolsPage = {
  template: `
  <div>
    <div class="card">
      <div class="card-head">
        <h3>学校管理 <span class="sub">每所学校拥有独立的年级、班级、教师、任课关系与课表，互不影响</span></h3>
        <button class="btn btn-primary btn-sm" @click="openEdit()">+ 新建学校</button>
      </div>
      <div class="section-tip">在顶部栏可快速切换学校；切换后所有基础数据与课表均为该校独立数据，无需删除重建。</div>
      <div class="table-wrap">
        <table class="tbl">
          <thead><tr><th>学校名称</th><th style="width:110px">状态</th><th style="width:170px">创建时间</th><th style="width:260px">操作</th></tr></thead>
          <tbody>
            <tr v-for="s in schools" :key="s.id">
              <td style="font-weight:600">🏫 {{ s.name }}</td>
              <td><span class="tag" :class="s.current ? 'tag-green' : 'tag-gray'">{{ s.current ? '当前使用' : '未使用' }}</span></td>
              <td class="muted">{{ s.created_at }}</td>
              <td class="ops">
                <button class="btn btn-sm btn-primary" v-if="!s.current" @click="switchTo(s)">🔄 切换到此校</button>
                <button class="btn btn-sm" @click="openEdit(s)">改名</button>
                <button class="btn btn-sm btn-danger" v-if="!s.current" @click="remove(s)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="modal-mask" v-if="showEdit">
      <div class="modal" style="max-width:420px">
        <div class="modal-head"><h4>{{ editForm.id ? '修改学校名称' : '新建学校' }}</h4></div>
        <div class="modal-body">
          <div class="form-item"><label>学校名称 <span class="req">*</span></label>
            <input v-model="editForm.name" placeholder="例如：实验中学" @keyup.enter="save"></div>
          <div class="section-tip" v-if="!editForm.id">新建后将自动切换到新学校，并初始化默认科目、课时时段与排课规则。</div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>`,
  data() { return { schools: [], showEdit: false, editForm: {} }; },
  mounted() { this.load(); },
  methods: {
    async load() {
      const r = await api.get("/api/schools");
      this.schools = r.data.list || [];
    },
    openEdit(s) {
      this.editForm = s ? { id: s.id, name: s.name } : { name: "" };
      this.showEdit = true;
    },
    async save() {
      if (!this.editForm.name) return toast("请填写学校名称", "warning");
      try {
        if (this.editForm.id) {
          const r = await api.put(`/api/schools/${this.editForm.id}`, { name: this.editForm.name });
          toast(r.message); this.showEdit = false; this.load();
          if (window.AppState.user) setTimeout(() => location.reload(), 400);
        } else {
          const r = await api.post("/api/schools", { name: this.editForm.name });
          toast(r.message || "学校已创建");
          setTimeout(() => location.reload(), 600);
        }
      } catch (e) {}
    },
    async switchTo(s) {
      const ok = await confirmDialog(`切换到「${s.name}」吗？切换后当前页面数据将刷新为此学校的数据。`);
      if (!ok) return;
      try {
        const r = await api.post(`/api/schools/${s.id}/switch`);
        toast(r.message);
        setTimeout(() => location.reload(), 400);
      } catch (e) {}
    },
    async remove(s) {
      const ok = await confirmDialog(`确定删除学校「${s.name}」吗？该校全部数据（年级/班级/教师/课表）将被永久删除，不可恢复！`);
      if (!ok) return;
      try { await api.del(`/api/schools/${s.id}`); toast("已删除"); this.load(); } catch (e) {}
    },
  },
};
