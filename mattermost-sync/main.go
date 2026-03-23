package main

import (
	"encoding/json"
	"strings"

	"github.com/extism/go-pdk"
)

//go:wasmexport plugin_info
func pluginInfo() int32 {
	out, _ := json.Marshal(map[string]string{
		"name": "mattermost-sync", "version": "1.0.0",
		"description": "Mattermost config.json management + restart hook",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_init
func pluginInit() int32 {
	out, _ := json.Marshal(map[string]any{
		"paths": []string{
			"/opt/mattermost/config/config.json",
			"/opt/mattermost/.env",
			"/etc/systemd/system/mattermost.service.d/override.conf",
		},
		"hooks": []map[string]any{
			{"name": "restart_mattermost", "cmd": []string{"systemctl", "restart", "mattermost"}, "depends": []string{"systemd-sync:reload_systemd"}},
		},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_destroy
func pluginDestroy() int32 {
	out, _ := json.Marshal(map[string]any{"cleaned_paths": []string{"/opt/mattermost/config/config.json", "/opt/mattermost/.env"}})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_hooks
func pluginHooks() int32 {
	out, _ := json.Marshal([]map[string]any{
		{"name": "restart_mattermost", "cmd": []string{"systemctl", "restart", "mattermost"}, "depends": []string{"systemd-sync:reload_systemd"}},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_paths
func pluginPaths() int32 {
	out, _ := json.Marshal([]string{"/opt/mattermost/config/config.json", "/opt/mattermost/.env", "/etc/systemd/system/mattermost.service.d/override.conf"})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_validate
func pluginValidate() int32 {
	input := pdk.Input()
	var req struct{ Path, Content string }
	json.Unmarshal(input, &req)

	if strings.HasSuffix(req.Path, "config.json") {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(req.Content), &cfg); err != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "invalid JSON: " + err.Error()})
			pdk.Output(out)
			return 0
		}
		svc, _ := cfg["ServiceSettings"].(map[string]any)
		sql, _ := cfg["SqlSettings"].(map[string]any)
		if svc == nil || strings.TrimSpace(svc["SiteURL"].(string)) == "" {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "ServiceSettings.SiteURL is required"})
			pdk.Output(out)
			return 0
		}
		if sql == nil || strings.TrimSpace(sql["DataSource"].(string)) == "" {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "SqlSettings.DataSource is required"})
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

	if req.Action != "generate_config" {
		out, _ := json.Marshal(map[string]any{"error": "unknown action"})
		pdk.Output(out)
		return 1
	}

	// Merge input into default Mattermost config structure
	cfg := map[string]any{
		"ServiceSettings": map[string]any{
			"SiteURL": req.Input["site_url"],
			"ListenAddress": req.Input["listen_address"],
		},
		"SqlSettings": map[string]any{
			"DataSource": req.Input["data_source"],
		},
		"TeamSettings": map[string]any{
			"SiteName": req.Input["site_name"],
			"MaxUsersPerTeam": req.Input["max_users"],
		},
	}

	raw, _ := json.MarshalIndent(cfg, "", "    ")
	out, _ := json.Marshal(map[string]any{"output": string(raw)})
	pdk.Output(out)
	return 0
}

func main() {}
