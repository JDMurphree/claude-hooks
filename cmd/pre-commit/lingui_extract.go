package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// linguiGenConfigFile is the per-project config shared with the auto-lingui-extract
// PostToolUse hook — one source of truth across edit-time and commit-time paths.
const linguiGenConfigFile = ".lingui-gen.json"

var (
	linguiDefaultMarkers    = []string{"@lingui/core/macro", "@lingui/react/macro"}
	linguiDefaultExtensions = []string{".ts", ".tsx"}
	linguiSkipSuffixes      = []string{".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx", ".d.ts"}
)

// linguiGenTarget maps a package's source directories to its extract command.
type linguiGenTarget struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	Command []string `json:"command"`
}

// linguiGenConfig mirrors .lingui-gen.json.
type linguiGenConfig struct {
	MacroMarkers []string          `json:"macroMarkers"`
	Extensions   []string          `json:"extensions"`
	Targets      []linguiGenTarget `json:"targets"`
}

// checkLinguiExtract regenerates Lingui catalogs at commit time. For every
// target in .lingui-gen.json that has a staged, macro-bearing source file, it
// runs that package's extract command and re-stages the regenerated catalogs
// (the target's exclude dirs are the catalog output locations). No-op when the
// config is missing/empty or no relevant file is staged.
func checkLinguiExtract(projectRoot string, stagedFiles []string) error {
	data, err := os.ReadFile(filepath.Join(projectRoot, linguiGenConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", linguiGenConfigFile, err)
	}

	var cfg linguiGenConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing %s: %w", linguiGenConfigFile, err)
	}
	if len(cfg.Targets) == 0 {
		return nil
	}

	markers := cfg.MacroMarkers
	if len(markers) == 0 {
		markers = linguiDefaultMarkers
	}
	extensions := cfg.Extensions
	if len(extensions) == 0 {
		extensions = linguiDefaultExtensions
	}

	// Commands + catalog dirs to stage, deduped, in target order.
	var commands [][]string
	var stageDirs []string
	seen := make(map[string]bool)

	for _, target := range cfg.Targets {
		if len(target.Command) == 0 {
			continue
		}
		if !linguiTargetHasStagedSource(projectRoot, target, stagedFiles, extensions, markers) {
			continue
		}
		key := strings.Join(target.Command, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		commands = append(commands, target.Command)
		stageDirs = append(stageDirs, target.Exclude...)
	}

	if len(commands) == 0 {
		return nil
	}

	for _, command := range commands {
		if err := runLinguiExtractCommand(projectRoot, command); err != nil {
			return err
		}
	}

	// Re-stage just the catalog output dirs (locales/), not the whole worktree,
	// so we never sweep in unrelated changes.
	if len(stageDirs) > 0 {
		stage := exec.Command("git", append([]string{"add"}, stageDirs...)...)
		stage.Dir = projectRoot
		_ = stage.Run()
	}

	return nil
}

// linguiTargetHasStagedSource reports whether any staged file is part of this
// target's source set (include minus exclude, right extension, not a test) and
// actually contains a Lingui macro marker.
func linguiTargetHasStagedSource(projectRoot string, target linguiGenTarget, stagedFiles, extensions, markers []string) bool {
	for _, f := range stagedFiles {
		rel := filepath.ToSlash(f)
		if !linguiHasExtension(rel, extensions) || linguiIsSkipped(rel) {
			continue
		}
		if !linguiMatchesAny(rel, target.Include) || linguiMatchesAny(rel, target.Exclude) {
			continue
		}
		if !linguiFileContainsMarker(filepath.Join(projectRoot, f), markers) {
			continue
		}
		return true
	}
	return false
}

func linguiHasExtension(relPath string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(relPath, ext) {
			return true
		}
	}
	return false
}

func linguiIsSkipped(relPath string) bool {
	for _, suffix := range linguiSkipSuffixes {
		if strings.HasSuffix(relPath, suffix) {
			return true
		}
	}
	return false
}

func linguiMatchesAny(relPath string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(relPath, filepath.ToSlash(p)) {
			return true
		}
	}
	return false
}

func linguiFileContainsMarker(path string, markers []string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	for _, m := range markers {
		if strings.Contains(content, m) {
			return true
		}
	}
	return false
}

func runLinguiExtractCommand(projectRoot string, command []string) error {
	bin, err := exec.LookPath(command[0])
	if err != nil {
		return fmt.Errorf("lingui-extract command not found: %s", command[0])
	}
	cmd := exec.Command(bin, command[1:]...)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("lingui-extract command failed: %w", err)
	}
	return nil
}
