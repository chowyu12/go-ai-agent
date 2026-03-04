<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">Agent 管理</span>
          <div>
            <el-input v-model="keyword" placeholder="搜索" clearable style="width: 200px; margin-right: 12px;" @clear="loadData" @keyup.enter="loadData">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-button v-if="authStore.isAdmin" type="primary" @click="openDialog()">
              <el-icon><Plus /></el-icon> 新增
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="uuid" label="UUID" width="140" show-overflow-tooltip />
        <el-table-column prop="model_name" label="模型" width="160" />
        <el-table-column prop="temperature" label="温度" width="80" />
        <el-table-column prop="max_tokens" label="Max Tokens" width="110" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column v-if="authStore.isAdmin" label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-popconfirm title="确定删除？" @confirm="handleDelete(row.id)">
              <template #reference>
                <el-button link type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="page" v-model:page-size="pageSize"
        :total="total" :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next" style="margin-top: 16px; justify-content: flex-end;"
        @size-change="loadData" @current-change="loadData"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑 Agent' : '新增 Agent'" width="720px" destroy-on-close>
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="Agent 名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="模型供应商" required>
          <el-select
            v-model="form.provider_id"
            placeholder="选择供应商"
            filterable
            style="width: 100%"
            @change="onProviderChange"
          >
            <el-option v-for="p in providers" :key="p.id" :label="p.name" :value="p.id">
              <span>{{ p.name }}</span>
              <el-tag size="small" style="margin-left: 8px;" type="info">{{ p.type }}</el-tag>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="模型名称" required>
          <el-select
            v-model="form.model_name"
            placeholder="先选择供应商，再选择或输入模型"
            filterable
            allow-create
            default-first-option
            style="width: 100%"
            :loading="modelLoading"
            :disabled="!form.provider_id"
            @focus="onModelFocus"
          >
            <el-option-group v-if="remoteModels.length > 0" label="远程模型 (API)">
              <el-option v-for="m in remoteModels" :key="'r-' + m" :label="m" :value="m" />
            </el-option-group>
            <el-option-group v-if="localOnlyModels.length > 0" :label="remoteModels.length > 0 ? '本地配置' : '模型列表'">
              <el-option v-for="m in localOnlyModels" :key="'l-' + m" :label="m" :value="m" />
            </el-option-group>
          </el-select>
          <div v-if="remoteFetchError" style="font-size: 12px; color: #E6A23C; margin-top: 2px;">
            {{ remoteFetchError }}
          </div>
        </el-form-item>
        <el-form-item label="System Prompt">
          <el-input v-model="form.system_prompt" type="textarea" :rows="4" placeholder="系统提示词" />
        </el-form-item>
        <el-form-item label="温度">
          <el-slider v-model="form.temperature" :min="0" :max="2" :step="0.1" show-input />
        </el-form-item>
        <el-form-item label="Max Tokens">
          <el-input-number v-model="form.max_tokens" :min="1" :max="128000" />
        </el-form-item>

        <el-divider content-position="left">关联配置</el-divider>

        <el-form-item label="关联工具">
          <el-select
            v-model="form.tool_ids"
            multiple filterable
            collapse-tags collapse-tags-tooltip
            :max-collapse-tags="3"
            placeholder="搜索并选择工具"
            style="width: 100%"
          >
            <el-option v-for="t in allTools" :key="t.id" :label="t.name" :value="t.id">
              <div style="display: flex; justify-content: space-between; align-items: center;">
                <span>{{ t.name }}</span>
                <span style="color: #909399; font-size: 12px; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ t.description }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="关联技能">
          <el-select
            v-model="form.skill_ids"
            multiple filterable
            collapse-tags collapse-tags-tooltip
            :max-collapse-tags="3"
            placeholder="搜索并选择技能"
            style="width: 100%"
          >
            <el-option v-for="s in allSkills" :key="s.id" :label="s.name" :value="s.id">
              <div style="display: flex; justify-content: space-between; align-items: center;">
                <span>{{ s.name }}</span>
                <span style="color: #909399; font-size: 12px; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ s.description }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="子 Agent">
          <el-select
            v-model="form.child_ids"
            multiple filterable
            collapse-tags collapse-tags-tooltip
            :max-collapse-tags="3"
            placeholder="搜索并选择子 Agent"
            style="width: 100%"
          >
            <el-option
              v-for="a in availableAgents"
              :key="a.id" :label="a.name" :value="a.id"
            >
              <div style="display: flex; justify-content: space-between; align-items: center;">
                <span>{{ a.name }}</span>
                <span style="color: #909399; font-size: 12px;">{{ a.model_name }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { agentApi, type Agent } from '../../api/agent'
import { providerApi, type Provider } from '../../api/provider'
import { toolApi, type Tool } from '../../api/tool'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
import { skillApi, type Skill } from '../../api/skill'

const list = ref<Agent[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')

const dialogVisible = ref(false)
const submitting = ref(false)
const form = ref<any>({})

const providers = ref<Provider[]>([])
const allTools = ref<Tool[]>([])
const allSkills = ref<Skill[]>([])
const allAgents = ref<Agent[]>([])

const providerModels = ref<string[]>([])
const remoteModels = ref<string[]>([])
const remoteFetched = ref(false)
const remoteFetchError = ref('')
const modelLoading = ref(false)

const localOnlyModels = computed(() => {
  const remoteSet = new Set(remoteModels.value)
  return providerModels.value.filter(m => !remoteSet.has(m))
})

const availableAgents = computed(() =>
  allAgents.value.filter(a => a.id !== form.value.id)
)

async function loadData() {
  loading.value = true
  try {
    const res: any = await agentApi.list({ page: page.value, page_size: pageSize.value, keyword: keyword.value })
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } finally {
    loading.value = false
  }
}

async function loadOptions() {
  const [p, t, s, a] = await Promise.all([
    providerApi.list({ page: 1, page_size: 100 }),
    toolApi.list({ page: 1, page_size: 100 }),
    skillApi.list({ page: 1, page_size: 100 }),
    agentApi.list({ page: 1, page_size: 100 }),
  ])
  providers.value = (p as any).data?.list || []
  allTools.value = (t as any).data?.list || []
  allSkills.value = (s as any).data?.list || []
  allAgents.value = (a as any).data?.list || []
}

async function loadProviderModels(providerId: number) {
  if (!providerId) {
    providerModels.value = []
    return
  }
  modelLoading.value = true
  try {
    const res: any = await providerApi.models(providerId)
    providerModels.value = res.data || []
  } catch {
    providerModels.value = []
  } finally {
    modelLoading.value = false
  }
}

async function onProviderChange(providerId: number) {
  form.value.model_name = ''
  remoteModels.value = []
  remoteFetched.value = false
  remoteFetchError.value = ''
  await loadProviderModels(providerId)
}

async function onModelFocus() {
  if (!form.value.provider_id || remoteFetched.value || modelLoading.value) return
  modelLoading.value = true
  remoteFetchError.value = ''
  try {
    const res: any = await providerApi.remoteModels(form.value.provider_id)
    remoteModels.value = res.data || []
    remoteFetched.value = true
  } catch (e: any) {
    remoteFetchError.value = '远程模型拉取失败，可手动输入模型名称'
    remoteFetched.value = true
  } finally {
    modelLoading.value = false
  }
}

async function openDialog(row?: Agent) {
  await loadOptions()
  if (row) {
    const res: any = await agentApi.get(row.id)
    const detail = res.data
    form.value = {
      ...detail,
      tool_ids: detail.tools?.map((t: any) => t.id) || [],
      skill_ids: detail.skills?.map((s: any) => s.id) || [],
      child_ids: detail.children?.map((a: any) => a.id) || [],
    }
    if (detail.provider_id) {
      await loadProviderModels(detail.provider_id)
    }
  } else {
    form.value = {
      name: '', description: '', system_prompt: '',
      provider_id: null, model_name: '',
      temperature: 0.7, max_tokens: 2048,
      tool_ids: [], skill_ids: [], child_ids: [],
    }
    providerModels.value = []
  }
  remoteModels.value = []
  remoteFetched.value = false
  remoteFetchError.value = ''
  dialogVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (form.value.id) {
      await agentApi.update(form.value.id, form.value)
      ElMessage.success('更新成功')
    } else {
      await agentApi.create(form.value)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadData()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  await agentApi.delete(id)
  ElMessage.success('删除成功')
  loadData()
}

onMounted(loadData)
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: 16px; font-weight: 600; }
</style>
