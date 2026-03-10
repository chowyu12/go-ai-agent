<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">{{ isEdit ? '编辑技能' : '新增技能' }}</span>
          <el-button @click="$router.back()">返回</el-button>
        </div>
      </template>

      <el-form :model="form" label-width="120px" style="max-width: 800px;" v-loading="loadingDetail">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="技能名称" />
        </el-form-item>
        <el-form-item label="目录名">
          <el-input v-model="form.dir_name" placeholder="workspace/skills/ 下的目录名，如 brave-web-search" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="作者">
          <el-input v-model="form.author" placeholder="技能作者" style="width: 240px;" />
        </el-form-item>
        <el-form-item label="版本">
          <el-input v-model="form.version" placeholder="如 1.0.0" style="width: 160px;" />
        </el-form-item>
        <el-form-item label="来源">
          <el-select v-model="form.source" style="width: 160px;">
            <el-option label="自定义" value="custom" />
            <el-option label="本地" value="local" />
            <el-option label="ClawHub" value="clawhub" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.source === 'clawhub'" label="Slug">
          <el-input v-model="form.slug" placeholder="ClawHub 技能标识，如 himalaya" />
        </el-form-item>
        <el-form-item label="入口文件">
          <el-input v-model="form.main_file" placeholder="如 index.js 或 index.py（留空表示纯指令技能）" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <el-divider content-position="left">技能指令 (SKILL.md)</el-divider>
        <el-form-item label="指令内容">
          <el-input v-model="form.instruction" type="textarea" :rows="10" placeholder="技能指令内容，支持 Markdown 格式，会注入到 Agent 的 System Prompt 中" />
        </el-form-item>

        <el-divider content-position="left">工具定义 (manifest.json tools)</el-divider>
        <el-form-item label="工具定义 JSON">
          <el-input v-model="toolDefsStr" type="textarea" :rows="8" placeholder='[{"name":"tool_name","description":"...","parameters":{...}}]' />
        </el-form-item>

        <el-divider content-position="left">关联已有工具</el-divider>
        <el-form-item label="关联工具">
          <el-select v-model="form.tool_ids" multiple placeholder="选择工具" style="width: 100%">
            <el-option v-for="t in allTools" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">{{ isEdit ? '保存' : '创建' }}</el-button>
          <el-button @click="$router.back()">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { skillApi } from '../../api/skill'
import { toolApi, type Tool } from '../../api/tool'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => !!route.params.id)
const loadingDetail = ref(false)
const submitting = ref(false)
const allTools = ref<Tool[]>([])
const toolDefsStr = ref('')

const form = ref<any>({
  name: '',
  description: '',
  instruction: '',
  source: 'custom',
  slug: '',
  version: '',
  author: '',
  dir_name: '',
  main_file: '',
  enabled: true,
  tool_ids: [],
})

async function loadOptions() {
  const toolRes: any = await toolApi.list({ page: 1, page_size: 200 })
  allTools.value = toolRes.data?.list || []
}

async function loadSkill() {
  if (!isEdit.value) return
  loadingDetail.value = true
  try {
    const res: any = await skillApi.get(Number(route.params.id))
    const d = res.data
    form.value = {
      name: d.name || '',
      description: d.description || '',
      instruction: d.instruction || '',
      source: d.source || 'custom',
      slug: d.slug || '',
      version: d.version || '',
      author: d.author || '',
      dir_name: d.dir_name || '',
      main_file: d.main_file || '',
      enabled: d.enabled ?? true,
      tool_ids: d.tools?.map((t: any) => t.id) || [],
    }
    if (d.tool_defs) {
      try {
        const parsed = typeof d.tool_defs === 'string' ? JSON.parse(d.tool_defs) : d.tool_defs
        toolDefsStr.value = JSON.stringify(parsed, null, 2)
      } catch {
        toolDefsStr.value = typeof d.tool_defs === 'string' ? d.tool_defs : JSON.stringify(d.tool_defs)
      }
    }
  } finally {
    loadingDetail.value = false
  }
}

async function handleSubmit() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入技能名称')
    return
  }
  submitting.value = true
  try {
    const payload: any = { ...form.value }
    if (toolDefsStr.value.trim()) {
      try {
        payload.tool_defs = JSON.parse(toolDefsStr.value.trim())
      } catch {
        ElMessage.error('工具定义 JSON 格式不正确')
        submitting.value = false
        return
      }
    }
    if (isEdit.value) {
      await skillApi.update(Number(route.params.id), payload)
      ElMessage.success('更新成功')
    } else {
      await skillApi.create(payload)
      ElMessage.success('创建成功')
    }
    router.push('/skills')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  await loadOptions()
  await loadSkill()
})
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: 16px; font-weight: 600; }
</style>
