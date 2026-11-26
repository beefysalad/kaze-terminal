package config

import (
	"os"
	"strings"
	"testing"
)

func TestSetOrchestratorEnabled(t *testing.T) {
	for _, src := range []string{"", "# keep\n[control]\nenabled = true\n", "# keep\n[orchestrator] # workers\nenabled = false # switch\nconfirm = 'never'\n[control]\nenabled = true\n", "shell = ['sh', '''\n[orchestrator]\nenabled = false\n''']\n[orchestrator]\nconfirm = 'first'\n"} {
		for _, nl := range []string{"\n", "\r\n"} {
			p := write(t, strings.ReplaceAll(src, "\n", nl))
			if err := SetOrchestratorEnabled(p, true); err != nil {
				t.Fatal(err)
			}
			c, _, err := Load(p)
			if err != nil || !c.Orchestrator.Enabled {
				t.Fatal(c, err)
			}
			b, _ := os.ReadFile(p)
			for _, keep := range []string{"# keep", "# switch", "confirm = 'never'", "[control]"} {
				if strings.Contains(src, keep) && !strings.Contains(string(b), keep) {
					t.Fatal("lost", keep)
				}
			}
			if err := SetOrchestratorEnabled(p, false); err != nil {
				t.Fatal(err)
			}
			c, _, err = Load(p)
			if err != nil || c.Orchestrator.Enabled {
				t.Fatal(c, err)
			}
		}
	}
	for _, src := range []string{"[orchestrator", "orchestrator = { enabled = false }\n", "orchestrator.enabled = false\n"} {
		p := write(t, src)
		if SetOrchestratorEnabled(p, true) == nil {
			t.Fatal("accepted unsupported form")
		}
		b, _ := os.ReadFile(p)
		if string(b) != src {
			t.Fatal("changed rejected config")
		}
	}
}
