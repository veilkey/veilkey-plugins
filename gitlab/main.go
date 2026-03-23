package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/extism/go-pdk"
)

//go:wasmexport plugin_info
func pluginInfo() int32 {
	out, _ := json.Marshal(map[string]string{
		"name":        "gitlab",
		"version":     "1.0.0",
		"description": "GitLab CE/EE config management",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_init
func pluginInit() int32 {
	out, _ := json.Marshal(map[string]any{
		"paths": []string{"/etc/gitlab/gitlab.rb"},
		"hooks": []map[string]any{{
			"name": "reconfigure_gitlab", "cmd": []string{"gitlab-ctl", "reconfigure"}, "depends": []string{},
		}},
		"api_routes": []map[string]string{
			{"method": "GET", "path": "/config"},
			{"method": "POST", "path": "/config"},
		},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_destroy
func pluginDestroy() int32 {
	out, _ := json.Marshal(map[string]any{"cleaned_paths": []string{"/etc/gitlab/gitlab.rb"}})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_hooks
func pluginHooks() int32 {
	out, _ := json.Marshal([]map[string]any{{
		"name": "reconfigure_gitlab", "cmd": []string{"gitlab-ctl", "reconfigure"}, "depends": []string{},
	}})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_paths
func pluginPaths() int32 {
	out, _ := json.Marshal([]string{"/etc/gitlab/gitlab.rb"})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_validate
func pluginValidate() int32 {
	input := pdk.Input()
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	json.Unmarshal(input, &req)
	if !strings.Contains(req.Content, "external_url") {
		out, _ := json.Marshal(map[string]any{"ok": false, "error": "gitlab.rb must contain external_url"})
		pdk.Output(out)
		return 0
	}
	out, _ := json.Marshal(map[string]any{"ok": true})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_render
func pluginRender() int32 {
	input := pdk.Input()
	var req struct {
		Action string         `json:"action"`
		Input  map[string]any `json:"input"`
	}
	json.Unmarshal(input, &req)
	if req.Action == "generate_config" {
		return generateConfig(req.Input)
	}
	out, _ := json.Marshal(map[string]any{"error": "unknown action: " + req.Action})
	pdk.Output(out)
	return 1
}

func generateConfig(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString("# VeilKey-managed GitLab configuration\n\n")
	url, _ := input["external_url"].(string)
	if url == "" {
		out, _ := json.Marshal(map[string]any{"error": "external_url is required"})
		pdk.Output(out)
		return 1
	}
	b.WriteString(fmt.Sprintf("external_url '%s'\n\n", url))
	if rails, ok := input["gitlab_rails"].(map[string]any); ok {
		for k, v := range rails {
			b.WriteString(fmt.Sprintf("gitlab_rails['%s'] = %s\n", k, rubyVal(v)))
		}
		b.WriteString("\n")
	}
	if nginx, ok := input["nginx"].(map[string]any); ok {
		for k, v := range nginx {
			b.WriteString(fmt.Sprintf("nginx['%s'] = %s\n", k, rubyVal(v)))
		}
		b.WriteString("\n")
	}
	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func rubyVal(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func main() {}
