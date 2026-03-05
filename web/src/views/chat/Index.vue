<template>
  <div class="chat-container">
    <el-card shadow="never" class="chat-sidebar">
      <template #header>
        <span style="font-weight: 600">选择 Agent</span>
      </template>
      <el-select v-model="selectedAgentUUID" placeholder="选择 Agent" style="width: 100%; margin-bottom: 12px;">
        <el-option v-for="a in agents" :key="a.uuid" :label="a.name" :value="a.uuid">
          <div style="display: flex; justify-content: space-between">
            <span>{{ a.name }}</span>
            <span style="color: #909399; font-size: 12px">{{ a.model_name }}</span>
          </div>
        </el-option>
      </el-select>
      <el-button style="width: 100%" @click="newConversation">
        <el-icon><Plus /></el-icon> 新对话
      </el-button>
    </el-card>

    <el-card shadow="never" class="chat-main">
      <div class="messages-area" ref="messagesArea">
        <div v-if="!selectedAgentUUID" class="empty-state">
          <el-icon :size="64" color="#dcdfe6"><ChatDotRound /></el-icon>
          <p>请先选择一个 Agent 开始对话</p>
        </div>
        <div v-else-if="messages.length === 0" class="empty-state">
          <el-icon :size="64" color="#dcdfe6"><ChatDotRound /></el-icon>
          <p>开始新对话</p>
        </div>
        <template v-else>
          <div v-for="(msg, i) in messages" :key="i" :class="['message', msg.role]">
            <div class="message-avatar">
              <el-icon :size="20" v-if="msg.role === 'user'"><User /></el-icon>
              <el-icon :size="20" v-else><Cpu /></el-icon>
            </div>
            <div class="message-content">
              <div class="message-role">{{ msg.role === 'user' ? '你' : 'Agent' }}</div>
              <div class="message-text" v-html="formatMessage(msg.content)"></div>

              <!-- 附件展示 -->
              <div v-if="msg.files && msg.files.length > 0" class="msg-files">
                <div v-for="f in msg.files" :key="f.uuid" class="msg-file">
                  <template v-if="f.file_type === 'image'">
                    <img :src="'/api/v1/files/' + f.uuid" :alt="f.filename" class="msg-file-img" />
                  </template>
                  <template v-else>
                    <a :href="'/api/v1/files/' + f.uuid" target="_blank" class="msg-file-link">
                      <span>{{ fileTypeIcon(f.file_type) }}</span>
                      <span>{{ f.filename }}</span>
                      <span class="msg-file-size">{{ formatFileSize(f.file_size) }}</span>
                    </a>
                  </template>
                </div>
              </div>

              <!-- 执行步骤面板 -->
              <div v-if="msg.role === 'assistant' && msg.steps && msg.steps.length > 0" class="steps-panel">
                <div class="steps-toggle" @click="msg._showSteps = !msg._showSteps">
                  <el-icon><Operation /></el-icon>
                  <span>执行步骤 ({{ msg.steps.length }})</span>
                  <el-icon class="toggle-arrow" :class="{ expanded: msg._showSteps }"><ArrowDown /></el-icon>
                </div>
                <transition name="slide">
                  <div v-if="msg._showSteps" class="steps-list">
                    <div v-for="step in msg.steps" :key="step.step_order" class="step-item">
                      <div class="step-header">
                        <el-tag :type="stepTagType(step.step_type)" size="small" effect="dark">
                          {{ stepTypeLabel(step.step_type) }}
                        </el-tag>
                        <span class="step-name">{{ step.name }}</span>
                        <el-tag :type="step.status === 'success' ? 'success' : 'danger'" size="small" round>
                          {{ step.status }}
                        </el-tag>
                        <span class="step-duration">{{ step.duration_ms }}ms</span>
                      </div>
                      <div class="step-detail">
                        <div class="step-section" v-if="step.input">
                          <span class="step-label">输入:</span>
                          <pre class="step-code">{{ truncateText(step.input, 500) }}</pre>
                        </div>
                        <div class="step-section" v-if="step.output">
                          <span class="step-label">输出:</span>
                          <pre class="step-code">{{ truncateText(step.output, 500) }}</pre>
                        </div>
                        <div class="step-section" v-if="step.error">
                          <span class="step-label error-label">错误:</span>
                          <pre class="step-code error-code">{{ step.error }}</pre>
                        </div>
                        <div class="step-meta" v-if="step.metadata">
                          <span v-if="step.metadata.provider">Provider: {{ step.metadata.provider }}</span>
                          <span v-if="step.metadata.model">Model: {{ step.metadata.model }}</span>
                          <span v-if="step.metadata.temperature">Temp: {{ step.metadata.temperature }}</span>
                          <span v-if="step.metadata.skill_name">Skill: {{ step.metadata.skill_name }}</span>
                          <span v-if="step.metadata.skill_tools?.length">Tools: {{ step.metadata.skill_tools.join(', ') }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </transition>
              </div>
            </div>
          </div>

          <!-- 流式内容 (Dify 风格) -->
          <div v-if="streaming" class="message assistant">
            <div class="message-avatar">
              <el-icon :size="20"><Cpu /></el-icon>
            </div>
            <div class="message-content">
              <div class="message-role">Agent</div>

              <!-- 实时执行步骤时间线 -->
              <div v-if="pendingSteps.length > 0 || !streamingContent" class="wf-timeline">
                <div
                  v-for="(step, idx) in pendingSteps" :key="idx"
                  class="wf-step"
                >
                  <div class="wf-step-header" @click="step._expanded = !step._expanded">
                    <span class="wf-badge" :class="'wf-badge--' + step.step_type">
                      {{ stepTypeLabel(step.step_type) }}
                    </span>
                    <span class="wf-step-name">{{ step.name }}</span>
                    <el-tag
                      v-if="step.status === 'success'" type="success" size="small" round
                    >{{ step.duration_ms }}ms</el-tag>
                    <el-tag
                      v-else-if="step.status === 'error'" type="danger" size="small" round
                    >failed</el-tag>
                    <el-icon class="wf-step-arrow" :class="{ expanded: step._expanded }">
                      <ArrowRight />
                    </el-icon>
                  </div>
                  <transition name="wf-slide">
                    <div v-if="step._expanded" class="wf-step-body">
                      <div v-if="step.input" class="wf-field">
                        <div class="wf-field-label">输入</div>
                        <pre class="wf-field-value">{{ truncateText(step.input, 500) }}</pre>
                      </div>
                      <div v-if="step.output" class="wf-field">
                        <div class="wf-field-label">输出</div>
                        <pre class="wf-field-value">{{ truncateText(step.output, 500) }}</pre>
                      </div>
                      <div v-if="step.error" class="wf-field">
                        <div class="wf-field-label wf-field-label--error">错误</div>
                        <pre class="wf-field-value wf-field-value--error">{{ step.error }}</pre>
                      </div>
                      <div v-if="step.metadata" class="wf-meta">
                        <span v-if="step.metadata.provider">Provider: {{ step.metadata.provider }}</span>
                        <span v-if="step.metadata.model">Model: {{ step.metadata.model }}</span>
                        <span v-if="step.metadata.tool_name">Tool: {{ step.metadata.tool_name }}</span>
                        <span v-if="step.metadata.skill_name">Skill: {{ step.metadata.skill_name }}</span>
                        <span v-if="step.metadata.skill_tools?.length">Tools: {{ step.metadata.skill_tools.join(', ') }}</span>
                      </div>
                    </div>
                  </transition>
                </div>

                <!-- 思考中指示器 -->
                <div v-if="!streamingContent" class="wf-step wf-step--thinking">
                  <div class="wf-step-header">
                    <span class="wf-badge wf-badge--thinking">
                      <el-icon class="is-loading"><Loading /></el-icon>
                    </span>
                    <span class="wf-step-name wf-step-name--muted">
                      {{ pendingSteps.length > 0 ? '生成回复中...' : '思考中...' }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- 流式文本 -->
              <div v-if="streamingContent" class="message-text">
                <span v-html="formatMessage(streamingContent)"></span>
                <span class="cursor-blink">|</span>
              </div>
            </div>
          </div>
        </template>
      </div>

      <div class="input-area">
        <!-- 待发送文件列表 -->
        <div v-if="pendingFiles.length > 0 || pendingURLs.length > 0" class="pending-files">
          <div v-for="(f, idx) in pendingFiles" :key="f.uuid" class="pending-file">
            <span class="pending-file-icon">{{ fileTypeIcon(f.file_type) }}</span>
            <span class="pending-file-name">{{ f.filename }}</span>
            <span class="pending-file-size">{{ formatFileSize(f.file_size) }}</span>
            <el-icon class="pending-file-remove" @click="removeFile(idx)"><Close /></el-icon>
          </div>
          <div v-for="(u, idx) in pendingURLs" :key="u" class="pending-file pending-url">
            <span class="pending-file-icon">🔗</span>
            <span class="pending-file-name" :title="u">{{ u.length > 50 ? u.slice(0, 50) + '...' : u }}</span>
            <el-icon class="pending-file-remove" @click="removeURL(idx)"><Close /></el-icon>
          </div>
        </div>
        <div v-if="showURLInput" class="url-input-row">
          <el-input
            v-model="urlInput"
            size="small"
            placeholder="输入文件 URL，按回车添加"
            @keydown.enter.prevent="addURL"
            clearable
          />
          <el-button size="small" type="primary" @click="addURL" :disabled="!urlInput.trim()">添加</el-button>
          <el-button size="small" @click="showURLInput = false; urlInput = ''">取消</el-button>
        </div>
        <div class="input-row">
          <label class="upload-btn" :class="{ disabled: !selectedAgentUUID || streaming || uploading }">
            <el-icon :size="18"><UploadFilled /></el-icon>
            <input
              type="file"
              multiple
              accept=".txt,.md,.json,.csv,.xml,.yaml,.yml,.log,.pdf,.docx,.doc,.xlsx,.xls,.png,.jpg,.jpeg,.gif,.webp"
              style="display: none"
              :disabled="!selectedAgentUUID || streaming || uploading"
              @change="handleFileUpload"
            />
          </label>
          <el-button
            class="url-btn"
            :class="{ disabled: !selectedAgentUUID || streaming }"
            :disabled="!selectedAgentUUID || streaming"
            @click="showURLInput = !showURLInput"
            circle
            size="small"
          >
            <el-icon :size="16"><Link /></el-icon>
          </el-button>
          <el-input
            v-model="inputMessage"
            type="textarea"
            :rows="2"
            placeholder="输入消息... (Enter 发送, Shift+Enter 换行)"
            :disabled="!selectedAgentUUID || streaming"
            @keydown="handleKeydown"
            resize="none"
          />
          <el-button
            type="primary"
            :disabled="!selectedAgentUUID || !inputMessage.trim() || streaming"
            :loading="streaming"
            @click="sendMessage"
            style="margin-left: 12px; height: 54px;"
          >
            <el-icon><Promotion /></el-icon>
          </el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick, watch, reactive } from 'vue'
import { agentApi, type Agent } from '../../api/agent'
import { streamChat, fileApi, type StreamChunk, type ExecutionStep, type FileInfo, type ChatFile } from '../../api/chat'
import { ElMessage } from 'element-plus'

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

onMounted(async () => {
  const res: any = await agentApi.list({ page: 1, page_size: 100 })
  agents.value = res.data?.list || []
  const first = agents.value[0]
  if (first && !selectedAgentUUID.value) {
    selectedAgentUUID.value = first.uuid
  }
})

watch(selectedAgentUUID, () => {
  newConversation()
})

function newConversation() {
  conversationId.value = ''
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

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function fileTypeIcon(type: string): string {
  switch (type) {
    case 'image': return '🖼'
    case 'document': return '📄'
    default: return '📝'
  }
}

function addURL() {
  const url = urlInput.value.trim()
  if (!url) return
  try {
    new URL(url)
  } catch {
    ElMessage.warning('请输入有效的 URL')
    return
  }
  if (pendingURLs.value.includes(url)) {
    ElMessage.warning('该 URL 已添加')
    return
  }
  pendingURLs.value.push(url)
  urlInput.value = ''
}

function removeURL(idx: number) {
  pendingURLs.value.splice(idx, 1)
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessage()
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (messagesArea.value) {
      messagesArea.value.scrollTop = messagesArea.value.scrollHeight
    }
  })
}

