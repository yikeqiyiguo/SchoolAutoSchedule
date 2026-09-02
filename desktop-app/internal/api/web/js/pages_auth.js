/* ============ 登录 / 仪表盘 / 系统配置 ============ */

window.Pages = window.Pages || {};

/* ---------------- 登录页 ---------------- */
window.Pages.LoginPage = {
  template: `
  <div class="login-page">
    <div class="login-box">
      <div class="login-head">
        <div class="login-logo">📅</div>
        <h1>学校智能自定义排课系统</h1>
        <p>多约束智能排课 · 可视化微调 · 多格式导出</p>
      </div>
      <div class="login-form">
        <div class="form-item"><label>用户名</label><input v-model="form.username" placeholder="请输入用户名" @keyup.enter="doLogin"></div>
        <div class="form-item"><label>密码</label><input type="password" v-model="form.password" placeholder="请输入密码" @keyup.enter="doLogin"></div>
        <button class="btn btn-primary btn-lg" style="width:100%;margin-top:8px" :disabled="loading" @click="doLogin">
          {{ loading ? '登录中...' : '登 录' }}
        </button>
        <div class="login-tip">默认管理员账号：admin / admin123</div>
      </div>
    </div>
  </div>`,
  data() {
    return { form: { username: "", password: "" }, loading: false };
  },
  methods: {
    async doLogin() {
      if (!this.form.username || !this.form.password) return toast("请输入用户名和密码", "warning");
      this.loading = true;
      try {
        const r = await api.post("/api/auth/login", this.form);
        window.AppState.user = r.data;
        toast("登录成功");
        location.hash = "#/dashboard";
      } catch (e) {} finally { this.loading = false; }
    },
  },
};

/* ---------------- 仪表盘 ---------------- */
window.Pages.DashboardPage = {
  template: `
  <div>
    <div class="card" v-if="me">
      <div class="flex-between">
        <div>
          <div style="font-size:17px;font-weight:700">欢迎，{{ me.real_name || me.username }}</div>
          <div class="muted mt-8" style="font-size:13px">当前身份：{{ me.role_name }} · {{ schoolName || '学校智能自定义排课系统' }}</div>
        </div>
        <button class="btn btn-primary" v-if="canSchedule" @click="goSchedule">🚀 去一键排课</button>
      </div>
    </div>

    <div class="stat-grid" v-if="ov">
      <div class="stat-card"><div class="s-ic" style="background:#e8efff;color:#3b6ef6">🏫</div><div><div class="s-num">{{ov.grades}}</div><div class="s-label">年级</div></div></div>
      <div class="stat-card"><div class="s-ic" style="background:#e6f7ed;color:#16a34a">🧑‍🏫</div><div><div class="s-num">{{ov.classes}}</div><div class="s-label">班级</div></div></div>
      <div class="stat-card"><div class="s-ic" style="background:#fef3c7;color:#d97706">👩‍🏫</div><div><div class="s-num">{{ov.teachers}}</div><div class="s-label">教师</div></div></div>
      <div class="stat-card"><div class="s-ic" style="background:#ede9fe;color:#7c3aed">📚</div><div><div class="s-num">{{ov.subjects}}</div><div class="s-label">科目</div></div></div>
      <div class="stat-card"><div class="s-ic" style="background:#fce7f3;color:#db2777">🔗</div><div><div class="s-num">{{ov.assignments}}</div><div class="s-label">任课关系</div></div></div>
      <div class="stat-card"><div class="s-ic" style="background:#e0f2fe;color:#0284c7">📊</div><div><div class="s-num">{{ov.schedule_percent}}%</div><div class="s-label">课时完成度</div></div></div>
    </div>

    <div class="card">
      <div class="card-head"><h3>排课进度</h3><span class="sub">已排 {{ ov ? ov.scheduled : 0 }} / 应排 {{ ov ? ov.need_total : 0 }} 节</span></div>
      <div class="progress" :class="{ok: ov && ov.schedule_percent>=100}"><div :style="{width: (ov?ov.schedule_percent:0)+'%'}"></div></div>
    </div>

    <div class="card">
      <div class="card-head"><h3>使用指引</h3></div>
      <div class="section-tip" style="line-height:2">
        <b>快速开始：</b>① 系统配置（课时时段/上课日） → ② 录入年级、班级、科目、教师 → ③ 配置任课关系 → ④ 调整排课规则 → ⑤ 一键排课 → ⑥ 查看/微调课表 → ⑦ 导出课表
      </div>
    </div>
  </div>`,
  data() {
    return { ov: null, me: null, schoolName: "" };
  },
  computed: {
    canSchedule() {
      // 权限已统一：登录用户即可使用排课
      return !!window.AppState.user;
    },
  },
  mounted() {
    this.load();
  },
  methods: {
    async load() {
      try {
        const [me, ov] = await Promise.all([api.get("/api/auth/me"), api.get("/api/stats/overview")]);
        this.me = me.data;
        this.schoolName = ov.data.school_name;
        this.ov = ov.data;
      } catch (e) {}
    },
    goSchedule() { location.hash = "#/schedule"; },
  },
};

