<script setup>
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import workflowMark from './assets/workflow-mark.svg'

const API = import.meta.env.VITE_API_URL || ''
const route = useRoute()
const router = useRouter()
const token = ref(localStorage.getItem('token') || '')
const user = ref(JSON.parse(localStorage.getItem('user') || 'null'))
const loading = ref(false)
const toast = reactive({ text: '', type: 'success', timer: 0 })
const codeSent = ref(false)

const authForm = reactive({ email: '', password: '', name: '', code: '' })
const teamForm = reactive({ name: 'Product Team' })
const workerForm = reactive({ id: '', name: '', email: '', skillsText: 'go' })
const taskForm = reactive({ id: '', title: '', description: '', status: 'backlog', skillsText: 'go', assignee_id: '' })
const filters = reactive({ status: '', limit: 50, offset: 0 })

const teams = ref([])
const workers = ref([])
const tasks = ref([])
const history = ref([])
const comments = ref([])
const commentBody = ref('')
const selectedTaskId = ref('')
const workerModalOpen = ref(false)
const taskModalOpen = ref(false)
const teamModalOpen = ref(false)

const statuses = [
  ['backlog', 'Бэклог'],
  ['todo', 'К выполнению'],
  ['in_progress', 'В работе'],
  ['review', 'Ревью'],
  ['ready_for_testing', 'Готово к тестированию'],
  ['testing', 'Тестирование'],
  ['done', 'Готово']
]

const navItems = [
  { id: 'tasks', label: 'Задачи', icon: '☑' },
  { id: 'workers', label: 'Работники', icon: '◫' },
  { id: 'planning', label: 'Распределение', icon: '⌁' }
]

const authed = computed(() => Boolean(token.value))
const authMode = computed(() => route.name === 'register' ? 'register' : 'login')
const page = computed(() => ['tasks', 'workers', 'planning'].includes(String(route.name)) ? String(route.name) : 'tasks')
const team = computed(() => teams.value[0] || null)
const selectedTask = computed(() => tasks.value.find((task) => String(task.id) === String(selectedTaskId.value)))

function statusLabel(status) {
  return statuses.find(([value]) => value === status)?.[1] || status
}

function parseSkills(text) {
  return text
    .split(',')
    .map((skill) => skill.trim().toLowerCase())
    .filter(Boolean)
}

function skillsText(skills) {
  return (skills || []).join(', ')
}

async function request(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) }
  if (token.value) headers.Authorization = `Bearer ${token.value}`
  const res = await fetch(`${API}${path}`, { ...options, headers })
  const data = await res.json().catch(() => null)
  if (!res.ok) throw new Error(data?.error || 'Request failed')
  return data
}

async function run(action, success = '') {
  loading.value = true
  try {
    const result = await action()
    if (success) showToast(success, 'success')
    return result
  } catch (err) {
    showToast(err.message, 'error')
    return null
  } finally {
    loading.value = false
  }
}

function showToast(text, type = 'success') {
  toast.text = text
  toast.type = type
  if (toast.timer) window.clearTimeout(toast.timer)
  toast.timer = window.setTimeout(() => {
    toast.text = ''
    toast.timer = 0
  }, 3000)
}

async function login() {
  await run(async () => {
    const data = await request('/api/v1/login', {
      method: 'POST',
      body: JSON.stringify({ email: authForm.email, password: authForm.password })
    })
    saveSession(data)
    await bootstrap()
    await router.push({ name: 'tasks' })
  })
}

async function requestCode() {
  await run(async () => {
    await request('/api/v1/register/request-code', {
      method: 'POST',
      body: JSON.stringify({ email: authForm.email, password: authForm.password, name: authForm.name })
    })
    codeSent.value = true
  }, 'Код подтверждения отправлен')
}

async function verifyCode() {
  await run(async () => {
    const data = await request('/api/v1/register/verify', {
      method: 'POST',
      body: JSON.stringify({ email: authForm.email, code: authForm.code })
    })
    saveSession(data)
    await bootstrap()
    await router.push({ name: 'tasks' })
  })
}

function saveSession(data) {
  token.value = data.token
  user.value = data.user
  localStorage.setItem('token', data.token)
  localStorage.setItem('user', JSON.stringify(data.user))
}

function logout() {
  token.value = ''
  user.value = null
  localStorage.clear()
  teams.value = []
  workers.value = []
  tasks.value = []
  router.push({ name: 'login' })
}

async function bootstrap() {
  await loadTeam()
  if (team.value) await Promise.all([loadWorkers(), loadTasks()])
}

async function loadTeam() {
  teams.value = await request('/api/v1/teams')
}

