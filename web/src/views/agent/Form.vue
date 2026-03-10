<template>
  <div class="agent-form-page">
    <el-page-header @back="goBack">
      <template #content>
        <span class="page-title">{{
          isEdit ? "编辑 Agent" : "新增 Agent"
        }}</span>
      </template>
    </el-page-header>

    <el-card shadow="never" style="margin-top: 20px" v-loading="pageLoading">
      <el-form :model="form" label-width="120px" style="max-width: 720px">
        <el-divider content-position="left">基本信息</el-divider>

        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="Agent 名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>

        <el-divider content-position="left">模型配置</el-divider>

        <el-form-item label="模型供应商" required>
          <el-select
            v-model="form.provider_id"
            placeholder="选择供应商"
            filterable
            style="width: 100%"
            @change="onProviderChange"
          >
            <el-option
              v-for="p in providers"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            >
              <span>{{ p.name }}</span>
              <el-tag size="small" style="margin-left: 8px" type="info">{{
                p.type
              }}</el-tag>
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
            <el-option-group
              v-if="remoteModels.length > 0"
              label="远程模型 (API)"
            >
              <el-option
                v-for="m in remoteModels"
                :key="'r-' + m"
                :label="m"
                :value="m"
              />
            </el-option-group>
            <el-option-group
              v-if="localOnlyModels.length > 0"
              :label="remoteModels.length > 0 ? '本地配置' : '模型列表'"
            >
              <el-option
                v-for="m in localOnlyModels"
                :key="'l-' + m"
                :label="m"
                :value="m"
              />
            </el-option-group>
          </el-select>
          <div
            v-if="remoteFetchError"
            style="font-size: 12px; color: #e6a23c; margin-top: 2px"
          >
            {{ remoteFetchError }}
          </div>
        </el-form-item>
        <el-form-item label="System Prompt">
          <el-input
            v-model="form.system_prompt"
            type="textarea"
            :rows="4"
            placeholder="系统提示词"
          />
        </el-form-item>

        <el-divider content-position="left">参数设置</el-divider>

        <el-form-item label="温度">
          <el-slider
            v-model="form.temperature"
            :min="0"
            :max="2"
            :step="0.1"
            show-input
            style="padding-right: 16px"
          />
        </el-form-item>
        <el-row :gutter="24">
          <el-col :span="12">
            <el-form-item label="Max Tokens">
              <el-input-number
                v-model="form.max_tokens"
                :min="1"
                :max="128000"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="超时(秒)">
              <el-input-number
                v-model="form.timeout"
                :min="0"
                style="width: 100%"
              />
              <div class="form-hint">0 表示不限制超时</div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="24">
          <el-col :span="12">
            <el-form-item label="历史条数">
              <el-input-number
                v-model="form.max_history"
                :min="1"
                :max="500"
                style="width: 100%"
              />
              <div class="form-hint">会话上下文最大消息数，默认 15</div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="最大迭代">
              <el-input-number
                v-model="form.max_iterations"
                :min="1"
                :max="50"
                style="width: 100%"
              />
              <div class="form-hint">工具调用轮次上限，默认 10</div>
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">API Token</el-divider>

        <el-form-item label="Token">
          <div
            style="display: flex; align-items: center; gap: 8px; width: 100%"
          >
            <template v-if="form.token">
              <code class="token-display">{{ form.token }}</code>
              <el-tooltip :content="copied ? '已复制' : '复制'" placement="top">
                <el-button
                  :type="copied ? 'success' : 'default'"
                  link
                  @click="copyToken(form.token)"
                >
                  <el-icon :size="16"
                    ><Select v-if="copied" /><DocumentCopy v-else
                  /></el-icon>
                </el-button>
              </el-tooltip>
              <el-popconfirm
                title="重置后旧 Token 将失效，确定重置？"
                @confirm="handleResetToken"
              >
                <template #reference>
                  <el-button type="warning" link>
                    <el-icon :size="16"><RefreshRight /></el-icon>
                  </el-button>
                </template>
              </el-popconfirm>
            </template>
            <template v-else-if="isEdit">
              <span style="color: #c0c4cc; font-size: 13px">尚未生成</span>
              <el-button type="primary" size="small" @click="handleResetToken">
                <el-icon><Key /></el-icon> 生成
              </el-button>
            </template>
            <template v-else>
              <span style="color: #909399; font-size: 13px"
                >创建后自动生成</span
              >
            </template>
          </div>
          <div class="form-hint">
            使用此 Token 通过 API 直接调用 Agent，无需 JWT 登录
          </div>
        </el-form-item>

        <el-divider content-position="left">关联配置</el-divider>

        <el-form-item label="关联工具">
          <el-select
            v-model="form.tool_ids"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            :max-collapse-tags="3"
            placeholder="搜索并选择工具"
            style="width: 100%"
          >
            <el-option
              v-for="t in allTools"
              :key="t.id"
              :label="t.name"
              :value="t.id"
            >
              <div
                style="
                  display: flex;
                  justify-content: space-between;
                  align-items: center;
                "
              >
                <span>{{ t.name }}</span>
                <span class="option-desc">{{ t.description }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="关联技能">
          <el-select
            v-model="form.skill_ids"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            :max-collapse-tags="3"
            placeholder="搜索并选择技能"
            style="width: 100%"
          >
            <el-option
              v-for="s in allSkills"
              :key="s.id"
              :label="s.name"
              :value="s.id"
            >
              <div
                style="
                  display: flex;
                  justify-content: space-between;
                  align-items: center;
                "
              >
                <span>{{ s.name }}</span>
                <span class="option-desc">{{ s.description }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="MCP 服务">
          <el-select
            v-model="form.mcp_server_ids"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            :max-collapse-tags="3"
            placeholder="搜索并选择 MCP 服务"
            style="width: 100%"
          >
            <el-option
              v-for="m in allMcpServers"
              :key="m.id"
              :label="m.name"
              :value="m.id"
            >
              <div
                style="
                  display: flex;
                  justify-content: space-between;
                  align-items: center;
                "
              >
                <span>{{ m.name }}</span>
                <span class="option-desc">{{ m.transport }} · {{ m.endpoint }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="子 Agent">
          <el-select
            v-model="form.child_ids"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            :max-collapse-tags="3"
            placeholder="搜索并选择子 Agent"
            style="width: 100%"
          >
            <el-option
              v-for="a in availableAgents"
              :key="a.id"
              :label="a.name"
              :value="a.id"
            >
              <div
                style="
                  display: flex;
                  justify-content: space-between;
                  align-items: center;
                "
              >
                <span>{{ a.name }}</span>
                <span class="option-desc">{{ a.model_name }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">
            {{ isEdit ? "保存" : "创建" }}
          </el-button>
          <el-button @click="goBack">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import {
  DocumentCopy,
  Key,
  RefreshRight,
  Select,
} from "@element-plus/icons-vue";
import { agentApi, type Agent } from "../../api/agent";
import { providerApi, type Provider } from "../../api/provider";
import { toolApi, type Tool } from "../../api/tool";
import { skillApi, type Skill } from "../../api/skill";
import { mcpApi, type McpServer } from "../../api/mcp";

const route = useRoute();
const router = useRouter();

const agentId = computed(() => {
  const id = route.params.id;
  return id ? Number(id) : null;
});
const isEdit = computed(() => !!agentId.value);

const pageLoading = ref(false);
const submitting = ref(false);
const copied = ref(false);

const form = ref<any>({
  name: "",
  description: "",
  system_prompt: "",
  provider_id: null,
  model_name: "",
  temperature: 0.7,
  max_tokens: 2048,
  timeout: 0,
  max_history: 50,
  max_iterations: 10,
  tool_ids: [],
  skill_ids: [],
  child_ids: [],
  mcp_server_ids: [],
});

const providers = ref<Provider[]>([]);
const allTools = ref<Tool[]>([]);
const allSkills = ref<Skill[]>([]);
const allAgents = ref<Agent[]>([]);
const allMcpServers = ref<McpServer[]>([]);

const providerModels = ref<string[]>([]);
const remoteModels = ref<string[]>([]);
const remoteFetched = ref(false);
const remoteFetchError = ref("");
const modelLoading = ref(false);

const localOnlyModels = computed(() => {
  const remoteSet = new Set(remoteModels.value);
  return providerModels.value.filter((m) => !remoteSet.has(m));
});

const availableAgents = computed(() =>
  allAgents.value.filter((a) => a.id !== form.value.id),
);

function goBack() {
  router.push({ name: "Agents" });
}

async function loadOptions() {
  const [p, t, s, a, m] = await Promise.all([
    providerApi.list({ page: 1, page_size: 100 }),
    toolApi.list({ page: 1, page_size: 100 }),
    skillApi.list({ page: 1, page_size: 100 }),
    agentApi.list({ page: 1, page_size: 100 }),
    mcpApi.list({ page: 1, page_size: 100 }),
  ]);
  providers.value = (p as any).data?.list || [];
  allTools.value = (t as any).data?.list || [];
  allSkills.value = (s as any).data?.list || [];
  allAgents.value = (a as any).data?.list || [];
  allMcpServers.value = (m as any).data?.list || [];
}

async function loadProviderModels(providerId: number) {
  if (!providerId) {
    providerModels.value = [];
    return;
  }
  modelLoading.value = true;
  try {
    const res: any = await providerApi.models(providerId);
    providerModels.value = res.data || [];
  } catch {
    providerModels.value = [];
  } finally {
    modelLoading.value = false;
  }
}

async function onProviderChange(providerId: number) {
  form.value.model_name = "";
  remoteModels.value = [];
  remoteFetched.value = false;
  remoteFetchError.value = "";
  await loadProviderModels(providerId);
}

async function onModelFocus() {
  if (!form.value.provider_id || remoteFetched.value || modelLoading.value)
    return;
  modelLoading.value = true;
  remoteFetchError.value = "";
  try {
    const res: any = await providerApi.remoteModels(form.value.provider_id);
    remoteModels.value = res.data || [];
    remoteFetched.value = true;
  } catch {
    remoteFetchError.value = "远程模型拉取失败，可手动输入模型名称";
    remoteFetched.value = true;
  } finally {
    modelLoading.value = false;
  }
}

async function loadAgent() {
  if (!agentId.value) return;
  pageLoading.value = true;
  try {
    const res: any = await agentApi.get(agentId.value);
    const detail = res.data;
    form.value = {
      ...detail,
      max_history: detail.max_history || 50,
      max_iterations: detail.max_iterations || 10,
      tool_ids: detail.tools?.map((t: any) => t.id) || [],
      skill_ids: detail.skills?.map((s: any) => s.id) || [],
      child_ids: detail.children?.map((a: any) => a.id) || [],
      mcp_server_ids: detail.mcp_servers?.map((m: any) => m.id) || [],
    };
    if (detail.provider_id) {
      await loadProviderModels(detail.provider_id);
    }
  } finally {
    pageLoading.value = false;
  }
}

async function handleSubmit() {
  submitting.value = true;
  try {
    if (isEdit.value) {
      await agentApi.update(agentId.value!, form.value);
      ElMessage.success("更新成功");
      goBack();
    } else {
      const res: any = await agentApi.create(form.value);
      const created = res.data;
      ElMessage.success("创建成功");
      router.replace({ name: "AgentEdit", params: { id: created.id } });
    }
  } finally {
    submitting.value = false;
  }
}

function copyToken(token: string) {
  if (!token) return;
  navigator.clipboard.writeText(token).then(() => {
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  });
}

async function handleResetToken() {
  if (!form.value.id) return;
  try {
    const res: any = await agentApi.resetToken(form.value.id);
    form.value.token = res.data?.token || "";
    ElMessage.success("Token 已重置");
  } catch {
    ElMessage.error("重置 Token 失败");
  }
}

onMounted(async () => {
  await loadOptions();
  await loadAgent();
});
</script>

<style scoped>
.agent-form-page {
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
.option-desc {
  color: #909399;
  font-size: 12px;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.token-display {
  font-family: "SF Mono", "Cascadia Code", "Fira Code", Consolas, monospace;
  font-size: 13px;
  color: #303133;
  background: #f4f4f5;
  padding: 6px 12px;
  border-radius: 4px;
  user-select: all;
  word-break: break-all;
  line-height: 1.6;
  flex: 1;
}
</style>
