package memory

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/provider"
	"github.com/chowyu12/go-ai-agent/internal/tool"
)

type LongTermMemory struct {
	store Store
}

func NewLongTermMemory(store Store) *LongTermMemory {
	return &LongTermMemory{store: store}
}

func (m *LongTermMemory) Recall(ctx context.Context, agentID int64, userID, query string, topK int) []model.MemoryEntry {
	if m == nil || m.store == nil {
		return nil
	}
	entries, err := m.store.Search(ctx, agentID, userID, query, topK)
	if err != nil {
		log.WithError(err).Warn("[Memory] recall failed")
		return nil
	}
	return entries
}

const memoryExtractPrompt = `从以下对话中提取值得长期记忆的关键信息。

用户: %s
助手: %s

输出JSON数组:
[{"content":"关键信息","category":"fact|preference|experience|knowledge","importance":3,"keywords":"关键词1,关键词2"}]

规则:
- 只提取有长期价值的信息(用户偏好、重要知识、经验教训)
- importance: 1=低价值 3=中等 5=高价值
- 没有值得记忆的内容则返回空数组 []
只输出JSON数组。`

func (m *LongTermMemory) ExtractAndStore(ctx context.Context, llm provider.LLMProvider, modelName string, agentID int64, userID, source, userMsg, response string) {
	if m == nil || m.store == nil {
		return
	}

	systemPrompt := fmt.Sprintf(memoryExtractPrompt,
		tool.Truncate(userMsg, 2000), tool.Truncate(response, 2000))

	content, err := provider.Complete(ctx, llm, modelName, systemPrompt, "请提取记忆")
	if err != nil {
		log.WithError(err).Debug("[Memory] extract failed")
		return
	}

	entries := parseMemoryEntries(content)
	for i := range entries {
		entries[i].AgentID = agentID
		entries[i].UserID = userID
		entries[i].Source = source
		if err := m.store.Save(ctx, &entries[i]); err != nil {
			log.WithError(err).Warn("[Memory] save entry failed")
		}
	}

	if len(entries) > 0 {
		log.WithField("count", len(entries)).Info("[Memory] new memories stored")
	}
}

func parseMemoryEntries(raw string) []model.MemoryEntry {
	cleaned := provider.ExtractJSON(raw)
	var entries []model.MemoryEntry
	if err := json.Unmarshal([]byte(cleaned), &entries); err != nil {
		return nil
	}
	return entries
}
