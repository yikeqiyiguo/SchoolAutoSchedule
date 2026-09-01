/* ============ 基础数据管理：年级/班级/科目/教师/任课关系 ============ */

window.Pages = window.Pages || {};

/* ---------------- 年级管理 ---------------- */
window.Pages.GradesPage = {
  template: `
  <div class="card">
    <div class="card-head"><h3>年级管理 <span class="sub">新增、编辑、删除、排序、启用/禁用</span></h3>
      <button class="btn btn-primary btn-sm" @click="openEdit()">+ 新增年级</button>
    </div>
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th style="width:60px">序号</th><th>年级名称</th><th style="width:100px">班级数</th><th style="width:90px">状态</th><th style="width:220px">操作</th></tr></thead>
        <tbody>
          <tr v-for="g in grades" :key="g.id">
            <td>{{ g.sort_order }}</td><td style="font-weight:600">{{ g.name }}</td>
            <td>{{ g.class_count }}</td>
            <td><span class="tag" :class="g.enabled ? 'tag-green' : 'tag-gray'">{{ g.enabled ? '启用' : '禁用' }}</span></td>
            <td class="ops">
              <button class="btn btn-sm" @click="move(g, -1)">⬆</button>
              <button class="btn btn-sm" @click="move(g, 1)">⬇</button>
              <button class="btn btn-sm" @click="openEdit(g)">编辑</button>
              <button class="btn btn-sm" :class="g.enabled ? 'btn-warning' : 'btn-success'" @click="toggle(g)">{{ g.enabled ? '禁用' : '启用' }}</button>
              <button class="btn btn-sm btn-danger" @click="remove(g)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="modal-mask" v-if="showEdit">
      <div class="modal" style="max-width:420px">
        <div class="modal-head"><h4>{{ editForm.id ? '编辑年级' : '新增年级' }}</h4></div>
        <div class="modal-body">
          <div class="form-item"><label>年级名称 <span class="req">*</span></label><input v-model="editForm.name" placeholder="如：一年级 / 初二 / 高三"></div>
          <div class="form-item mt-8"><label>排序序号</label><input type="number" v-model="editForm.sort_order"></div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>`,
  data() { return { grades: [], showEdit: false, editForm: {} }; },
  mounted() { this.load(); },
  methods: {
    async load() {
      const r = await api.get("/api/base/grades");
      this.grades = r.data.sort((a, b) => a.sort_order - b.sort_order);
    },
    openEdit(g) {
      this.editForm = g ? { ...g } : { name: "", sort_order: (this.grades.length || 0) + 1 };
      this.showEdit = true;
    },
    async save() {
      try {
        if (this.editForm.id) await api.put(`/api/base/grades/${this.editForm.id}`, this.editForm);
        else await api.post("/api/base/grades", this.editForm);
        this.showEdit = false; toast("保存成功"); this.load();
      } catch (e) {}
    },
    async move(g, dir) {
      const sorted = [...this.grades].sort((a, b) => a.sort_order - b.sort_order);
      const idx = sorted.findIndex((x) => x.id === g.id);
      const other = sorted[idx + dir];
      if (!other) return;
      await Promise.all([
        api.put(`/api/base/grades/${g.id}`, { sort_order: other.sort_order }),
        api.put(`/api/base/grades/${other.id}`, { sort_order: g.sort_order }),
      ]);
      this.load();
    },
    async toggle(g) {
      await api.put(`/api/base/grades/${g.id}`, { enabled: !g.enabled });
      toast(g.enabled ? "已禁用" : "已启用"); this.load();
    },
    async remove(g) {
      const ok = await confirmDialog(`确定删除年级「${g.name}」吗？`);
      if (!ok) return;
      try { await api.del(`/api/base/grades/${g.id}`); toast("删除成功"); this.load(); } catch (e) {}
    },
  },
};

