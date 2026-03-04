<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">技能管理</span>
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
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="instruction" label="指令" min-width="200" show-overflow-tooltip />
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

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑技能' : '新增技能'" width="640px" destroy-on-close>
      <el-form :model="form" label-width="100px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="技能名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="指令">
          <el-input v-model="form.instruction" type="textarea" :rows="6" placeholder="技能指令，会注入到 Agent 的 System Prompt 中" />
        </el-form-item>
        <el-form-item label="关联工具">
          <el-select v-model="form.tool_ids" multiple placeholder="选择工具" style="width: 100%">
            <el-option v-for="t in allTools" :key="t.id" :label="t.name" :value="t.id" />
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
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { skillApi, type Skill } from '../../api/skill'
import { toolApi, type Tool } from '../../api/tool'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const list = ref<Skill[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')

const dialogVisible = ref(false)
const submitting = ref(false)
const form = ref<any>({})
const allTools = ref<Tool[]>([])

async function loadData() {
  loading.value = true
  try {
    const res: any = await skillApi.list({ page: page.value, page_size: pageSize.value, keyword: keyword.value })
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } finally {
    loading.value = false
  }
}

async function openDialog(row?: Skill) {
  const toolRes: any = await toolApi.list({ page: 1, page_size: 100 })
  allTools.value = toolRes.data?.list || []

  if (row) {
    const res: any = await skillApi.get(row.id)
    const detail = res.data
    form.value = {
      ...detail,
      tool_ids: detail.tools?.map((t: any) => t.id) || [],
    }
  } else {
    form.value = { name: '', description: '', instruction: '', tool_ids: [] }
  }
  dialogVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (form.value.id) {
      await skillApi.update(form.value.id, form.value)
      ElMessage.success('更新成功')
    } else {
      await skillApi.create(form.value)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadData()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  await skillApi.delete(id)
  ElMessage.success('删除成功')
  loadData()
}

onMounted(loadData)
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: 16px; font-weight: 600; }
</style>