/* ---------------- 系统配置 ---------------- */
window.Pages.SettingsPage = {
  template: `
  <div>
    <div class="card">
      <div class="card-head"><h3>学校信息</h3></div>
      <div class="form-grid">
        <div class="form-item"><label>学校名称</label><input v-model="sys.school_name" placeholder="示例学校"></div>
        <div class="form-item"><label>午休时间</label><input v-model="sys.noon_rest" placeholder="12:00-14:00"></div>
        <div class="form-item"><label>放学时间</label><input v-model="sys.school_over" placeholder="17:40"></div>
        <div class="form-item"><label>上课日（可多选，支持单休/双休）</label>
          <div class="flex" style="gap:6px">
            <label v-for="d in 7" :key="d" style="display:flex;align-items:center;gap:4px;font-size:13px">
              <input type="checkbox" :value="d" v-model="sys.school_days"> {{ dayName(d) }}
            </label>
          </div>
        </div>
      </div>
      <div class="form-actions"><button class="btn btn-primary" @click="saveSys">保存学校信息与周次</button></div>
    </div>

    <div class="card">
      <div class="card-head">
        <h3>课时时段配置 <span class="sub">默认10节：上午1-4节、下午5-8节、晚自习9-10节</span></h3>
        <div>
          <button class="btn btn-sm" @click="addPeriod">+ 添加一节</button>
        </div>
      </div>
      <div class="table-wrap">
        <table class="tbl">
          <thead><tr><th style="width:70px">节次</th><th>开始时间</th><th>结束时间</th><th>时段类型</th><th style="width:90px">操作</th></tr></thead>
          <tbody>
            <tr v-for="(p, i) in periods" :key="i">
              <td>第{{ p.period_index }}节</td>
              <td><input type="time" v-model="p.start_time" style="width:130px"></td>
              <td><input type="time" v-model="p.end_time" style="width:130px"></td>
              <td>
                <select v-model="p.period_type" style="width:130px">
                  <option value="morning">上午</option><option value="afternoon">下午</option><option value="evening">晚间（晚自习）</option>
                </select>
              </td>
              <td><button class="btn-link danger" @click="removePeriod(i)">删除</button></td>
            </tr>
            <tr v-if="!periods.length"><td colspan="5"><div class="empty-tip">暂无时段配置，请先添加</div></td></tr>
          </tbody>
        </table>
      </div>
      <div class="form-actions">
        <button class="btn btn-primary" @click="savePeriods">保存课时时段</button>
        <button class="btn" @click="resetDefault">恢复默认(10节)</button>
      </div>
    </div>
  </div>`,
  data() {
    return { sys: { school_name: "", school_days: [1, 2, 3, 4, 5], noon_rest: "", school_over: "" }, periods: [] };
  },
  mounted() { this.load(); },
  methods: {
    dayName(d) { return dayName2(d); },
    async load() {
      try {
        const [s, p] = await Promise.all([api.get("/api/config/system"), api.get("/api/config/periods")]);
        this.sys = s.data;
        this.periods = p.data.map((x) => ({ ...x }));
      } catch (e) {}
    },
    addPeriod() {
      const last = this.periods[this.periods.length - 1];
      this.periods.push({
        period_index: (last ? last.period_index : 0) + 1,
        start_time: "19:00", end_time: "19:45", period_type: "evening",
      });
    },
    removePeriod(i) { this.periods.splice(i, 1); this.renumber(); },
    renumber() { this.periods.forEach((p, i) => { p.period_index = i + 1; }); },
    async savePeriods() {
      try {
        const r = await api.put("/api/config/periods", { periods: this.periods.map((p, i) => ({ ...p, period_index: i + 1 })) });
        toast(r.message);
        this.load();
      } catch (e) {}
    },
    async saveSys() {
      if (!this.sys.school_days.length) return toast("请至少选择一个上课日", "warning");
      try {
        const r = await api.put("/api/config/system", this.sys);
        toast(r.message);
      } catch (e) {}
    },
    async resetDefault() {
      const ok = await confirmDialog("将课时时段恢复为默认10节配置？");
      if (!ok) return;
      const defs = [
        [1, "08:00", "08:45", "morning"], [2, "08:55", "09:40", "morning"],
        [3, "10:00", "10:45", "morning"], [4, "10:55", "11:40", "morning"],
        [5, "14:00", "14:45", "afternoon"], [6, "14:55", "15:40", "afternoon"],
        [7, "16:00", "16:45", "afternoon"], [8, "16:55", "17:40", "afternoon"],
        [9, "19:00", "19:45", "evening"], [10, "19:55", "20:40", "evening"],
      ];
      this.periods = defs.map(([period_index, start_time, end_time, period_type]) => ({ period_index, start_time, end_time, period_type }));
      await this.savePeriods();
    },
  },
};

function dayName2(d) { return DAY_NAMES[d] || `周${d}`; }
