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

export type TopicListResult = {
  topics: any[]
  total: number
  page: number
  limit: number
}

export async function listTopics(page = 1, limit = 20, status: 'all' | 'open' | 'closed' = 'all', tag?: string): Promise<TopicListResult> {
  const params: Record<string, string | number> = { page, limit }
  if (status !== 'all') {
    params.status = status
  }
  if (tag) {
    params.tag = tag
  }

  const r = await api.get('/api/topics', { params })
  const data = r.data

  if (Array.isArray(data)) {
    return {
      topics: data,
      total: data.length,
      page,
      limit,
    }
  }

  return {
    topics: Array.isArray(data?.topics) ? data.topics : [],
    total: typeof data?.total === 'number' ? data.total : 0,
    page: typeof data?.page === 'number' ? data.page : page,
    limit: typeof data?.limit === 'number' ? data.limit : limit,
  }
}

export async function getTopic(id: number) {
  const r = await api.get(`/api/topics/${id}`)
  return r.data
}

export async function closeTopic(id: number) {
  const r = await api.put(`/api/topics/${id}/close`, {})
  return r.data
}

export async function createReply(topicId: number, content: string) {
  const r = await api.post(`/api/topics/${topicId}/replies`, { content })
  return r.data
}

export async function createTopic(title: string, content: string, mentions: string[], tags: string[] = []) {
  const r = await api.post('/api/topics', { title, content, mentions, tags })
  return r.data
}

export async function getTopicTags(topicId: number) {
  const r = await api.get(`/api/topics/${topicId}/tags`)
  return r.data
}

export async function addTopicTags(topicId: number, tags: string[]) {
  const r = await api.post(`/api/topics/${topicId}/tags`, { tags })
  return r.data
}

export async function setTopicTags(topicId: number, tags: string[]) {
  const r = await api.put(`/api/topics/${topicId}/tags`, { tags })
  return r.data
}

export async function removeTopicTag(topicId: number, tag: string) {
  const r = await api.delete(`/api/topics/${topicId}/tags/${encodeURIComponent(tag)}`)
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