/* ---------------- 班级管理 ---------------- */
window.Pages.ClassesPage = {
  template: `
  <div class="card">
    <div class="card-head">
      <h3>班级管理 <span class="sub">批量新增、编辑、禁用（毕业班级禁用不删除）</span></h3>
      <div class="flex">
        <select v-model="filterGrade" style="width:160px" @change="load"><option value="">全部年级</option>
          <option v-for="g in grades" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
        <button class="btn btn-primary btn-sm" @click="openBatch">+ 批量新增班级</button>
        <button class="btn btn-sm" @click="openImport('classes')">📥 Excel导入</button>
      </div>
    </div>
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th>年级</th><th>班级名称</th><th style="width:80px">班级序号</th><th style="width:90px">状态</th><th style="width:220px">操作</th></tr></thead>
        <tbody>
          <tr v-for="c in classes" :key="c.id">
            <td>{{ c.grade_name }}</td><td style="font-weight:600">{{ c.name }}</td>
            <td>{{ c.class_no }}</td>
            <td><span class="tag" :class="c.enabled ? 'tag-green' : 'tag-gray'">{{ c.enabled ? '启用' : '禁用' }}</span></td>
            <td class="ops">
              <button class="btn btn-sm" @click="openEdit(c)">编辑</button>
              <button class="btn btn-sm" :class="c.enabled ? 'btn-warning' : 'btn-success'" @click="toggle(c)">{{ c.enabled ? '禁用' : '启用' }}</button>
              <button class="btn btn-sm btn-danger" @click="remove(c)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="modal-mask" v-if="showBatch">
      <div class="modal">
        <div class="modal-head"><h4>批量新增班级</h4></div>
        <div class="modal-body">
          <div class="form-item"><label>所属年级 <span class="req">*</span></label>
            <select v-model="batch.grade_id"><option v-for="g in grades" :key="g.id" :value="g.id">{{ g.name }}</option></select>
          </div>
          <div class="form-item mt-8"><label>班级名称（每行一个，或逗号分隔）<span class="req">*</span></label>
            <textarea v-model="batch.names" placeholder="1班&#10;2班&#10;3班"></textarea>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showBatch=false">取消</button>
          <button class="btn btn-primary" @click="saveBatch">批量创建</button>
        </div>
      </div>
    </div>

    <div class="modal-mask" v-if="showEdit">
      <div class="modal" style="max-width:420px">
        <div class="modal-head"><h4>编辑班级</h4></div>
        <div class="modal-body">
          <div class="form-item"><label>所属年级</label>
            <select v-model="editForm.grade_id"><option v-for="g in grades" :key="g.id" :value="g.id">{{ g.name }}</option></select>
          </div>
          <div class="form-item mt-8"><label>班级名称</label><input v-model="editForm.name"></div>
          <div class="form-item mt-8"><label>班级序号</label><input type="number" v-model="editForm.class_no"></div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="saveEdit">保存</button>
        </div>
      </div>
    </div>
    <import-modal v-if="showImport" :kind="importKind" @close="showImport=false" @done="afterImport"></import-modal>
  </div>`,
  data() { return { grades: [], classes: [], filterGrade: "", showBatch: false, showEdit: false, showImport: false, importKind: "classes", batch: { grade_id: "", names: "" }, editForm: {} }; },
  mounted() { this.loadGrades(); this.load(); },
  methods: {
    async loadGrades() {
      const r = await api.get("/api/base/grades");
      this.grades = r.data;
    },
    async load() {
      const r = await api.get("/api/base/classes");
      this.classes = r.data.filter((c) => !this.filterGrade || c.grade_id === Number(this.filterGrade));
    },
    openBatch() {
      this.batch = { grade_id: this.grades.length ? this.grades[0].id : "", names: "" };
      this.showBatch = true;
    },
    async saveBatch() {
      const names = this.batch.names.split(/[\n,，]+/).map((s) => s.trim()).filter(Boolean);
      if (!this.batch.grade_id || !names.length) return toast("请选择年级并填写班级名称", "warning");
      try {
        const r = await api.post("/api/base/classes", { grade_id: this.batch.grade_id, names });
        toast(`已创建 ${r.data.length} 个班级`);
        this.showBatch = false; this.load();
      } catch (e) {}
    },
    openEdit(c) { this.editForm = { ...c }; this.showEdit = true; },
    async saveEdit() {
      try { await api.put(`/api/base/classes/${this.editForm.id}`, this.editForm); this.showEdit = false; toast("保存成功"); this.load(); } catch (e) {}
    },
    async toggle(c) {
      await api.put(`/api/base/classes/${c.id}`, { enabled: !c.enabled });
      toast(c.enabled ? "已禁用（保留数据）" : "已启用"); this.load();
    },
    async remove(c) {
      const ok = await confirmDialog(`确定删除班级「${c.name}」吗？将同时删除其任课关系与课表数据！`);
      if (!ok) return;
      try { await api.del(`/api/base/classes/${c.id}`); toast("删除成功"); this.load(); } catch (e) {}
    },
    openImport(kind) { this.importKind = kind; this.showImport = true; },
    afterImport() { this.load(); },
  },
};

