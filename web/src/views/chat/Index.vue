<template>
  <div class="chat-page">
    <!-- 侧栏 -->
    <aside class="chat-aside">
      <div class="aside-header">
        <span class="aside-title">Agent</span>
        <el-button size="small" text @click="newConversation" :disabled="!selectedAgentUUID" title="新对话">
          <el-icon><Plus /></el-icon>
        </el-button>
      </div>
      <div class="aside-agents">
        <div
          v-for="a in agents" :key="a.uuid"
          class="agent-item"
          :class="{ active: selectedAgentUUID === a.uuid }"
          @click="selectAgent(a.uuid)"
        >
          <div class="agent-icon">
            <el-icon :size="18"><Cpu /></el-icon>
          </div>
          <div class="agent-info">
            <div class="agent-name">{{ a.name }}</div>
            <div class="agent-model">{{ a.model_name }}</div>
          </div>
        </div>
      </div>

      <!-- 会话历史 -->
      <div class="aside-divider" v-if="conversations.length > 0">
        <span>历史会话</span>
      </div>
      <div class="aside-convs" v-if="conversations.length > 0">
        <div
          v-for="conv in conversations" :key="conv.id"
          class="conv-item"
          :class="{ active: activeConvId === conv.id }"
          @click="loadConversation(conv)"
        >
          <el-icon :size="14" class="conv-icon"><ChatDotRound /></el-icon>
          <div class="conv-info">
            <div class="conv-title">{{ conv.title || '未命名对话' }}</div>
            <div class="conv-time">{{ formatTime(conv.updated_at) }}</div>
          </div>
          <el-icon
            class="conv-delete"
            :size="14"
            @click.stop="deleteConv(conv.id)"
            title="删除"
          ><Delete /></el-icon>
        </div>
      </div>
    </aside>

    <!-- 主区域 -->
    <main class="chat-main">
      <div class="messages-area" ref="messagesArea">
        <!-- 加载中 -->
        <div v-if="loadingHistory" class="empty-state">
          <el-icon class="is-loading" :size="32" color="#3370ff"><Loading /></el-icon>
          <div class="empty-desc" style="margin-top: 12px">加载会话中...</div>
        </div>
        <!-- 空状态 -->
        <div v-else-if="!selectedAgentUUID || messages.length === 0" class="empty-state">
          <div class="empty-icon-wrap">
            <el-icon :size="40"><ChatDotRound /></el-icon>
          </div>
          <div class="empty-title">{{ !selectedAgentUUID ? '请选择一个 Agent' : '开始新对话' }}</div>
          <div class="empty-desc" v-if="selectedAgentUUID">
            输入消息开始与 <strong>{{ currentAgentName }}</strong> 对话
          </div>
        </div>

        <!-- 消息列表 -->
        <template v-else>
          <div v-for="(msg, i) in messages" :key="i" :class="['msg-row', msg.role]">
            <div class="msg-avatar" :class="msg.role">
              <el-icon :size="16" v-if="msg.role === 'user'"><User /></el-icon>
              <el-icon :size="16" v-else><Cpu /></el-icon>
            </div>
            <div class="msg-body">
              <div class="msg-meta">
                <span class="msg-sender">{{ msg.role === 'user' ? '你' : currentAgentName }}</span>
              </div>

              <!-- 附件 -->
              <div v-if="msg.files && msg.files.length > 0" class="msg-attachments">
                <template v-for="f in msg.files" :key="f.uuid">
                  <img v-if="f.file_type === 'image'" :src="'/api/v1/files/' + f.uuid" :alt="f.filename" class="attach-img" />
                  <a v-else :href="'/api/v1/files/' + f.uuid" target="_blank" class="attach-file">
                    <span class="attach-file-icon">{{ fileTypeIcon(f.file_type) }}</span>
                    <span class="attach-file-name">{{ f.filename }}</span>
                    <span class="attach-file-size" v-if="f.file_size">{{ formatFileSize(f.file_size) }}</span>
                  </a>
                </template>
              </div>

              <!-- 消息内容 -->
              <div class="msg-bubble" v-html="formatMessage(msg.content)"></div>

              <!-- 执行步骤 -->
              <div v-if="msg.role === 'assistant' && msg.steps && msg.steps.length > 0" class="steps-panel">
                <div class="steps-toggle" @click="msg._showSteps = !msg._showSteps">
                  <el-icon :size="14"><Operation /></el-icon>
                  <span>{{ msg.steps.length }} 个执行步骤</span>
                  <el-icon class="toggle-icon" :class="{ open: msg._showSteps }"><ArrowDown /></el-icon>
                </div>
                <transition name="fold">
                  <div v-if="msg._showSteps" class="steps-list">
                    <div v-for="step in msg.steps" :key="step.step_order" class="step-row">
                      <div class="step-indicator">
                        <span class="step-dot" :class="'dot--' + step.step_type"></span>
                        <span class="step-line"></span>
                      </div>
                      <div class="step-body">
                        <div class="step-head">
                          <span class="step-badge" :class="'badge--' + step.step_type">{{ stepTypeLabel(step.step_type) }}</span>
                          <span class="step-title">{{ step.name }}</span>
                          <el-tag
                            :type="step.status === 'success' ? 'success' : 'danger'"
                            size="small" round effect="plain"
                          >{{ step.status === 'success' ? step.duration_ms + 'ms' : 'failed' }}</el-tag>
                        </div>
                        <div class="step-detail">
                          <template v-if="step.input">
                            <div class="detail-label">Input</div>
                            <pre class="detail-code">{{ truncateText(step.input, 500) }}</pre>
                          </template>
                          <template v-if="step.output">
                            <div class="detail-label">Output</div>
                            <pre class="detail-code">{{ truncateText(step.output, 500) }}</pre>
                          </template>
                          <template v-if="step.error">
                            <div class="detail-label detail-label--err">Error</div>
                            <pre class="detail-code detail-code--err">{{ step.error }}</pre>
                          </template>
                          <div class="detail-meta" v-if="step.metadata">
                            <span v-if="step.metadata.provider">{{ step.metadata.provider }}</span>
                            <span v-if="step.metadata.model">{{ step.metadata.model }}</span>
                            <span v-if="step.metadata.skill_name">Skill: {{ step.metadata.skill_name }}</span>
                            <span v-if="step.metadata.skill_tools?.length">{{ step.metadata.skill_tools.join(', ') }}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </transition>
              </div>
            </div>
          </div>

          <!-- 流式响应 -->
          <div v-if="streaming" class="msg-row assistant">
            <div class="msg-avatar assistant">
              <el-icon :size="16"><Cpu /></el-icon>
            </div>
            <div class="msg-body">
              <div class="msg-meta">
                <span class="msg-sender">{{ currentAgentName }}</span>
              </div>

              <!-- 实时步骤时间线 -->
              <div v-if="pendingSteps.length > 0 || !streamingContent" class="wf-timeline">
                <div v-for="(step, idx) in pendingSteps" :key="idx" class="wf-node">
                  <div class="wf-node-head" @click="step._expanded = !step._expanded">
                    <span class="wf-dot" :class="'wf-dot--' + step.step_type"></span>
                    <span class="wf-label">{{ stepTypeLabel(step.step_type) }}</span>
                    <span class="wf-name">{{ step.name }}</span>
                    <el-tag v-if="step.status === 'success'" type="success" size="small" round effect="plain">{{ step.duration_ms }}ms</el-tag>
                    <el-tag v-else-if="step.status === 'error'" type="danger" size="small" round effect="plain">failed</el-tag>
                    <el-icon class="wf-arrow" :class="{ open: step._expanded }"><ArrowRight /></el-icon>
                  </div>
                  <transition name="fold">
                    <div v-if="step._expanded" class="wf-node-body">
                      <template v-if="step.input">
                        <div class="detail-label">Input</div>
                        <pre class="detail-code">{{ truncateText(step.input, 500) }}</pre>
                      </template>
                      <template v-if="step.output">
                        <div class="detail-label">Output</div>
                        <pre class="detail-code">{{ truncateText(step.output, 500) }}</pre>
                      </template>
                      <template v-if="step.error">
                        <div class="detail-label detail-label--err">Error</div>
                        <pre class="detail-code detail-code--err">{{ step.error }}</pre>
                      </template>
                      <div class="detail-meta" v-if="step.metadata">
                        <span v-if="step.metadata.provider">{{ step.metadata.provider }}</span>
                        <span v-if="step.metadata.model">{{ step.metadata.model }}</span>
                        <span v-if="step.metadata.skill_name">Skill: {{ step.metadata.skill_name }}</span>
                        <span v-if="step.metadata.skill_tools?.length">{{ step.metadata.skill_tools.join(', ') }}</span>
                      </div>
                    </div>
                  </transition>
                </div>

                <div v-if="!streamingContent" class="wf-node wf-node--thinking">
                  <span class="wf-dot wf-dot--thinking"><el-icon class="is-loading" :size="10"><Loading /></el-icon></span>
                  <span class="wf-thinking-text">{{ pendingSteps.length > 0 ? '生成回复中...' : '思考中...' }}</span>
                </div>
              </div>

              <!-- 流式文本 -->
              <div v-if="streamingContent" class="msg-bubble">
                <span v-html="formatMessage(streamingContent)"></span>
                <span class="typing-cursor"></span>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- 输入区域 -->
      <div class="input-area" :class="{ disabled: !selectedAgentUUID }">
        <!-- 附件预览条 -->
        <div v-if="pendingFiles.length > 0 || pendingURLs.length > 0" class="attach-bar">
          <div v-for="(f, idx) in pendingFiles" :key="f.uuid" class="attach-chip">
            <span class="chip-icon">{{ fileTypeIcon(f.file_type) }}</span>
            <span class="chip-name">{{ f.filename }}</span>
            <span class="chip-size">{{ formatFileSize(f.file_size) }}</span>
            <el-icon class="chip-close" @click="removeFile(idx)"><Close /></el-icon>
          </div>
          <div v-for="(u, idx) in pendingURLs" :key="u" class="attach-chip chip--url">
            <el-icon :size="12"><Link /></el-icon>
            <span class="chip-name" :title="u">{{ u.length > 40 ? u.slice(0, 40) + '...' : u }}</span>
            <el-icon class="chip-close" @click="removeURL(idx)"><Close /></el-icon>
          </div>
        </div>

        <!-- URL 输入 -->
        <div v-if="showURLInput" class="url-bar">
          <el-input
            v-model="urlInput"
            size="small"
            placeholder="粘贴文件 URL，回车添加"
            @keydown.enter.prevent="addURL"
            clearable
            class="url-input"
          />
          <el-button size="small" type="primary" @click="addURL" :disabled="!urlInput.trim()">添加</el-button>
          <el-button size="small" text @click="showURLInput = false; urlInput = ''">取消</el-button>
        </div>

        <!-- 输入框 -->
        <div class="composer">
          <div class="composer-tools">
            <label class="tool-btn" :class="{ off: !selectedAgentUUID || streaming || uploading }" title="上传文件">
              <el-icon :size="18"><UploadFilled /></el-icon>
              <input
                type="file" multiple style="display:none"
                accept=".txt,.md,.json,.csv,.xml,.yaml,.yml,.log,.pdf,.docx,.doc,.xlsx,.xls,.png,.jpg,.jpeg,.gif,.webp"
                :disabled="!selectedAgentUUID || streaming || uploading"
                @change="handleFileUpload"
              />
            </label>
            <button
              class="tool-btn"
              :class="{ off: !selectedAgentUUID || streaming, active: showURLInput }"
              :disabled="!selectedAgentUUID || streaming"
              @click="showURLInput = !showURLInput"
              title="添加 URL"
            >
              <el-icon :size="18"><Link /></el-icon>
            </button>
          </div>
          <div class="composer-input">
            <el-input
              v-model="inputMessage"
              type="textarea"
              :autosize="{ minRows: 1, maxRows: 5 }"
              placeholder="输入消息，Enter 发送，Shift + Enter 换行"
              :disabled="!selectedAgentUUID || streaming"
              @keydown="handleKeydown"
              resize="none"
            />
          </div>
          <button
            class="send-btn"
            :class="{ ready: selectedAgentUUID && inputMessage.trim() && !streaming }"
            :disabled="!selectedAgentUUID || !inputMessage.trim() || streaming"
            @click="sendMessage"
          >
            <el-icon v-if="streaming" class="is-loading"><Loading /></el-icon>
            <el-icon v-else><Promotion /></el-icon>
          </button>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, reactive } from 'vue'
