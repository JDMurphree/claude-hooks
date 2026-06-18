package nextchecks

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckImages(t *testing.T) {
	app := t.TempDir()
	writeFile(t, filepath.Join(app, "public", "a.png"), "x")
	writeFile(t, filepath.Join(app, "public", "images", "hero.jpg"), "x")
	writeFile(t, filepath.Join(app, "app", "page.tsx"), `
		import Image from "next/image";
		export default function P() {
			return (<>
				<Image src="/a.png" />
				<Image src="/images/hero.jpg" />
				<Image src="/images/missing.png" />
				<a href="https://cdn.example.com/remote/photo.png">x</a>
			</>);
		}
	`)

	res, err := CheckImages(app, ImageConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped {
		t.Fatal("should not skip — public/ exists")
	}
	if len(res.Misses) != 1 {
		t.Fatalf("want 1 miss, got %d: %+v", len(res.Misses), res.Misses)
	}
	if res.Misses[0].Ref != "/images/missing.png" {
		t.Fatalf("want /images/missing.png, got %q", res.Misses[0].Ref)
	}
	// The remote https URL must NOT be treated as a public asset.
	for _, m := range res.Misses {
		if m.Ref == "/remote/photo.png" {
			t.Fatal("remote URL path was wrongly treated as a public asset")
		}
	}
}

func TestCheckImagesSkipsTestFiles(t *testing.T) {
	app := t.TempDir()
	writeFile(t, filepath.Join(app, "public", "real.png"), "x")
	// Real source references a real asset → OK.
	writeFile(t, filepath.Join(app, "app", "page.tsx"), `<img src="/real.png"/>`)
	// Test fixtures reference fake asset-looking paths by design. These must
	// NOT be flagged — a path referenced only in a test can't 404 in prod.
	writeFile(t, filepath.Join(app, "components", "url.test.ts"),
		`expect(getCdnUrl("/images/photo.jpg")).toBe("https://cdn/images/photo.jpg");`)
	writeFile(t, filepath.Join(app, "components", "card.spec.tsx"),
		`render(<img src="/uploads/missing.jpg" />);`)
	writeFile(t, filepath.Join(app, "components", "__mocks__", "media.ts"),
		`export const fixture = "/mock/asset.png";`)

	res, err := CheckImages(app, ImageConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Misses) != 0 {
		t.Fatalf("test artifacts must be skipped; got misses %+v", res.Misses)
	}
}

func TestCheckImagesSkipsNonNext(t *testing.T) {
	app := t.TempDir()
	writeFile(t, filepath.Join(app, "app", "page.tsx"), `<img src="/x.png"/>`)
	res, err := CheckImages(app, ImageConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Skipped {
		t.Fatal("want skipped (no public/ dir)")
	}
}
