package functions

import (
	"path/filepath"
	"testing"
)

// TestConfigurationsLoad decodes every checked-in configuration and asserts
// that any `script` action compiles cleanly — a guard against a broken inline
// JS program (or a stale listmenu/interactive block) slipping into the repo.
func TestConfigurationsLoad(t *testing.T) {
	paths, err := filepath.Glob("../../configurations/*.toml")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no configuration files found")
	}
	for _, p := range paths {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			def, err := LoadFromFile(p)
			if err != nil {
				t.Fatalf("load %s: %v", p, err)
			}
			for _, fn := range def.Functions {
				for i := range fn.Actions {
					a := &fn.Actions[i]
					if a.Script.Code == "" {
						continue
					}
					if err := a.Script.CompileErr(); err != nil {
						t.Errorf("%s fn %q action %q: script compile error: %v", filepath.Base(p), fn.Name, a.Name, err)
					}
					if a.Script.Program() == nil {
						t.Errorf("%s fn %q action %q: script did not compile to a program", filepath.Base(p), fn.Name, a.Name)
					}
					if typ, _ := a.Type(); typ != "script" {
						t.Errorf("%s fn %q action %q: Type()=%q, want script", filepath.Base(p), fn.Name, a.Name, typ)
					}
				}
			}
		})
	}
}