/* ---------------- 科目管理 ---------------- */
window.Pages.SubjectsPage = {
  template: `
  <div class="card">
    <div class="card-head"><h3>科目管理 <span class="sub">内置常用科目，支持自定义新增任意科目</span></h3>
      <button class="btn btn-primary btn-sm" @click="openEdit()">+ 新增科目</button>
    </div>
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th style="width:70px">排序</th><th>科目名称</th><th style="width:100px">科目类型</th><th style="width:90px">状态</th><th style="width:200px">操作</th></tr></thead>
        <tbody>
          <tr v-for="s in subjects" :key="s.id">
            <td>{{ s.sort_order }}</td><td style="font-weight:600">{{ s.name }}</td>
            <td><span class="tag" :class="s.subject_type==='main' ? 'tag-main' : 'tag-sub'">{{ s.subject_type_name }}</span></td>
            <td><span class="tag" :class="s.enabled ? 'tag-green' : 'tag-gray'">{{ s.enabled ? '启用' : '禁用' }}</span></td>
            <td class="ops">
              <button class="btn btn-sm" @click="openEdit(s)">编辑</button>
              <button class="btn btn-sm" :class="s.enabled ? 'btn-warning' : 'btn-success'" @click="toggle(s)">{{ s.enabled ? '禁用' : '启用' }}</button>
              <button class="btn btn-sm btn-danger" v-if="!s.is_default" @click="remove(s)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="modal-mask" v-if="showEdit">
      <div class="modal" style="max-width:420px">
        <div class="modal-head"><h4>{{ editForm.id ? '编辑科目' : '新增科目' }}</h4></div>
        <div class="modal-body">
          <div class="form-item"><label>科目名称 <span class="req">*</span></label><input v-model="editForm.name"></div>
          <div class="form-item mt-8"><label>科目类型</label>
            <select v-model="editForm.subject_type"><option value="main">主科</option><option value="sub">副科</option></select>
          </div>
          <div class="form-item mt-8"><label>排序序号</label><input type="number" v-model="editForm.sort_order"></div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>`,
  data() { return { subjects: [], showEdit: false, editForm: {} }; },
  mounted() { this.load(); },
  methods: {
    async load() { const r = await api.get("/api/base/subjects"); this.subjects = r.data.sort((a, b) => a.sort_order - b.sort_order); },
    openEdit(s) { this.editForm = s ? { ...s } : { name: "", subject_type: "main", sort_order: (this.subjects.length || 0) + 1 }; this.showEdit = true; },
    async save() {
      try {
        if (this.editForm.id) await api.put(`/api/base/subjects/${this.editForm.id}`, this.editForm);
        else await api.post("/api/base/subjects", this.editForm);
        this.showEdit = false; toast("保存成功"); this.load();
      } catch (e) {}
    },
    async toggle(s) { await api.put(`/api/base/subjects/${s.id}`, { enabled: !s.enabled }); toast("已更新"); this.load(); },
    async remove(s) {
      const ok = await confirmDialog(`确定删除科目「${s.name}」吗？`);
      if (!ok) return;
      try { await api.del(`/api/base/subjects/${s.id}`); toast("删除成功"); this.load(); } catch (e) {}
    },
  },
};