async function saveTeam() {
  await run(async () => {
    const created = await request('/api/v1/teams', { method: 'POST', body: JSON.stringify({ name: teamForm.name }) })
    teams.value = [created]
    teamModalOpen.value = false
    await Promise.all([loadWorkers(), loadTasks()])
  }, 'Команда сохранена')
}

function openTeamModal() {
  teamForm.name = team.value?.name || ''
  teamModalOpen.value = true
}

async function loadWorkers() {
  if (!team.value) return
  workers.value = await request(`/api/v1/teams/${team.value.id}/workers`)
}

async function saveWorker() {
  await run(async () => {
    const body = JSON.stringify({ name: workerForm.name, email: workerForm.email, skills: parseSkills(workerForm.skillsText) })
    if (workerForm.id) {
      await request(`/api/v1/teams/${team.value.id}/workers/${workerForm.id}`, { method: 'PUT', body })
    } else {
      await request(`/api/v1/teams/${team.value.id}/workers`, { method: 'POST', body })
    }
    Object.assign(workerForm, { id: '', name: '', email: '', skillsText: 'go' })
    workerModalOpen.value = false
    await Promise.all([loadWorkers(), loadTasks()])
  }, 'Работник сохранен')
}

function createWorker() {
  Object.assign(workerForm, { id: '', name: '', email: '', skillsText: 'go' })
  workerModalOpen.value = true
}

function editWorker(worker) {
  Object.assign(workerForm, { id: worker.id, name: worker.name, email: worker.email, skillsText: skillsText(worker.skills) })
  workerModalOpen.value = true
}

async function deleteWorker(worker) {
  await run(async () => {
    await request(`/api/v1/teams/${team.value.id}/workers/${worker.id}`, { method: 'DELETE' })
    await Promise.all([loadWorkers(), loadTasks()])
  }, 'Работник удален')
}

async function loadTasks() {
  if (!team.value) return
  const params = new URLSearchParams({ team_id: team.value.id, limit: filters.limit, offset: filters.offset })
  if (filters.status) params.set('status', filters.status)
  tasks.value = await request(`/api/v1/tasks?${params}`)
}

async function saveTask() {
  await run(async () => {
    const body = JSON.stringify({
      team_id: team.value.id,
      title: taskForm.title,
      description: taskForm.description,
      status: taskForm.status,
      skills: parseSkills(taskForm.skillsText),
      assignee_id: taskForm.assignee_id ? Number(taskForm.assignee_id) : null
    })
    if (taskForm.id) {
      await request(`/api/v1/tasks/${taskForm.id}`, { method: 'PUT', body })
    } else {
      await request('/api/v1/tasks', { method: 'POST', body })
    }
    Object.assign(taskForm, { id: '', title: '', description: '', status: 'backlog', skillsText: 'go', assignee_id: '' })
    taskModalOpen.value = false
    await loadTasks()
  }, 'Задача сохранена')
}

function createTask() {
  Object.assign(taskForm, { id: '', title: '', description: '', status: 'backlog', skillsText: 'go', assignee_id: '' })
  taskModalOpen.value = true
}

function editTask(task) {
  Object.assign(taskForm, {
    id: task.id,
    title: task.title,
    description: task.description,
    status: task.status,
    skillsText: skillsText(task.skills),
    assignee_id: task.assignee_id || ''
  })
  selectedTaskId.value = String(task.id)
  taskModalOpen.value = true
  loadTaskActivity()
}

function selectTask(task) {
  selectedTaskId.value = String(task.id)
  loadTaskActivity()
}

async function deleteTask(task) {
  await run(async () => {
    await request(`/api/v1/tasks/${task.id}`, { method: 'DELETE' })
    if (String(selectedTaskId.value) === String(task.id)) selectedTaskId.value = ''
    await loadTasks()
  }, 'Задача удалена')
}

async function autoAssign() {
  await run(async () => {
    const result = await request(`/api/v1/teams/${team.value.id}/auto-assign`, { method: 'POST' })
    await loadTasks()
    showToast(`Назначено задач: ${result.assigned}`)
  })
}

async function loadTaskActivity() {
  if (!selectedTaskId.value) return
  const [taskHistory, taskComments] = await Promise.all([
    request(`/api/v1/tasks/${selectedTaskId.value}/history`),
    request(`/api/v1/tasks/${selectedTaskId.value}/comments`)
  ])
  history.value = taskHistory
  comments.value = taskComments
}

async function addComment() {
  await run(async () => {
    await request(`/api/v1/tasks/${selectedTaskId.value}/comments`, {
      method: 'POST',
      body: JSON.stringify({ body: commentBody.value })
    })
    commentBody.value = ''
    await loadTaskActivity()
  }, 'Комментарий добавлен')
}

if (token.value) run(bootstrap)
</script>

