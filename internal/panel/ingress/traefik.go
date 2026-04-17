package ingress

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/julianbenz1/SpareCompute/internal/common"
)

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

type Target struct {
	Domain     string
	TargetURL  string
	TLSEnabled bool
}

type TraefikFileManager struct {
	dynamicConfigPath string
	certResolver      string
}

func NewTraefikFileManager(dynamicConfigPath, certResolver string) *TraefikFileManager {
	return &TraefikFileManager{
		dynamicConfigPath: strings.TrimSpace(dynamicConfigPath),
		certResolver:      strings.TrimSpace(certResolver),
	}
}

func (m *TraefikFileManager) Enabled() bool {
	return m.dynamicConfigPath != ""
}

func (m *TraefikFileManager) Sync(targets []Target) error {
	if !m.Enabled() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.dynamicConfigPath), 0o755); err != nil {
		return err
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Domain < targets[j].Domain })
	var b strings.Builder
	b.WriteString("[http]\n")
	b.WriteString("  [http.routers]\n")
	b.WriteString("  [http.services]\n")
	for _, t := range targets {
		if strings.TrimSpace(t.Domain) == "" || strings.TrimSpace(t.TargetURL) == "" {
			continue
		}
		name := safeName(t.Domain)
		b.WriteString(fmt.Sprintf("  [http.routers.%s]\n", name))
		b.WriteString(fmt.Sprintf("    rule = \"Host(`%s`)\"\n", escapeDoubleQuotes(t.Domain)))
		b.WriteString(fmt.Sprintf("    service = \"%s\"\n", name))
		if t.TLSEnabled {
			b.WriteString("    entryPoints = [\"websecure\"]\n")
			b.WriteString(fmt.Sprintf("    [http.routers.%s.tls]\n", name))
			if m.certResolver != "" {
				b.WriteString(fmt.Sprintf("      certResolver = \"%s\"\n", escapeDoubleQuotes(m.certResolver)))
			}
		} else {
			b.WriteString("    entryPoints = [\"web\"]\n")
		}
		b.WriteString(fmt.Sprintf("  [http.services.%s.loadBalancer]\n", name))
		b.WriteString(fmt.Sprintf("    [[http.services.%s.loadBalancer.servers]]\n", name))
		b.WriteString(fmt.Sprintf("      url = \"%s\"\n", escapeDoubleQuotes(t.TargetURL)))
	}
	return os.WriteFile(m.dynamicConfigPath, []byte(b.String()), 0o644)
}

func BuildTargets(routes []common.ServiceRoute, instances map[string]common.Instance, nodes map[string]common.Node) []Target {
	out := make([]Target, 0, len(routes))
	for _, route := range routes {
		if route.ActiveInstanceID == "" {
			continue
		}
		instance, ok := instances[route.ActiveInstanceID]
		if !ok || instance.HostPort <= 0 {
			continue
		}
		node, ok := nodes[instance.NodeID]
		if !ok || strings.TrimSpace(node.PublicAddress) == "" {
			continue
		}
		out = append(out, Target{
			Domain:     route.Domain,
			TargetURL:  fmt.Sprintf("http://%s:%d", node.PublicAddress, instance.HostPort),
			TLSEnabled: route.TLSEnabled,
		})
	}
	return out
}

func safeName(v string) string {
	return unsafeChars.ReplaceAllString(v, "_")
}

func escapeDoubleQuotes(v string) string {
	return strings.ReplaceAll(v, `"`, `\"`)
}
