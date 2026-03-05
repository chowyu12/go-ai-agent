<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">Agent 管理</span>
          <div>
            <el-input
              v-model="keyword"
              placeholder="搜索"
              clearable
              style="width: 200px; margin-right: 12px"
              @clear="loadData"
              @keyup.enter="loadData"
            >
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-button v-if="authStore.isAdmin" type="primary" @click="$router.push({ name: 'AgentCreate' })">
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
        <el-table-column prop="timeout" label="超时(秒)" width="100" />
        <el-table-column label="API Token" width="200">
          <template #default="{ row }">
            <div v-if="row.token" style="display: flex; align-items: center; gap: 4px">
              <span class="token-cell">{{ row.token }}</span>
              <el-tooltip :content="copiedId === row.id ? '已复制' : '复制'" placement="top">
                <el-button link size="small" @click="copyToken(row)">
                  <el-icon><Select v-if="copiedId === row.id" /><DocumentCopy v-else /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
            <span v-else style="color: #c0c4cc; font-size: 12px">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column v-if="authStore.isAdmin" label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="$router.push({ name: 'AgentEdit', params: { id: row.id } })">编辑</el-button>
            <el-popconfirm title="确定删除？" @confirm="handleDelete(row.id)">
              <template #reference>
                <el-button link type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        style="margin-top: 16px; justify-content: flex-end"
        @size-change="loadData"
        @current-change="loadData"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { DocumentCopy, Select } from '@element-plus/icons-vue'
import { agentApi, type Agent } from '../../api/agent'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const list = ref<Agent[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const copiedId = ref<number | null>(null)

async function loadData() {
  loading.value = true
  try {
    const res: any = await agentApi.list({ page: page.value, page_size: pageSize.value, keyword: keyword.value })
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } finally { loading.value = false }
}

async function handleDelete(id: number) {
  try {
    await agentApi.delete(id)
    ElMessage.success('删除成功')
    loadData()
  } catch { ElMessage.error('删除失败') }
}

function copyToken(row: Agent) {
  if (!row.token) return
  navigator.clipboard.writeText(row.token).then(() => {
    copiedId.value = row.id
    setTimeout(() => { copiedId.value = null }, 2000)
  })
}

onMounted(loadData)
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-title {
  font-size: 16px;
  font-weight: 600;
}
.token-cell {
  font-family: monospace;
  font-size: 12px;
  color: #606266;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 140px;
}
</style>
