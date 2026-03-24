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
		"name": "cloudflare-dns", "version": "1.0.0",
		"description": "Cloudflare DNS record management — A/CNAME/TXT records via API",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_init
func pluginInit() int32 {
	out, _ := json.Marshal(map[string]any{
		"paths": []string{"/etc/cloudflare/dns-records.json"},
		"hooks": []map[string]any{
			{"name": "apply_dns", "cmd": []string{"echo", "dns-applied"}, "depends": []string{}},
		},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_destroy
func pluginDestroy() int32 {
	out, _ := json.Marshal(map[string]any{"cleaned_paths": []string{"/etc/cloudflare/dns-records.json"}})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_hooks
func pluginHooks() int32 {
	out, _ := json.Marshal([]map[string]any{
		{"name": "apply_dns", "cmd": []string{"echo", "dns-applied"}, "depends": []string{}},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_paths
func pluginPaths() int32 {
	out, _ := json.Marshal([]string{"/etc/cloudflare/dns-records.json"})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_validate
func pluginValidate() int32 {
	input := pdk.Input()
	var req struct{ Path, Content string }
	json.Unmarshal(input, &req)

	if strings.HasSuffix(req.Path, ".json") {
		var parsed any
		if err := json.Unmarshal([]byte(req.Content), &parsed); err != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "invalid JSON: " + err.Error()})
			pdk.Output(out)
			return 0
		}

		// Validate DNS record structure if it looks like a records file
		records, ok := parsed.(map[string]any)
		if ok {
			if recs, exists := records["records"]; exists {
				recsSlice, isSlice := recs.([]any)
				if !isSlice {
					out, _ := json.Marshal(map[string]any{"ok": false, "error": "\"records\" must be an array"})
					pdk.Output(out)
					return 0
				}
				for i, rec := range recsSlice {
					r, isMap := rec.(map[string]any)
					if !isMap {
						out, _ := json.Marshal(map[string]any{"ok": false, "error": fmt.Sprintf("record[%d] must be an object", i)})
						pdk.Output(out)
						return 0
					}
					for _, field := range []string{"type", "name", "content"} {
						if _, has := r[field]; !has {
							out, _ := json.Marshal(map[string]any{"ok": false, "error": fmt.Sprintf("record[%d] missing required field \"%s\"", i, field)})
							pdk.Output(out)
							return 0
						}
					}
					recType, _ := r["type"].(string)
					validTypes := map[string]bool{"A": true, "AAAA": true, "CNAME": true, "TXT": true, "MX": true, "SRV": true, "NS": true}
					if !validTypes[recType] {
						out, _ := json.Marshal(map[string]any{"ok": false, "error": fmt.Sprintf("record[%d] has invalid type \"%s\"", i, recType)})
						pdk.Output(out)
						return 0
					}
				}
			}
		}
	}

	out, _ := json.Marshal(map[string]any{"ok": true})
	pdk.Output(out)
	return 0
}

type dnsRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl,omitempty"`
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
	case "generate_records":
		return generateRecords(req.Input)
	case "generate_env":
		return generateEnv(req.Input)
	default:
		out, _ := json.Marshal(map[string]any{"error": "unknown action: " + req.Action})
		pdk.Output(out)
		return 1
	}
}

func generateRecords(input map[string]any) int32 {
	zone, _ := input["zone"].(string)

	recordsRaw, _ := json.Marshal(input["records"])
	var records []dnsRecord
	json.Unmarshal(recordsRaw, &records)

	// Set default TTL for records that don't specify one
	for i := range records {
		if records[i].TTL == 0 {
			records[i].TTL = 1 // 1 = "automatic" in Cloudflare API
		}
	}

	result := map[string]any{
		"zone":    zone,
		"records": records,
	}
	rendered, _ := json.MarshalIndent(result, "", "  ")

	out, _ := json.Marshal(map[string]any{"output": string(rendered)})
	pdk.Output(out)
	return 0
}

func generateEnv(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString("# Cloudflare API credentials — managed by VeilKey\n")
	if email, ok := input["cf_api_email"].(string); ok {
		b.WriteString(fmt.Sprintf("CF_API_EMAIL=%s\n", email))
	}
	if token, ok := input["cf_api_token"].(string); ok {
		b.WriteString(fmt.Sprintf("CF_API_TOKEN=%s\n", token))
	}
	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func main() {}
