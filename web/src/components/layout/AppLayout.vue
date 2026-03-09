<template>
  <el-container class="app-layout">
    <el-aside :width="isCollapse ? '64px' : '220px'" class="app-aside">
      <div class="logo">
        <el-icon :size="24"><Cpu /></el-icon>
        <span v-show="!isCollapse" class="logo-text">AI Agent</span>
      </div>
      <el-menu
        :default-active="route.path"
        :collapse="isCollapse"
        router
        class="app-menu"
        background-color="#1d1e2c"
        text-color="#a3a6b7"
        active-text-color="#409eff"
      >
        <el-menu-item index="/">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>概览</template>
        </el-menu-item>
        <el-menu-item index="/providers">
          <el-icon><Connection /></el-icon>
          <template #title>模型供应商</template>
        </el-menu-item>
        <el-menu-item index="/agents">
          <el-icon><UserFilled /></el-icon>
          <template #title>Agent 管理</template>
        </el-menu-item>
        <el-menu-item index="/tools">
          <el-icon><SetUp /></el-icon>
          <template #title>工具管理</template>
        </el-menu-item>
        <el-menu-item index="/skills">
          <el-icon><MagicStick /></el-icon>
          <template #title>技能管理</template>
        </el-menu-item>
        <el-menu-item index="/mcp-servers">
          <el-icon><Promotion /></el-icon>
          <template #title>MCP 服务</template>
        </el-menu-item>
        <el-menu-item index="/chat">
          <el-icon><ChatDotRound /></el-icon>
          <template #title>对话测试</template>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-icon><Document /></el-icon>
          <template #title>执行日志</template>
        </el-menu-item>
        <el-menu-item v-if="authStore.isAdmin" index="/users">
          <el-icon><Avatar /></el-icon>
          <template #title>用户管理</template>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="app-header">
        <el-icon class="collapse-btn" @click="isCollapse = !isCollapse" :size="20">
          <Fold v-if="!isCollapse" />
          <Expand v-else />
        </el-icon>
        <div class="header-right">
          <span class="header-title">AI Agent 管理平台</span>
          <div class="user-area">
            <el-tag :type="authStore.isAdmin ? 'primary' : 'info'" size="small" class="role-tag">
              {{ authStore.isAdmin ? '超管' : '访客' }}
            </el-tag>
            <span class="username">{{ authStore.user?.username }}</span>
            <el-button text type="danger" @click="handleLogout">退出</el-button>
          </div>
        </div>
      </el-header>
      <el-main class="app-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const isCollapse = ref(false)

function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body, #app { height: 100%; }
</style>

<style scoped>
.app-layout {
  height: 100vh;
}
.app-aside {
  background-color: #1d1e2c;
  transition: width 0.3s;
  overflow: hidden;
}
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #fff;
  font-size: 18px;
  font-weight: 600;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}
.logo-text {
  white-space: nowrap;
}
.app-menu {
  border-right: none;
}
.app-header {
  display: flex;
  align-items: center;
  background: #fff;
  border-bottom: 1px solid #e8e8e8;
  padding: 0 20px;
  height: 60px;
}
.collapse-btn {
  cursor: pointer;
  color: #666;
}
.collapse-btn:hover {
  color: #409eff;
}
.header-right {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-left: 16px;
}
.header-title {
  font-size: 16px;
  font-weight: 500;
  color: #333;
}
.user-area {
  display: flex;
  align-items: center;
  gap: 8px;
}
.role-tag {
  font-size: 12px;
}
.username {
  font-size: 14px;
  color: #666;
}
.app-main {
  background-color: #f5f7fa;
  padding: 20px;
  overflow-y: auto;
}
</style>
