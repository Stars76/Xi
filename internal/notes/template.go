package notes

import (
	"regexp"
	"strings"

	"github.com/Stars76/526Hz/internal/store"
)

type Template struct {
	Name string
	Text string
}

var DefaultTemplates = map[string]Template{
	"daily": {
		Name: "每日复盘",
		Text: strings.Join([]string{
			"# 每日复盘 · {{date_cn}} {{weekday}}",
			"",
			"> 专注 {{focus_minutes}} 分钟 · {{focus_rounds}} 轮 · 完成任务 {{tasks_done_count}} 件",
			"",
			"## 时间轴",
			"{{#timeline}}- `{{start}}–{{end}}` {{title}}（{{minutes}} 分钟）",
			"{{/timeline}}{{^timeline}}- 今天还没有专注记录",
			"{{/timeline}}",
			"## 完成清单",
			"{{#tasks_done}}- [x] {{title}}",
			"{{/tasks_done}}{{^tasks_done}}- 今天还没有完成的任务",
			"{{/tasks_done}}",
		}, "\n"),
	},
	"task": {
		Name: "单任务记录",
		Text: strings.Join([]string{
			"# {{task_title}}",
			"",
			"> {{date_cn}} · 累计专注 {{task_minutes}} 分钟",
			"",
			"## 专注时段",
			"{{#timeline}}- `{{start}}–{{end}}` {{minutes}} 分钟",
			"{{/timeline}}",
		}, "\n"),
	},
	"weekly": {
		Name: "周报",
		Text: strings.Join([]string{
			"# 周报 · {{week_since}} ~ {{week_until}}",
			"",
			"> 专注 {{focus_minutes}} 分钟 · {{focus_rounds}} 轮 · 完成任务 {{tasks_done_count}} 件 · 连续 {{streak}} 天",
			"",
			"## 最常专注时段",
			"- {{peak_range}}",
			"",
			"## 完成清单",
			"{{#tasks_done}}- [x] {{title}}（{{minutes}} 分钟）",
			"{{/tasks_done}}",
		}, "\n"),
	},
}

type RenderData struct {
	Vars  map[string]string
	Lists map[string][]map[string]string
}

var (
	eachBlockRe  = regexp.MustCompile(`\{\{#(\w+)\}\}([\s\S]*?)\{\{/\w+\}\}`)
	emptyBlockRe = regexp.MustCompile(`\{\{\^(\w+)\}\}([\s\S]*?)\{\{/\w+\}\}`)
	blockTagRe   = regexp.MustCompile(`\{\{([#^])(\w+)\}\}([\s\S]*?)\{\{/(\w+)\}\}`)
	varRe        = regexp.MustCompile(`\{\{(\w+)\}\}`)
)

func renderVars(tpl string, vars map[string]string) string {
	return varRe.ReplaceAllStringFunc(tpl, func(m string) string {
		name := m[2 : len(m)-2]
		if v, ok := vars[name]; ok {
			return v
		}
		return m
	})
}

func Render(tpl string, data RenderData) string {
	vars := data.Vars
	if vars == nil {
		vars = map[string]string{}
	}
	lists := data.Lists
	if lists == nil {
		lists = map[string][]map[string]string{}
	}
	out := tpl
	for pass := 0; pass < 3; pass++ {
		next := replaceMatchingBlocks(out, vars, lists)
		if next == out {
			break
		}
		out = next
	}
	return renderVars(out, vars)
}

func replaceMatchingBlocks(src string, vars map[string]string, lists map[string][]map[string]string) string {
	var sb strings.Builder
	pos := 0
	for pos < len(src) {
		openIdx := strings.Index(src[pos:], "{{")
		if openIdx < 0 {
			sb.WriteString(src[pos:])
			break
		}
		openIdx += pos
		if openIdx+3 >= len(src) || (src[openIdx+2] != '#' && src[openIdx+2] != '^') {
			sb.WriteString(src[pos : openIdx+2])
			pos = openIdx + 2
			continue
		}
		kind := src[openIdx+2]
		endOpen := strings.Index(src[openIdx+3:], "}}")
		if endOpen < 0 {
			sb.WriteString(src[pos:])
			break
		}
		name := src[openIdx+3 : openIdx+3+endOpen]
		if !isWord(name) {
			sb.WriteString(src[pos : openIdx+2])
			pos = openIdx + 2
			continue
		}
		closeTag := "{{/" + name + "}}"
		bodyStart := openIdx + 3 + endOpen + 2
		closeIdx := strings.Index(src[bodyStart:], closeTag)
		if closeIdx < 0 {
			sb.WriteString(src[pos : openIdx+2])
			pos = openIdx + 2
			continue
		}
		closeIdx += bodyStart
		body := src[bodyStart:closeIdx]
		sb.WriteString(src[pos:openIdx])

		list, exists := lists[name]
		if !exists {
			sb.WriteString(src[openIdx : closeIdx+len(closeTag)])
		} else if kind == '#' {
			for _, item := range list {
				merged := make(map[string]string, len(vars)+len(item))
				for k, v := range vars {
					merged[k] = v
				}
				for k, v := range item {
					merged[k] = v
				}
				sb.WriteString(renderVars(body, merged))
			}
		} else {
			if len(list) == 0 {
				sb.WriteString(renderVars(body, vars))
			}
		}
		pos = closeIdx + len(closeTag)
	}
	return sb.String()
}

func isWord(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func Pick(stored *store.ExportTemplate, kind string) Template {
	if stored != nil && stored.Templates != nil {
		if t, ok := stored.Templates[kind]; ok && t.Text != "" {
			name := t.Name
			if name == "" {
				name = kind
			}
			return Template{Name: name, Text: t.Text}
		}
	}
	return DefaultTemplates[kind]
}
