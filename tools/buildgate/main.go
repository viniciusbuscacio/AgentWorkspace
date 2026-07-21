package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// localSigningIdentity is the stable self-signed code-signing identity used to
// sign the macOS app after every build. A stable identity keeps the app's
// designated requirement constant across builds, so macOS TCC permissions
// (Accessibility, Automation, Screen Recording) survive recompiles instead of
// being revoked each time an ad-hoc signature changes the cdhash. Create it
// once with scripts/create-local-signing-cert.sh; absent it, builds still work
// (they stay ad-hoc) and this step is skipped with a notice.
//
// An Apple-issued "Apple Development" identity, when present, takes precedence
// (see resolveSigningIdentity): only Apple certificates carry a team ID, which
// makes macOS key Keychain partition IDs and TCC grants to the TEAM instead of
// the per-build binary hash — so Touch ID and permissions survive rebuilds
// with no re-prompt at all. Keep this list in sync with
// scripts/sign-macos-app.sh.
const localSigningIdentity = "aw-Local Code Signing"

func main() {
	var (
		frontendBuild = flag.Bool("frontend-build", false, "run the Wails frontend build gate")
		skipBuild     = flag.Bool("skip-build", false, "run tests only")
	)
	flag.Parse()

	root, err := findProjectRoot()
	if err != nil {
		fail(err)
	}

	if *frontendBuild {
		if os.Getenv("aw_SKIP_BUILD_GATE_TESTS") != "1" {
			runGoLint(root)
			runGoTests(root)
		}
		run(filepath.Join(root, "frontend"), "Vite build", "npm", "run", "build:frontend")
		return
	}

	runGoLint(root)
	runGoTests(root)
	if *skipBuild {
		return
	}

	env := os.Environ()
	env = append(env, "aw_SKIP_BUILD_GATE_TESTS=1")
	// Version stamp: releases set AW_VERSION (scripts/release.sh); everything
	// else builds as "dev" so a local binary never masquerades as a release.
	version := os.Getenv("AW_VERSION")
	if version == "" {
		version = "dev"
	}
	runWithEnv(root, "Wails build", env, "wails", "build", "-trimpath",
		"-ldflags", "-X aw/internal/appcore.appVersion="+version)
	// Skills are no longer copied next to the binary — they ship embedded in the
	// executable as the vault seed and live in the encrypted vault at runtime.
	signMacApp(root)
	touchAppBundle(root)
}

// touchAppBundle bumps the .app directory mtime after a build. Rewriting files
// inside the bundle does not update the bundle directory itself, so Finder
// keeps showing a stale date and the app looks like it was never rebuilt.
func touchAppBundle(root string) {
	if runtime.GOOS != "darwin" {
		return
	}
	app := filepath.Join(root, "build", "bin", "Agent Workspace.app")
	if info, err := os.Stat(app); err != nil || !info.IsDir() {
		return
	}
	now := time.Now()
	_ = os.Chtimes(app, now, now)
}

// signMacApp re-signs the built macOS .app with the stable local identity so
// TCC permissions persist across builds. It is a no-op off macOS or when the
// identity is not installed (so CI and fresh checkouts still build).
func signMacApp(root string) {
	if runtime.GOOS != "darwin" {
		return
	}
	app := filepath.Join(root, "build", "bin", "Agent Workspace.app")
	if info, err := os.Stat(app); err != nil || !info.IsDir() {
		return
	}
	plist := filepath.Join(app, "Contents", "Info.plist")
	_ = exec.Command("plutil", "-replace", "CFBundleName", "-string", "Agent Workspace", plist).Run()
	_ = exec.Command("plutil", "-replace", "CFBundleDisplayName", "-string", "Agent Workspace", plist).Run()
	_ = exec.Command("plutil", "-replace", "CFBundleIdentifier", "-string", "net.buscacio.agent-workspace", plist).Run()
	identity := resolveSigningIdentity()
	if identity == "" {
		fmt.Printf("\n==> Codesign (ad-hoc fallback)\n"+
			"    identity %q not found; app stays ad-hoc and macOS will re-prompt for\n"+
			"    permissions after each build. Run scripts/create-local-signing-cert.sh once to fix.\n",
			localSigningIdentity)
		cmd := exec.Command("codesign", "--force", "--deep", "--sign", "-", app)
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		return
	}
	fmt.Printf("\n==> Codesign (%s)\n", identity)
	cmd := exec.Command("codesign", "--force", "--deep", "--sign", identity, app)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("codesign with %q failed: %w", identity, err))
	}
}

// resolveSigningIdentity picks the best available signing identity: an
// Apple-issued "Apple Development" certificate first (stable team ID), then
// the self-signed local identity, then "" (caller falls back to ad-hoc).
func resolveSigningIdentity() string {
	if id := appleDevelopmentIdentity(); id != "" {
		return id
	}
	if localSigningIdentityInstalled() {
		return localSigningIdentity
	}
	return ""
}

// appleDevelopmentIdentity returns the full name of the first VALID
// "Apple Development" identity in the keychain, or "".
func appleDevelopmentIdentity() string {
	out, err := exec.Command("security", "find-identity", "-v", "-p", "codesigning").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		i := strings.Index(line, `"Apple Development`)
		if i < 0 {
			continue
		}
		rest := line[i+1:]
		if j := strings.Index(rest, `"`); j >= 0 {
			return rest[:j]
		}
	}
	return ""
}

// localSigningIdentityInstalled reports whether the stable signing identity is
// present in the keychain. It is self-signed (and thus untrusted by policy),
// so it does not appear under find-identity -v; query without -v and match the
// common name.
func localSigningIdentityInstalled() bool {
	out, err := exec.Command("security", "find-identity", "-p", "codesigning").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), localSigningIdentity)
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "wails.json")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find aw project root")
		}
		dir = parent
	}
}

func run(dir, name, command string, args ...string) {
	runWithEnv(dir, name, os.Environ(), command, args...)
}

func runGoLint(root string) {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		fail(errors.New("golangci-lint not found in PATH; install it with:\n" +
			"  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest\n" +
			"then ensure ~/go/bin is in PATH"))
	}
	run(root, "Go lint (golangci-lint)", "golangci-lint", "run", "./...")
}

func runGoTests(root string) {
	packages := goPackages(root)
	run(root, "Go tests", "go", append([]string{"test"}, packages...)...)
}

func goPackages(root string) []string {
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = root
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		fail(fmt.Errorf("Go package list failed: %w\n%s", err, out))
	}
	packages := make([]string, 0)
	for _, pkg := range strings.Fields(string(out)) {
		if generatedOrVendoredPackage(pkg) {
			continue
		}
		packages = append(packages, pkg)
	}
	if len(packages) == 0 {
		fail(errors.New("Go package list is empty after filtering generated and vendored packages"))
	}
	return packages
}

func generatedOrVendoredPackage(pkg string) bool {
	return strings.Contains(pkg, "/build/") ||
		strings.Contains(pkg, "/frontend/node_modules/") ||
		strings.Contains(pkg, "/frontend/wailsjs/")
}

func runWithEnv(dir, name string, env []string, command string, args ...string) {
	fmt.Printf("\n==> %s\n", name)

	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("%s failed: %w", name, err))
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "build gate failed: %v\n", err)
	os.Exit(1)
}
