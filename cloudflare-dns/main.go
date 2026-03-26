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
		"name": "cloudflare-dns", "version": "2.0.0",
		"description": "Cloudflare DNS record management — A/CNAME/TXT records via CF API v4",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_init
func pluginInit() int32 {
	out, _ := json.Marshal(map[string]any{
		"paths": []string{
			"/etc/cloudflare/dns-records.json",
			"/etc/cloudflare/apply-dns.sh",
		},
		"hooks": []map[string]any{
			{"name": "apply_dns", "cmd": []string{"bash", "/etc/cloudflare/apply-dns.sh"}, "depends": []string{}},
		},
		"api_routes": []map[string]string{
			{"method": "GET", "path": "/records"},
			{"method": "POST", "path": "/apply"},
		},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_destroy
func pluginDestroy() int32 {
	out, _ := json.Marshal(map[string]any{"cleaned_paths": []string{
		"/etc/cloudflare/dns-records.json",
		"/etc/cloudflare/apply-dns.sh",
	}})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_hooks
func pluginHooks() int32 {
	out, _ := json.Marshal([]map[string]any{
		{"name": "apply_dns", "cmd": []string{"bash", "/etc/cloudflare/apply-dns.sh"}, "depends": []string{}},
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_paths
func pluginPaths() int32 {
	out, _ := json.Marshal([]string{
		"/etc/cloudflare/dns-records.json",
		"/etc/cloudflare/apply-dns.sh",
	})
	pdk.Output(out)
	return 0
}

//go:wasmexport plugin_validate
func pluginValidate() int32 {
	input := pdk.Input()
	var req struct{ Path, Content string }
	json.Unmarshal(input, &req)

	if strings.HasSuffix(req.Path, ".json") {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(req.Content), &parsed); err != nil {
			pdk.Output(errJSON("invalid JSON: " + err.Error()))
			return 0
		}
		if recs, ok := parsed["records"]; ok {
			recsSlice, isSlice := recs.([]any)
			if !isSlice {
				pdk.Output(errJSON("\"records\" must be an array"))
				return 0
			}
			for i, rec := range recsSlice {
				r, isMap := rec.(map[string]any)
				if !isMap {
					pdk.Output(errJSON(fmt.Sprintf("record[%d] must be an object", i)))
					return 0
				}
				for _, field := range []string{"type", "name", "content"} {
					if _, has := r[field]; !has {
						pdk.Output(errJSON(fmt.Sprintf("record[%d] missing \"%s\"", i, field)))
						return 0
					}
				}
				recType, _ := r["type"].(string)
				validTypes := map[string]bool{"A": true, "AAAA": true, "CNAME": true, "TXT": true, "MX": true, "SRV": true, "NS": true}
				if !validTypes[recType] {
					pdk.Output(errJSON(fmt.Sprintf("record[%d] invalid type \"%s\"", i, recType)))
					return 0
				}
			}
		}
		// Validate required fields
		if _, ok := parsed["zone"]; !ok {
			pdk.Output(errJSON("missing required field \"zone\""))
			return 0
		}
		if _, ok := parsed["cf_api_token"]; !ok {
			pdk.Output(errJSON("missing required field \"cf_api_token\""))
			return 0
		}
	}

	if strings.HasSuffix(req.Path, ".sh") {
		if !strings.Contains(req.Content, "#!/bin/bash") {
			pdk.Output(errJSON("shell script must start with #!/bin/bash"))
			return 0
		}
	}

	out, _ := json.Marshal(map[string]any{"ok": true})
	pdk.Output(out)
	return 0
}

func errJSON(msg string) []byte {
	out, _ := json.Marshal(map[string]any{"ok": false, "error": msg})
	return out
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
	case "generate_apply_script":
		return generateApplyScript(req.Input)
	case "generate_env":
		return generateEnv(req.Input)
	case "api_request":
		return handleAPIRequest(req.Input)
	default:
		out, _ := json.Marshal(map[string]any{"error": "unknown action: " + req.Action})
		pdk.Output(out)
		return 1
	}
}

func generateRecords(input map[string]any) int32 {
	zone, _ := input["zone"].(string)
	token, _ := input["cf_api_token"].(string)

	recordsRaw, _ := json.Marshal(input["records"])
	var records []dnsRecord
	json.Unmarshal(recordsRaw, &records)

	for i := range records {
		if records[i].TTL == 0 {
			records[i].TTL = 1 // 1 = "automatic" in Cloudflare
		}
	}

	result := map[string]any{
		"zone":         zone,
		"cf_api_token": token,
		"records":      records,
	}
	rendered, _ := json.MarshalIndent(result, "", "  ")

	out, _ := json.Marshal(map[string]any{"output": string(rendered)})
	pdk.Output(out)
	return 0
}

// generateApplyScript creates a bash script that calls Cloudflare API v4
// to create/update/delete DNS records based on the JSON config.
func generateApplyScript(input map[string]any) int32 {
	var b strings.Builder
	b.WriteString(`#!/bin/bash
# Cloudflare DNS Apply Script — generated by VeilKey cloudflare-dns plugin
# Reads /etc/cloudflare/dns-records.json and applies via CF API v4
set -euo pipefail

CONFIG="/etc/cloudflare/dns-records.json"
if [ ! -f "$CONFIG" ]; then
  echo "ERROR: $CONFIG not found"
  exit 1
fi

ZONE=$(jq -r '.zone' "$CONFIG")
TOKEN=$(jq -r '.cf_api_token' "$CONFIG")
CF_API="https://api.cloudflare.com/client/v4"

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "ERROR: cf_api_token not set"
  exit 1
fi

# Get Zone ID from zone name
ZONE_ID=$(curl -s -X GET "$CF_API/zones?name=$ZONE" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" | jq -r '.result[0].id')

if [ -z "$ZONE_ID" ] || [ "$ZONE_ID" = "null" ]; then
  echo "ERROR: zone '$ZONE' not found"
  exit 1
fi
echo "Zone: $ZONE (ID: $ZONE_ID)"

# Process each record
RECORDS=$(jq -c '.records[]' "$CONFIG")
APPLIED=0
ERRORS=0

while IFS= read -r record; do
  TYPE=$(echo "$record" | jq -r '.type')
  NAME=$(echo "$record" | jq -r '.name')
  CONTENT=$(echo "$record" | jq -r '.content')
  PROXIED=$(echo "$record" | jq -r '.proxied // false')
  TTL=$(echo "$record" | jq -r '.ttl // 1')

  # Check if record already exists
  EXISTING=$(curl -s -X GET "$CF_API/zones/$ZONE_ID/dns_records?type=$TYPE&name=$NAME" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")
  EXISTING_ID=$(echo "$EXISTING" | jq -r '.result[0].id // empty')

  if [ -n "$EXISTING_ID" ]; then
    # Update existing record
    RESULT=$(curl -s -X PUT "$CF_API/zones/$ZONE_ID/dns_records/$EXISTING_ID" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      --data "{\"type\":\"$TYPE\",\"name\":\"$NAME\",\"content\":\"$CONTENT\",\"proxied\":$PROXIED,\"ttl\":$TTL}")
    SUCCESS=$(echo "$RESULT" | jq -r '.success')
    if [ "$SUCCESS" = "true" ]; then
      echo "  UPDATED: $TYPE $NAME -> $CONTENT"
      APPLIED=$((APPLIED + 1))
    else
      echo "  ERROR updating $NAME: $(echo "$RESULT" | jq -r '.errors[0].message // "unknown"')"
      ERRORS=$((ERRORS + 1))
    fi
  else
    # Create new record
    RESULT=$(curl -s -X POST "$CF_API/zones/$ZONE_ID/dns_records" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      --data "{\"type\":\"$TYPE\",\"name\":\"$NAME\",\"content\":\"$CONTENT\",\"proxied\":$PROXIED,\"ttl\":$TTL}")
    SUCCESS=$(echo "$RESULT" | jq -r '.success')
    if [ "$SUCCESS" = "true" ]; then
      echo "  CREATED: $TYPE $NAME -> $CONTENT"
      APPLIED=$((APPLIED + 1))
    else
      echo "  ERROR creating $NAME: $(echo "$RESULT" | jq -r '.errors[0].message // "unknown"')"
      ERRORS=$((ERRORS + 1))
    fi
  fi
done <<< "$RECORDS"

echo ""
echo "Done: $APPLIED applied, $ERRORS errors"
[ "$ERRORS" -eq 0 ] || exit 1
`)

	out, _ := json.Marshal(map[string]any{"output": b.String()})
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

func handleAPIRequest(input map[string]any) int32 {
	method, _ := input["method"].(string)
	path, _ := input["path"].(string)

	switch {
	case method == "GET" && path == "/records":
		// Return current config
		out, _ := json.Marshal(map[string]any{
			"output": "Read /etc/cloudflare/dns-records.json for current records",
		})
		pdk.Output(out)
	case method == "POST" && path == "/apply":
		// Trigger apply (hook will execute)
		out, _ := json.Marshal(map[string]any{
			"output": "Use bulk-apply to trigger apply_dns hook",
		})
		pdk.Output(out)
	default:
		out, _ := json.Marshal(map[string]any{"error": "not found"})
		pdk.Output(out)
		return 1
	}
	return 0
}

func main() {}
