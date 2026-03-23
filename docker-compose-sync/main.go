package main

import (
	"encoding/json"
	"strings"

	"github.com/extism/go-pdk"
)

//go:wasmexport plugin_info
func pluginInfo() int32 {
	out, _ := json.Marshal(map[string]string{
		"name": "docker-compose-sync", "version": "1.0.0",
		"description": "Docker Compose service management",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_init
func pluginInit() int32 {
	input := pdk.Input()
	var cfg struct {
		ServiceDir string `json:"service_dir"`
		Project    string `json:"project"`
	}
	json.Unmarshal(input, &cfg)
	if cfg.ServiceDir == "" { cfg.ServiceDir = "/opt/service" }
	if cfg.Project == "" { cfg.Project = "service" }

	out, _ := json.Marshal(map[string]any{
		"paths": []string{cfg.ServiceDir + "/.env", cfg.ServiceDir + "/docker-compose.yml"},
		"hooks": []map[string]any{
			{"name": "recreate_" + cfg.Project, "cmd": []string{"docker", "compose", "-f", cfg.ServiceDir + "/docker-compose.yml", "up", "-d", "--force-recreate"}, "depends": []string{}},
			{"name": "down_" + cfg.Project, "cmd": []string{"docker", "compose", "-f", cfg.ServiceDir + "/docker-compose.yml", "down"}, "depends": []string{}},
		},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_destroy
func pluginDestroy() int32 {
	out, _ := json.Marshal(map[string]any{"cleaned_paths": []string{}})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_hooks
func pluginHooks() int32 {
	out, _ := json.Marshal([]map[string]any{
		{"name": "recreate", "cmd": []string{"docker", "compose", "up", "-d", "--force-recreate"}, "depends": []string{}},
		{"name": "down", "cmd": []string{"docker", "compose", "down"}, "depends": []string{}},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_paths
func pluginPaths() int32 {
	out, _ := json.Marshal([]string{"/opt/service/.env", "/opt/service/docker-compose.yml"})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_validate
func pluginValidate() int32 {
	input := pdk.Input()
	var req struct{ Path, Content string }
	json.Unmarshal(input, &req)

	if strings.HasSuffix(req.Path, ".env") {
		for _, line := range strings.Split(req.Content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") { continue }
			if !strings.Contains(line, "=") {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": "invalid .env: " + line})
				pdk.Output(out)
				return 0
			}
		}
	}
	if strings.HasSuffix(req.Path, ".yml") && strings.Contains(req.Content, "\t") {
		out, _ := json.Marshal(map[string]any{"ok": false, "error": "YAML must not contain tabs"})
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

	if req.Action == "generate_env" {
		return renderEnv(req.Input)
	}
	out, _ := json.Marshal(map[string]any{"error": "unknown action"})
	pdk.Output(out)
	return 1
}

func renderEnv(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString("# VeilKey-managed Docker Compose environment\n\n")
	if vars, ok := input["vars"].(map[string]any); ok {
		for k, v := range vars {
			b.WriteString(k + "=" + toString(v) + "\n")
		}
	}
	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func toString(v any) string {
	switch val := v.(type) {
	case string: return val
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func main() {}
