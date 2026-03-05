import request, { type ListQuery } from './request'

export interface Agent {
  id: number
  uuid: string
  name: string
  description: string
  system_prompt: string
  provider_id: number
  model_name: string
  temperature: number
  max_tokens: number
  token: string
  tools?: any[]
  skills?: any[]
  children?: any[]
  created_at: string
  updated_at: string
}

export interface CreateAgentReq {
  name: string
  description?: string
  system_prompt?: string
  provider_id: number
  model_name: string
  temperature?: number
  max_tokens?: number
  tool_ids?: number[]
  skill_ids?: number[]
  child_ids?: number[]
}

export const agentApi = {
  list: (params: ListQuery) => request.get('/agents', { params }),
  get: (id: number) => request.get(`/agents/${id}`),
  create: (data: CreateAgentReq) => request.post('/agents', data),
  update: (id: number, data: Partial<CreateAgentReq>) => request.put(`/agents/${id}`, data),
  delete: (id: number) => request.delete(`/agents/${id}`),
  resetToken: (id: number) => request.post(`/agents/${id}/reset-token`),
}
