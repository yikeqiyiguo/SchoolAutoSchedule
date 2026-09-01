/* ============ 排课规则 / 一键排课 / 课表 / 微调 / 冲突 / 导出 / 统计 ============ */

window.Pages = window.Pages || {};

/* ---------------- 排课规则 ---------------- */
window.Pages.RulesPage = {
  template: `
  <div>
    <div class="card">
      <div class="card-head">
        <h3>软约束规则 <span class="sub">可自定义新增/启用/禁用/调整优先级，冲突时自动让步</span></h3>
        <button class="btn btn-primary btn-sm" @click="openCreate">+ 新增规则</button>
      </div>
      <div class="section-tip">
        <b>系统固定硬约束（不可关闭，100%满足）：</b>① 同一教师同一时间只能上一个班 ② 同一班级同一时间只有一门课 ③ 教师周课时 ≤ 上限 ④ 班级科目周课时严格匹配预设
      </div>
      <div v-for="r in rules" :key="r.id" class="rule-card">
        <div style="flex:1">
          <div class="flex">
            <span class="r-name">{{ r.name }}</span>
            <span class="tag" :class="r.is_default ? 'tag-purple' : 'tag-gray'">{{ r.is_default ? '默认' : '自定义' }}</span>
            <span class="tag" :class="r.enabled ? 'tag-green' : 'tag-red'">{{ r.enabled ? '启用' : '禁用' }}</span>
            <span class="r-prio">优先级：{{ r.priority }}</span>
          </div>
          <div class="r-desc">{{ r.description }}</div>
        </div>
        <div class="ops">
          <button class="btn btn-sm" @click="move(r, -1)">⬆</button>
          <button class="btn btn-sm" @click="move(r, 1)">⬇</button>
          <button class="btn btn-sm" @click="openEdit(r)">编辑</button>
          <button class="btn btn-sm" :class="r.enabled ? 'btn-warning' : 'btn-success'" @click="toggle(r)">{{ r.enabled ? '禁用' : '启用' }}</button>
          <button class="btn btn-sm btn-danger" @click="remove(r)">删除</button>
        </div>
      </div>
      <div v-if="!rules.length" class="empty-tip">暂无规则</div>
    </div>

    <div class="modal-mask" v-if="showEdit">
      <div class="modal">
        <div class="modal-head"><h4>{{ editForm.id ? '编辑规则' : '新增规则' }}</h4></div>
        <div class="modal-body">
          <div class="form-item"><label>规则名称 <span class="req">*</span></label><input v-model="editForm.name"></div>
          <div class="form-item mt-8"><label>规则类型</label>
            <select v-model="editForm.rule_type" @change="onTypeChange" :disabled="!!editForm.id">
              <option v-for="(v, k) in ruleTypes" :key="k" :value="k">{{ v.name }}</option>
            </select>
            <div class="muted mt-8" style="font-size:12.5px">{{ ruleTypes[editForm.rule_type]?.desc }}</div>
          </div>
          <template v-if="editForm.rule_type">
            <div class="form-item mt-8" v-if="hasParam('subject_ids')">
              <label>{{ paramLabel('subject_ids') }}</label>
              <div class="list-check">
                <label v-for="s in subjects" :key="s.id">
                  <input type="checkbox" :value="s.id" v-model="editForm.params.subject_ids"> {{ s.name }}
                </label>
              </div>
              <div class="muted mt-8" style="font-size:12px">留空表示不限定具体科目（按科目类型自动生效）</div>
            </div>
            <div class="form-item mt-8" v-if="hasParam('teacher_ids')">
              <label>{{ paramLabel('teacher_ids') }}</label>
              <div class="list-check">
                <label v-for="t in teachers" :key="t.id">
                  <input type="checkbox" :value="t.id" v-model="editForm.params.teacher_ids"> {{ t.name }}
                </label>
              </div>
              <div class="muted mt-8" style="font-size:12px">留空表示不限定教师</div>
            </div>
            <div class="form-item mt-8" v-if="hasParam('period_types')">
              <label>{{ paramLabel('period_types') }}</label>
              <div class="flex" style="gap:12px">
                <label v-for="(n, k) in periodTypeNames" :key="k" style="display:flex;align-items:center;gap:4px">
                  <input type="checkbox" :value="k" v-model="editForm.params.period_types"> {{ n }}
                </label>
              </div>
            </div>
            <div class="form-item mt-8" v-if="hasParam('max_consecutive')">
              <label>同天最多连续节数</label>
              <input type="number" min="1" v-model="editForm.params.max_consecutive" style="width:120px">
            </div>
          </template>
          <div class="form-item mt-8"><label>说明</label><input v-model="editForm.description"></div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>`,
  data() {
    return {
      rules: [], subjects: [], teachers: [], ruleTypes: {}, periodTypeNames: {},
      showEdit: false, editForm: { params: {} },
    };
  },
  mounted() { this.load(); },
  methods: {
    async load() {
      const [r, s, t, m] = await Promise.all([
        api.get("/api/rules"), api.get("/api/base/subjects"),
        api.get("/api/base/teachers"), api.get("/api/config/meta"),
      ]);
      this.rules = r.data;
      this.subjects = s.data.filter((x) => x.enabled);
      this.teachers = t.data.filter((x) => x.enabled);
      this.ruleTypes = m.data.rule_types;
      this.periodTypeNames = m.data.period_type_names;
    },
    hasParam(k) { return (this.ruleTypes[this.editForm.rule_type]?.params || []).some((p) => p.key === k); },
    paramLabel(k) { return (this.ruleTypes[this.editForm.rule_type]?.params || []).find((p) => p.key === k)?.label || k; },
    onTypeChange() {
      const params = this.editForm.params || {};
      params.subject_ids = params.subject_ids || [];
      params.teacher_ids = params.teacher_ids || [];
      params.period_types = params.period_types || [];
      params.max_consecutive = params.max_consecutive || 2;
    },
    openCreate() {
      const firstType = Object.keys(this.ruleTypes)[0];
      this.editForm = { id: null, name: "", rule_type: firstType, params: { subject_ids: [], teacher_ids: [], period_types: [], max_consecutive: 2 }, description: "" };
      this.showEdit = true;
    },
    openEdit(r) {
      this.editForm = JSON.parse(JSON.stringify({ ...r }));
      if (!this.editForm.params) this.editForm.params = {};
      this.editForm.params.subject_ids = this.editForm.params.subject_ids || [];
      this.editForm.params.teacher_ids = this.editForm.params.teacher_ids || [];
      this.editForm.params.period_types = this.editForm.params.period_types || [];
      this.showEdit = true;
    },
    async save() {
      if (!this.editForm.name) return toast("请填写规则名称", "warning");
      try {
        if (this.editForm.id) await api.put(`/api/rules/${this.editForm.id}`, this.editForm);
        else await api.post("/api/rules", this.editForm);
        this.showEdit = false; toast("保存成功"); this.load();
      } catch (e) {}
    },
    async toggle(r) {
      await api.put(`/api/rules/${r.id}`, { enabled: !r.enabled });
      toast(r.enabled ? "已禁用" : "已启用"); this.load();
    },
    async move(r, dir) {
      const sorted = [...this.rules].sort((a, b) => b.priority - a.priority);
      const idx = sorted.findIndex((x) => x.id === r.id);
      const other = sorted[idx + dir];
      if (!other) return;
      await Promise.all([
        api.put(`/api/rules/${r.id}`, { priority: other.priority }),
        api.put(`/api/rules/${other.id}`, { priority: r.priority }),
      ]);
      this.load();
    },
    async remove(r) {
      const ok = await confirmDialog(`确定删除规则「${r.name}」吗？`);
      if (!ok) return;
      try { await api.del(`/api/rules/${r.id}`); toast("删除成功"); this.load(); } catch (e) {}
    },
  },
};

