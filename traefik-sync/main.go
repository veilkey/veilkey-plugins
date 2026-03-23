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
		"name": "traefik-sync", "version": "1.0.0",
		"description": "Traefik reverse proxy route management",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_init
func pluginInit() int32 {
	out, _ := json.Marshal(map[string]any{
		"paths": []string{"/etc/traefik/conf.d", "/etc/traefik/traefik.yml", "/etc/traefik/.env"},
		"hooks": []map[string]any{
			{"name": "reload_traefik", "cmd": []string{"systemctl", "reload", "traefik"}, "depends": []string{}},
		},
		"api_routes": []map[string]string{
			{"method": "GET", "path": "/routes"},
			{"method": "POST", "path": "/routes"},
			{"method": "POST", "path": "/deploy"},
		},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_destroy
func pluginDestroy() int32 {
	out, _ := json.Marshal(map[string]any{"cleaned_paths": []string{"/etc/traefik/conf.d"}})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_hooks
func pluginHooks() int32 {
	out, _ := json.Marshal([]map[string]any{
		{"name": "reload_traefik", "cmd": []string{"systemctl", "reload", "traefik"}, "depends": []string{}},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_paths
func pluginPaths() int32 {
	out, _ := json.Marshal([]string{"/etc/traefik/conf.d", "/etc/traefik/traefik.yml", "/etc/traefik/.env"})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_validate
func pluginValidate() int32 {
	input := pdk.Input()
	var req struct{ Path, Content string }
	json.Unmarshal(input, &req)

	if strings.HasSuffix(req.Path, ".yml") || strings.HasSuffix(req.Path, ".yaml") {
		if strings.Contains(req.Content, "\t") {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "YAML must not contain tabs"})
			pdk.Output(out)
			return 0
		}
	}
	out, _ := json.Marshal(map[string]any{"ok": true})
	pdk.Output(out)
	return 0
}

type route struct {
	Name    string    `json:"name"`
	Domain  string    `json:"domain"`
	Backend string    `json:"backend"`
	TLS     *tlsCfg   `json:"tls,omitempty"`
	Auth    *authCfg  `json:"auth,omitempty"`
}
type tlsCfg struct {
	CertResolver string `json:"certResolver,omitempty"`
}
type authCfg struct {
	BasicAuth string `json:"basicAuth,omitempty"`
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
	case "generate_routes":
		return generateRoutes(req.Input)
	case "generate_static":
		return generateStatic(req.Input)
	case "generate_env":
		return generateEnv(req.Input)
	default:
		out, _ := json.Marshal(map[string]any{"error": "unknown action: " + req.Action})
		pdk.Output(out)
		return 1
	}
}

func generateRoutes(input map[string]any) int32 {
	routesRaw, _ := json.Marshal(input["routes"])
	var routes []route
	json.Unmarshal(routesRaw, &routes)

	var b strings.Builder
	b.WriteString("# VeilKey-managed Traefik dynamic configuration\n\n")
	b.WriteString("http:\n  routers:\n")

	for _, r := range routes {
		ep := "web"
		if r.TLS != nil { ep = "websecure" }
		b.WriteString(fmt.Sprintf("    %s:\n", r.Name))
		b.WriteString(fmt.Sprintf("      rule: \"Host(`%s`)\"\n", r.Domain))
		b.WriteString(fmt.Sprintf("      entryPoints:\n        - %s\n", ep))
		if r.TLS != nil {
			resolver := r.TLS.CertResolver
			if resolver == "" { resolver = "letsencrypt" }
			b.WriteString(fmt.Sprintf("      tls:\n        certResolver: %s\n", resolver))
		}
		var mws []string
		if r.Auth != nil && r.Auth.BasicAuth != "" { mws = append(mws, r.Name+"-auth") }
		if len(mws) > 0 {
			b.WriteString("      middlewares:\n")
			for _, m := range mws { b.WriteString(fmt.Sprintf("        - %s\n", m)) }
		}
		b.WriteString(fmt.Sprintf("      service: %s\n", r.Name))
	}

	b.WriteString("\n  services:\n")
	for _, r := range routes {
		b.WriteString(fmt.Sprintf("    %s:\n      loadBalancer:\n        servers:\n          - url: \"%s\"\n", r.Name, r.Backend))
	}

	hasAuth := false
	for _, r := range routes { if r.Auth != nil && r.Auth.BasicAuth != "" { hasAuth = true; break } }
	if hasAuth {
		b.WriteString("\n  middlewares:\n")
		for _, r := range routes {
			if r.Auth != nil && r.Auth.BasicAuth != "" {
				b.WriteString(fmt.Sprintf("    %s-auth:\n      basicAuth:\n        users:\n          - \"%s\"\n", r.Name, r.Auth.BasicAuth))
			}
		}
	}

	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func generateStatic(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString("# VeilKey-managed Traefik static configuration\n\n")
	b.WriteString("entryPoints:\n")
	b.WriteString("  web:\n    address: \":80\"\n")
	b.WriteString("    http:\n      redirections:\n        entryPoint:\n          to: websecure\n          scheme: https\n")
	b.WriteString("  websecure:\n    address: \":443\"\n\n")

	b.WriteString("certificatesResolvers:\n  letsencrypt:\n    acme:\n")
	if email, ok := input["acme_email"].(string); ok {
		b.WriteString(fmt.Sprintf("      email: %s\n", email))
	}
	b.WriteString("      storage: /etc/traefik/acme.json\n")
	b.WriteString("      dnsChallenge:\n        provider: cloudflare\n        resolvers:\n          - \"1.1.1.1:53\"\n\n")

	b.WriteString("providers:\n  file:\n    directory: /etc/traefik/conf.d\n    watch: true\n\n")
	b.WriteString("api:\n  dashboard: true\n")

	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func generateEnv(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString("# Traefik environment — managed by VeilKey\n")
	if email, ok := input["cf_api_email"].(string); ok { b.WriteString(fmt.Sprintf("CF_API_EMAIL=%s\n", email)) }
	if token, ok := input["cf_dns_api_token"].(string); ok { b.WriteString(fmt.Sprintf("CF_DNS_API_TOKEN=%s\n", token)) }
	out, _ := json.Marshal(map[string]any{"output": b.String()})
	pdk.Output(out)
	return 0
}

func main() {}
