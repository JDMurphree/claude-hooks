package main

import (
	"strings"
	"testing"
)

// typedArgsFixture is a minimal flat-file project with one mutation and one
// action, used to exercise the `dataLayer.typedArgs` flag.
func typedArgsFixture() fixture {
	return fixture{
		name:          "thingco",
		convexPath:    "packages/convex/convex",
		dataLayerPath: "packages/data-layer/src",
		fileStructure: "grouped",
		functionFiles: map[string]string{
			"things.ts": `import { mutation, action } from './_generated/server';
import { v } from 'convex/values';

export const createThing = mutation({
  args: { name: v.string() },
  handler: async (ctx, { name }) => {
    return null;
  },
});

export const runThing = action({
  args: { id: v.string() },
  handler: async (ctx, { id }) => {
    return null;
  },
});
`,
		},
	}
}

// TestTypedArgs_DisabledIsBackwardsCompatible is the load-bearing guarantee:
// projects whose .convex-gen.json omits `typedArgs` (the default, false) must
// get byte-for-byte the historical output — no ReactMutation/ReactAction import,
// no signature annotation, untyped hook returned exactly as before.
func TestTypedArgs_DisabledIsBackwardsCompatible(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := typedArgsFixture().build(t, tmpDir)

	if cfg.DataLayer.TypedArgs {
		t.Fatal("TypedArgs must default to false when absent from config")
	}

	_, fns := runPipeline(t, cfg)
	hooksGen := NewHooksGenerator(cfg)

	mutationContent := hooksGen.generateGroupedHookFileContent(
		"things", filterByType(fns, FunctionTypeMutation), "mutation")
	actionContent := hooksGen.generateGroupedHookFileContent(
		"things", filterByType(fns, FunctionTypeAction), "action")

	// No new tokens may leak into disabled output.
	for _, banned := range []string{"ReactMutation", "ReactAction"} {
		if strings.Contains(mutationContent, banned) {
			t.Errorf("disabled typedArgs leaked %q into mutation output:\n%s", banned, mutationContent)
		}
		if strings.Contains(actionContent, banned) {
			t.Errorf("disabled typedArgs leaked %q into action output:\n%s", banned, actionContent)
		}
	}

	// Historical signatures + bodies must be exactly as before.
	wantMutation := []string{
		`import { useMutation } from "convex/react";`,
		"export function useThingsCreateThing() {",
		"  // @ts-ignore - TS2589: Deep type instantiation with nested API path\n  return useMutation(api.things.createThing);",
	}
	for _, want := range wantMutation {
		if !strings.Contains(mutationContent, want) {
			t.Errorf("disabled mutation output missing historical substring %q:\n%s", want, mutationContent)
		}
	}

	wantAction := []string{
		`import { useAction } from "convex/react";`,
		"export function useThingsRunThing() {",
		"  return useAction(api.things.runThing);",
	}
	for _, want := range wantAction {
		if !strings.Contains(actionContent, want) {
			t.Errorf("disabled action output missing historical substring %q:\n%s", want, actionContent)
		}
	}
}

// TestTypedArgs_EnabledTypesMutationsAndActions verifies that, with the flag on,
// the generator hoists the precise call signature onto the hook so caller args
// are type-checked — while leaving the @ts-ignore'd body untouched.
func TestTypedArgs_EnabledTypesMutationsAndActions(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := typedArgsFixture().build(t, tmpDir)
	cfg.DataLayer.TypedArgs = true

	_, fns := runPipeline(t, cfg)
	hooksGen := NewHooksGenerator(cfg)

	mutationContent := hooksGen.generateGroupedHookFileContent(
		"things", filterByType(fns, FunctionTypeMutation), "mutation")
	actionContent := hooksGen.generateGroupedHookFileContent(
		"things", filterByType(fns, FunctionTypeAction), "action")

	// Mutation file: ReactMutation import + annotation, body unchanged, no ReactAction.
	wantMutation := []string{
		`import type { ReactMutation } from "convex/react";`,
		"export function useThingsCreateThing(): ReactMutation<typeof api.things.createThing> {",
		"  return useMutation(api.things.createThing);",
	}
	for _, want := range wantMutation {
		if !strings.Contains(mutationContent, want) {
			t.Errorf("enabled mutation output missing %q:\n%s", want, mutationContent)
		}
	}
	if strings.Contains(mutationContent, "ReactAction") {
		t.Errorf("mutation file should not import ReactAction:\n%s", mutationContent)
	}

	// Action file: ReactAction import + annotation, body unchanged, no ReactMutation.
	wantAction := []string{
		`import type { ReactAction } from "convex/react";`,
		"export function useThingsRunThing(): ReactAction<typeof api.things.runThing> {",
		"  return useAction(api.things.runThing);",
	}
	for _, want := range wantAction {
		if !strings.Contains(actionContent, want) {
			t.Errorf("enabled action output missing %q:\n%s", want, actionContent)
		}
	}
	if strings.Contains(actionContent, "ReactMutation") {
		t.Errorf("action file should not import ReactMutation:\n%s", actionContent)
	}
}

// TestTypedArgs_EnabledSplitFiles covers the second emission path
// (generateSplitHookFileContent, single-quote imports).
func TestTypedArgs_EnabledSplitFiles(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := typedArgsFixture().build(t, tmpDir)
	cfg.DataLayer.TypedArgs = true
	cfg.DataLayer.FileStructure = "split"

	_, fns := runPipeline(t, cfg)
	hooksGen := NewHooksGenerator(cfg)

	content := hooksGen.generateSplitHookFileContent(
		"things", "things", filterByType(fns, FunctionTypeMutation), "mutation")

	want := []string{
		`import type { ReactMutation } from 'convex/react';`,
		": ReactMutation<typeof api.things.createThing>",
	}
	for _, w := range want {
		if !strings.Contains(content, w) {
			t.Errorf("split mutation output missing %q:\n%s", w, content)
		}
	}
}