/* ---------------- 一键排课 ---------------- */
window.Pages.SchedulePage = {
  template: `
  <div>
    <div class="card">
      <div class="card-head"><h3>一键智能排课</h3></div>
      <div class="flex" style="gap:16px;align-items:center">
        <button class="btn btn-success btn-lg" :disabled="running" @click="runSchedule">
          {{ running ? '⏳ 排课运算中...' : '🚀 一键排课' }}
        </button>
        <button class="btn btn-danger" @click="clearTimetable">🗑 清空课表</button>
        <label style="display:flex;align-items:center;gap:6px;font-size:13px">
          <input type="checkbox" v-model="keepManual" style="width:auto"> 保留手动微调的课
        </label>
        <span v-if="lastResult" class="muted" style="font-size:13px">上次排课耗时 {{ lastResult.cost_ms }}ms</span>
      </div>
      <div v-if="lastResult" class="mt-16">
        <div class="section-tip" :class="lastResult.success ? '' : 'warn'">
          {{ lastResult.message }}，检测到 {{ lastResult.conflict_count || 0 }} 项冲突
        </div>
        <div v-if="lastResult.errors && lastResult.errors.length" class="section-tip warn">
          <div v-for="(e, i) in lastResult.errors" :key="i">⚠️ {{ e }}</div>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-head"><h3>排课总览</h3>
        <span class="sub">共 {{ overview.total_classes }} 个班级，每周可排 {{ overview.total_periods_per_week }} 节</span></div>
      <div class="table-wrap">
        <table class="tbl">
          <thead><tr><th>班级</th><th style="width:120px">应排课时</th><th style="width:120px">已排课时</th><th style="width:200px">完成度</th></tr></thead>
          <tbody>
            <tr v-for="c in overview.classes" :key="c.class_id">
              <td style="font-weight:600">{{ c.grade_name }} {{ c.class_name }}</td>
              <td>{{ c.need }}</td><td>{{ c.have }}</td>
              <td>
                <div class="flex" style="gap:8px">
                  <div class="progress" :class="{ok: c.percent>=100}" style="flex:1"><div :style="{width: c.percent+'%'}"></div></div>
                  <span style="font-size:12px;width:44px">{{ c.percent }}%</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>`,
  data() { return { overview: { classes: [] }, running: false, keepManual: true, lastResult: null }; },
  mounted() { this.load(); },
  methods: {
    async load() {
      try { const r = await api.get("/api/schedule/overview"); this.overview = r.data; } catch (e) {}
    },
    async runSchedule() {
      this.running = true;
      this.lastResult = null;
      try {
        const r = await api.post("/api/schedule/run", { keep_manual: this.keepManual });
        this.lastResult = r;
        toast(r.message);
        this.load();
        if (r.conflict_count > 0) location.hash = "#/conflicts";
      } catch (e) {
        if (e.errors) this.lastResult = { success: false, message: e.message, errors: e.errors, cost_ms: e.cost_ms };
      } finally { this.running = false; }
    },
    async clearTimetable() {
      const ok = await confirmDialog("确定清空全部课表数据吗？此操作不可恢复！");
      if (!ok) return;
      try {
        const r = await api.post("/api/schedule/clear", { type: "all" });
        toast(r.message); this.load();
      } catch (e) {}
    },
  },
};

