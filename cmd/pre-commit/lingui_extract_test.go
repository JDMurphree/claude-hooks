package main

import (
	"os"
	"path/filepath"
	"testing"
)

func linguiWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLinguiTargetHasStagedSource(t *testing.T) {
	root := t.TempDir()
	const macro = "import { msg } from '@lingui/core/macro';\nconst x = msg`Save`;"
	const plain = "export const Foo = () => null;"
	linguiWriteFile(t, filepath.Join(root, "apps/mobile/components/Foo.tsx"), macro)
	linguiWriteFile(t, filepath.Join(root, "apps/mobile/components/Plain.tsx"), plain)
	linguiWriteFile(t, filepath.Join(root, "apps/mobile/components/Foo.test.tsx"), macro)
	linguiWriteFile(t, filepath.Join(root, "apps/mobile/src/locales/en.ts"), macro)

	target := linguiGenTarget{
		Include: []string{"apps/mobile/src/", "apps/mobile/components/"},
		Exclude: []string{"apps/mobile/src/locales/"},
		Command: []string{"true"},
	}
	exts := []string{".ts", ".tsx"}
	markers := []string{"@lingui/core/macro", "@lingui/react/macro"}

	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{"macro file in include", []string{"apps/mobile/components/Foo.tsx"}, true},
		{"plain file (no marker)", []string{"apps/mobile/components/Plain.tsx"}, false},
		{"test file skipped", []string{"apps/mobile/components/Foo.test.tsx"}, false},
		{"excluded locales dir", []string{"apps/mobile/src/locales/en.ts"}, false},
		{"non-matching path", []string{"apps/story/components/Foo.tsx"}, false},
		{"mixed: one macro file matches", []string{"apps/story/x.tsx", "apps/mobile/components/Foo.tsx"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linguiTargetHasStagedSource(root, target, tt.files, exts, markers)
			if got != tt.want {
				t.Errorf("linguiTargetHasStagedSource(%v) = %v, want %v", tt.files, got, tt.want)
			}
		})
	}
}

func TestCheckLinguiExtract_RunsOnlyMatchingTargets(t *testing.T) {
	root := t.TempDir()
	linguiWriteFile(t, filepath.Join(root, "apps/mobile/components/Foo.tsx"),
		"import { msg } from '@lingui/core/macro';\nmsg`Save`;")

	// Two targets; only the mobile one has a staged macro file. Each command
	// touches a distinct sentinel so we can assert which ran.
	mobileSentinel := filepath.Join(root, "mobile.ran")
	storySentinel := filepath.Join(root, "story.ran")
	cfg := `{
      "targets": [
        { "include": ["apps/mobile/components/"], "exclude": ["apps/mobile/src/locales/"],
          "command": ["sh", "-c", "touch ` + mobileSentinel + `"] },
        { "include": ["apps/story/components/"],
          "command": ["sh", "-c", "touch ` + storySentinel + `"] }
      ]
    }`
	linguiWriteFile(t, filepath.Join(root, linguiGenConfigFile), cfg)

	if err := checkLinguiExtract(root, []string{"apps/mobile/components/Foo.tsx"}); err != nil {
		t.Fatalf("checkLinguiExtract returned error: %v", err)
	}
	if _, err := os.Stat(mobileSentinel); err != nil {
		t.Error("expected mobile extract command to run")
	}
	if _, err := os.Stat(storySentinel); err == nil {
		t.Error("story extract command should NOT have run (no staged story source)")
	}
}

func TestCheckLinguiExtract_NoConfigIsNoOp(t *testing.T) {
	root := t.TempDir()
	if err := checkLinguiExtract(root, []string{"apps/mobile/components/Foo.tsx"}); err != nil {
		t.Errorf("missing .lingui-gen.json should be a silent no-op, got: %v", err)
	}
}

func TestCheckLinguiExtract_NoMatchIsNoOp(t *testing.T) {
	root := t.TempDir()
	linguiWriteFile(t, filepath.Join(root, "apps/mobile/components/Plain.tsx"), "export const Foo = 1;")
	sentinel := filepath.Join(root, "ran")
	cfg := `{"targets":[{"include":["apps/mobile/components/"],"command":["sh","-c","touch ` + sentinel + `"]}]}`
	linguiWriteFile(t, filepath.Join(root, linguiGenConfigFile), cfg)

	if err := checkLinguiExtract(root, []string{"apps/mobile/components/Plain.tsx"}); err != nil {
		t.Fatalf("checkLinguiExtract returned error: %v", err)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("no macro marker in staged file — extract should not run")
	}
}
