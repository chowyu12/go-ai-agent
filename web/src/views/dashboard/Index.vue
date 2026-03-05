<template>
  <div class="dashboard">
    <el-row :gutter="20" v-loading="statsLoading">
      <el-col :span="6" v-for="card in cards" :key="card.title">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-card-body">
            <div class="stat-info">
              <div class="stat-title">{{ card.title }}</div>
              <div class="stat-value">{{ card.value }}</div>
            </div>
            <el-icon :size="40" :color="card.color">
              <component :is="card.icon" />
            </el-icon>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="welcome-card" shadow="hover" style="margin-top: 20px;">
      <template #header>
        <span style="font-weight: 600">快速开始</span>
      </template>
      <el-steps :active="0" align-center>
        <el-step title="配置 Provider" description="添加 LLM 模型供应商" />
        <el-step title="创建 Tools" description="注册可用的工具" />
        <el-step title="创建 Skills" description="定义技能和指令" />
        <el-step title="创建 Agent" description="组装智能体" />
        <el-step title="开始对话" description="在 Playground 测试" />
      </el-steps>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { providerApi } from '../../api/provider'
import { agentApi } from '../../api/agent'
import { toolApi } from '../../api/tool'
import { skillApi } from '../../api/skill'

const statsLoading = ref(true)
const cards = ref([
  { title: '模型供应商', value: 0, icon: 'Connection', color: '#409eff' },
  { title: 'Agent', value: 0, icon: 'UserFilled', color: '#67c23a' },
  { title: '工具', value: 0, icon: 'SetUp', color: '#e6a23c' },
  { title: '技能', value: 0, icon: 'MagicStick', color: '#f56c6c' },
])

onMounted(async () => {
  try {
    const [providers, agents, tools, skills] = await Promise.all([
      providerApi.list({ page: 1, page_size: 1 }),
      agentApi.list({ page: 1, page_size: 1 }),
      toolApi.list({ page: 1, page_size: 1 }),
      skillApi.list({ page: 1, page_size: 1 }),
    ])
    cards.value[0]!.value = (providers as any).data?.total || 0
    cards.value[1]!.value = (agents as any).data?.total || 0
    cards.value[2]!.value = (tools as any).data?.total || 0
    cards.value[3]!.value = (skills as any).data?.total || 0
  } catch {
    ElMessage.warning('统计数据加载失败')
  } finally {
    statsLoading.value = false
  }
})
</script>

<style scoped>
.stat-card-body {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.stat-title {
  font-size: 14px;
  color: #909399;
  margin-bottom: 8px;
}
.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
}
</style>