/* ---------------- 教师管理 ---------------- */
window.Pages.TeachersPage = {
  template: `
  <div class="card">
    <div class="card-head"><h3>教师管理 <span class="sub">可绑定多科目多班级，独立设置周课时上限（硬约束）</span></h3>
      <div class="flex">
        <button class="btn btn-primary btn-sm" @click="openEdit()">+ 新增教师</button>
        <button class="btn btn-sm" @click="openImport('teachers')">📥 Excel导入</button>
      </div>
    </div>
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th>姓名</th><th>任教科目</th><th style="width:90px">岗位</th><th style="width:90px">周课时上限</th><th style="width:90px">晚自习</th><th style="width:90px">状态</th><th style="width:220px">操作</th></tr></thead>
        <tbody>
          <tr v-for="t in teachers" :key="t.id">
            <td style="font-weight:600">{{ t.name }}</td>
            <td>{{ t.subject_names || '—' }}</td>
            <td><span class="tag" :class="t.is_class_teacher ? 'tag-purple' : 'tag-gray'">{{ t.post_name }}</span></td>
            <td>{{ t.weekly_hour_limit }}</td>
            <td><span class="tag" :class="t.allow_evening ? 'tag-green' : 'tag-red'">{{ t.allow_evening ? '允许' : '不允许' }}</span></td>
            <td><span class="tag" :class="t.enabled ? 'tag-green' : 'tag-gray'">{{ t.enabled ? '启用' : '禁用' }}</span></td>
            <td class="ops">
              <button class="btn btn-sm" @click="openEdit(t)">编辑</button>
              <button class="btn btn-sm" :class="t.enabled ? 'btn-warning' : 'btn-success'" @click="toggle(t)">{{ t.enabled ? '禁用' : '启用' }}</button>
              <button class="btn btn-sm btn-danger" @click="remove(t)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <import-modal v-if="showImport" :kind="importKind" @close="showImport=false" @done="load"></import-modal>

    <div class="modal-mask" v-if="showEdit">
      <div class="modal">
        <div class="modal-head"><h4>{{ editForm.id ? '编辑教师' : '新增教师' }}</h4></div>
        <div class="modal-body">
          <div class="form-grid">
            <div class="form-item"><label>姓名 <span class="req">*</span></label><input v-model="editForm.name"></div>
            <div class="form-item"><label>手机号</label><input v-model="editForm.phone"></div>
            <div class="form-item"><label>岗位标识</label>
              <select v-model="editForm.is_class_teacher"><option :value="true">班主任</option><option :value="false">普通教师</option></select></div>
            <div class="form-item"><label>每周最大课时（硬约束）</label><input type="number" min="1" v-model="editForm.weekly_hour_limit"></div>
            <div class="form-item"><label>是否允许排晚自习</label>
              <select v-model="editForm.allow_evening"><option :value="true">允许</option><option :value="false">不允许</option></select></div>
            <div class="form-item"><label>状态</label>
              <select v-model="editForm.enabled"><option :value="true">启用</option><option :value="false">禁用</option></select></div>
          </div>
          <div class="form-item mt-8"><label>任教科目（可多选）</label>
            <div class="list-check">
              <label v-for="s in subjects" :key="s.id" :class="s.subject_type">
                <input type="checkbox" :checked="editForm.subject_ids.includes(s.id)" @change="toggleSubject(s.id)"> {{ s.name }}
              </label>
            </div>
          </div>
          <div class="form-item mt-8"><label>备注</label><input v-model="editForm.remark"></div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>`,
  data() { return { teachers: [], subjects: [], showEdit: false, editForm: {}, showImport: false, importKind: "teachers" }; },
  mounted() { this.load(); this.loadSubjects(); },
  methods: {
    async load() { const r = await api.get("/api/base/teachers"); this.teachers = r.data; },
    async loadSubjects() { const r = await api.get("/api/base/subjects"); this.subjects = r.data.filter((s) => s.enabled); },
    openEdit(t) {
      this.editForm = t ? { ...t, subject_ids: t.subjects.map((s) => s.id) }
        : { name: "", phone: "", is_class_teacher: false, weekly_hour_limit: 20, allow_evening: true, remark: "", enabled: true, subject_ids: [] };
      this.showEdit = true;
    },
    toggleSubject(id) {
      const i = this.editForm.subject_ids.indexOf(id);
      if (i >= 0) this.editForm.subject_ids.splice(i, 1); else this.editForm.subject_ids.push(id);
    },
    async save() {
      if (!this.editForm.name) return toast("请填写教师姓名", "warning");
      try {
        if (this.editForm.id) await api.put(`/api/base/teachers/${this.editForm.id}`, this.editForm);
        else await api.post("/api/base/teachers", this.editForm);
        this.showEdit = false; toast("保存成功"); this.load();
      } catch (e) {}
    },
    async toggle(t) { await api.put(`/api/base/teachers/${t.id}`, { enabled: !t.enabled }); toast("已更新"); this.load(); },
    async remove(t) {
      const ok = await confirmDialog(`确定删除教师「${t.name}」吗？`);
      if (!ok) return;
      try { await api.del(`/api/base/teachers/${t.id}`); toast("删除成功"); this.load(); } catch (e) {}
    },
    openImport(kind) { this.importKind = kind; this.showImport = true; },
  },
};