/* ---------------- 班级课表（含手动微调） ---------------- */
window.Pages.ClassTimetablePage = {
  template: `
  <div>
    <div class="card">
      <div class="card-head">
        <h3>班级课表</h3>
        <div class="flex">
          <select v-model="gradeId" style="width:130px" @change="onGradeChange"><option value="">选择年级</option>
            <option v-for="g in grades" :key="g.id" :value="g.id">{{ g.name }}</option></select>
          <select v-model="classId" style="width:150px" @change="load"><option value="">选择班级</option>
            <option v-for="c in gradeClasses" :key="c.id" :value="c.id">{{ c.name }}</option></select>
          <button class="btn btn-sm" @click="printView">🖨 打印/预览</button>
          <button class="btn btn-sm" @click="download('/api/export/class/' + classId + '/excel')">⬇ Excel</button>
          <button class="btn btn-sm" @click="download('/api/export/class/' + classId + '/pdf')">⬇ PDF</button>
        </div>
      </div>
      <div v-if="canEdit" class="section-tip" style="margin-bottom:12px">
        💡 点击单元格可编辑/清空；拖拽单元格到另一格可交换课程；左上角黄点表示手动微调过的课程
      </div>
      <div v-if="!classId" class="empty-tip">请选择班级查看课表</div>
      <div v-else class="table-wrap">
        <table class="timetable">
          <thead><tr><th style="width:70px">节次</th><th style="width:110px">时间</th><th v-for="d in days" :key="d.index">{{ d.name }}</th></tr></thead>
          <tbody>
            <tr v-for="p in periods" :key="p.period_index">
              <td class="period-col">第{{ p.period_index }}节</td>
              <td class="time-col">{{ p.start_time }}<br>{{ p.end_time }}</td>
              <td v-for="d in days" :key="d.index"
                  class="cell" :class="cellClass(matrix, d.index, p)" 
                  :draggable="canEdit && !!getCell(matrix, d.index, p.period_index)"
                  @click="onCellClick(matrix, d.index, p.period_index)"
                  @dragstart="onDragStart(matrix, d.index, p.period_index, $event)"
                  @dragover.prevent @drop="onDrop(matrix, d.index, p.period_index, $event)">
                <div v-if="getCell(matrix, d.index, p.period_index)" class="cell-content">
                  <div class="subj" :class="subjClass(getCell(matrix, d.index, p.period_index))">{{ getCell(matrix, d.index, p.period_index).subject_name }}</div>
                  <div class="teach">{{ getCell(matrix, d.index, p.period_index).teacher_name }}</div>
                </div>
                <div v-else class="cell-content empty-cell">＋</div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="modal-mask" v-if="showEdit">
      <div class="modal" style="max-width:460px">
        <div class="modal-head"><h4>微调课程 — {{ currentCell?.subject_name || '空课' }}</h4></div>
        <div class="modal-body">
          <div class="section-tip">班级「{{ currentClass?.name }}」{{ currentDay }} 第{{ currentPeriod }}节</div>
          <div class="form-item"><label>科目</label>
            <select v-model="editForm.subject_id" @change="onSubjectChange">
              <option value="">— 选择科目 —</option>
              <option v-for="a in subjectOptions" :key="a.subject_id" :value="a.subject_id">{{ a.subject_name }}</option>
            </select>
          </div>
          <div class="form-item mt-8"><label>授课教师</label>
            <select v-model="editForm.teacher_id">
              <option value="">— 选择教师 —</option>
              <option v-for="t in teacherOptions" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn btn-danger" @click="clearCell">清空此节</button>
          <div style="flex:1"></div>
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="saveCell">保存</button>
        </div>
      </div>
    </div>
  </div>`,
  data() {
    return {
      grades: [], classes: [], periods: [], gradeId: "", classId: "", matrix: {},
      days: [{ index: 1, name: "周一" }, { index: 2, name: "周二" }, { index: 3, name: "周三" }, { index: 4, name: "周四" }, { index: 5, name: "周五" }],
      assignments: [], showEdit: false, editForm: { subject_id: "", teacher_id: "" },
      currentCell: null, currentPeriod: null, currentDay: "", dragFrom: null, canEdit: false,
    };
  },
  computed: {
    gradeClasses() { return this.classes.filter((c) => c.grade_id === Number(this.gradeId)); },
    currentClass() { return this.classes.find((c) => c.id === Number(this.classId)); },
    subjectOptions() {
      const list = this.assignments.filter((a) => a.class_id === Number(this.classId));
      const seen = new Set();
      return list.filter((a) => !seen.has(a.subject_id) && seen.add(a.subject_id));
    },
    teacherOptions() {
      if (!this.editForm.subject_id) return [];
      const sid = Number(this.editForm.subject_id);
      return this.assignments.filter((a) => a.class_id === Number(this.classId) && a.subject_id === sid);
    },
  },
  mounted() { this.init(); },
  methods: {
    async init() {
      const u = window.AppState.user;
      this.canEdit = u && (u.role === "super" || u.role === "operator");
      const [g, c, p] = await Promise.all([
        api.get("/api/base/grades"), api.get("/api/base/classes"), api.get("/api/config/periods"),
      ]);
      this.grades = g.data; this.classes = c.data; this.periods = p.data;
      // 恢复上次选择
      const saved = sessionStorage.getItem("ttClass");
      if (saved) { const s = JSON.parse(saved); this.gradeId = s.gradeId; this.classId = s.classId; this.load(); }
    },
    onGradeChange() { this.classId = ""; this.matrix = {}; },
    async load() {
      if (!this.classId) { this.matrix = {}; return; }
      sessionStorage.setItem("ttClass", JSON.stringify({ gradeId: this.gradeId, classId: this.classId }));
      try {
        const [tt, as] = await Promise.all([
          api.get(`/api/schedule/timetable/class?class_id=${this.classId}`),
          api.get("/api/base/assignments"),
        ]);
        this.matrix = tt.data.matrix;
        this.assignments = as.data;
        this.days = tt.data.periods.length
          ? this.days
          : this.days;
        // 同步学校上课日（简化：固定周一~周五展示）
      } catch (e) {}
    },
    getCell(m, d, p) { return m[d]?.[p]; },
    cellClass(m, d, p) {
      const cell = this.getCell(m, d, p);
      if (!cell) return "empty-cell";
      let cls = "";
      if (cell.subject_type === "main") cls += " cell-main";
      else cls += " cell-sub";
      if (cell.source === "manual") cls += " manual-flag";
      const period = this.periods.find((x) => x.period_index === p);
      if (period && period.period_type === "evening") cls += " cell-evening";
      if (this.dragFrom && this.dragFrom.classId === this.classId
        && this.dragFrom.day === d && this.dragFrom.period === p) cls += " selected";
      return cls;
    },
    subjClass(cell) { return cell.subject_type === "main" ? "" : ""; },
    onCellClick(m, d, p) {
      if (!this.canEdit) return;
      const cell = this.getCell(m, d, p);
      this.currentCell = cell || null;
      this.currentDay = dayName(d);
      this.editDay = d;
      this.currentPeriod = p;
      this.editForm = { subject_id: cell?.subject_id || "", teacher_id: cell?.teacher_id || "" };
      this.showEdit = true;
    },
    onSubjectChange() { this.editForm.teacher_id = ""; },
    onDragStart(m, d, p, e) {
      if (!this.canEdit) return;
      this.dragFrom = { classId: this.classId, day: d, period: p };
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", `${d}-${p}`);
    },
    async onDrop(m, d, p, e) {
      if (!this.canEdit || !this.dragFrom) return;
      e.preventDefault();
      const f = this.dragFrom;
      this.dragFrom = null;
      if (f.day === d && f.period === p) return;
      try {
        const r = await api.post("/api/schedule/swap", {
          class_id: Number(this.classId),
          from: { day: f.day, period: f.period },
          to: { day: d, period: p },
        });
        toast(r.message);
        this.load();
      } catch (err) {}
    },
    async saveCell() {
      if (!this.editForm.subject_id || !this.editForm.teacher_id) return toast("请选择科目和教师", "warning");
      try {
        const r = await api.post("/api/schedule/cell", {
          class_id: Number(this.classId),
          day_index: Number(this.editDay),
          period_index: this.currentPeriod,
          subject_id: Number(this.editForm.subject_id),
          teacher_id: Number(this.editForm.teacher_id),
        });
        this.showEdit = false;
        toast(r.message);
        this.load();
      } catch (e) {}
    },
    async clearCell() {
      const ok = await confirmDialog("确定清空这一节课吗？");
      if (!ok) return;
      try {
        const r = await api.post("/api/schedule/cell", {
          class_id: Number(this.classId), day_index: this.editDay, period_index: this.currentPeriod, clear: true,
        });
        this.showEdit = false; toast(r.message); this.load();
      } catch (e) {}
    },
    download(url) { downloadFile(url); },
    printView() { window.open(`/api/export/class/${this.classId}/pdf`, "_blank"); },
  },
};

