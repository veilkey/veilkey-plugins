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
		"name": "soulflow-sync", "version": "1.0.0",
		"description": "SoulFlow Orchestrator config sync",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_init
func pluginInit() int32 {
	out, _ := json.Marshal(map[string]any{
		"paths": []string{
			"/root/workspace/.env",
			"/root/workspace/.agents/.claude/.credentials.json",
			"/root/workspace/.agents/.claude.json",
			"/root/workspace/.agents/.codex/auth.json",
		},
		"hooks": []map[string]any{
			{"name": "restart_soulflow", "cmd": []string{"docker", "compose", "-p", "soulflow-dev", "restart", "orchestrator"}, "depends": []string{}},
		},
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
	out, _ := json.Marshal(map[string]any{
		"cleaned_paths": []string{"/root/workspace/.env"},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_hooks
func pluginHooks() int32 {
	out, _ := json.Marshal([]map[string]any{
		{"name": "restart_soulflow", "cmd": []string{"docker", "compose", "-p", "soulflow-dev", "restart", "orchestrator"}, "depends": []string{}},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_paths
func pluginPaths() int32 {
	out, _ := json.Marshal([]string{
		"/root/workspace/.env",
		"/root/workspace/.agents/.claude/.credentials.json",
		"/root/workspace/.agents/.claude.json",
		"/root/workspace/.agents/.codex/auth.json",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_validate
func pluginValidate() int32 {
	input := pdk.Input()
	var req struct{ Path, Content string }
	json.Unmarshal(input, &req)

	if strings.HasSuffix(req.Path, ".env") {
		// .env 기본 검증 — 빈 줄/주석 제외 후 KEY=VALUE 형식
		for _, line := range strings.Split(req.Content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") { continue }
			if !strings.Contains(line, "=") {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": "invalid .env line: " + line})
				pdk.Output(out)
				return 0
			}
		}
	}

	if strings.HasSuffix(req.Path, ".json") {
		var js any
		if err := json.Unmarshal([]byte(req.Content), &js); err != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "invalid JSON: " + err.Error()})
			pdk.Output(out)
			return 0
		}
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

	switch req.Action {
	case "generate_env":
		return generateEnv(req.Input)
	case "generate_provider":
		return generateProvider(req.Input)
	default:
		out, _ := json.Marshal(map[string]any{"error": "unknown action: " + req.Action})
		pdk.Output(out)
		return 1
	}
}

func generateEnv(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString("# VeilKey-managed SoulFlow configuration\n\n")

	keys := []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"SLACK_BOT_TOKEN", "MATTERMOST_BOT_TOKEN", "MATTERMOST_URL",
		"SOULFLOW_ADMIN_PASSWORD", "ROOT_PASSWORD", "ADMIN_PASSWORD",
	}
	for _, k := range keys {
		if v, ok := input[strings.ToLower(k)]; ok {
			b.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		} else if v, ok := input[k]; ok {
			b.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}

	// 추가 키
	if extra, ok := input["extra"].(map[string]any); ok {
		b.WriteString("\n# Extra configuration\n")
		for k, v := range extra {
			b.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}

	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func generateProvider(input map[string]any) int32 {
	providerType, _ := input["type"].(string)
	instanceID, _ := input["instance_id"].(string)
	if instanceID == "" { instanceID = "orchestrator_llm" }

	provider := map[string]any{
		"instance_id":   instanceID,
		"provider_type": providerType,
		"label":         input["label"],
		"enabled":       true,
		"settings":      input["settings"],
	}

	raw, _ := json.MarshalIndent(provider, "", "  ")
	out, _ := json.Marshal(map[string]any{"output": string(raw)})
	pdk.Output(out)
	return 0
}

func main() {}