/* ---------------- 任课关系 ---------------- */
window.Pages.AssignmentsPage = {
  template: `
  <div class="card">
    <div class="card-head">
      <h3>任课关系配置 <span class="sub">班级-科目-教师-每周课时（核心关联数据）</span></h3>
      <div class="flex">
        <select v-model="filterGrade" style="width:130px" @change="load"><option value="">全部年级</option>
          <option v-for="g in grades" :key="g.id" :value="g.id">{{ g.name }}</option></select>
        <select v-model="filterClass" style="width:150px" @change="load"><option value="">全部班级</option>
          <option v-for="c in filteredClasses" :key="c.id" :value="c.id">{{ c.grade_name }}{{ c.name }}</option></select>
        <button class="btn btn-primary btn-sm" @click="openEdit()">+ 新增</button>
        <button class="btn btn-sm" @click="openImport('assignments')">📥 Excel导入</button>
        <button class="btn btn-sm btn-danger" @click="clearAll">一键清空</button>
      </div>
    </div>
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th>班级</th><th>科目</th><th>授课教师</th><th style="width:90px">每周课时</th><th style="width:90px">优先上午</th><th style="width:200px">操作</th></tr></thead>
        <tbody>
          <tr v-for="a in assignments" :key="a.id">
            <td style="font-weight:600">{{ a.grade_name }} {{ a.class_name }}</td>
            <td>{{ a.subject_name }}</td>
            <td>{{ a.teacher_name }}</td>
            <td>{{ a.weekly_hours }}</td>
            <td><span class="tag" :class="a.prefer_morning ? 'tag-blue' : 'tag-gray'">{{ a.prefer_morning ? '是' : '否' }}</span></td>
            <td class="ops">
              <button class="btn btn-sm" @click="openEdit(a)">编辑</button>
              <button class="btn btn-sm btn-danger" @click="remove(a)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="modal-mask" v-if="showEdit">
      <div class="modal">
        <div class="modal-head"><h4>{{ editForm.id ? '编辑任课关系' : '新增任课关系' }}</h4></div>
        <div class="modal-body">
          <div class="form-grid">
            <div class="form-item"><label>年级</label>
              <select v-model="editForm.grade_id" @change="onGradeChange"><option value="">请选择</option>
                <option v-for="g in grades" :key="g.id" :value="g.id">{{ g.name }}</option></select></div>
            <div class="form-item"><label>班级 <span class="req">*</span></label>
              <select v-model="editForm.class_id" @change="onClassChange"><option value="">请选择</option>
                <option v-for="c in gradeClasses" :key="c.id" :value="c.id">{{ c.name }}</option></select></div>
            <div class="form-item"><label>科目 <span class="req">*</span></label>
              <select v-model="editForm.subject_id" @change="onSubjectChange"><option value="">请选择</option>
                <option v-for="s in subjects" :key="s.id" :value="s.id">{{ s.name }}</option></select></div>
            <div class="form-item"><label>授课教师 <span class="req">*</span></label>
              <select v-model="editForm.teacher_id"><option value="">请选择</option>
                <option v-for="t in filteredTeachers" :key="t.id" :value="t.id">{{ t.name }}</option></select></div>
            <div class="form-item"><label>每周课时数 <span class="req">*</span></label><input type="number" min="1" v-model="editForm.weekly_hours"></div>
            <div class="form-item"><label>是否优先上午时段</label>
              <select v-model="editForm.prefer_morning"><option :value="true">是</option><option :value="false">否</option></select></div>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showEdit=false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
    <import-modal v-if="showImport" :kind="importKind" @close="showImport=false" @done="load"></import-modal>
  </div>`,
  data() {
    return { grades: [], classes: [], subjects: [], teachers: [], assignments: [], filterGrade: "", filterClass: "", showEdit: false, showImport: false, importKind: "assignments", editForm: {} };
  },
  computed: {
    filteredClasses() {
      return this.classes.filter((c) => !this.filterGrade || c.grade_id === Number(this.filterGrade));
    },
    gradeClasses() {
      return this.classes.filter((c) => c.grade_id === Number(this.editForm.grade_id));
    },
    filteredTeachers() {
      const sid = this.editForm.subject_id;
      if (!sid) return this.teachers;
      return this.teachers.filter((t) => t.enabled && (t.subjects.some((s) => s.id === Number(sid))));
    },
  },
  mounted() { this.loadMeta(); this.load(); },
  methods: {
    async loadMeta() {
      const [g, c, s, t] = await Promise.all([
        api.get("/api/base/grades"), api.get("/api/base/classes"),
        api.get("/api/base/subjects"), api.get("/api/base/teachers"),
      ]);
      this.grades = g.data; this.classes = c.data; this.subjects = s.data.filter((x) => x.enabled); this.teachers = t.data;
    },
    async load() {
      const r = await api.get("/api/base/assignments");
      let list = r.data;
      if (this.filterGrade) list = list.filter((a) => a.grade_name === (this.grades.find((g) => g.id === Number(this.filterGrade)) || {}).name);
      if (this.filterClass) list = list.filter((a) => a.class_id === Number(this.filterClass));
      this.assignments = list;
    },
    openEdit(a) {
      this.editForm = a ? { ...a, grade_id: (this.classes.find((c) => c.id === a.class_id) || {}).grade_id || "" }
        : { grade_id: "", class_id: "", subject_id: "", teacher_id: "", weekly_hours: 1, prefer_morning: false };
      this.showEdit = true;
    },
    onGradeChange() { this.editForm.class_id = ""; this.editForm.teacher_id = ""; },
    onClassChange() { this.editForm.teacher_id = ""; },
    onSubjectChange() { this.editForm.teacher_id = ""; },
    async save() {
      if (!this.editForm.class_id || !this.editForm.subject_id || !this.editForm.teacher_id) return toast("请完整填写班级/科目/教师", "warning");
      try {
        if (this.editForm.id) await api.put(`/api/base/assignments/${this.editForm.id}`, this.editForm);
        else await api.post("/api/base/assignments", this.editForm);
        this.showEdit = false; toast("保存成功"); this.load();
      } catch (e) {}
    },
    async remove(a) {
      const ok = await confirmDialog(`删除「${a.class_name}-${a.subject_name}-${a.teacher_name}」任课关系？`);
      if (!ok) return;
      try { await api.del(`/api/base/assignments/${a.id}`); toast("删除成功"); this.load(); } catch (e) {}
    },
    async clearAll() {
      const ok = await confirmDialog("确定清空当前筛选下的全部任课关系？（不会删除课表数据，排课前请确认）");
      if (!ok) return;
      try {
        const payload = this.filterClass ? { class_id: Number(this.filterClass) } : {};
        const r = await api.post("/api/base/assignments/clear", payload);
        toast(r.message); this.load();
      } catch (e) {}
    },
    openImport(kind) { this.importKind = kind; this.showImport = true; },
  },
};

