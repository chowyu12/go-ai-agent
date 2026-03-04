import request, { type ListQuery } from './request'

export interface ChatRequest {
  agent_id: string
  conversation_id?: string
  user_id?: string
  message: string
  stream?: boolean
}

export interface ExecutionStep {
  id: number
  message_id: number
  conversation_id: number
  step_order: number
  step_type: 'llm_call' | 'tool_call' | 'agent_call' | 'skill_match'
  name: string
  input: string
  output: string
  status: 'success' | 'error' | 'pending'
  error?: string
  duration_ms: number
  tokens_used: number
  metadata?: {
    provider?: string
    model?: string
    temperature?: number
    tool_name?: string
    skill_name?: string
    skill_tools?: string[]
    agent_uuid?: string
    agent_name?: string
  }
  created_at: string
  _expanded?: boolean
}

export interface ChatResponse {
  conversation_id: string
  message: string
  tokens_used: number
  steps?: ExecutionStep[]
}

export interface StreamChunk {
  conversation_id?: string
  delta?: string
  done: boolean
  step?: ExecutionStep
  steps?: ExecutionStep[]
}

export interface Conversation {
  id: number
  uuid: string
  agent_id: number
  user_id: string
  title: string
  created_at: string
  updated_at: string
}

export interface Message {
  id: number
  conversation_id: number
  role: string
  content: string
  steps?: ExecutionStep[]
  created_at: string
}

export const chatApi = {
  complete: (data: ChatRequest) => request.post('/chat/completions', data),
  conversations: (params: ListQuery & { agent_id?: number; user_id?: string }) =>
    request.get('/conversations', { params }),
  messages: (id: number, limit?: number, withSteps?: boolean) =>
    request.get(`/conversations/${id}/messages`, { params: { limit, with_steps: withSteps ? 'true' : undefined } }),
  messageSteps: (messageId: number) => request.get(`/messages/${messageId}/steps`),
  conversationSteps: (convId: number) => request.get(`/conversations/${convId}/steps`),
  deleteConversation: (id: number) => request.delete(`/conversations/${id}`),
}

export function streamChat(
  data: ChatRequest,
  onChunk: (chunk: StreamChunk) => void,
  onDone: () => void,
  onError: (err: string) => void,
) {
  const controller = new AbortController()

  const token = localStorage.getItem('token') || ''
  fetch('/api/v1/chat/stream', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify(data),
    signal: controller.signal,
  }).then(async (response) => {
    if (!response.ok) {
      onError(`HTTP ${response.status}`)
      return
    }
    const reader = response.body?.getReader()
    if (!reader) {
      onError('No reader')
      return
    }
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const payload = line.slice(6).trim()
          if (payload === '[DONE]') {
            onDone()
            return
          }
          try {
            const chunk: StreamChunk = JSON.parse(payload)
            onChunk(chunk)
          } catch {
            // skip invalid JSON
          }
        }
      }
    }
    onDone()
  }).catch((err) => {
    if (err.name !== 'AbortError') {
      onError(err.message)
    }
  })

  return controller
}
