import request, { type ListQuery } from './request'

export interface Skill {
  id: number
  uuid: string
  name: string
  description: string
  instruction: string
  tools?: any[]
  created_at: string
  updated_at: string
}

export interface CreateSkillReq {
  name: string
  description?: string
  instruction?: string
  tool_ids?: number[]
}

export const skillApi = {
  list: (params: ListQuery) => request.get('/skills', { params }),
  get: (id: number) => request.get(`/skills/${id}`),
  create: (data: CreateSkillReq) => request.post('/skills', data),
  update: (id: number, data: Partial<CreateSkillReq>) => request.put(`/skills/${id}`, data),
  delete: (id: number) => request.delete(`/skills/${id}`),
}