/* ---------------- 教师课表 ---------------- */
window.Pages.TeacherTimetablePage = {
  template: `
  <div class="card">
    <div class="card-head">
      <h3>教师个人课表</h3>
      <div class="flex" v-if="canPick">
        <select v-model="teacherId" style="width:180px" @change="load"><option value="">选择教师</option>
          <option v-for="t in teachers" :key="t.id" :value="t.id">{{ t.name }}</option></select>
        <button class="btn btn-sm" @click="download('/api/export/teacher/' + teacherId + '/excel')">⬇ Excel</button>
        <button class="btn btn-sm" @click="download('/api/export/teacher/' + teacherId + '/pdf')">⬇ PDF</button>
      </div>
      <div class="flex" v-else>
        <span class="tag tag-blue">{{ teacherInfo.name }}</span>
        <button class="btn btn-sm" @click="download('/api/export/teacher/' + teacherId + '/excel')">⬇ Excel</button>
        <button class="btn btn-sm" @click="download('/api/export/teacher/' + teacherId + '/pdf')">⬇ PDF</button>
      </div>
    </div>
    <div v-if="teacherInfo" class="flex" style="gap:16px;margin-bottom:12px">
      <span class="muted">本周已排：<b class="text-success">{{ haveCount }}</b> 节</span>
      <span class="muted">上限：{{ teacherInfo.weekly_hour_limit }} 节</span>
      <span class="muted">剩余：<b>{{ teacherInfo.weekly_hour_limit - haveCount }}</b> 节</span>
      <span class="muted">任教科目：{{ teacherInfo.subject_names }}</span>
    </div>
    <div v-if="!teacherId" class="empty-tip">请选择教师查看课表</div>
    <div v-else class="table-wrap">
      <table class="timetable">
        <thead><tr><th style="width:70px">节次</th><th style="width:110px">时间</th><th v-for="d in days" :key="d.index">{{ d.name }}</th></tr></thead>
        <tbody>
          <tr v-for="p in periods" :key="p.period_index">
            <td class="period-col">第{{ p.period_index }}节</td>
            <td class="time-col">{{ p.start_time }}<br>{{ p.end_time }}</td>
            <td v-for="d in days" :key="d.index" class="cell" :class="cellCls(matrix, d, p)">
              <div v-if="getCell(matrix, d, p)" class="cell-content">
                <div class="subj">{{ getCell(matrix, d, p).subject_name }}</div>
                <div class="teach">{{ getCell(matrix, d, p).class_name }}</div>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>`,
  data() {
    return {
      teachers: [], teacherId: "", teacherInfo: null, matrix: {}, periods: [], haveCount: 0,
      canPick: false,
      days: [{ index: 1, name: "周一" }, { index: 2, name: "周二" }, { index: 3, name: "周三" }, { index: 4, name: "周四" }, { index: 5, name: "周五" }],
    };
  },
  mounted() { this.init(); },
  methods: {
    async init() {
      const u = window.AppState.user;
      this.canPick = u && (u.role === "super" || u.role === "operator");
      const p = await api.get("/api/config/periods");
      this.periods = p.data;
      if (u && u.role === "teacher" && u.teacher_id) {
        this.teacherId = u.teacher_id;
        this.load();
      } else {
        const t = await api.get("/api/base/teachers");
        this.teachers = t.data.filter((x) => x.enabled);
      }
    },
    async load() {
      if (!this.teacherId) return;
      const r = await api.get(`/api/schedule/timetable/teacher?teacher_id=${this.teacherId}`);
      this.matrix = r.data.matrix;
      this.teacherInfo = r.data.teacher;
      this.haveCount = Object.values(this.matrix).reduce((n, m) => n + Object.keys(m).length, 0);
    },
    getCell(m, d, p) { return m[d]?.[p]; },
    cellCls(m, d, p) {
      const cell = this.getCell(m, d, p);
      if (!cell) return "empty-cell";
      const period = this.periods.find((x) => x.period_index === p);
      let cls = cell.subject_type === "main" ? " cell-main" : " cell-sub";
      if (period && period.period_type === "evening") cls += " cell-evening";
      return cls;
    },
    download(url) { downloadFile(url); },
  },
};

