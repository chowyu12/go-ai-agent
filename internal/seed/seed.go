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
			Description: "读取指定 URL 的网页内容。优先通过 HTTP 直接获取，失败时自动回退到浏览器渲染提取文本。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			Timeout:     60,
			FunctionDef: mustJSON(map[string]any{
				"name":        "url_reader",
				"description": "Read the text content of a URL. Automatically extracts text from webpages, supports both static and dynamically rendered pages.",
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
			Name:        "cron_parser",
			Description: "解析 Cron 表达式，验证合法性并计算接下来的执行时间。支持标准 5 字段、6 字段（含秒）及 @daily/@hourly/@every 等描述符。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "cron_parser",
				"description": "Parse and validate a cron expression, show next scheduled execution times. Supports standard 5-field (minute hour dom month dow), 6-field with seconds, and descriptors like @daily, @hourly, @every 5m.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expression": map[string]any{
							"type":        "string",
							"description": "Cron expression, e.g. '*/5 * * * *', '0 9 * * 1-5', '@daily', '@every 30m'",
						},
						"count": map[string]any{
							"type":        "integer",
							"description": "Number of next execution times to show, default 5, max 20",
						},
						"timezone": map[string]any{
							"type":        "string",
							"description": "Timezone for display, e.g. 'Asia/Shanghai', 'UTC'. Default: server local timezone",
						},
					},
					"required": []string{"expression"},
				},
			}),
		},
		{
			Name:        "crontab",
			Description: "定时任务管理工具。支持保存脚本、添加/查看/删除 crontab 定时任务。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			FunctionDef: mustJSON(map[string]any{
				"name":        "crontab",
				"description": "Manage cron jobs and shell scripts. Actions: save_script (create executable script), add_job (add crontab entry), list_jobs (show current crontab), remove_job (remove entry by pattern).",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"save_script", "add_job", "list_jobs", "remove_job"},
							"description": "save_script: create a shell script; add_job: add crontab entry; list_jobs: show current crontab; remove_job: remove matching entries",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Script name (for save_script), e.g. 'backup_db', auto-appends .sh",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Shell script content (for save_script). Shebang added automatically if missing.",
						},
						"expression": map[string]any{
							"type":        "string",
							"description": "Cron expression (for add_job), e.g. '0 9 * * *', '*/5 * * * *'",
						},
						"command": map[string]any{
							"type":        "string",
							"description": "Command to schedule (for add_job), typically the script path from save_script",
						},
						"pattern": map[string]any{
							"type":        "string",
							"description": "Text pattern to match crontab entries for removal (for remove_job)",
						},
						"log_output": map[string]any{
							"type":        "boolean",
							"description": "Auto-redirect stdout/stderr to log file (for add_job), default false",
						},
					},
					"required": []string{"action"},
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
			Name:        "browser",
			Description: "浏览器控制工具，支持网页导航、截图、元素快照与交互、表单填充等操作。先用 snapshot 获取页面元素引用，再通过 ref 执行点击、输入等操作。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			Timeout:     120,
			FunctionDef: mustJSON(map[string]any{
				"name":        "browser",
				"description": "Browser automation tool. Use 'snapshot' to see interactive elements with refs, then use refs for click/type/etc.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"navigate", "screenshot", "snapshot", "get_text", "evaluate", "pdf", "click", "type", "hover", "drag", "select", "fill_form", "scroll", "upload", "wait", "dialog", "tabs", "open_tab", "close_tab", "close"},
							"description": "Action to perform: navigate(url), snapshot(get elements), screenshot, click(ref), type(ref+text), hover(ref), drag(start_ref+end_ref), select(ref+values), fill_form(fields), scroll(ref/scroll_y), wait(wait_time/wait_text/wait_selector/wait_url), evaluate(expression), pdf, upload(ref+paths), dialog(accept), tabs, open_tab(url), close_tab(target_id), close",
						},
						"url":           map[string]any{"type": "string", "description": "URL for navigate/open_tab"},
						"ref":           map[string]any{"type": "string", "description": "Element ref from snapshot (e.g. 'e1')"},
						"text":          map[string]any{"type": "string", "description": "Text to type"},
						"expression":    map[string]any{"type": "string", "description": "JavaScript expression for evaluate"},
						"selector":      map[string]any{"type": "string", "description": "CSS selector (alternative to ref)"},
						"full_page":     map[string]any{"type": "boolean", "description": "Full page screenshot"},
						"submit":        map[string]any{"type": "boolean", "description": "Press Enter after typing"},
						"slowly":        map[string]any{"type": "boolean", "description": "Type character by character"},
						"button":        map[string]any{"type": "string", "enum": []string{"left", "right", "middle"}, "description": "Mouse button (default: left)"},
						"double_click":  map[string]any{"type": "boolean", "description": "Double-click"},
						"start_ref":     map[string]any{"type": "string", "description": "Drag start element ref"},
						"end_ref":       map[string]any{"type": "string", "description": "Drag end element ref"},
						"values":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Select option values"},
						"fields":        map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"ref": map[string]any{"type": "string"}, "value": map[string]any{"type": "string"}, "type": map[string]any{"type": "string"}}}, "description": "Form fields [{ref,value,type}]"},
						"target_id":     map[string]any{"type": "string", "description": "Tab ID from tabs action"},
						"wait_time":     map[string]any{"type": "integer", "description": "Wait milliseconds"},
						"wait_text":     map[string]any{"type": "string", "description": "Wait for text to appear"},
						"wait_selector": map[string]any{"type": "string", "description": "Wait for CSS selector to match"},
						"wait_url":      map[string]any{"type": "string", "description": "Wait for URL to contain string"},
						"accept":        map[string]any{"type": "boolean", "description": "Accept (true) or dismiss (false) dialog"},
						"prompt_text":   map[string]any{"type": "string", "description": "Prompt dialog input text"},
						"paths":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "File paths for upload"},
						"scroll_y":      map[string]any{"type": "integer", "description": "Scroll to Y offset (pixels)"},
					},
					"required": []string{"action"},
				},
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
			Name:        "定时任务",
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
