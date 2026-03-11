package model

import "time"

// ============================================================
//  Planning & Goal
// ============================================================

type PlanStatus string

const (
	PlanActive    PlanStatus = "active"
	PlanCompleted PlanStatus = "completed"
	PlanFailed    PlanStatus = "failed"
)

type PlanStepStatus string

const (
	PlanStepPending   PlanStepStatus = "pending"
	PlanStepActive    PlanStepStatus = "active"
	PlanStepCompleted PlanStepStatus = "completed"
	PlanStepFailed    PlanStepStatus = "failed"
	PlanStepSkipped   PlanStepStatus = "skipped"
)

type Goal struct {
	Description     string   `json:"description"`
	SuccessCriteria []string `json:"success_criteria"`
}

type Plan struct {
	Goal     Goal       `json:"goal"`
	Steps    []PlanStep `json:"steps"`
	Status   PlanStatus `json:"status"`
	Revision int        `json:"revision"`
}

type PlanStep struct {
	ID          int            `json:"id"`
	Description string         `json:"description"`
	ToolHint    string         `json:"tool_hint,omitzero"`
	DependsOn   []int          `json:"depends_on,omitzero"`
	Status      PlanStepStatus `json:"status"`
	Result      string         `json:"result,omitzero"`
}

// NextPendingStep returns the first step whose status is pending
// and all dependencies are completed.
func (p *Plan) NextPendingStep() *PlanStep {
	for i := range p.Steps {
		if p.Steps[i].Status != PlanStepPending {
			continue
		}
		allMet := true
		for _, depID := range p.Steps[i].DependsOn {
			for j := range p.Steps {
				if p.Steps[j].ID == depID && p.Steps[j].Status != PlanStepCompleted {
					allMet = false
					break
				}
			}
			if !allMet {
				break
			}
		}
		if allMet {
			return &p.Steps[i]
		}
	}
	return nil
}

func (p *Plan) IsComplete() bool {
	for _, s := range p.Steps {
		if s.Status == PlanStepPending || s.Status == PlanStepActive {
			return false
		}
	}
	return true
}

func (p *Plan) CompletedSteps() []PlanStep {
	var result []PlanStep
	for _, s := range p.Steps {
		if s.Status == PlanStepCompleted {
			result = append(result, s)
		}
	}
	return result
}

func (p *Plan) UpdateStep(stepID int, status PlanStepStatus, result string) {
	for i := range p.Steps {
		if p.Steps[i].ID == stepID {
			p.Steps[i].Status = status
			p.Steps[i].Result = result
			return
		}
	}
}

// ============================================================
//  Reflection
// ============================================================

type Reflection struct {
	GoalMet     bool     `json:"goal_met"`
	Quality     int      `json:"quality"`
	Issues      []string `json:"issues,omitzero"`
	Suggestions []string `json:"suggestions,omitzero"`
	NeedReplan  bool     `json:"need_replan"`
	Summary     string   `json:"summary"`
}

// ============================================================
//  Thought (ReAct)
// ============================================================

type Thought struct {
	Reasoning   string `json:"reasoning"`
	NextAction  string `json:"next_action"`
	ActionInput string `json:"action_input,omitzero"`
	Confidence  int    `json:"confidence"`
}

// ============================================================
//  Long-term Memory
// ============================================================

type MemoryCategory string

const (
	MemoryFact       MemoryCategory = "fact"
	MemoryPreference MemoryCategory = "preference"
	MemoryExperience MemoryCategory = "experience"
	MemoryKnowledge  MemoryCategory = "knowledge"
)

type MemoryEntry struct {
	ID         int64          `json:"id"`
	AgentID    int64          `json:"agent_id"`
	UserID     string         `json:"user_id"`
	Content    string         `json:"content"`
	Category   MemoryCategory `json:"category"`
	Importance int            `json:"importance"`
	Keywords   string         `json:"keywords"`
	Source     string         `json:"source,omitzero"`
	CreatedAt  time.Time      `json:"created_at"`
}
