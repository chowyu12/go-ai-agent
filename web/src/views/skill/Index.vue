<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">技能管理</span>
          <div class="header-actions">
            <el-input v-model="keyword" placeholder="搜索" clearable style="width: 200px;" @clear="loadData" @keyup.enter="loadData">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-button @click="handleSync" :loading="syncing">
              <el-icon><Refresh /></el-icon> 同步本地
            </el-button>
            <el-button type="warning" @click="installDialogVisible = true">
              <el-icon><Download /></el-icon> ClawHub 安装
            </el-button>
            <el-button v-if="authStore.isAdmin" type="primary" @click="$router.push('/skills/create')">
              <el-icon><Plus /></el-icon> 新增
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column label="来源" width="110">
          <template #default="{ row }">
            <el-tag v-if="row.source === 'clawhub'" type="warning" size="small">ClawHub</el-tag>
            <el-tag v-else-if="row.source === 'local'" type="success" size="small">本地</el-tag>
            <el-tag v-else type="info" size="small">自定义</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="90" />
        <el-table-column prop="author" label="作者" width="100" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'danger'" size="small">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="工具" width="80">
          <template #default="{ row }">
            <span v-if="row.tool_defs">{{ toolCount(row.tool_defs) }}</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="170" />
        <el-table-column v-if="authStore.isAdmin" label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="$router.push(`/skills/${row.id}/edit`)">编辑</el-button>
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

    <el-dialog v-model="installDialogVisible" title="从 ClawHub 安装技能" width="480px" destroy-on-close>
      <el-form @submit.prevent="handleInstall">
        <el-form-item label="技能名称" required>
          <el-input v-model="installSlug" placeholder="himalaya" />
        </el-form-item>
        <p style="color: #909399; font-size: 13px;">
          输入 ClawHub 技能标识，例如 <code>himalaya</code>、<code>gmail</code>、<code>tavily-search</code>
        </p>
      </el-form>
      <template #footer>
        <el-button @click="installDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleInstall" :loading="installing">安装</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { skillApi, type Skill } from '../../api/skill'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const list = ref<Skill[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const syncing = ref(false)

const installDialogVisible = ref(false)
const installSlug = ref('')
const installing = ref(false)

function toolCount(toolDefs: any): number {
  if (!toolDefs) return 0
  if (Array.isArray(toolDefs)) return toolDefs.length
  try {
    const parsed = typeof toolDefs === 'string' ? JSON.parse(toolDefs) : toolDefs
    return Array.isArray(parsed) ? parsed.length : 0
  } catch { return 0 }
}

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

async function handleSync() {
  syncing.value = true
  try {
    const res: any = await skillApi.sync()
    ElMessage.success(`同步完成，已同步 ${res.data?.synced || 0} 个技能`)
    loadData()
  } catch {
    ElMessage.error('同步失败')
  } finally {
    syncing.value = false
  }
}

async function handleInstall() {
  if (!installSlug.value.trim()) {
    ElMessage.warning('请输入技能 Slug')
    return
  }
  installing.value = true
  try {
    await skillApi.install(installSlug.value.trim())
    ElMessage.success('安装成功')
    installDialogVisible.value = false
    installSlug.value = ''
    loadData()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '安装失败')
  } finally {
    installing.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await skillApi.delete(id)
    ElMessage.success('删除成功')
    loadData()
  } catch {
    ElMessage.error('删除失败')
  }
}

onMounted(loadData)
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
.card-title { font-size: 16px; font-weight: 600; }
.header-actions { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
</style>
