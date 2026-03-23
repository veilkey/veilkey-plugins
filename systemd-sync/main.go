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
		"name": "systemd-sync", "version": "1.0.0",
		"description": "Systemd service management — daemon-reload, unit override, restart",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_init
func pluginInit() int32 {
	out, _ := json.Marshal(map[string]any{
		"paths": []string{"/etc/systemd/system"},
		"hooks": []map[string]any{
			{"name": "reload_systemd", "cmd": []string{"systemctl", "daemon-reload"}, "depends": []string{}},
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
		{"name": "reload_systemd", "cmd": []string{"systemctl", "daemon-reload"}, "depends": []string{}},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_paths
func pluginPaths() int32 {
	out, _ := json.Marshal([]string{"/etc/systemd/system"})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_validate
func pluginValidate() int32 {
	input := pdk.Input()
	var req struct{ Path, Content string }
	json.Unmarshal(input, &req)

	if strings.HasSuffix(req.Path, ".service") || strings.Contains(req.Path, "override.conf") {
		if !strings.Contains(req.Content, "[Service]") && !strings.Contains(req.Content, "[Unit]") {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "systemd unit must contain [Service] or [Unit] section"})
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
	case "generate_override":
		return generateOverride(req.Input)
	case "generate_service":
		return generateService(req.Input)
	default:
		out, _ := json.Marshal(map[string]any{"error": "unknown action: " + req.Action})
		pdk.Output(out)
		return 1
	}
}

func generateOverride(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString("[Service]\n")
	if env, ok := input["environment"].(map[string]any); ok {
		for k, v := range env {
			b.WriteString(fmt.Sprintf("Environment=%s=%s\n", k, v))
		}
	}
	if envFile, ok := input["environment_file"].(string); ok {
		b.WriteString(fmt.Sprintf("EnvironmentFile=%s\n", envFile))
	}
	if restart, ok := input["restart"].(string); ok {
		b.WriteString(fmt.Sprintf("Restart=%s\n", restart))
	}
	if limit, ok := input["limit_nofile"].(float64); ok {
		b.WriteString(fmt.Sprintf("LimitNOFILE=%d\n", int64(limit)))
	}
	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func generateService(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString("[Unit]\n")
	if desc, ok := input["description"].(string); ok {
		b.WriteString(fmt.Sprintf("Description=%s\n", desc))
	}
	if after, ok := input["after"].(string); ok {
		b.WriteString(fmt.Sprintf("After=%s\n", after))
	}
	b.WriteString("\n[Service]\n")
	if execStart, ok := input["exec_start"].(string); ok {
		b.WriteString(fmt.Sprintf("ExecStart=%s\n", execStart))
	}
	if user, ok := input["user"].(string); ok {
		b.WriteString(fmt.Sprintf("User=%s\n", user))
	}
	if workDir, ok := input["working_directory"].(string); ok {
		b.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", workDir))
	}
	b.WriteString("Restart=on-failure\n")
	b.WriteString("\n[Install]\n")
	b.WriteString("WantedBy=multi-user.target\n")
	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func main() {}