/* ---------------- 冲突检测 ---------------- */
window.Pages.ConflictsPage = {
  template: `
  <div class="card">
    <div class="card-head"><h3>排课冲突检测</h3>
      <div class="flex">
        <span class="tag" :class="conflicts.length ? 'tag-red' : 'tag-green'">{{ conflicts.length ? conflicts.length + ' 项冲突' : '无冲突' }}</span>
        <button class="btn btn-sm" @click="load">🔄 刷新检测</button>
      </div>
    </div>
    <div v-if="!conflicts.length" class="empty-tip">✅ 未检测到任何冲突：教师无撞课、班级无时段冲突、课时未超限</div>
    <div v-else class="table-wrap">
      <table class="tbl">
        <thead><tr><th style="width:80px">级别</th><th style="width:110px">类型</th><th>冲突描述</th></tr></thead>
        <tbody>
          <tr v-for="(c, i) in conflicts" :key="i">
            <td><span class="tag" :class="c.level==='error' ? 'tag-red' : 'tag-warning'">{{ c.level==='error' ? '严重' : '警告' }}</span></td>
            <td>{{ typeName(c.type) }}</td>
            <td>{{ c.message }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>`,
  data() { return { conflicts: [] }; },
  mounted() { this.load(); },
  methods: {
    async load() {
      const r = await api.get("/api/schedule/conflicts");
      this.conflicts = r.data;
    },
    typeName(t) {
      return { teacher_conflict: "教师撞课", class_conflict: "班级冲突", hour_limit: "课时超限", hour_mismatch: "课时不匹配", evening_limit: "晚自习违规" }[t] || t;
    },
  },
};

