package seed

import "github.com/chowyu12/go-ai-agent/internal/model"

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
			Description: "浏览器控制工具，支持网页导航、截图、元素快照与交互、表单填充、Cookie/Storage管理、Console/Network监控、设备仿真等操作。先用 snapshot 获取页面元素引用，再通过 ref 执行点击、输入等操作。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			Timeout:     120,
			FunctionDef: mustJSON(browserToolDef()),
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
		{
			Name: "code_interpreter",
			Description: "代码解释器，支持编写并执行 Python/JavaScript/Shell 代码。" +
				"Agent 传入语言类型和代码，工具自动在沙箱目录中创建文件并执行，返回 stdout/stderr 结果。" +
				"适用于数据处理、数学计算、文件生成、API 调试、格式转换等场景。",
			HandlerType: model.HandlerBuiltin,
			Enabled:     true,
			Timeout:     120,
			FunctionDef: mustJSON(map[string]any{
				"name": "code_interpreter",
				"description": "Execute code in a sandboxed environment. Supports Python, JavaScript, and Shell. " +
					"Write code to solve problems like data processing, math computation, file generation, API testing, and format conversion. " +
					"The code is saved to a sandbox directory and executed with the appropriate runtime. " +
					"Returns stdout, stderr, exit code, and execution duration.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"language": map[string]any{
							"type":        "string",
							"enum":        []string{"python", "javascript", "shell"},
							"description": "Programming language: python (python3), javascript (node), or shell (sh)",
						},
						"code": map[string]any{
							"type":        "string",
							"description": "The source code to execute",
						},
						"timeout": map[string]any{
							"type":        "integer",
							"description": "Execution timeout in seconds (default: 60 for python/js, 30 for shell, max: 120)",
						},
					},
					"required": []string{"language", "code"},
				},
			}),
		},
	}
}

func browserToolDef() map[string]any {
	allActions := []string{
		"navigate", "screenshot", "snapshot", "get_text", "evaluate", "pdf",
		"click", "type", "hover", "drag", "select", "fill_form", "scroll",
		"upload", "wait", "dialog", "tabs", "open_tab", "close_tab", "close",
		"console", "network", "cookies", "storage", "press",
		"back", "forward", "reload",
		"extract_table", "resize",
		"set_device", "set_media", "highlight",
	}

	return map[string]any{
		"name": "browser",
		"description": "Browser automation tool. Actions: " +
			"navigate/back/forward/reload (navigation), " +
			"snapshot (get interactive elements with refs), " +
			"click/type/press/hover/drag/select/fill_form/scroll (interaction), " +
			"screenshot/pdf/get_text/extract_table (data extraction), " +
			"console/network (monitoring), " +
			"cookies/storage (state management), " +
			"resize/set_device/set_media (emulation), " +
			"highlight (debugging), " +
			"evaluate (run JS), " +
			"wait (wait for condition), " +
			"tabs/open_tab/close_tab/close (tab management), " +
			"dialog/upload (misc). " +
			"Use 'snapshot' first to see elements with refs like 'e1', then use refs for click/type/etc.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        allActions,
					"description": "Action to perform",
				},
				"url":           map[string]any{"type": "string", "description": "URL for navigate/open_tab"},
				"ref":           map[string]any{"type": "string", "description": "Element ref from snapshot (e.g. 'e1')"},
				"text":          map[string]any{"type": "string", "description": "Text to type"},
				"expression":    map[string]any{"type": "string", "description": "JavaScript expression for evaluate"},
				"selector":      map[string]any{"type": "string", "description": "CSS selector (alternative to ref)"},
				"full_page":     map[string]any{"type": "boolean", "description": "Full page screenshot"},
				"submit":        map[string]any{"type": "boolean", "description": "Press Enter after typing"},
				"slowly":        map[string]any{"type": "boolean", "description": "Type character by character"},
				"button":        map[string]any{"type": "string", "enum": []string{"left", "right", "middle"}, "description": "Mouse button for click"},
				"double_click":  map[string]any{"type": "boolean", "description": "Double-click"},
				"start_ref":     map[string]any{"type": "string", "description": "Drag start element ref"},
				"end_ref":       map[string]any{"type": "string", "description": "Drag end element ref"},
				"values":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Select option values"},
				"fields":        map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"ref": map[string]any{"type": "string"}, "value": map[string]any{"type": "string"}, "type": map[string]any{"type": "string"}}}, "description": "Form fields [{ref,value,type}]"},
				"target_id":     map[string]any{"type": "string", "description": "Tab ID from tabs action"},
				"wait_time":     map[string]any{"type": "integer", "description": "Wait milliseconds"},
				"wait_text":     map[string]any{"type": "string", "description": "Wait for text to appear on page"},
				"wait_selector": map[string]any{"type": "string", "description": "Wait for CSS selector to become visible"},
				"wait_url":      map[string]any{"type": "string", "description": "Wait for URL to contain string"},
				"wait_fn":       map[string]any{"type": "string", "description": "JS expression to poll until truthy (e.g. 'window.ready===true')"},
				"wait_load":     map[string]any{"type": "string", "enum": []string{"networkidle", "domcontentloaded", "load"}, "description": "Wait for page load state"},
				"accept":        map[string]any{"type": "boolean", "description": "Accept (true) or dismiss (false) dialog"},
				"prompt_text":   map[string]any{"type": "string", "description": "Prompt dialog input text"},
				"paths":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "File paths for upload"},
				"scroll_y":      map[string]any{"type": "integer", "description": "Scroll to Y offset (pixels, 0=bottom)"},
				"level":         map[string]any{"type": "string", "enum": []string{"error", "warn", "info", "log"}, "description": "Console log level filter"},
				"filter":        map[string]any{"type": "string", "description": "URL keyword filter for network requests"},
				"clear":         map[string]any{"type": "boolean", "description": "Clear buffer after reading (console/network)"},
				"operation":     map[string]any{"type": "string", "enum": []string{"get", "set", "clear"}, "description": "Operation for cookies/storage (default: get)"},
				"cookie_name":   map[string]any{"type": "string", "description": "Cookie name for set"},
				"cookie_value":  map[string]any{"type": "string", "description": "Cookie value for set"},
				"cookie_url":    map[string]any{"type": "string", "description": "Cookie URL scope for set"},
				"cookie_domain": map[string]any{"type": "string", "description": "Cookie domain for set"},
				"storage_type":  map[string]any{"type": "string", "enum": []string{"local", "session"}, "description": "Storage type (default: local)"},
				"key":           map[string]any{"type": "string", "description": "Storage key for get/set"},
				"value":         map[string]any{"type": "string", "description": "Storage value for set"},
				"key_name":      map[string]any{"type": "string", "description": "Key name for press (Enter/Tab/Escape/Backspace/ArrowUp/Down/Left/Right/Space/F1-F12 or single char)"},
				"width":         map[string]any{"type": "integer", "description": "Viewport width for resize"},
				"height":        map[string]any{"type": "integer", "description": "Viewport height for resize"},
				"device":        map[string]any{"type": "string", "description": "Device name for set_device (e.g. 'iPhone 14', 'iPad', 'Pixel 7')"},
				"color_scheme":  map[string]any{"type": "string", "enum": []string{"dark", "light", "no-preference"}, "description": "Color scheme for set_media"},
			},
			"required": []string{"action"},
		},
	}
}
