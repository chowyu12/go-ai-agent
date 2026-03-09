<template>
  <div class="mcp-form-page">
    <el-page-header @back="goBack">
      <template #content>
        <span class="page-title">{{ isEdit ? '编辑 MCP 服务' : '新增 MCP 服务' }}</span>
      </template>
    </el-page-header>

    <el-card shadow="never" style="margin-top: 20px" v-loading="pageLoading">
      <el-form :model="form" label-width="120px" style="max-width: 720px">
        <el-divider content-position="left">基本信息</el-divider>

        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="MCP 服务名称（如 filesystem、github）" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="MCP 服务功能描述" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <el-divider content-position="left">连接配置</el-divider>

        <el-form-item label="传输方式" required>
          <el-radio-group v-model="form.transport">
            <el-radio-button value="stdio">
              Stdio（本地进程）
            </el-radio-button>
            <el-radio-button value="sse">
              SSE（远程 HTTP）
            </el-radio-button>
          </el-radio-group>
        </el-form-item>

        <template v-if="form.transport === 'stdio'">
          <el-form-item label="命令" required>
            <el-input v-model="form.endpoint" placeholder="可执行文件路径，如 npx, uvx, /usr/local/bin/my-server" />
            <div class="form-hint">MCP 服务器启动命令（将通过 stdin/stdout 通信）</div>
          </el-form-item>
          <el-form-item label="参数">
            <div class="tag-editor">
              <el-tag
                v-for="(arg, idx) in args" :key="idx"
                closable size="default"
                @close="args.splice(idx, 1)"
              >{{ arg }}</el-tag>
              <el-input
                v-if="argInputVisible"
                ref="argInputRef"
                v-model="argInputVal"
                size="small"
                style="width: 200px"
                @keyup.enter="confirmArg"
                @blur="confirmArg"
                placeholder="输入参数后按回车"
              />
              <el-button v-else size="small" @click="showArgInput">
                <el-icon><Plus /></el-icon> 添加参数
              </el-button>
            </div>
            <div class="form-hint">传给命令的参数列表，如 -y @modelcontextprotocol/server-filesystem /tmp</div>
          </el-form-item>
          <el-form-item label="环境变量">
            <div class="kv-editor">
              <div v-for="(item, idx) in envList" :key="idx" class="kv-row">
                <el-input v-model="item.key" placeholder="KEY" size="small" style="width: 160px" />
                <span class="kv-sep">=</span>
                <el-input v-model="item.value" placeholder="VALUE" size="small" style="flex: 1" />
                <el-button link type="danger" @click="envList.splice(idx, 1)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <el-button size="small" @click="envList.push({ key: '', value: '' })">
                <el-icon><Plus /></el-icon> 添加
              </el-button>
            </div>
          </el-form-item>
        </template>

        <template v-if="form.transport === 'sse'">
          <el-form-item label="SSE URL" required>
            <el-input v-model="form.endpoint" placeholder="http://localhost:8080/sse" />
            <div class="form-hint">MCP 服务器的 SSE 端点 URL</div>
          </el-form-item>
          <el-form-item label="HTTP Headers">
            <div class="kv-editor">
              <div v-for="(item, idx) in headerList" :key="idx" class="kv-row">
                <el-input v-model="item.key" placeholder="Header-Name" size="small" style="width: 160px" />
                <span class="kv-sep">:</span>
                <el-input v-model="item.value" placeholder="value" size="small" style="flex: 1" />
                <el-button link type="danger" @click="headerList.splice(idx, 1)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <el-button size="small" @click="headerList.push({ key: '', value: '' })">
                <el-icon><Plus /></el-icon> 添加
              </el-button>
            </div>
          </el-form-item>
        </template>

        <el-form-item>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">
            {{ isEdit ? '保存' : '创建' }}
          </el-button>
          <el-button @click="goBack">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Delete, Plus } from '@element-plus/icons-vue'
import { mcpApi } from '../../api/mcp'

interface KVItem { key: string; value: string }

const route = useRoute()
const router = useRouter()

const mcpId = computed(() => {
  const id = route.params.id
  return id ? Number(id) : null
})
const isEdit = computed(() => !!mcpId.value)

const pageLoading = ref(false)
const submitting = ref(false)

const form = ref({
  name: '',
  description: '',
  transport: 'stdio' as 'stdio' | 'sse',
  endpoint: '',
  enabled: true,
})

const args = ref<string[]>([])
const argInputVisible = ref(false)
const argInputVal = ref('')
const argInputRef = ref<any>(null)

const envList = ref<KVItem[]>([])
const headerList = ref<KVItem[]>([])

function showArgInput() {
  argInputVisible.value = true
  nextTick(() => argInputRef.value?.focus())
}

function confirmArg() {
  const val = argInputVal.value.trim()
  if (val) {
    args.value.push(val)
  }
  argInputVisible.value = false
  argInputVal.value = ''
}

function kvToMap(list: KVItem[]): Record<string, string> | undefined {
  const filtered = list.filter(i => i.key.trim())
  if (filtered.length === 0) return undefined
  const m: Record<string, string> = {}
  for (const i of filtered) m[i.key.trim()] = i.value
  return m
}

function mapToKV(m: Record<string, string> | null | undefined): KVItem[] {
  if (!m) return []
  return Object.entries(m).map(([key, value]) => ({ key, value }))
}

function goBack() {
  router.push({ name: 'McpServers' })
}

async function loadMcp() {
  if (!mcpId.value) return
  pageLoading.value = true
  try {
    const res: any = await mcpApi.get(mcpId.value)
    const d = res.data
    form.value = {
      name: d.name || '',
      description: d.description || '',
      transport: d.transport || 'stdio',
      endpoint: d.endpoint || '',
      enabled: d.enabled ?? true,
    }
    args.value = Array.isArray(d.args) ? [...d.args] : []
    envList.value = mapToKV(d.env)
    headerList.value = mapToKV(d.headers)
  } finally {
    pageLoading.value = false
  }
}

async function handleSubmit() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入名称')
    return
  }
  if (!form.value.endpoint.trim()) {
    ElMessage.warning('请输入端点')
    return
  }
  submitting.value = true
  try {
    const data: any = { ...form.value }
    if (form.value.transport === 'stdio') {
      if (args.value.length > 0) data.args = args.value
      data.env = kvToMap(envList.value)
      data.headers = undefined
    } else {
      data.headers = kvToMap(headerList.value)
      data.args = undefined
      data.env = undefined
    }

    if (isEdit.value) {
      await mcpApi.update(mcpId.value!, data)
      ElMessage.success('更新成功')
      goBack()
    } else {
      await mcpApi.create(data)
      ElMessage.success('创建成功')
      goBack()
    }
  } finally {
    submitting.value = false
  }
}

onMounted(loadMcp)
</script>

<style scoped>
.mcp-form-page {
  padding: 4px;
}
.page-title {
  font-size: 16px;
  font-weight: 600;
}
.form-hint {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
  line-height: 1.4;
}
.tag-editor {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.kv-editor {
  width: 100%;
}
.kv-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
.kv-sep {
  color: #909399;
  font-weight: 600;
}
</style>