import { agentApi, type Agent } from '../../api/agent'
import { chatApi, streamChat, fileApi, type StreamChunk, type ExecutionStep, type FileInfo, type ChatFile, type Conversation, type Message } from '../../api/chat'
import { ElMessage, ElMessageBox } from 'element-plus'

interface UploadedFile {
  uuid: string
  filename: string
  file_type: 'text' | 'image' | 'document'
  file_size: number
}

interface ChatMessage {
  role: string
  content: string
  steps?: ExecutionStep[]
  files?: FileInfo[]
  _showSteps?: boolean
}

const agents = ref<Agent[]>([])
const selectedAgentUUID = ref('')
const conversationId = ref('')
const messages = ref<ChatMessage[]>([])
const inputMessage = ref('')
const streaming = ref(false)
const streamingContent = ref('')
const messagesArea = ref<HTMLElement>()
const pendingSteps = ref<ExecutionStep[]>([])
const pendingFiles = ref<UploadedFile[]>([])
const pendingURLs = ref<string[]>([])
const urlInput = ref('')
const showURLInput = ref(false)
const uploading = ref(false)
const conversations = ref<Conversation[]>([])
const activeConvId = ref<number>(0)
const loadingHistory = ref(false)

const currentAgentName = computed(() => {
  const a = agents.value.find(a => a.uuid === selectedAgentUUID.value)
  return a?.name || 'Agent'
})

