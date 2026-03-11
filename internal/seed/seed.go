package seed

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/store"
	"github.com/chowyu12/go-ai-agent/internal/tool"
	"github.com/chowyu12/go-ai-agent/internal/workspace"
)

func Init(ctx context.Context, s store.Store, registry *tool.Registry) {
	seedTools(ctx, s)
	seedSkillDirs(ctx, s)
	registry.LoadDefaults()
	log.Info("seed data initialized (DB + tool registry)")
}

func seedTools(ctx context.Context, s store.Store) {
	for _, def := range defaultTools() {
		existing, _, _ := s.ListTools(ctx, model.ListQuery{Page: 1, PageSize: 1, Keyword: def.Name})
		for _, t := range existing {
			if t.Name == def.Name {
				goto next
			}
		}
		if err := s.CreateTool(ctx, &def); err != nil {
			log.WithFields(log.Fields{"name": def.Name, "error": err}).Warn("seed tool failed")
		} else {
			log.WithField("name", def.Name).Info("seed tool created")
		}
	next:
	}
}

func seedSkillDirs(ctx context.Context, s store.Store) {
	skillsDir := workspace.Skills()
	if skillsDir == "" {
		log.Warn("workspace not initialized, falling back to DB-only seed")
		seedSkillsLegacy(ctx, s)
		return
	}

	for _, def := range builtinSkillDefs() {
		dirPath := filepath.Join(skillsDir, def.DirName)

		if _, err := os.Stat(filepath.Join(dirPath, "manifest.json")); err == nil {
			syncSkillToDB(ctx, s, def)
			continue
		}

		os.MkdirAll(dirPath, 0o755)

		manifest := model.SkillManifest{
			Name:        def.Name,
			Version:     "1.0.0",
			Description: def.Description,
			Author:      "system",
		}
		if data, err := json.MarshalIndent(manifest, "", "  "); err == nil {
			os.WriteFile(filepath.Join(dirPath, "manifest.json"), data, 0o644)
		}
		if def.Instruction != "" {
			os.WriteFile(filepath.Join(dirPath, "SKILL.md"), []byte(def.Instruction), 0o644)
		}

		syncSkillToDB(ctx, s, def)
		log.WithField("name", def.Name).Info("seed skill dir created")
	}
}

func syncSkillToDB(ctx context.Context, s store.Store, def builtinSkill) {
	existing, err := s.GetSkillByDirName(ctx, def.DirName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	}

	if existing != nil {
		return
	}

	existingByName, _, _ := s.ListSkills(ctx, model.ListQuery{Page: 1, PageSize: 1, Keyword: def.Name})
	for _, sk := range existingByName {
		if sk.Name == def.Name {
			src := model.SkillSourceLocal
			dirName := def.DirName
			version := "1.0.0"
			author := "system"
			req := model.UpdateSkillReq{
				Source:  &src,
				DirName: &dirName,
				Version: &version,
				Author:  &author,
			}
			s.UpdateSkill(ctx, sk.ID, req)
			return
		}
	}

	sk := &model.Skill{
		Name:        def.Name,
		Description: def.Description,
		Instruction: def.Instruction,
		Source:      model.SkillSourceLocal,
		Version:     "1.0.0",
		Author:      "system",
		DirName:     def.DirName,
		Enabled:     true,
	}
	if err := s.CreateSkill(ctx, sk); err != nil {
		log.WithFields(log.Fields{"name": def.Name, "error": err}).Warn("seed skill to DB failed")
	}
}

func seedSkillsLegacy(ctx context.Context, s store.Store) {
	for _, def := range builtinSkillDefs() {
		existing, _, _ := s.ListSkills(ctx, model.ListQuery{Page: 1, PageSize: 1, Keyword: def.Name})
		for _, sk := range existing {
			if sk.Name == def.Name {
				goto next
			}
		}
		{
			sk := &model.Skill{
				Name:        def.Name,
				Description: def.Description,
				Instruction: def.Instruction,
				Source:      model.SkillSourceCustom,
				Enabled:     true,
			}
			if err := s.CreateSkill(ctx, sk); err != nil {
				log.WithFields(log.Fields{"name": def.Name, "error": err}).Warn("seed skill failed")
			} else {
				log.WithField("name", def.Name).Info("seed skill created")
			}
		}
	next:
	}
}

func mustJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

type builtinSkill struct {
	Name        string
	DirName     string
	Description string
	Instruction string
}

