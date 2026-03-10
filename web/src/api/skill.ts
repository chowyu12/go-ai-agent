import request, { type ListQuery } from './request'

export interface Skill {
  id: number
  uuid: string
  name: string
  description: string
  instruction: string
  source: string
  slug: string
  version: string
  author: string
  dir_name: string
  main_file: string
  config: any
  permissions: any
  tool_defs: any
  enabled: boolean
  tools?: any[]
  created_at: string
  updated_at: string
}

export interface CreateSkillReq {
  name: string
  description?: string
  instruction?: string
  source?: string
  slug?: string
  version?: string
  author?: string
  dir_name?: string
  main_file?: string
  config?: any
  permissions?: any
  tool_defs?: any
  enabled?: boolean
  tool_ids?: number[]
}

export const skillApi = {
  list: (params: ListQuery) => request.get('/skills', { params }),
  get: (id: number) => request.get(`/skills/${id}`),
  create: (data: CreateSkillReq) => request.post('/skills', data),
  update: (id: number, data: Partial<CreateSkillReq>) => request.put(`/skills/${id}`, data),
  delete: (id: number) => request.delete(`/skills/${id}`),
  install: (slug: string) => request.post('/skills/install', { slug }),
  sync: () => request.post('/skills/sync'),
}
