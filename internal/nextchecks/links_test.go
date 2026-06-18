package nextchecks

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLinksStatic(t *testing.T) {
	app := t.TempDir()
	// Routes
	writeFile(t, filepath.Join(app, "app", "page.tsx"), "export default function P(){return null}")
	writeFile(t, filepath.Join(app, "app", "about", "page.tsx"), "export default function P(){return null}")
	writeFile(t, filepath.Join(app, "app", "rooftalk", "page.tsx"), "export default function P(){return null}")
	writeFile(t, filepath.Join(app, "app", "rooftalk", "[slug]", "page.tsx"), "export default function P(){return null}")
	writeFile(t, filepath.Join(app, "app", "(marketing)", "pricing", "page.tsx"), "export default function P(){return null}")
	writeFile(t, filepath.Join(app, "app", "blog", "[...slug]", "page.tsx"), "export default function P(){return null}")
	// next.config redirect source
	writeFile(t, filepath.Join(app, "next.config.mjs"), `const c={async redirects(){return [{source:"/old-about",destination:"/about",permanent:true}]}};export default c;`)
	// Links (good + bad)
	writeFile(t, filepath.Join(app, "components", "nav.tsx"), `
		import Link from "next/link";
		const slug = "x";
		export function Nav(){return (<>
			<Link href="/">home</Link>
			<Link href="/about">about</Link>
			<Link href="/rooftalk">blog</Link>
			<Link href="/rooftalk/financing">post</Link>
			<Link href={`+"`"+`/rooftalk/${slug}`+"`"+`}>dyn</Link>
			<Link href="/pricing">pricing</Link>
			<Link href="/blog/a/b">catchall</Link>
			<Link href="/old-about">redirected</Link>
			<a href="https://example.com/external">ext</a>
			<a href="/sitemap.xml">sitemap</a>
			<Link href="/nope">dead</Link>
			<Link href={`+"`"+`/badbase/${slug}`+"`"+`}>deadprefix</Link>
		</>)}
	`)

	res, err := CheckLinks(app, LinkConfig{Mode: "static"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, m := range res.Misses {
		got[m.Ref] = true
	}
	want := []string{"/nope", "/badbase/"}
	if len(got) != len(want) {
		t.Fatalf("misses = %+v, want exactly %v", res.Misses, want)
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("expected miss %q; got %+v", w, res.Misses)
		}
	}
}

func TestCheckLinksLocalePrefix(t *testing.T) {
	app := t.TempDir()
	// Routes live under a leading [lang] locale segment + a (dashboard) group.
	writeFile(t, filepath.Join(app, "app", "[lang]", "page.tsx"), "export default function P(){return null}")
	writeFile(t, filepath.Join(app, "app", "[lang]", "(dashboard)", "badges", "collections", "page.tsx"), "export default function P(){return null}")
	writeFile(t, filepath.Join(app, "app", "[lang]", "(dashboard)", "marketplace", "category", "[category]", "page.tsx"), "export default function P(){return null}")
	// Links are written locale-less, the way i18n middleware expects.
	writeFile(t, filepath.Join(app, "components", "nav.tsx"), `
		import Link from "next/link";
		export function Nav(){return (<>
			<Link href="/">home</Link>
			<Link href="/badges/collections">badges</Link>
			<Link href="/marketplace/category/car">cat</Link>
			<Link href="/totally/missing">dead</Link>
		</>)}
	`)

	// Without LocalePrefix every locale-less link to a [lang]-prefixed route is
	// a segment short → miss. (A single-segment link like "/x" is NOT a miss —
	// it matches /[lang] with x treated as the locale.)
	off, err := CheckLinks(app, LinkConfig{Mode: "static"})
	if err != nil {
		t.Fatal(err)
	}
	offMisses := map[string]bool{}
	for _, m := range off.Misses {
		offMisses[m.Ref] = true
	}
	for _, ref := range []string{"/", "/badges/collections", "/marketplace/category/car", "/totally/missing"} {
		if !offMisses[ref] {
			t.Fatalf("LocalePrefix off: expected %q to be a miss; got %+v", ref, off.Misses)
		}
	}

	// With LocalePrefix the leading [lang] is optional → real routes resolve,
	// only the genuinely-missing /totally/missing remains.
	on, err := CheckLinks(app, LinkConfig{Mode: "static", LocalePrefix: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(on.Misses) != 1 || on.Misses[0].Ref != "/totally/missing" {
		t.Fatalf("LocalePrefix on: want exactly [/totally/missing]; got %+v", on.Misses)
	}
}

func TestCrawlLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<a href="/ok">ok</a><a href="/dead">dead</a>`)
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<p>ok</p>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := CheckLinks("", LinkConfig{Mode: "crawl", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Misses) != 1 || res.Misses[0].Ref != "/dead" {
		t.Fatalf("want one miss /dead, got %+v", res.Misses)
	}
}

func TestCrawlUnreachable(t *testing.T) {
	_, err := CheckLinks("", LinkConfig{Mode: "crawl", BaseURL: "http://127.0.0.1:0"})
	if err == nil || !strings.Contains(err.Error(), "could not reach") {
		t.Fatalf("want unreachable error, got %v", err)
	}
}
