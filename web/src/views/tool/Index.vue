<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">工具管理</span>
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
        <el-table-column prop="handler_type" label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.handler_type === 'builtin' ? 'success' : row.handler_type === 'http' ? 'warning' : 'info'" size="small">
              {{ row.handler_type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'danger'" size="small">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
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

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑工具' : '新增工具'" width="640px" destroy-on-close>
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="工具名称（英文标识）" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="工具功能描述" />
        </el-form-item>
        <el-form-item label="处理器类型" required>
          <el-select v-model="form.handler_type" placeholder="选择类型" style="width: 100%">
            <el-option label="内置函数 (builtin)" value="builtin" />
            <el-option label="HTTP 回调 (http)" value="http" />
            <el-option label="脚本 (script)" value="script" />
          </el-select>
        </el-form-item>
        <template v-if="form.handler_type === 'http'">
          <el-form-item label="请求 URL">
            <el-input v-model="httpConfig.url" placeholder="https://api.example.com/tool" />
          </el-form-item>
          <el-form-item label="请求方法">
            <el-select v-model="httpConfig.method" style="width: 100%">
              <el-option label="POST" value="POST" />
              <el-option label="GET" value="GET" />
            </el-select>
          </el-form-item>
        </template>
        <el-form-item label="Function Def">
          <el-input v-model="form.function_def_str" type="textarea" :rows="6" placeholder="OpenAI Function Calling JSON Schema" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
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
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { toolApi, type Tool } from '../../api/tool'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const list = ref<Tool[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')

const dialogVisible = ref(false)
const submitting = ref(false)
const form = ref<any>({})
const httpConfig = reactive({ url: '', method: 'POST', headers: {} })

async function loadData() {
  loading.value = true
  try {
    const res: any = await toolApi.list({ page: page.value, page_size: pageSize.value, keyword: keyword.value })
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } finally {
    loading.value = false
  }
}

function openDialog(row?: Tool) {
  if (row) {
    form.value = {
      ...row,
      function_def_str: row.function_def ? JSON.stringify(row.function_def, null, 2) : '',
    }
    if (row.handler_type === 'http' && row.handler_config) {
      Object.assign(httpConfig, row.handler_config)
    }
  } else {
    form.value = { name: '', description: '', handler_type: 'builtin', enabled: true, function_def_str: '' }
    Object.assign(httpConfig, { url: '', method: 'POST', headers: {} })
  }
  dialogVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    const data: any = { ...form.value }
    if (data.function_def_str) {
      try { data.function_def = JSON.parse(data.function_def_str) } catch { ElMessage.error('Function Def JSON 格式错误'); submitting.value = false; return }
    }
    delete data.function_def_str
    if (data.handler_type === 'http') {
      data.handler_config = { ...httpConfig }
    }
    if (data.id) {
      await toolApi.update(data.id, data)
      ElMessage.success('更新成功')
    } else {
      await toolApi.create(data)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadData()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  await toolApi.delete(id)
  ElMessage.success('删除成功')
  loadData()
}

onMounted(loadData)
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: 16px; font-weight: 600; }
</style>