<template>
  <main class="shell">
    <section v-if="!authed" class="auth-screen">
      <div class="auth-panel">
        <div class="auth-brand">
          <img :src="workflowMark" alt="" />
          <div>
            <p class="eyebrow">Task Desk</p>
            <h1>{{ codeSent ? 'Подтверждение email' : 'Рабочее пространство команды' }}</h1>
          </div>
        </div>
        <div v-if="!codeSent" class="tabs">
          <button :class="{ active: authMode === 'login' }" @click="router.push({ name: 'login' })">Вход</button>
          <button :class="{ active: authMode === 'register' }" @click="router.push({ name: 'register' })">Регистрация</button>
        </div>
        <template v-if="!codeSent">
          <input v-model="authForm.email" autocomplete="email" placeholder="email" />
          <input v-if="authMode === 'register'" v-model="authForm.name" autocomplete="name" placeholder="имя" />
          <input v-model="authForm.password" autocomplete="current-password" type="password" placeholder="пароль" />
        </template>
        <input v-else v-model="authForm.code" autocomplete="one-time-code" inputmode="numeric" placeholder="код подтверждения" />
        <button v-if="authMode === 'login'" class="primary" :disabled="loading" @click="login">Войти</button>
        <button v-else-if="!codeSent" class="primary" :disabled="loading" @click="requestCode">Получить код</button>
        <button v-else class="primary" :disabled="loading" @click="verifyCode">Подтвердить</button>
        <div v-if="toast.text" class="toast" :class="toast.type">{{ toast.text }}</div>
      </div>
    </section>

    <template v-else>
      <aside class="sidebar">
        <div class="brand">
          <img :src="workflowMark" alt="" />
          <div>
            <strong>Task Desk</strong>
            <small>team workspace</small>
          </div>
        </div>
        <nav>
          <button v-for="item in navItems" :key="item.id" :class="{ active: page === item.id }" @click="router.push({ name: item.id })">
            <span>{{ item.icon }}</span>
            {{ item.label }}
          </button>
        </nav>
        <div class="team-switcher">
          <label>Команда</label>
          <strong>{{ team?.name || 'Команда не создана' }}</strong>
          <button class="primary subtle" @click="openTeamModal">{{ team ? 'Настроить' : 'Создать' }}</button>
        </div>
        <div class="account">
          <div>
            <strong>{{ user?.name }}</strong>
            <small>{{ user?.email }}</small>
          </div>
          <button class="ghost" @click="logout">Выйти</button>
        </div>
      </aside>

      <section class="workspace">
        <header class="topbar">
          <div>
            <h1>{{ team?.name || 'Создайте команду' }}</h1>
          </div>
        </header>

        <div v-if="toast.text" class="toast" :class="toast.type">{{ toast.text }}</div>

        <section v-if="!team" class="panel">
          <div class="section-head"><h2>Первая команда</h2></div>
          <button class="primary" @click="openTeamModal">Создать команду</button>
        </section>

        <section v-else-if="page === 'workers'" class="two-column">
          <article class="panel wide-panel">
            <div class="section-head">
              <h2>Работники</h2>
              <div class="actions">
                <button @click="loadWorkers">Обновить</button>
                <button class="primary" @click="createWorker">Добавить работника</button>
              </div>
            </div>
            <div class="member-table">
              <div v-for="worker in workers" :key="worker.id" class="member-row">
                <div class="worker-main">
                  <strong>{{ worker.name }}</strong>
                  <small>{{ worker.email }}</small>
                  <div class="skill-list">
                    <span v-for="skill in worker.skills" :key="skill" class="role">{{ skill }}</span>
                  </div>
                </div>
                <div class="actions">
                  <button @click="editWorker(worker)">Редактировать</button>
                  <button @click="deleteWorker(worker)">Удалить</button>
                </div>
              </div>
            </div>
          </article>
        </section>

        <section v-else-if="page === 'tasks'" class="tasks-page">
          <article class="panel wide-panel">
            <div class="section-head">
              <h2>Пул задач</h2>
              <div class="actions">
                <button @click="loadTasks">Обновить</button>
                <button class="primary subtle" :disabled="!team" @click="autoAssign">Распределить задачи</button>
                <button class="primary" @click="createTask">Создать задачу</button>
              </div>
            </div>
            <div class="filters">
              <select v-model="filters.status">
                <option value="">Все статусы</option>
                <option v-for="[value, label] in statuses" :key="value" :value="value">{{ label }}</option>
              </select>
              <input v-model.number="filters.limit" type="number" min="1" max="100" />
              <button @click="loadTasks">Фильтр</button>
            </div>
            <div class="task-board">
              <article v-for="task in tasks" :key="task.id" class="task-card">
                <button class="task-title" @click="selectTask(task)">
                  <strong>{{ task.title }}</strong>
                  <span class="status" :class="task.status">{{ statusLabel(task.status) }}</span>
                </button>
                <p>{{ task.description || 'Без описания' }}</p>
                <div class="meta-line">
                  <span v-for="skill in task.skills" :key="skill" class="role">{{ skill }}</span>
                  <small>{{ task.assignee_name || 'не назначена' }}</small>
                </div>
                <div class="actions">
                  <button @click="editTask(task)">Редактировать</button>
                  <button @click="deleteTask(task)">Удалить</button>
                </div>
              </article>
            </div>
          </article>
        </section>

        <section v-else class="panel wide-panel">
          <div class="section-head">
            <h2>Задачи</h2>
            <button :disabled="!selectedTask" @click="loadTaskActivity">Обновить активность</button>
          </div>
          <div class="selectable-list">
            <button v-for="task in tasks" :key="task.id" :class="{ selected: String(task.id) === String(selectedTaskId) }" @click="selectTask(task)">
              <span>{{ task.title }}</span>
              <small>{{ task.assignee_name || 'без исполнителя' }}</small>
            </button>
          </div>
          <div v-if="selectedTask" class="activity-block">
            <div class="section-head compact">
              <h2>{{ selectedTask.title }}</h2>
              <button @click="editTask(selectedTask)">Редактировать</button>
            </div>
            <form class="comment-form" @submit.prevent="addComment">
              <textarea v-model="commentBody" placeholder="Комментарий"></textarea>
              <button class="primary">Отправить</button>
            </form>
            <div class="timeline">
              <div v-for="comment in comments" :key="`c-${comment.id}`" class="timeline-item">
                <strong>{{ comment.user_name }}</strong>
                <p>{{ comment.body }}</p>
              </div>
              <div v-for="item in history" :key="`h-${item.id}`" class="timeline-item muted">
                <strong>{{ item.field }}</strong>
                <p>{{ item.old_value || 'empty' }} -> {{ item.new_value || 'empty' }}</p>
              </div>
            </div>
          </div>
          <p v-else class="empty">Выберите задачу из списка выше.</p>
        </section>
      </section>

      <div v-if="workerModalOpen" class="modal-backdrop" @click.self="workerModalOpen = false">
        <section class="modal">
          <div class="section-head">
            <h2>{{ workerForm.id ? 'Редактировать работника' : 'Новый работник' }}</h2>
            <button @click="workerModalOpen = false">Закрыть</button>
          </div>
          <form class="stack" @submit.prevent="saveWorker">
            <input v-model="workerForm.name" placeholder="Имя" />
            <input v-model="workerForm.email" placeholder="email" />
            <input v-model="workerForm.skillsText" placeholder="skills через запятую: go, postgres, redis" />
            <button class="primary">{{ workerForm.id ? 'Сохранить' : 'Добавить' }}</button>
          </form>
        </section>
      </div>

      <div v-if="teamModalOpen" class="modal-backdrop" @click.self="teamModalOpen = false">
        <section class="modal">
          <div class="section-head">
            <h2>{{ team ? 'Настройки команды' : 'Новая команда' }}</h2>
            <button @click="teamModalOpen = false">Закрыть</button>
          </div>
          <form class="stack" @submit.prevent="saveTeam">
            <input v-model="teamForm.name" placeholder="Название команды" />
            <button class="primary">{{ team ? 'Сохранить' : 'Создать' }}</button>
          </form>
        </section>
      </div>

      <div v-if="taskModalOpen" class="modal-backdrop" @click.self="taskModalOpen = false">
        <section class="modal">
          <div class="section-head">
            <h2>{{ taskForm.id ? 'Редактировать задачу' : 'Новая задача' }}</h2>
            <button @click="taskModalOpen = false">Закрыть</button>
          </div>
          <form class="task-form" @submit.prevent="saveTask">
            <input v-model="taskForm.title" placeholder="Название" />
            <textarea v-model="taskForm.description" placeholder="Описание"></textarea>
            <select v-model="taskForm.status">
              <option v-for="[value, label] in statuses" :key="value" :value="value">{{ label }}</option>
            </select>
            <input v-model="taskForm.skillsText" placeholder="skills задачи: go, postgres" />
            <select v-model="taskForm.assignee_id">
              <option value="">Без назначения</option>
              <option v-for="worker in workers" :key="worker.id" :value="worker.id">{{ worker.name }} · {{ skillsText(worker.skills) }}</option>
            </select>
            <button class="primary">{{ taskForm.id ? 'Сохранить' : 'Создать' }}</button>
          </form>
        </section>
      </div>
    </template>
  </main>
</template>
