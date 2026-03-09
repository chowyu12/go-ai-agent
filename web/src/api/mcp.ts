import request, { type ListQuery } from './request'

export interface McpServer {
  id: number
  uuid: string
  name: string
  description: string
  transport: 'stdio' | 'sse'
  endpoint: string
  args: string[] | null
  env: Record<string, string> | null
  headers: Record<string, string> | null
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateMcpServerReq {
  name: string
  description?: string
  transport: 'stdio' | 'sse'
  endpoint: string
  args?: string[]
  env?: Record<string, string>
  headers?: Record<string, string>
  enabled?: boolean
}

export const mcpApi = {
  list: (params: ListQuery) => request.get('/mcp-servers', { params }),
  get: (id: number) => request.get(`/mcp-servers/${id}`),
  create: (data: CreateMcpServerReq) => request.post('/mcp-servers', data),
  update: (id: number, data: Partial<CreateMcpServerReq>) => request.put(`/mcp-servers/${id}`, data),
  delete: (id: number) => request.delete(`/mcp-servers/${id}`),
}