func builtinSkillDefs() []builtinSkill {
	return []builtinSkill{
		{
			Name:        "定时任务",
			DirName:     "cron-task",
			Description: "根据用户的自然语言描述，自动生成 Shell 脚本并配置 cron 定时执行。",
			Instruction: `你是一个 Linux 定时任务专家。用户会用自然语言描述他们想定时执行的任务，你需要：

## 工作流程

1. **理解需求**：确认用户要做什么、多久执行一次、在哪个时区
2. **生成 cron 表达式**：用 cron_parser 工具验证表达式并展示执行时间，让用户确认
3. **编写脚本**：用 crontab 工具的 save_script 动作保存脚本
4. **安装定时任务**：用 crontab 工具的 add_job 动作注册到 crontab

## 脚本编写规范

- 开头加 set -euo pipefail，遇到错误立即停止
- 关键操作前后加 echo 打印进度日志（带时间戳）
- 涉及文件操作时先检查路径是否存在
- 涉及清理/删除操作时一定要加安全校验（路径非空、非根目录等）
- 需要的环境变量在脚本顶部用变量声明，便于修改
- 添加简要注释说明脚本用途

## 输出格式

完成后汇总告知用户：
- 脚本路径
- Cron 表达式及含义
- 接下来几次执行时间
- 日志文件路径（如果启用了 log_output）
- 如何查看/修改/删除该定时任务

## 安全原则

- 不要执行 rm -rf / 等危险命令
- 清理任务要限定明确的目录范围
- 涉及数据库操作建议先备份
- 建议用户 add_job 时开启 log_output 以便排查问题`,
		},
		{
			Name:        "翻译助手",
			DirName:     "translator",
			Description: "多语言翻译技能，能够将文本在中英日韩等多种语言之间互译。",
			Instruction: `你是一个专业的多语言翻译专家。请遵循以下规则：
1. 自动检测输入文本的语言
2. 如果输入是中文，默认翻译为英文；如果输入是其他语言，默认翻译为中文
3. 用户可以指定目标语言
4. 保持原文的语气、风格和格式
5. 对于专业术语，在翻译后用括号标注原文
6. 如果原文有歧义，提供多个翻译版本并解释差异`,
		},
		{
			Name:        "代码审查",
			DirName:     "code-review",
			Description: "代码审查技能，能够分析代码质量、发现潜在问题并提出改进建议。",
			Instruction: `你是一个资深的代码审查专家。审查代码时请关注：
1. **代码质量**：命名规范、代码结构、可读性
2. **潜在 Bug**：空指针、数组越界、并发安全、资源泄漏
3. **性能问题**：不必要的内存分配、低效算法、N+1 查询
4. **安全隐患**：SQL 注入、XSS、敏感信息泄漏
5. **最佳实践**：是否遵循语言惯用法和设计模式
6. **测试覆盖**：是否有足够的单元测试

输出格式：
- 严重程度标记：[严重] [警告] [建议] [优化]
- 指出具体行号和代码片段
- 给出修改建议和示例代码`,
		},
		{
			Name:        "文章摘要",
			DirName:     "summarizer",
			Description: "文章摘要技能，能够将长文本提炼为简洁的摘要。",
			Instruction: `你是一个专业的文本摘要专家。请遵循以下规则：
1. 提取文章的核心观点和关键信息
2. 摘要长度控制在原文的 20% 以内
3. 保持客观中立，不添加个人观点
4. 按重要性排序，最重要的信息放在前面
5. 如果是技术文章，保留关键的技术细节
6. 输出格式：
   - 一句话摘要
   - 核心要点（3-5 条）
   - 关键数据或结论`,
		},
		{
			Name:        "写作助手",
			DirName:     "writing-assistant",
			Description: "通用写作助手，帮助用户撰写、润色、改写各类文本内容。",
			Instruction: `你是一个专业的写作助手。你可以帮助：
1. **撰写内容**：邮件、报告、文案、技术文档
2. **润色修改**：改善表达、修正语法、优化结构
3. **风格转换**：正式/非正式、学术/通俗、简洁/详细
4. **改写重述**：用不同方式表达相同意思，避免重复

写作原则：
- 清晰简洁，避免冗余
- 逻辑连贯，结构合理
- 根据目标读者调整语言风格
- 注意标点和格式规范`,
		},
		{
			Name:        "数据分析",
			DirName:     "data-analyst",
			Description: "数据分析技能，帮助用户解读数据、发现规律、生成分析报告。",
			Instruction: `你是一个专业的数据分析师。你可以帮助：
1. **数据解读**：解释数据含义、识别趋势和异常
2. **统计分析**：计算均值、中位数、标准差等统计量
3. **对比分析**：多组数据的横向/纵向对比
4. **可视化建议**：推荐合适的图表类型和展示方式
5. **结论提炼**：从数据中提取可操作的洞察

分析框架：
- 数据概览（样本量、时间范围、维度）
- 关键发现（趋势、异常、相关性）
- 深度分析（原因推测、影响评估）
- 行动建议（基于数据的可执行建议）`,
		},
		{
			Name:        "SQL 助手",
			DirName:     "sql-assistant",
			Description: "SQL 查询助手，帮助编写、优化和解释 SQL 查询语句。",
			Instruction: `你是一个资深的数据库专家。你可以帮助：
1. **编写 SQL**：根据自然语言描述生成 SQL 查询
2. **优化 SQL**：分析查询性能，建议索引和重写方案
3. **解释 SQL**：将复杂 SQL 转换为自然语言描述
4. **表设计**：数据库表结构设计和规范化建议

注意事项：
- 默认使用 MySQL 语法
- 避免 SELECT *，明确指定字段
- 注意 SQL 注入防护
- 大数据量查询注意分页和索引
- 联表查询不超过 3 张表`,
		},
	}
}
