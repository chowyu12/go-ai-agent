<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">MCP 服务管理</span>
          <div>
            <el-input v-model="keyword" placeholder="搜索" clearable style="width: 200px; margin-right: 12px;" @clear="loadData" @keyup.enter="loadData">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-button v-if="authStore.isAdmin" type="primary" @click="router.push({ name: 'McpCreate' })">
              <el-icon><Plus /></el-icon> 新增
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="transport" label="传输方式" width="100">
          <template #default="{ row }">
            <el-tag :type="row.transport === 'stdio' ? 'success' : 'warning'" size="small">
              {{ row.transport }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="endpoint" label="端点" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <code style="font-size: 12px;">{{ row.endpoint }}</code>
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
            <el-button link type="primary" @click="router.push({ name: 'McpEdit', params: { id: row.id } })">编辑</el-button>
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { mcpApi, type McpServer } from '../../api/mcp'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const list = ref<McpServer[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')

async function loadData() {
  loading.value = true
  try {
    const res: any = await mcpApi.list({ page: page.value, page_size: pageSize.value, keyword: keyword.value })
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } finally {
    loading.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await mcpApi.delete(id)
    ElMessage.success('删除成功')
    loadData()
  } catch {
    ElMessage.error('删除失败')
  }
}

onMounted(loadData)
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: 16px; font-weight: 600; }
</style>
