package model

type AgentTool struct {
	AgentID int64 `gorm:"primaryKey"`
	ToolID  int64 `gorm:"primaryKey"`
}

func (AgentTool) TableName() string { return "agent_tools" }

type AgentSkill struct {
	AgentID int64 `gorm:"primaryKey"`
	SkillID int64 `gorm:"primaryKey"`
}

func (AgentSkill) TableName() string { return "agent_skills" }

type AgentMCPServer struct {
	AgentID     int64 `gorm:"primaryKey"`
	MCPServerID int64 `gorm:"primaryKey"`
}

func (AgentMCPServer) TableName() string { return "agent_mcp_servers" }

type SkillTool struct {
	SkillID int64 `gorm:"primaryKey"`
	ToolID  int64 `gorm:"primaryKey"`
}

func (SkillTool) TableName() string { return "skill_tools" }