/* ---------------- 导出中心 ---------------- */
window.Pages.ExportPage = {
  template: `
  <div>
    <div class="card">
      <div class="card-head"><h3>班级课表导出</h3></div>
      <div class="form-row">
        <select v-model="exportClass" style="width:220px"><option value="">选择单个班级（可选）</option>
          <option v-for="c in classes" :key="c.id" :value="c.id">{{ c.grade_name }} {{ c.name }}</option></select>
        <button class="btn btn-primary" @click="download('/api/export/all-classes/excel')">📥 全校班级课表 (Excel)</button>
        <button class="btn" :disabled="!exportClass" @click="download('/api/export/class/' + exportClass + '/excel')">📥 班级课表 (Excel)</button>
        <button class="btn" :disabled="!exportClass" @click="download('/api/export/class/' + exportClass + '/pdf')">📥 班级课表 (PDF)</button>
      </div>
    </div>
    <div class="card">
      <div class="card-head"><h3>教师课表导出</h3></div>
      <div class="form-row">
        <select v-model="exportTeacher" style="width:220px"><option value="">选择单个教师（可选）</option>
          <option v-for="t in teachers" :key="t.id" :value="t.id">{{ t.name }}</option></select>
        <button class="btn btn-primary" @click="download('/api/export/all-teachers/excel')">📥 全校教师课表 (Excel)</button>
        <button class="btn" :disabled="!exportTeacher" @click="download('/api/export/teacher/' + exportTeacher + '/excel')">📥 教师课表 (Excel)</button>
        <button class="btn" :disabled="!exportTeacher" @click="download('/api/export/teacher/' + exportTeacher + '/pdf')">📥 教师课表 (PDF)</button>
      </div>
    </div>
    <div class="card">
      <div class="card-head"><h3>统计报表导出</h3></div>
      <div class="form-row">
        <button class="btn btn-primary" @click="download('/api/export/stats/excel')">📥 教师课时统计报表 (Excel)</button>
      </div>
    </div>
  </div>`,
  data() { return { classes: [], teachers: [], exportClass: "", exportTeacher: "" }; },
  mounted() {
    Promise.all([api.get("/api/base/classes"), api.get("/api/base/teachers")]).then(([c, t]) => {
      this.classes = c.data; this.teachers = t.data;
    });
  },
  methods: {
    download(url) {
      downloadFile(url);
      toast("导出中，请稍候");
    },
  },
};

