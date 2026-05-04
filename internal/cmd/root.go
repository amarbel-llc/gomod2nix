// Package cmd implements the command line interface for gomod2nix
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	generate "github.com/nix-community/gomod2nix/internal/generate"
	schema "github.com/nix-community/gomod2nix/internal/schema"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const directoryDefault = "./"

var (
	flagDirectory      string
	flagOutDir         string
	maxJobs            int
	flagWithDeps       bool
	flagTargets        []string
	flagNoFlakeTargets bool
)

// resolveTargets decides which (GOOS, GOARCH) targets to feed into
// GenerateVendorPackages. Policy:
//  1. Explicit --target wins, even if a flake.nix exists.
//  2. Otherwise, if --no-flake-targets is set or no flake.nix is
//     adjacent to the workspace, return nil (host-only legacy).
//  3. Otherwise, run `nix flake show --json` against the project flake.
//     Failures bubble up (fail-loudly) so the user can pick an escape
//     hatch instead of silently producing a host-shaped lockfile.
func resolveTargets(directory string, flags []string, noFlake bool) ([]generate.Target, error) {
	if len(flags) > 0 {
		return generate.ParseTargetFlags(flags)
	}
	if noFlake || !generate.HasFlake(directory) {
		return nil, nil
	}
	targets, err := generate.DiscoverFlakeTargets(directory)
	if err != nil {
		return nil, fmt.Errorf("auto-discovering targets from flake.nix failed: %w\n"+
			"Pass --target GOOS/GOARCH (repeatable) to set targets explicitly, "+
			"or --no-flake-targets to fall back to host-only generation", err)
	}
	return targets, nil
}

func generateFunc(cmd *cobra.Command, args []string) {
	directory := flagDirectory
	outDir := flagOutDir

	// If we are dealing with a project packaged by passing packages on the command line
	// we need to create a temporary project.
	var tmpProj *generate.TempProject
	if len(args) > 0 {
		var err error

		if directory != directoryDefault {
			panic(fmt.Errorf("directory flag not supported together with import arguments"))
		}
		if outDir == "" {
			pwd, err := os.Getwd()
			if err != nil {
				panic(err)
			}

			outDir = pwd
		}

		tmpProj, err = generate.NewTempProject(args)
		if err != nil {
			panic(err)
		}
		defer func() {
			err := tmpProj.Remove()
			if err != nil {
				panic(err)
			}
		}()

		directory = tmpProj.Dir
	} else if outDir == "" {
		// Default out to current working directory if we are developing some software in the current repo.
		outDir = directory
	}

	// Write gomod2nix.toml
	{
		goMod2NixPath := filepath.Join(outDir, "gomod2nix.toml")
		outFile := goMod2NixPath
		pkgs, err := generate.GeneratePkgs(directory, goMod2NixPath, maxJobs)
		if err != nil {
			panic(fmt.Errorf("error generating pkgs: %v", err))
		}

		// For workspace builds, generate per-module vendor package lists
		if generate.HasGoWork(directory) && len(pkgs) > 0 {
			moduleNames := make([]string, len(pkgs))
			for i, pkg := range pkgs {
				moduleNames[i] = pkg.GoPackagePath
			}
			targets, err := resolveTargets(directory, flagTargets, flagNoFlakeTargets)
			if err != nil {
				panic(err)
			}
			vendorPkgs, err := generate.GenerateVendorPackages(directory, moduleNames, targets)
			if err != nil {
				panic(fmt.Errorf("error generating vendor packages: %v", err))
			}
			for _, pkg := range pkgs {
				if vp, ok := vendorPkgs[pkg.GoPackagePath]; ok {
					pkg.VendorPackages = vp
				}
			}
		}

		var goPackagePath string
		var subPackages []string

		if tmpProj != nil {
			subPackages = tmpProj.SubPackages
			goPackagePath = tmpProj.GoPackagePath
		}

		var cachePackages []string
		if flagWithDeps {
			cachePackages, err = generate.GenerateCacheDeps(directory)
			if err != nil {
				panic(fmt.Errorf("error generating cache packages: %v", err))
			}
		}

		output, err := schema.Marshal(pkgs, goPackagePath, subPackages, cachePackages)
		if err != nil {
			panic(fmt.Errorf("error marshaling output: %v", err))
		}

		err = os.WriteFile(outFile, output, 0644)
		if err != nil {
			panic(fmt.Errorf("error writing file: %v", err))
		}
		log.Info(fmt.Sprintf("Wrote: %s", outFile))
	}
}

var rootCmd = &cobra.Command{
	Use:   "gomod2nix",
	Short: "Convert applications using Go modules -> Nix",
	Run:   generateFunc,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Run gomod2nix.toml generator",
	Run:   generateFunc,
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import Go sources into the Nix store",
	Run: func(cmd *cobra.Command, args []string) {
		err := generate.ImportPkgs(flagDirectory, maxJobs)
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagDirectory, "dir", "./", "Go project directory")
	rootCmd.PersistentFlags().StringVar(&flagOutDir, "outdir", "", "Output directory (defaults to project directory)")
	rootCmd.PersistentFlags().IntVar(&maxJobs, "jobs", 10, "Max number of concurrent jobs")
	rootCmd.PersistentFlags().BoolVar(&flagWithDeps, "with-deps", false, "Include dependencies in gomod2nix.toml for build cache priming")
	rootCmd.PersistentFlags().StringSliceVar(&flagTargets, "target", nil, "Target platform GOOS/GOARCH for vendorPackages generation (repeatable, e.g. linux/amd64,darwin/arm64). Overrides flake auto-discovery.")
	rootCmd.PersistentFlags().BoolVar(&flagNoFlakeTargets, "no-flake-targets", false, "Disable auto-discovery of targets from flake.nix; use the host's GOOS/GOARCH instead.")

	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(importCmd)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
