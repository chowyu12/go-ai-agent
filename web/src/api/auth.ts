import request from './request'

export interface LoginReq {
  username: string
  password: string
}

export interface LoginResp {
  token: string
  user: { id: number; username: string; role: 'admin' | 'guest' }
}

export const authApi = {
  setupCheck: () => request.get('/auth/setup-check'),
  setup: (data: LoginReq) => request.post('/auth/setup', data),
  login: (data: LoginReq) => request.post('/auth/login', data),
  me: () => request.get('/auth/me'),
}
