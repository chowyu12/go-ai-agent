<template>
  <div class="setup-page">
    <div class="setup-card">
      <div class="setup-header">
        <el-icon :size="48" color="#409eff"><Cpu /></el-icon>
        <h1>AI Agent 管理平台</h1>
        <p class="setup-subtitle">首次使用，请创建超级管理员账号</p>
      </div>

      <div class="setup-steps">
        <el-steps :active="1" align-center finish-status="success" class="step-bar">
          <el-step title="配置数据库" />
          <el-step title="创建超管" />
          <el-step title="开始使用" />
        </el-steps>
      </div>

      <el-form ref="formRef" :model="form" :rules="rules" label-width="0" class="setup-form">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="管理员用户名" :prefix-icon="User" size="large" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="设置密码" :prefix-icon="Lock" size="large" show-password />
        </el-form-item>
        <el-form-item prop="confirmPassword">
          <el-input v-model="form.confirmPassword" type="password" placeholder="确认密码" :prefix-icon="Lock" size="large" show-password @keyup.enter="handleSetup" />
        </el-form-item>
        <el-button type="primary" size="large" :loading="loading" class="setup-btn" @click="handleSetup">
          创建并开始使用
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { User, Lock } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { resetSetupStatus } from '@/router'

const router = useRouter()
const authStore = useAuthStore()
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({ username: '', password: '', confirmPassword: '' })

const rules: FormRules = {
  username: [
    { required: true, message: '请输入管理员用户名', trigger: 'blur' },
    { min: 3, max: 32, message: '用户名长度 3-32 个字符', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请设置密码', trigger: 'blur' },
    { min: 6, max: 64, message: '密码长度 6-64 个字符', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    {
      validator: (_rule: unknown, value: string, callback: (err?: Error) => void) => {
        if (value !== form.password) {
          callback(new Error('两次密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
}

async function handleSetup() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res: any = await authApi.setup({ username: form.username, password: form.password })
    authStore.setAuth(res.data.token, res.data.user)
    resetSetupStatus()
    ElMessage.success('超级管理员创建成功，欢迎使用!')
    router.push('/')
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.setup-page {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1d1e2c 0%, #2b2d42 50%, #1d1e2c 100%);
}
.setup-card {
  width: 440px;
  padding: 44px 40px 36px;
  background: #fff;
  border-radius: 14px;
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.3);
}
.setup-header {
  text-align: center;
  margin-bottom: 24px;
}
.setup-header h1 {
  margin: 14px 0 0;
  font-size: 24px;
  font-weight: 700;
  color: #1d1e2c;
}
.setup-subtitle {
  margin-top: 8px;
  color: #999;
  font-size: 14px;
}
.setup-steps {
  margin-bottom: 28px;
}
.step-bar {
  --el-color-primary: #409eff;
}
.setup-form {
  margin-top: 4px;
}
.setup-btn {
  width: 100%;
  margin-top: 8px;
  font-size: 16px;
}
</style>