const currentAgent = computed(() => agents.value.find(a => a.uuid === selectedAgentUUID.value))

onMounted(async () => {
  const res: any = await agentApi.list({ page: 1, page_size: 100 })
  agents.value = res.data?.list || []
  const first = agents.value[0]
  if (first && !selectedAgentUUID.value) {
    selectAgent(first.uuid)
  }
})

function selectAgent(uuid: string) {
  if (selectedAgentUUID.value === uuid) return
  selectedAgentUUID.value = uuid
  resetChat()
  loadConversations()
}

async function loadConversations() {
  const ag = currentAgent.value
  if (!ag) { conversations.value = []; return }
  try {
    const res: any = await chatApi.conversations({ page: 1, page_size: 50, agent_id: ag.id })
    conversations.value = res.data?.list || []
  } catch {
    conversations.value = []
  }
}

async function loadConversation(conv: Conversation) {
  activeConvId.value = conv.id
  conversationId.value = conv.uuid
  loadingHistory.value = true
  try {
    const res: any = await chatApi.messages(conv.id, 100, true)
    const msgs: Message[] = res.data || []
    messages.value = msgs
      .filter(m => m.role === 'user' || m.role === 'assistant')
      .map(m => reactive({
        role: m.role,
        content: m.content,
        steps: m.steps,
        files: m.files,
        _showSteps: false,
      }))
    scrollToBottom()
  } catch {
    ElMessage.error('加载会话失败')
  } finally {
    loadingHistory.value = false
  }
}

