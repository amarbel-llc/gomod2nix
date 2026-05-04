package generate

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTargetFlags(t *testing.T) {
	cases := []struct {
		name    string
		in      []string
		want    []Target
		wantErr string
	}{
		{
			name: "single",
			in:   []string{"linux/amd64"},
			want: []Target{{GOOS: "linux", GOARCH: "amd64"}},
		},
		{
			name: "multiple sorted and deduped",
			in:   []string{"linux/amd64", "darwin/arm64", "linux/amd64"},
			want: []Target{
				{GOOS: "darwin", GOARCH: "arm64"},
				{GOOS: "linux", GOARCH: "amd64"},
			},
		},
		{
			name: "ignores empty entries",
			in:   []string{"", "linux/amd64", "  "},
			want: []Target{{GOOS: "linux", GOARCH: "amd64"}},
		},
		{
			name:    "rejects bare arch",
			in:      []string{"amd64"},
			wantErr: "invalid --target",
		},
		{
			name:    "rejects empty GOOS",
			in:      []string{"/amd64"},
			wantErr: "invalid --target",
		},
		{
			name:    "rejects empty GOARCH",
			in:      []string{"linux/"},
			wantErr: "invalid --target",
		},
		{
			name:    "rejects three segments",
			in:      []string{"linux/amd64/extra"},
			wantErr: "invalid --target",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTargetFlags(tc.in)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNixSystemToTarget(t *testing.T) {
	cases := []struct {
		system string
		want   Target
		ok     bool
	}{
		{"x86_64-linux", Target{GOOS: "linux", GOARCH: "amd64"}, true},
		{"aarch64-linux", Target{GOOS: "linux", GOARCH: "arm64"}, true},
		{"x86_64-darwin", Target{GOOS: "darwin", GOARCH: "amd64"}, true},
		{"aarch64-darwin", Target{GOOS: "darwin", GOARCH: "arm64"}, true},
		{"riscv64-linux", Target{GOOS: "linux", GOARCH: "riscv64"}, true},
		{"i686-linux", Target{GOOS: "linux", GOARCH: "386"}, true},
		{"armv7l-linux", Target{GOOS: "linux", GOARCH: "arm"}, true},
		{"armv6l-linux", Target{GOOS: "linux", GOARCH: "arm"}, true},
		{"powerpc64le-linux", Target{GOOS: "linux", GOARCH: "ppc64le"}, true},
		{"wasm32-wasi", Target{}, false},
		{"", Target{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.system, func(t *testing.T) {
			got, ok := nixSystemToTarget(tc.system)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDiscoverFlakeTargetsFromJSON(t *testing.T) {
	// Modeled on `nix flake show --json` output: the top level is a map
	// of category → system → attr → value. Inner attr values are
	// truncated to {} since we only consume the system keys.
	//
	// devShells contains a system (riscv64-linux) that does not appear
	// under packages or legacyPackages — it must NOT show up in the
	// output, since the chosen discovery scope is packages +
	// legacyPackages only.
	const fixture = `{
	  "packages": {
	    "x86_64-linux":   {"default": {}},
	    "aarch64-darwin": {"default": {}}
	  },
	  "legacyPackages": {
	    "x86_64-linux":  {},
	    "aarch64-linux": {}
	  },
	  "devShells": {
	    "x86_64-linux":  {"default": {}},
	    "riscv64-linux": {"default": {}}
	  }
	}`

	got, err := discoverFlakeTargetsFromJSON([]byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Target{
		{GOOS: "darwin", GOARCH: "arm64"},
		{GOOS: "linux", GOARCH: "amd64"},
		{GOOS: "linux", GOARCH: "arm64"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDiscoverFlakeTargetsFromJSON_skipsUnknown(t *testing.T) {
	// Unknown systems should be skipped silently (warning only) so that
	// flakes targeting platforms outside the Go matrix don't break
	// gomod2nix.
	const fixture = `{
	  "packages": {
	    "x86_64-linux":   {"default": {}},
	    "wasm32-wasi":    {"default": {}}
	  }
	}`
	got, err := discoverFlakeTargetsFromJSON([]byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Target{{GOOS: "linux", GOARCH: "amd64"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDiscoverFlakeTargetsFromJSON_invalidJSON(t *testing.T) {
	_, err := discoverFlakeTargetsFromJSON([]byte("not json"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDiscoverFlakeTargetsFromJSON_empty(t *testing.T) {
	// A flake with no packages or legacyPackages outputs (e.g. one that
	// only exports overlays/templates) yields no targets — the CLI then
	// falls back to host-only.
	got, err := discoverFlakeTargetsFromJSON([]byte(`{"templates": {"default": {}}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty targets, got %+v", got)
	}
}

func TestDiscoverFlakeTargetsFromJSON_heterogeneousTopLevel(t *testing.T) {
	// Regression for #8: some `nix flake show --json` invocations emit
	// non-object values at the top level for outputs whose schema isn't
	// fully evaluated (numbers, strings, nulls). Discovery must only
	// fail on outright unparseable JSON, not on irrelevant-but-shaped
	// siblings of `packages` / `legacyPackages`.
	const fixture = `{
	  "packages": {"x86_64-linux": {"default": {"type": "derivation"}}},
	  "weirdField": 42,
	  "lib": "unevaluated",
	  "nullThing": null
	}`
	got, err := discoverFlakeTargetsFromJSON([]byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Target{{GOOS: "linux", GOARCH: "amd64"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDiscoverFlakeTargetsFromJSON_malformedCategorySkipped(t *testing.T) {
	// If `packages` itself isn't a system→attrs object — e.g. a buggy
	// nix emits an array — discovery should skip that category and
	// fall through to legacyPackages, not panic.
	const fixture = `{
	  "packages": [1, 2, 3],
	  "legacyPackages": {"aarch64-darwin": {}}
	}`
	got, err := discoverFlakeTargetsFromJSON([]byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Target{{GOOS: "darwin", GOARCH: "arm64"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