/* ---------------- Excel 导入弹窗 ---------------- */
window.Pages.ImportModal = {
  name: "ImportModal",
  template: `
  <div class="modal-mask">
    <div class="modal" style="max-width:460px">
      <div class="modal-head"><h4>Excel 批量导入（{{ kindName }}）</h4></div>
      <div class="modal-body">
        <div class="section-tip">请按模板填写数据后上传。模板列说明：
          <span v-if="kind==='classes'">年级 | 班级名称</span>
          <span v-else-if="kind==='teachers'">姓名 | 手机号 | 岗位(班主任/普通) | 周课时上限 | 允许晚自习(是/否) | 任教科目(/分隔)</span>
          <span v-else>年级 | 班级 | 科目 | 教师 | 每周课时 | 优先上午(是/否)</span>
        </div>
        <div class="flex mt-8">
          <a class="btn btn-sm" :href="'/api/base/import/template/' + kind">⬇ 下载模板</a>
        </div>
        <div class="form-item mt-16">
          <label>选择 Excel 文件</label>
          <input type="file" accept=".xlsx,.xls" @change="onFile">
        </div>
        <div v-if="result" class="mt-16 section-tip" :class="result.hasFail ? 'warn' : ''">{{ result.msg }}</div>
        <div v-if="result && result.fails.length" class="mt-8 list-check" style="grid-template-columns:1fr">
          <div v-for="(f, i) in result.fails" :key="i" style="font-size:12.5px;color:#92400e">• {{ f }}</div>
        </div>
      </div>
      <div class="modal-foot">
        <button class="btn" @click="$emit('close')">关闭</button>
      </div>
    </div>
  </div>`,
  props: { kind: String },
  data() { return { result: null, loading: false }; },
  computed: {
    kindName() { return { classes: "班级", teachers: "教师", assignments: "任课关系" }[this.kind] || this.kind; },
  },
  methods: {
    async onFile(e) {
      const file = e.target.files[0];
      if (!file) return;
      const fd = new FormData();
      fd.append("file", file);
      this.loading = true;
      try {
        const res = await fetch(`/api/base/import/${this.kind}`, { method: "POST", body: fd, credentials: "same-origin" });
        const j = await res.json();
        if (j.success) {
          this.result = { msg: j.message, hasFail: j.data.fail_msgs.length > 0, fails: j.data.fail_msgs };
          if (!this.result.hasFail) { toast(j.message); this.$emit("done"); }
        }
      } catch (err) {} finally { this.loading = false; }
    },
  },
};