async function deleteConv(id: number) {
  try {
    await ElMessageBox.confirm('确定删除该会话？', '删除', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch { return }
  try {
    await chatApi.deleteConversation(id)
    conversations.value = conversations.value.filter(c => c.id !== id)
    if (activeConvId.value === id) {
      resetChat()
    }
  } catch {
    ElMessage.error('删除失败')
  }
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const isToday = d.toDateString() === now.toDateString()
  if (isToday) return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function newConversation() {
  resetChat()
}

function resetChat() {
  conversationId.value = ''
  activeConvId.value = 0
  messages.value = []
  streamingContent.value = ''
  pendingSteps.value = []
  pendingFiles.value = []
  pendingURLs.value = []
  urlInput.value = ''
  showURLInput.value = false
}

async function handleFileUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (!files || files.length === 0) return
  uploading.value = true
  for (const file of Array.from(files)) {
    try {
      const res: any = await fileApi.upload(file)
      const f = res.data as FileInfo
      pendingFiles.value.push({ uuid: f.uuid, filename: f.filename, file_type: f.file_type, file_size: f.file_size })
    } catch {
      ElMessage.error(`上传 ${file.name} 失败`)
    }
  }
  uploading.value = false
  input.value = ''
}

function removeFile(idx: number) {
  const f = pendingFiles.value[idx]
  if (!f) return
  pendingFiles.value.splice(idx, 1)
  fileApi.delete(f.uuid).catch(() => {})
}

function addURL() {
  const url = urlInput.value.trim()
  if (!url) return
  try { new URL(url) } catch { ElMessage.warning('请输入有效的 URL'); return }
  if (pendingURLs.value.includes(url)) { ElMessage.warning('该 URL 已添加'); return }
  pendingURLs.value.push(url)
  urlInput.value = ''
}

function removeURL(idx: number) { pendingURLs.value.splice(idx, 1) }

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

function fileTypeIcon(type: string): string {
  switch (type) {
    case 'image': return '🖼'
    case 'document': return '📄'
    default: return '📝'
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessage()
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (messagesArea.value) messagesArea.value.scrollTop = messagesArea.value.scrollHeight
  })
}

function sendMessage() {
  const text = inputMessage.value.trim()
  if (!text || !selectedAgentUUID.value) return

  const chatFiles: ChatFile[] = [
    ...pendingFiles.value.map(f => ({ type: f.file_type as ChatFile['type'], transfer_method: 'local_file' as const, upload_file_id: f.uuid })),
    ...pendingURLs.value.map(u => ({ type: 'document' as const, transfer_method: 'remote_url' as const, url: u })),
  ]

  const displayFiles: FileInfo[] = [
    ...pendingFiles.value.map(f => ({ ...f, id: 0, conversation_id: 0, message_id: 0, content_type: '', created_at: '' }) as FileInfo),
    ...pendingURLs.value.map(u => ({ id: 0, uuid: u, conversation_id: 0, message_id: 0, filename: u.split('/').pop() || 'url', content_type: '', file_size: 0, file_type: 'text' as const, created_at: '' })),
  ]

  messages.value.push(reactive({ role: 'user', content: text, files: displayFiles.length > 0 ? displayFiles : undefined }))
  inputMessage.value = ''
  pendingFiles.value = []
  pendingURLs.value = []
  urlInput.value = ''
  showURLInput.value = false
  streaming.value = true
  streamingContent.value = ''
  pendingSteps.value = []
  scrollToBottom()

  streamChat(
    { agent_id: selectedAgentUUID.value, conversation_id: conversationId.value, message: text, files: chatFiles.length > 0 ? chatFiles : undefined },
    (chunk: StreamChunk) => {
      if (chunk.conversation_id) conversationId.value = chunk.conversation_id
      if (chunk.delta) { streamingContent.value += chunk.delta; scrollToBottom() }
      if (chunk.steps?.length) { for (const s of chunk.steps) pendingSteps.value.push(reactive({ ...s, _expanded: false })) }
      else if (chunk.step) pendingSteps.value.push(reactive({ ...chunk.step, _expanded: false }))
      if (chunk.done) {
        messages.value.push(reactive({ role: 'assistant', content: streamingContent.value, steps: [...pendingSteps.value], _showSteps: false }))
        streamingContent.value = ''
        pendingSteps.value = []
        streaming.value = false
        scrollToBottom()
        loadConversations()
      }
    },
    () => {
      if (streaming.value && streamingContent.value) {
        messages.value.push(reactive({ role: 'assistant', content: streamingContent.value, steps: [...pendingSteps.value], _showSteps: false }))
        streamingContent.value = ''
        pendingSteps.value = []
      }
      streaming.value = false
      loadConversations()
    },
    (err: string) => {
      messages.value.push(reactive({ role: 'assistant', content: `[错误] ${err}` }))
      streaming.value = false
      scrollToBottom()
    },
  )
}

function formatMessage(text: string): string {
  return text.replace(/\n/g, '<br/>')
}

function stepTypeLabel(t: string) {
  switch (t) {
    case 'llm_call': return 'LLM'
    case 'tool_call': return 'Tool'
    case 'agent_call': return 'Agent'
    case 'skill_match': return 'Skill'
    default: return t
  }
}

function truncateText(text: string, maxLen: number): string {
  return text.length <= maxLen ? text : text.slice(0, maxLen) + '...[truncated]'
}
</script>

<style scoped>
/* ===== Layout ===== */
.chat-page {
  display: flex;
  height: calc(100vh - 100px);
  background: #f5f6f8;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid #e8eaed;
}

/* ===== Sidebar ===== */
.chat-aside {
  width: 240px;
  background: #fff;
  border-right: 1px solid #ebedf0;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}
.aside-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 16px 12px;
  border-bottom: 1px solid #f0f1f3;
}
.aside-title {
  font-size: 14px;
  font-weight: 600;
  color: #1d2129;
}
.aside-agents {
  padding: 8px;
  flex-shrink: 0;
}
.aside-divider {
  padding: 8px 16px 4px;
  font-size: 11px;
  font-weight: 500;
  color: #c0c4cc;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-top: 1px solid #f0f1f3;
}
.aside-convs {
  flex: 1;
  overflow-y: auto;
  padding: 4px 8px 8px;
}
.conv-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
  margin-bottom: 2px;
}
.conv-item:hover {
  background: #f2f3f5;
}
.conv-item.active {
  background: #e8f3ff;
}
.conv-icon {
  color: #86909c;
  flex-shrink: 0;
}
.conv-item.active .conv-icon {
  color: #3370ff;
}
.conv-info {
  flex: 1;
  min-width: 0;
}
.conv-title {
  font-size: 12px;
  color: #1d2129;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.4;
}
.conv-time {
  font-size: 10px;
  color: #c0c4cc;
  margin-top: 1px;
}
.conv-delete {
  color: transparent;
  flex-shrink: 0;
  transition: color 0.15s;
}
.conv-item:hover .conv-delete {
  color: #c0c4cc;
}
.conv-delete:hover {
  color: #f56c6c !important;
}
.agent-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s;
  margin-bottom: 2px;
}
.agent-item:hover {
  background: #f2f3f5;
}
.agent-item.active {
  background: #e8f3ff;
}
.agent-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.agent-item.active .agent-icon {
  background: linear-gradient(135deg, #3370ff, #5b8def);
}
.agent-info {
  min-width: 0;
  flex: 1;
}
.agent-name {
  font-size: 13px;
  font-weight: 500;
  color: #1d2129;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.agent-model {
  font-size: 11px;
  color: #86909c;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* ===== Main ===== */
.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: #fafbfc;
}

/* ===== Messages ===== */
.messages-area {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
}

/* Empty State */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 8px;
}
.empty-icon-wrap {
  width: 72px;
  height: 72px;
  border-radius: 20px;
  background: linear-gradient(135deg, #e8f3ff, #d4e4ff);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #3370ff;
  margin-bottom: 8px;
}
.empty-title {
  font-size: 16px;
  font-weight: 600;
  color: #1d2129;
}
.empty-desc {
  font-size: 13px;
  color: #86909c;
}

/* Message Row */
.msg-row {
  display: flex;
  gap: 12px;
  margin-bottom: 24px;
  animation: msg-in 0.25s ease;
}
@keyframes msg-in {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}
.msg-avatar {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: #fff;
  font-size: 14px;
}
.msg-avatar.user {
  background: linear-gradient(135deg, #3370ff, #5b8def);
}
.msg-avatar.assistant {
  background: linear-gradient(135deg, #00b894, #2dd4a8);
}
.msg-body {
  flex: 1;
  min-width: 0;
}
.msg-meta {
  margin-bottom: 6px;
}
.msg-sender {
  font-size: 12px;
  font-weight: 500;
  color: #86909c;
}

/* Bubble */
.msg-bubble {
  display: inline-block;
  max-width: 100%;
  padding: 10px 16px;
  border-radius: 4px 14px 14px 14px;
  font-size: 14px;
  line-height: 1.7;
  word-break: break-word;
  color: #1d2129;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0,0,0,0.04);
}
.msg-row.user .msg-bubble {
  background: #e8f3ff;
  border-radius: 14px 4px 14px 14px;
  box-shadow: none;
}

/* Attachments */
.msg-attachments {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}
.attach-img {
  max-width: 200px;
  max-height: 150px;
  border-radius: 10px;
  border: 1px solid #ebedf0;
  cursor: pointer;
  transition: transform 0.2s;
}
.attach-img:hover { transform: scale(1.03); }
.attach-file {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #fff;
  border: 1px solid #ebedf0;
  border-radius: 8px;
  padding: 6px 12px;
  font-size: 12px;
  color: #3370ff;
  text-decoration: none;
  transition: all 0.15s;
}
.attach-file:hover { border-color: #3370ff; background: #f0f6ff; }
.attach-file-icon { font-size: 14px; }
.attach-file-name { max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.attach-file-size { color: #c0c4cc; font-size: 11px; }

/* ===== Steps Panel ===== */
.steps-panel {
  margin-top: 10px;
}
.steps-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-size: 12px;
  color: #86909c;
  padding: 4px 10px;
  border-radius: 6px;
  transition: all 0.15s;
  user-select: none;
}
.steps-toggle:hover { background: #f2f3f5; color: #4e5969; }
.toggle-icon {
  transition: transform 0.25s;
  font-size: 12px;
}
.toggle-icon.open { transform: rotate(180deg); }

.steps-list {
  margin-top: 8px;
  padding-left: 4px;
}
.step-row {
  display: flex;
  gap: 12px;
}
.step-indicator {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 16px;
  flex-shrink: 0;
}
.step-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 6px;
}
.dot--llm_call { background: #3370ff; }
.dot--tool_call { background: #f57c00; }
.dot--agent_call { background: #00b894; }
.dot--skill_match { background: #7c3aed; }
.step-line {
  width: 2px;
  flex: 1;
  background: #e5e6eb;
  margin: 4px 0;
  min-height: 8px;
}
.step-row:last-child .step-line { display: none; }

.step-body {
  flex: 1;
  padding-bottom: 14px;
  min-width: 0;
}
.step-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.step-badge {
  font-size: 10px;
  font-weight: 600;
  color: #fff;
  padding: 1px 6px;
  border-radius: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.badge--llm_call { background: #3370ff; }
.badge--tool_call { background: #f57c00; }
.badge--agent_call { background: #00b894; }
.badge--skill_match { background: #7c3aed; }

.step-title {
  font-size: 13px;
  font-weight: 500;
  color: #1d2129;
}

/* Shared detail styles */
.step-detail, .wf-node-body {
  margin-top: 8px;
}
.detail-label {
  font-size: 11px;
  color: #86909c;
  font-weight: 500;
  margin-bottom: 4px;
  margin-top: 6px;
}
.detail-label:first-child { margin-top: 0; }
.detail-label--err { color: #f56c6c; }
.detail-code {
  background: #f7f8fa;
  border: 1px solid #ebedf0;
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.5;
  font-family: 'SF Mono', 'Menlo', 'Monaco', monospace;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 160px;
  overflow-y: auto;
  margin: 0;
  color: #1d2129;
}
.detail-code--err {
  background: #fff1f0;
  border-color: #fde2e2;
  color: #f56c6c;
}
.detail-meta {
  display: flex;
  gap: 10px;
  margin-top: 6px;
  font-size: 11px;
  color: #c0c4cc;
  flex-wrap: wrap;
}

/* ===== Workflow Timeline (streaming) ===== */
.wf-timeline {
  background: #f2f3f5;
  border-radius: 12px;
  padding: 8px 12px;
  margin-bottom: 10px;
}
.wf-node {
  margin-bottom: 2px;
  border-radius: 8px;
  overflow: hidden;
  animation: msg-in 0.25s ease;
}
.wf-node-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.12s;
}
.wf-node-head:hover { background: #eaecf0; }
.wf-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.wf-dot--llm_call { background: #3370ff; }
.wf-dot--tool_call { background: #f57c00; }
.wf-dot--agent_call { background: #00b894; }
.wf-dot--skill_match { background: #7c3aed; }
.wf-dot--thinking {
  width: 18px;
  height: 18px;
  background: #c0c4cc;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}
.wf-label {
  font-size: 11px;
  font-weight: 600;
  color: #86909c;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  flex-shrink: 0;
}
.wf-name {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  color: #1d2129;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.wf-arrow {
  color: #c0c4cc;
  font-size: 12px;
  transition: transform 0.2s;
  flex-shrink: 0;
}
.wf-arrow.open { transform: rotate(90deg); }
.wf-node-body {
  padding: 4px 12px 12px 28px;
}
.wf-node--thinking {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
}
.wf-thinking-text {
  font-size: 13px;
  color: #86909c;
}

/* ===== Input Area ===== */
.input-area {
  background: #fff;
  border-top: 1px solid #ebedf0;
  padding: 12px 24px 16px;
}
.input-area.disabled { opacity: 0.5; pointer-events: none; }

.attach-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 10px;
}
.attach-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: #f2f3f5;
  border: 1px solid #e5e6eb;
  border-radius: 8px;
  padding: 4px 10px;
  font-size: 12px;
  color: #4e5969;
  animation: msg-in 0.2s ease;
}
.chip--url { color: #3370ff; }
.chip-icon { font-size: 14px; }
.chip-name {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chip-size { color: #c0c4cc; font-size: 11px; }
.chip-close {
  cursor: pointer;
  color: #c0c4cc;
  transition: color 0.15s;
  font-size: 12px;
}
.chip-close:hover { color: #f56c6c; }

.url-bar {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 10px;
}
.url-input { flex: 1; }

.composer {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  background: #f7f8fa;
  border: 1px solid #e5e6eb;
  border-radius: 12px;
  padding: 6px 8px 6px 4px;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.composer:focus-within {
  border-color: #3370ff;
  box-shadow: 0 0 0 2px rgba(51, 112, 255, 0.1);
}
.composer-tools {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  padding-bottom: 2px;
}
.tool-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: #86909c;
  cursor: pointer;
  transition: all 0.15s;
}
.tool-btn:hover:not(.off) { color: #3370ff; background: #e8f3ff; }
.tool-btn.active { color: #3370ff; background: #e8f3ff; }
.tool-btn.off { color: #c9cdd4; cursor: not-allowed; }
.composer-input {
  flex: 1;
  min-width: 0;
}
.composer-input :deep(.el-textarea__inner) {
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  padding: 6px 0;
  font-size: 14px;
  line-height: 1.5;
  resize: none;
}
.send-btn {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  border: none;
  background: #e5e6eb;
  color: #c0c4cc;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: not-allowed;
  transition: all 0.2s;
  flex-shrink: 0;
  font-size: 16px;
}
.send-btn.ready {
  background: #3370ff;
  color: #fff;
  cursor: pointer;
}
.send-btn.ready:hover {
  background: #245bdb;
}

/* ===== Transitions ===== */
.fold-enter-active, .fold-leave-active {
  transition: all 0.25s ease;
  max-height: 2000px;
  overflow: hidden;
}
.fold-enter-from, .fold-leave-to {
  max-height: 0;
  opacity: 0;
}

/* Typing cursor */
.typing-cursor {
  display: inline-block;
  width: 2px;
  height: 16px;
  background: #3370ff;
  margin-left: 2px;
  vertical-align: text-bottom;
  animation: blink 0.8s infinite;
}
@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

/* Scrollbar */
.messages-area::-webkit-scrollbar,
.aside-body::-webkit-scrollbar {
  width: 4px;
}
.messages-area::-webkit-scrollbar-thumb,
.aside-body::-webkit-scrollbar-thumb {
  background: #d9dbe1;
  border-radius: 4px;
}
.messages-area::-webkit-scrollbar-track,
.aside-body::-webkit-scrollbar-track {
  background: transparent;
}
</style>
