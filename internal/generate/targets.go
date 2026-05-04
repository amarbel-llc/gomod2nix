package generate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Target is a (GOOS, GOARCH) pair that the lockfile's vendorPackages
// list must cover.
type Target struct {
	GOOS   string
	GOARCH string
}

func (t Target) String() string {
	return t.GOOS + "/" + t.GOARCH
}

// nixSystemMap translates nixpkgs system identifiers to Go (GOOS, GOARCH).
// Systems absent from this map are skipped (logged at warn level) by
// DiscoverFlakeTargets.
var nixSystemMap = map[string]Target{
	"x86_64-linux":      {GOOS: "linux", GOARCH: "amd64"},
	"aarch64-linux":     {GOOS: "linux", GOARCH: "arm64"},
	"x86_64-darwin":     {GOOS: "darwin", GOARCH: "amd64"},
	"aarch64-darwin":    {GOOS: "darwin", GOARCH: "arm64"},
	"riscv64-linux":     {GOOS: "linux", GOARCH: "riscv64"},
	"i686-linux":        {GOOS: "linux", GOARCH: "386"},
	"armv7l-linux":      {GOOS: "linux", GOARCH: "arm"},
	"armv6l-linux":      {GOOS: "linux", GOARCH: "arm"},
	"powerpc64le-linux": {GOOS: "linux", GOARCH: "ppc64le"},
}

func nixSystemToTarget(system string) (Target, bool) {
	t, ok := nixSystemMap[system]
	return t, ok
}

// ParseTargetFlags converts user-supplied "GOOS/GOARCH" strings (as
// produced by cobra's StringSliceVar) into Targets, sorted and deduped.
func ParseTargetFlags(flags []string) ([]Target, error) {
	seen := make(map[Target]bool)
	var targets []Target
	for _, raw := range flags {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		parts := strings.Split(s, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid --target %q: expected GOOS/GOARCH (e.g. linux/amd64)", raw)
		}
		t := Target{GOOS: parts[0], GOARCH: parts[1]}
		if seen[t] {
			continue
		}
		seen[t] = true
		targets = append(targets, t)
	}
	sortTargets(targets)
	return targets, nil
}

func sortTargets(ts []Target) {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].GOOS != ts[j].GOOS {
			return ts[i].GOOS < ts[j].GOOS
		}
		return ts[i].GOARCH < ts[j].GOARCH
	})
}

// HasFlake reports whether directory contains a flake.nix.
func HasFlake(directory string) bool {
	_, err := os.Stat(filepath.Join(directory, "flake.nix"))
	return err == nil
}

// DiscoverFlakeTargets evaluates the flake at directory via
// `nix flake show --json --all-systems` and unions system keys under
// packages.* and legacyPackages.*, mapping each to a Target.
//
// On evaluation failure, returns an error including nix's stderr — the
// CLI surfaces this so the user can see why eval failed and pick an
// escape hatch (--target or --no-flake-targets).
func DiscoverFlakeTargets(directory string) ([]Target, error) {
	cmd := exec.Command("nix", "flake", "show", "--json", "--all-systems")
	cmd.Dir = directory
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("`nix flake show --json --all-systems` failed in %s: %w\n%s", directory, err, stderr.String())
	}
	return discoverFlakeTargetsFromJSON(stdout)
}

// discoverFlakeTargetsFromJSON parses the output of `nix flake show
// --json` and returns the Targets implied by system keys under
// packages.* and legacyPackages.*. Split out from DiscoverFlakeTargets
// for unit testability.
//
// Only the categories we actually consume are inner-decoded. Other
// top-level outputs (apps, devShells, lib, templates, formatter, ...)
// can legitimately be non-object on some nix builds, so the top level
// is decoded as raw messages and a malformed packages/legacyPackages
// shape is skipped with a debug log rather than failing discovery.
func discoverFlakeTargetsFromJSON(data []byte) ([]Target, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing `nix flake show` JSON: %w", err)
	}

	systems := make(map[string]bool)
	for _, category := range []string{"packages", "legacyPackages"} {
		raw, ok := top[category]
		if !ok {
			continue
		}
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			log.WithError(err).WithField("category", category).Debug(
				"skipping malformed flake category during target discovery",
			)
			continue
		}
		for system := range entries {
			systems[system] = true
		}
	}

	seen := make(map[Target]bool)
	var targets []Target
	var skipped []string
	for system := range systems {
		t, ok := nixSystemToTarget(system)
		if !ok {
			skipped = append(skipped, system)
			continue
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		targets = append(targets, t)
	}

	if len(skipped) > 0 {
		sort.Strings(skipped)
		log.WithField("systems", skipped).Warn("Skipping flake systems with no known Go GOOS/GOARCH mapping")
	}

	sortTargets(targets)
	return targets, nil
}
