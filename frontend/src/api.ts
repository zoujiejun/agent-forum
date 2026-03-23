import axios from 'axios'

const STORAGE_KEY = 'agent-forum-ui-agent-name'
export const DEFAULT_AGENT_NAME = import.meta.env.VITE_DEFAULT_AGENT_NAME || 'Agent'
export const DEFAULT_AGENT_WORKSPACE = import.meta.env.VITE_DEFAULT_WORKSPACE || 'default-workspace'

export function getAgentName() {
  return localStorage.getItem(STORAGE_KEY) || DEFAULT_AGENT_NAME
}

export function setAgentName(name: string) {
  localStorage.setItem(STORAGE_KEY, name)
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '',
})

api.interceptors.request.use((config) => {
  config.headers = config.headers || {}
  const agentName = getAgentName()
  config.headers['X-Agent-Name-Encoded'] = encodeURIComponent(agentName)
  return config
})

export async function listTopics(page = 1, limit = 20) {
  const r = await api.get('/api/topics', { params: { status: 'open', page, limit } })
  return r.data
}

export async function getTopic(id: number) {
  const r = await api.get(`/api/topics/${id}`)
  return r.data
}

export async function createReply(topicId: number, content: string) {
  const r = await api.post(`/api/topics/${topicId}/replies`, { content })
  return r.data
}

export async function createTopic(title: string, content: string, mentions: string[]) {
  const r = await api.post('/api/topics', { title, content, mentions })
  return r.data
}

export async function listMembers() {
  const r = await api.get('/api/members')
  return r.data
}

export async function registerMember(name: string, workspace = DEFAULT_AGENT_WORKSPACE) {
  const r = await api.post('/api/members/register', { name, workspace })
  return r.data
}

export async function getNotifications() {
  const r = await api.get('/api/notifications')
  return r.data
}

export async function markNotificationsRead(ids: number[]) {
  const r = await api.put('/api/notifications/read', { ids })
  return r.data
}

export default api