/* ---------------- 课时统计 ---------------- */
window.Pages.StatsPage = {
  template: `
  <div>
    <div class="card">
      <div class="card-head"><h3>课时统计</h3>
        <div class="flex">
          <button class="btn btn-sm" :class="tab==='teacher' ? 'btn-primary' : ''" @click="tab='teacher';load()">教师维度</button>
          <button class="btn btn-sm" :class="tab==='class' ? 'btn-primary' : ''" @click="tab='class';load()">班级维度</button>
          <button class="btn btn-sm" @click="download('/api/export/stats/excel')">📥 导出统计</button>
        </div>
      </div>
      <div class="table-wrap">
        <table class="tbl">
          <thead v-if="tab==='teacher'">
            <tr><th>教师</th><th>任教科目</th><th>应排课时</th><th>已排课时</th><th>剩余课时</th><th>课时使用率</th><th>上限</th></tr>
          </thead>
          <tbody v-if="tab==='teacher'">
            <tr v-for="r in teacherRows" :key="r.teacher_id">
              <td style="font-weight:600">{{ r.teacher_name }}</td><td>{{ r.subjects }}</td>
              <td>{{ r.need }}</td><td class="text-success">{{ r.have }}</td><td>{{ r.remain }}</td>
              <td><div class="flex" style="gap:8px"><div class="progress" :class="{ok:r.percent>=100}" style="flex:1;min-width:90px"><div :style="{width:r.percent+'%'}"></div></div><span style="font-size:12px">{{ r.percent }}%</span></div></td>
              <td>{{ r.limit }}</td>
            </tr>
          </tbody>
          <thead v-else>
            <tr><th>班级</th><th>年级</th><th>应排课时</th><th>已排课时</th><th>剩余课时</th><th>完成度</th></tr>
          </thead>
          <tbody v-else>
            <tr v-for="r in classRows" :key="r.class_id">
              <td style="font-weight:600">{{ r.class_name }}</td><td>{{ r.grade_name }}</td>
              <td>{{ r.need }}</td><td class="text-success">{{ r.have }}</td><td>{{ r.remain }}</td>
              <td><div class="flex" style="gap:8px"><div class="progress" :class="{ok:r.percent>=100}" style="flex:1;min-width:90px"><div :style="{width:r.percent+'%'}"></div></div><span style="font-size:12px">{{ r.percent }}%</span></div></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>`,
  data() { return { tab: "teacher", teacherRows: [], classRows: [] }; },
  mounted() { this.load(); },
  methods: {
    async load() {
      if (this.tab === "teacher") {
        const r = await api.get("/api/stats/teacher");
        this.teacherRows = r.data;
      } else {
        const r = await api.get("/api/stats/class");
        this.classRows = r.data;
      }
    },
    download(url) { downloadFile(url); },
  },
};