function sendMessage() {
  const text = inputMessage.value.trim()
  if (!text || !selectedAgentUUID.value) return

  const chatFiles: ChatFile[] = [
    ...pendingFiles.value.map(f => ({
      type: f.file_type as ChatFile['type'],
      transfer_method: 'local_file' as const,
      upload_file_id: f.uuid,
    })),
    ...pendingURLs.value.map(u => ({
      type: 'document' as const,
      transfer_method: 'remote_url' as const,
      url: u,
    })),
  ]

  const displayFiles: FileInfo[] = [
    ...pendingFiles.value.map(f => ({ ...f, id: 0, conversation_id: 0, message_id: 0, content_type: '', created_at: '' }) as FileInfo),
    ...pendingURLs.value.map(u => ({
      id: 0, uuid: u, conversation_id: 0, message_id: 0,
      filename: u.split('/').pop() || 'url', content_type: '',
      file_size: 0, file_type: 'text' as const, created_at: '',
    })),
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
    {
      agent_id: selectedAgentUUID.value,
      conversation_id: conversationId.value,
      message: text,
      files: chatFiles.length > 0 ? chatFiles : undefined,
    },
    (chunk: StreamChunk) => {
      if (chunk.conversation_id) {
        conversationId.value = chunk.conversation_id
      }
      if (chunk.delta) {
        streamingContent.value += chunk.delta
        scrollToBottom()
      }
      if (chunk.steps && chunk.steps.length > 0) {
        for (const s of chunk.steps) {
          pendingSteps.value.push(reactive({ ...s, _expanded: false }))
        }
      } else if (chunk.step) {
        pendingSteps.value.push(reactive({ ...chunk.step, _expanded: false }))
      }
      if (chunk.done) {
        messages.value.push(reactive({
          role: 'assistant',
          content: streamingContent.value,
          steps: [...pendingSteps.value],
          _showSteps: false,
        }))
        streamingContent.value = ''
        pendingSteps.value = []
        streaming.value = false
        scrollToBottom()
      }
    },
    () => {
      if (streaming.value && streamingContent.value) {
        messages.value.push(reactive({
          role: 'assistant',
          content: streamingContent.value,
          steps: [...pendingSteps.value],
          _showSteps: false,
        }))
        streamingContent.value = ''
        pendingSteps.value = []
      }
      streaming.value = false
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

function stepTagType(t: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  switch (t) {
    case 'llm_call': return ''
    case 'tool_call': return 'warning'
    case 'agent_call': return 'success'
    case 'skill_match': return 'info'
    default: return 'info'
  }
}

function truncateText(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen) + '...[truncated]'
}
</script>

<style scoped>
.chat-container {
  display: flex;
  gap: 16px;
  height: calc(100vh - 120px);
}
.chat-sidebar {
  width: 260px;
  flex-shrink: 0;
}
.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
}
.chat-main :deep(.el-card__body) {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0;
  overflow: hidden;
}
.messages-area {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #909399;
}
.empty-state p {
  margin-top: 12px;
  font-size: 14px;
}
.message {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}
.message-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.message.user .message-avatar {
  background-color: #409eff;
  color: #fff;
}
.message.assistant .message-avatar {
  background-color: #67c23a;
  color: #fff;
}
.message-role {
  font-size: 12px;
  color: #909399;
  margin-bottom: 4px;
}
.message-content {
  flex: 1;
  min-width: 0;
}
.message-text {
  background-color: #f4f4f5;
  border-radius: 8px;
  padding: 10px 14px;
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}
.message.user .message-text {
  background-color: #ecf5ff;
}
.input-area {
  padding: 12px 20px;
  border-top: 1px solid #e8e8e8;
}
.input-row {
  display: flex;
  align-items: flex-start;
}
.upload-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 54px;
  cursor: pointer;
  color: #606266;
  flex-shrink: 0;
  margin-right: 8px;
  border-radius: 6px;
  transition: all 0.2s;
}
.upload-btn:hover:not(.disabled) {
  color: #409eff;
  background-color: #ecf5ff;
}
.upload-btn.disabled {
  color: #c0c4cc;
  cursor: not-allowed;
}
.url-btn {
  flex-shrink: 0;
  margin-right: 8px;
  height: 54px !important;
  width: 36px !important;
  border: none;
  padding: 0;
}
.url-btn.disabled {
  color: #c0c4cc;
}
.url-input-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  align-items: center;
}
.url-input-row .el-input {
  flex: 1;
}
.pending-url .pending-file-name {
  color: #409eff;
  font-style: italic;
}
.pending-files {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}
.pending-file {
  display: flex;
  align-items: center;
  gap: 6px;
  background: #f4f4f5;
  border: 1px solid #e8e8e8;
  border-radius: 6px;
  padding: 4px 8px;
  font-size: 12px;
  color: #606266;
}
.pending-file-icon { font-size: 14px; }
.pending-file-name {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pending-file-size { color: #909399; }
.pending-file-remove {
  cursor: pointer;
  color: #909399;
  transition: color 0.2s;
}
.pending-file-remove:hover { color: #f56c6c; }
.msg-files {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}
.msg-file-img {
  max-width: 200px;
  max-height: 150px;
  border-radius: 8px;
  border: 1px solid #e8e8e8;
  cursor: pointer;
  transition: transform 0.2s;
}
.msg-file-img:hover { transform: scale(1.02); }
.msg-file-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  color: #409eff;
  text-decoration: none;
  transition: all 0.2s;
}
.msg-file-link:hover {
  border-color: #409eff;
  background: #ecf5ff;
}
.msg-file-size {
  color: #909399;
  font-size: 11px;
}

/* Steps Panel */
.steps-panel {
  margin-top: 8px;
}
.steps-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-size: 12px;
  color: #909399;
  padding: 4px 8px;
  border-radius: 4px;
  transition: background-color 0.2s;
}
.steps-toggle:hover {
  background-color: #f0f0f0;
  color: #606266;
}
.toggle-arrow {
  transition: transform 0.3s;
  margin-left: auto;
}
.toggle-arrow.expanded {
  transform: rotate(180deg);
}
.steps-list {
  margin-top: 8px;
  border: 1px solid #e8e8e8;
  border-radius: 8px;
  overflow: hidden;
}
.step-item {
  padding: 10px 14px;
  border-bottom: 1px solid #f0f0f0;
}
.step-item:last-child {
  border-bottom: none;
}
.step-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.step-name {
  font-size: 13px;
  font-weight: 500;
  color: #303133;
}
.step-duration {
  margin-left: auto;
  font-size: 12px;
  color: #909399;
}
.step-detail {
  font-size: 12px;
}
.step-section {
  margin-bottom: 4px;
}
.step-label {
  color: #909399;
  font-weight: 500;
}
.error-label {
  color: #f56c6c;
}
.step-code {
  background-color: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
  padding: 6px 8px;
  margin-top: 2px;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 200px;
  overflow-y: auto;
  font-family: 'SF Mono', 'Monaco', 'Menlo', monospace;
  font-size: 11px;
  line-height: 1.5;
}
.error-code {
  background-color: #fef0f0;
  border-color: #fde2e2;
  color: #f56c6c;
}
.step-meta {
  display: flex;
  gap: 12px;
  margin-top: 6px;
  font-size: 11px;
  color: #c0c4cc;
}

/* Transitions */
.slide-enter-active, .slide-leave-active {
  transition: all 0.3s ease;
  max-height: 2000px;
  overflow: hidden;
}
.slide-enter-from, .slide-leave-to {
  max-height: 0;
  opacity: 0;
}

.cursor-blink {
  animation: blink 1s infinite;
}
@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

/* ===== Dify-style Workflow Timeline ===== */
.wf-timeline {
  background: #f7f8fa;
  border-radius: 12px;
  padding: 6px;
  margin-bottom: 10px;
}
.wf-step {
  border-radius: 8px;
  margin-bottom: 2px;
  overflow: hidden;
}
.wf-step:last-child {
  margin-bottom: 0;
}
.wf-step-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background-color 0.15s;
}
.wf-step-header:hover {
  background-color: #eef0f3;
}
.wf-step--thinking .wf-step-header {
  cursor: default;
}
.wf-step--thinking .wf-step-header:hover {
  background-color: transparent;
}
.wf-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 36px;
  height: 24px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  color: #fff;
  padding: 0 6px;
  flex-shrink: 0;
}
.wf-badge--tool_call {
  background: linear-gradient(135deg, #ff9a44, #f57c00);
}
.wf-badge--llm_call {
  background: linear-gradient(135deg, #5b8def, #3370ff);
}
.wf-badge--agent_call {
  background: linear-gradient(135deg, #2dd4a8, #00b894);
}
.wf-badge--skill_match {
  background: linear-gradient(135deg, #a78bfa, #7c3aed);
}
.wf-badge--thinking {
  background: #c0c4cc;
  min-width: 24px;
  width: 24px;
  height: 24px;
  font-size: 14px;
}
.wf-step-name {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  color: #1d2129;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.wf-step-name--muted {
  color: #86909c;
  font-weight: 400;
}
.wf-step-arrow {
  color: #c0c4cc;
  font-size: 12px;
  transition: transform 0.2s;
  flex-shrink: 0;
}
.wf-step-arrow.expanded {
  transform: rotate(90deg);
}
.wf-step-body {
  padding: 4px 12px 12px 58px;
}
.wf-field {
  margin-bottom: 8px;
}
.wf-field:last-child {
  margin-bottom: 0;
}
.wf-field-label {
  font-size: 11px;
  color: #86909c;
  margin-bottom: 4px;
  font-weight: 500;
}
.wf-field-label--error {
  color: #f56c6c;
}
.wf-field-value {
  background: #fff;
  border: 1px solid #e5e6eb;
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.5;
  font-family: 'SF Mono', 'Monaco', 'Menlo', monospace;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 150px;
  overflow-y: auto;
  margin: 0;
  color: #1d2129;
}
.wf-field-value--error {
  background: #fef0f0;
  border-color: #fde2e2;
  color: #f56c6c;
}
.wf-meta {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: #c0c4cc;
  padding-top: 4px;
}

/* Workflow slide transition */
.wf-slide-enter-active, .wf-slide-leave-active {
  transition: all 0.25s ease;
  max-height: 500px;
  overflow: hidden;
}
.wf-slide-enter-from, .wf-slide-leave-to {
  max-height: 0;
  opacity: 0;
}

/* Step appear animation */
.wf-step {
  animation: wf-appear 0.3s ease;
}
@keyframes wf-appear {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
