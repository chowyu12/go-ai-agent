package seed

import (
	"context"
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/store"
)

func Init(ctx context.Context, s store.Store) {
	seedTools(ctx, s)
	seedSkills(ctx, s)
	log.Info("seed data initialized")
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

func seedSkills(ctx context.Context, s store.Store) {
	for _, def := range defaultSkills() {
		existing, _, _ := s.ListSkills(ctx, model.ListQuery{Page: 1, PageSize: 1, Keyword: def.Name})
		for _, sk := range existing {
			if sk.Name == def.Name {
				goto next
			}
		}
		if err := s.CreateSkill(ctx, &def); err != nil {
			log.WithFields(log.Fields{"name": def.Name, "error": err}).Warn("seed skill failed")
		} else {
			log.WithField("name", def.Name).Info("seed skill created")
		}
	next:
	}
}

func mustJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func defaultTools() []model.Tool {
	return []model.Tool{
		{
			Name:        "current_time",
			Description: "获取当前系统时间，返回 ISO 8601 格式的时间字符串。无需输入参数。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "current_time",
				"description": "Get the current system time in ISO 8601 format",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			}),
		},
		{
			Name:        "calculator",
			Description: "数学计算器，支持基本的数学表达式计算。输入一个数学表达式字符串。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "calculator",
				"description": "Evaluate a mathematical expression",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expression": map[string]any{
							"type":        "string",
							"description": "The mathematical expression to evaluate, e.g. '2 + 3 * 4'",
						},
					},
					"required": []string{"expression"},
				},
			}),
		},
		{
			Name:        "uuid_generator",
			Description: "生成一个随机的 UUID v4 字符串。无需输入参数。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "uuid_generator",
				"description": "Generate a random UUID v4 string",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			}),
		},
		{
			Name:        "base64_encode",
			Description: "将输入文本进行 Base64 编码。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "base64_encode",
				"description": "Encode the input text to Base64",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text": map[string]any{
							"type":        "string",
							"description": "The text to encode",
						},
					},
					"required": []string{"text"},
				},
			}),
		},
		{
			Name:        "base64_decode",
			Description: "将 Base64 编码的字符串解码为原始文本。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "base64_decode",
				"description": "Decode a Base64 encoded string",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text": map[string]any{
							"type":        "string",
							"description": "The Base64 encoded text to decode",
						},
					},
					"required": []string{"text"},
				},
			}),
		},
		{
			Name:        "json_formatter",
			Description: "将 JSON 字符串格式化为带缩进的可读格式，同时验证 JSON 是否合法。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "json_formatter",
				"description": "Format and validate a JSON string with indentation",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"json_string": map[string]any{
							"type":        "string",
							"description": "The JSON string to format",
						},
					},
					"required": []string{"json_string"},
				},
			}),
		},
		{
			Name:        "weather",
			Description: "通过 HTTP 调用 wttr.in 获取指定城市的天气信息。输入城市名称（支持中英文）。",
			HandlerType: model.HandlerHTTP,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "weather",
				"description": "Get weather information for a city using wttr.in",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{
							"type":        "string",
							"description": "City name, e.g. 'Beijing', 'Shanghai', 'London'",
						},
					},
					"required": []string{"city"},
				},
			}),
			HandlerConfig: mustJSON(model.HTTPHandlerConfig{
				URL:    "https://wttr.in/{city}?format=j1",
				Method: "GET",
				Headers: map[string]string{
					"Accept-Language": "zh-CN",
				},
			}),
		},
		{
			Name:        "ip_lookup",
			Description: "查询 IP 地址的地理位置信息。不传参数则返回本机公网 IP 信息。",
			HandlerType: model.HandlerHTTP,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "ip_lookup",
				"description": "Look up geographic information for an IP address",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"ip": map[string]any{
							"type":        "string",
							"description": "IP address to look up (leave empty for your own IP)",
						},
					},
				},
			}),
			HandlerConfig: mustJSON(model.HTTPHandlerConfig{
				URL:    "http://ip-api.com/json/{ip}?lang=zh-CN",
				Method: "GET",
			}),
		},
		{
			Name:        "url_reader",
			Description: "读取指定 URL 的网页内容（纯文本形式）。",
			HandlerType: model.HandlerHTTP,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "url_reader",
				"description": "Read the content of a URL and return as plain text",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"url": map[string]any{
							"type":        "string",
							"description": "The URL to read",
						},
					},
					"required": []string{"url"},
				},
			}),
			HandlerConfig: mustJSON(model.HTTPHandlerConfig{
				URL:    "{url}",
				Method: "GET",
				Headers: map[string]string{
					"User-Agent": "Mozilla/5.0 (compatible; AIAgent/1.0)",
				},
			}),
		},
		{
			Name:        "hash_text",
			Description: "对输入文本进行哈希计算，支持 MD5、SHA1、SHA256。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "hash_text",
				"description": "Compute hash of the input text, supports md5, sha1, sha256",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text": map[string]any{
							"type":        "string",
							"description": "The text to hash",
						},
						"algorithm": map[string]any{
							"type":        "string",
							"description": "Hash algorithm: md5, sha1, or sha256",
							"enum":        []string{"md5", "sha1", "sha256"},
						},
					},
					"required": []string{"text"},
				},
			}),
		},
		{
			Name:        "random_number",
			Description: "生成指定范围内的随机整数。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "random_number",
				"description": "Generate a random integer within a specified range",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"min": map[string]any{
							"type":        "integer",
							"description": "Minimum value (inclusive), default 1",
						},
						"max": map[string]any{
							"type":        "integer",
							"description": "Maximum value (inclusive), default 100",
						},
					},
				},
			}),
		},
		{
			Name:        "shell_exec",
			Description: "在本地服务器上执行 Shell 命令并返回输出结果。支持任意命令，超时 30 秒。",
			HandlerType: model.HandlerCommand,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "shell_exec",
				"description": "Execute a shell command on the local server and return the output",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The shell command to execute, e.g., 'ls -la', 'date', 'whoami'",
						},
					},
					"required": []string{"command"},
				},
			}),
			HandlerConfig: mustJSON(model.CommandHandlerConfig{
				Command: "{command}",
				Timeout: 30,
			}),
		},
		{
			Name:        "disk_usage",
			Description: "查看服务器磁盘使用情况。",
			HandlerType: model.HandlerCommand,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "disk_usage",
				"description": "Check disk usage of the server",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			}),
			HandlerConfig: mustJSON(model.CommandHandlerConfig{
				Command: "df -h",
				Timeout: 10,
			}),
		},
		{
			Name:        "system_info",
			Description: "获取服务器系统信息，包括主机名、系统版本、运行时间、负载等。",
			HandlerType: model.HandlerCommand,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "system_info",
				"description": "Get server system information including hostname, OS version, uptime and load",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			}),
			HandlerConfig: mustJSON(model.CommandHandlerConfig{
				Command: "uname -a && uptime",
				Timeout: 10,
			}),
		},
		{
			Name:        "list_files",
			Description: "列出指定目录下的文件和目录。",
			HandlerType: model.HandlerCommand,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "list_files",
				"description": "List files and directories in the specified path",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The directory path to list, e.g., '/tmp', '.'",
						},
					},
					"required": []string{"path"},
				},
			}),
			HandlerConfig: mustJSON(model.CommandHandlerConfig{
				Command: "ls -lah {path}",
				Timeout: 10,
			}),
		},
		{
			Name:        "read_file",
			Description: "读取指定文件的内容。",
			HandlerType: model.HandlerCommand,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "read_file",
				"description": "Read the content of a file",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The file path to read",
						},
					},
					"required": []string{"path"},
				},
			}),
			HandlerConfig: mustJSON(model.CommandHandlerConfig{
				Command: "cat {path}",
				Timeout: 10,
			}),
		},
	}
}

func defaultSkills() []model.Skill {
	return []model.Skill{
		{
			Name:        "翻译助手",
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
