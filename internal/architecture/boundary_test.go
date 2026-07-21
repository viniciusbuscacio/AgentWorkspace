package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "aw"

func TestCleanArchitectureBoundaries(t *testing.T) {
	root := repoRoot(t)
	files := collectGoFiles(t, root)
	for _, file := range files {
		from := layerForFile(root, file)
		if from == "" {
			continue
		}
		for _, imported := range importsForFile(t, file) {
			to := layerForImport(imported)
			if to == "" || from == to {
				continue
			}
			if boundaryAllowed(from, to) {
				continue
			}
			t.Fatalf("%s layer must not import %s layer: %s imports %s", from, to, rel(t, root, file), imported)
		}
	}
}

// TestInnerLayersHaveNoExternalDependencies enforces the AW2 spirit: the inner
// Clean Architecture layers (domain, domain/ports, application, dto) must depend
// only on the Go standard library and other aw layers — never on third-party
// packages. This keeps business rules pure, framework-agnostic and trivially
// testable. If a real need ever appears, add it to allowedExternalImports with a
// TODO, mirroring AW2's shrinking-allowlist approach.
func TestInnerLayersHaveNoExternalDependencies(t *testing.T) {
	root := repoRoot(t)
	pureLayers := map[string]bool{
		"domain":      true,
		"application": true,
		"dto":         true,
	}
	// Empty for now: domain/application/dto are clean. Grow only with a TODO.
	allowedExternalImports := map[string]bool{}

	for _, file := range collectGoFiles(t, root) {
		from := layerForFile(root, file)
		if !pureLayers[from] {
			continue
		}
		for _, imported := range importsForFile(t, file) {
			if !isExternalImport(imported) {
				continue
			}
			if allowedExternalImports[imported] {
				continue
			}
			t.Fatalf("%s layer must not import external package: %s imports %s", from, rel(t, root, file), imported)
		}
	}
}

// isExternalImport reports whether an import path is a third-party dependency.
// Go stdlib paths have no dot in their first segment (fmt, encoding/json, os),
// aw internal packages start with the module path, and everything else with a
// dotted first segment (github.com/..., golang.org/x/...) is external.
func isExternalImport(importPath string) bool {
	if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
		return false
	}
	firstSegment := importPath
	if idx := strings.IndexByte(importPath, '/'); idx >= 0 {
		firstSegment = importPath[:idx]
	}
	return strings.Contains(firstSegment, ".")
}

func TestCleanArchitectureLayerDirectoriesExist(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{
		"internal/domain",
		"internal/domain/ports",
		"internal/dto",
		"internal/application",
		"internal/infrastructure",
	} {
		info, err := os.Stat(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("%s missing: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}
}

func TestCleanArchitectureHasNoLegacyInternalLayerDirectories(t *testing.T) {
	root := repoRoot(t)
	allowed := map[string]bool{
		"architecture":   true,
		"application":    true,
		"appcore":        true, // composition root (the App), extracted so awd can import it
		"domain":         true,
		"dto":            true,
		"infrastructure": true,
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !allowed[entry.Name()] {
			t.Fatalf("internal/%s must be moved under a Clean Architecture layer", entry.Name())
		}
	}
}

func TestInterfaceLayerDoesNotCallStatefulAdaptersDirectly(t *testing.T) {
	root := repoRoot(t)

	// Guarded adapter fields are derived automatically from the App struct: any
	// field whose type lives in an infrastructure package or is a domain port is
	// a stateful adapter the interface layer must not call directly — it has to
	// delegate to an application use case. This is fail-closed: a newly added
	// infrastructure/port field on App is guarded with no edit to this test.
	guarded := guardedAdapterFields(t, root)
	if len(guarded) == 0 {
		t.Fatal("expected to derive at least one guarded adapter field from the App struct")
	}

	// Composition-root transport plumbing explicitly allowed to be called
	// directly. Keep this list tiny and justified; everything else is forbidden.
	allowedDirectCalls := map[string]bool{
		"pipHub": true, // PiP event hub: pure transport fan-out owned by the composition root
		"webHub": true, // Web event hub: pure transport fan-out owned by the composition root
	}

	var violations []string
	for _, file := range collectGoFiles(t, root) {
		if layerForFile(root, file) != "interface" {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel(t, root, file), err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver := selectorPath(selector.X)
			if receiver == "" {
				return true
			}
			segments := strings.Split(receiver, ".")
			// Field access always goes through a receiver (a.field / b.app.field),
			// so it has >= 2 segments. A single segment is a package qualifier
			// (agent.NewRuntime) or a local variable, not a direct adapter call.
			if len(segments) < 2 {
				return true
			}
			field := segments[len(segments)-1]
			if !guarded[field] || allowedDirectCalls[field] {
				return true
			}
			position := fset.Position(selector.Pos())
			violations = append(violations, rel(t, root, file)+":"+strconv.Itoa(position.Line)+" calls "+receiver+"."+selector.Sel.Name)
			return true
		})
	}
	if len(violations) > 0 {
		t.Fatalf("interface layer must call application use cases instead of stateful adapters directly:\n%s", strings.Join(violations, "\n"))
	}
}

// guardedAdapterFields parses the App struct in the interface layer and returns
// the set of field names whose type is a stateful adapter — an infrastructure
// package type or a domain port. These must not be invoked directly from the
// interface layer.
// TestInterfaceLayerDoesNotEmbedInfrastructureIO keeps raw I/O and external
// protocol code out of the composition root: sockets, HTTP servers/clients,
// SQL and low-level crypto belong behind a port in internal/infrastructure,
// not inlined in app.go. This closes the gap that let the provider OAuth flow
// (an HTTP callback server + PKCE) live in the interface layer — the boundary
// import checks did not catch it because the interface layer may import any
// other aw layer. The composition root wires adapters; it must not be one.
func TestInterfaceLayerDoesNotEmbedInfrastructureIO(t *testing.T) {
	root := repoRoot(t)
	// Forbidden in the interface layer. Parsing/encoding/id helpers (net/url,
	// encoding/*, crypto/rand) are allowed: they are pure, not external I/O.
	forbidden := map[string]bool{
		"net":           true,
		"net/http":      true,
		"database/sql":  true,
		"crypto/sha256": true,
		"crypto/subtle": true,
		"crypto/aes":    true,
		"crypto/cipher": true,
	}
	for _, file := range collectGoFiles(t, root) {
		if layerForFile(root, file) != "interface" {
			continue
		}
		for _, imported := range importsForFile(t, file) {
			if forbidden[imported] {
				t.Fatalf("interface layer must not import %s — move the I/O behind a port in internal/infrastructure: %s",
					imported, rel(t, root, file))
			}
		}
	}
}

// TestInterfaceLayerDoesNotCallInfrastructureIO closes the last gap the other
// boundary checks leave open. The interface layer may import infrastructure (it
// is the composition root), and TestInterfaceLayerDoesNotCallStatefulAdapters-
// Directly only catches calls routed through an App field. That combination let
// package-level infrastructure functions that perform real I/O — provider HTTP
// probes, voice transcription (ffmpeg), macOS system calls, PiP process
// spawning, config persistence — be invoked straight from a handler, bypassing
// the application use case and its domain port.
//
// This test forbids interface -> infrastructure calls except:
//   - constructors (New*): composition-root wiring of adapters;
//   - an explicit allowlist of PURE helpers (no I/O — policy checks, path/URL
//     computation, in-memory key hygiene, static inventories, sanitizers);
//   - app-config access from the composition-root files (app.go / main.go),
//     where loading and persisting config to wire/configure adapters is the
//     legitimate job of the Main component.
//
// It catches both `pkg.Func(...)` and `pkg.Type{}.Method(...)` (an adapter
// constructed inline and called on the spot), so neither form can regress.
func TestInterfaceLayerDoesNotCallInfrastructureIO(t *testing.T) {
	root := repoRoot(t)

	// Pure infrastructure helpers the interface may call directly: deterministic,
	// no external I/O. Keyed by <canonical-package>.<Func>. Grow only with a
	// justification that the function performs no I/O.
	pureInfraHelpers := map[string]bool{
		"securemem.Zero":               true, // in-memory key/byte wipe
		"sandbox.IsDenied":             true, // pure path policy check
		"sandbox.CheckCommand":         true, // pure command policy check
		"sandbox.ResolveAndCheck":      true, // pure path policy check
		"sandbox.ExpandPath":           true, // pure path expansion
		"sandbox.Roots":                true, // pure visible-roots computation
		"appconfig.BaseDir":            true, // pure data-dir path
		"selfdev.ResolveRepoRoot":      true, // read-only repo-root resolution
		"vault.DefaultDir":             true, // pure default-path helper
		"profile.DefaultBaseDir":       true, // pure default-path helper
		"pip.BuildURL":                 true, // pure URL builder
		"pip.AssetHandler":             true, // pure handler over embedded assets
		"webserver.SignSession":        true, // pure HMAC token mint, no I/O
		"webserver.VerifySession":      true, // pure HMAC token verify, no I/O
		"webserver.DetectTailscaleIP":  true, // read-only local interface enumeration, no external I/O
		"webserver.ListBindCandidates": true, // read-only local interface enumeration, no external I/O
		"macosperm.All":                true, // static permission inventory
		"macosperm.AllForBuild":        true, // static permission inventory
		"emailsafe.Sanitize":           true, // pure HTML sanitizer
	}

	// Composition-root files may load/persist app config to wire and configure
	// adapters. No other interface file may touch infrastructure config I/O.
	compositionRoot := map[string]bool{"app.go": true, "main.go": true}

	var violations []string
	for _, file := range collectGoFiles(t, root) {
		if layerForFile(root, file) != "interface" {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel(t, root, file), err)
		}
		imports := importNameMap(parsed)
		base := filepath.Base(file)
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg := infraPackageForReceiver(selector.X, imports)
			if pkg == "" {
				return true
			}
			fn := selector.Sel.Name
			if strings.HasPrefix(fn, "New") {
				return true // constructor: composition-root wiring
			}
			if pureInfraHelpers[pkg+"."+fn] {
				return true
			}
			if pkg == "appconfig" && compositionRoot[base] {
				return true // config bootstrap in the composition root
			}
			position := fset.Position(selector.Pos())
			violations = append(violations, rel(t, root, file)+":"+strconv.Itoa(position.Line)+" calls "+pkg+"."+fn)
			return true
		})
	}
	if len(violations) > 0 {
		t.Fatalf("interface layer must not invoke infrastructure I/O directly — route it through an application use case and a domain port:\n%s", strings.Join(violations, "\n"))
	}
}

// infraPackageForReceiver returns the canonical infrastructure package name for
// a call's selector receiver, or "" when the receiver is not an infrastructure
// package. It resolves `pkg.Func(...)` (Ident receiver) and
// `pkg.Type{}.Method(...)` (composite-literal receiver — an adapter built and
// called inline).
func infraPackageForReceiver(expr ast.Expr, imports map[string]string) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return canonicalInfraPackage(imports[typed.Name])
	case *ast.CompositeLit:
		if sel, ok := typed.Type.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				return canonicalInfraPackage(imports[ident.Name])
			}
		}
	case *ast.SelectorExpr:
		if ident, ok := typed.X.(*ast.Ident); ok {
			return canonicalInfraPackage(imports[ident.Name])
		}
	}
	return ""
}

// canonicalInfraPackage maps an import path under internal/infrastructure to its
// package directory name (the alias used in code is irrelevant), or "".
func canonicalInfraPackage(importPath string) string {
	const infraPrefix = modulePath + "/internal/infrastructure/"
	if !strings.HasPrefix(importPath, infraPrefix) {
		return ""
	}
	rest := strings.TrimPrefix(importPath, infraPrefix)
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		rest = rest[:idx]
	}
	return rest
}

func guardedAdapterFields(t *testing.T, root string) map[string]bool {
	t.Helper()
	guarded := map[string]bool{}
	for _, file := range collectGoFiles(t, root) {
		if layerForFile(root, file) != "interface" {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel(t, root, file), err)
		}
		imports := importNameMap(parsed)
		ast.Inspect(parsed, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok || spec.Name.Name != "App" {
				return true
			}
			structType, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range structType.Fields.List {
				qualifier := typePackageQualifier(field.Type)
				if qualifier == "" {
					continue
				}
				if !isGuardedAdapterImport(imports[qualifier]) {
					continue
				}
				for _, name := range field.Names {
					guarded[name.Name] = true
				}
			}
			return false
		})
	}
	return guarded
}

// isGuardedAdapterImport reports whether an import path belongs to a stateful
// adapter layer: an infrastructure package or the domain ports package.
func isGuardedAdapterImport(importPath string) bool {
	if importPath == "" {
		return false
	}
	if strings.HasPrefix(importPath, modulePath+"/internal/infrastructure/") {
		return true
	}
	return importPath == modulePath+"/internal/domain/ports"
}

// typePackageQualifier returns the package qualifier of a possibly pointer/slice
// selector type expression: "*pip.Server" -> "pip", "ports.AutoLockPolicy" -> "ports".
func typePackageQualifier(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return typePackageQualifier(typed.X)
	case *ast.ArrayType:
		return typePackageQualifier(typed.Elt)
	case *ast.SelectorExpr:
		if ident, ok := typed.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// importNameMap maps each import's local name (alias or package name) to its path.
func importNameMap(file *ast.File) map[string]string {
	names := map[string]string{}
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		name := path
		if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
			name = path[idx+1:]
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = path
	}
	return names
}

func boundaryAllowed(from string, to string) bool {
	switch from {
	case "domain":
		return false
	case "application":
		return to == "domain"
	case "dto":
		return to == "domain"
	case "infrastructure":
		return to == "domain"
	case "interface":
		return true
	default:
		return true
	}
}

func layerForFile(root string, file string) string {
	relPath, err := filepath.Rel(root, file)
	if err != nil {
		return ""
	}
	relPath = filepath.ToSlash(relPath)
	switch {
	case strings.HasPrefix(relPath, "internal/domain/") || relPath == "internal/domain":
		return "domain"
	case strings.HasPrefix(relPath, "internal/application/") || relPath == "internal/application":
		return "application"
	case strings.HasPrefix(relPath, "internal/dto/") || relPath == "internal/dto":
		return "dto"
	case strings.HasPrefix(relPath, "internal/infrastructure/") || relPath == "internal/infrastructure":
		return "infrastructure"
	case strings.HasPrefix(relPath, "internal/appcore/") || relPath == "internal/appcore":
		// appcore is the extracted composition root (the App). Same rules as the
		// root-level interface files it was moved from.
		return "interface"
	case strings.HasPrefix(relPath, "internal/architecture/"):
		return ""
	case strings.HasPrefix(relPath, "internal/"):
		return "legacy-internal"
	case !strings.Contains(relPath, "/"):
		return "interface"
	default:
		return ""
	}
}

func layerForImport(importPath string) string {
	switch {
	case importPath == modulePath:
		return "interface"
	case strings.HasPrefix(importPath, modulePath+"/internal/domain"):
		return "domain"
	case strings.HasPrefix(importPath, modulePath+"/internal/application"):
		return "application"
	case strings.HasPrefix(importPath, modulePath+"/internal/dto"):
		return "dto"
	case strings.HasPrefix(importPath, modulePath+"/internal/infrastructure"):
		return "infrastructure"
	case strings.HasPrefix(importPath, modulePath+"/internal/appcore"):
		return "interface"
	case strings.HasPrefix(importPath, modulePath+"/internal/architecture"):
		return ""
	case strings.HasPrefix(importPath, modulePath+"/internal/"):
		return "legacy-internal"
	default:
		return ""
	}
}

func collectGoFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "build" || name == "frontend" {
				return filepath.SkipDir
			}
			if path != root && filepath.Dir(path) == root && name != "internal" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func importsForFile(t *testing.T, file string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports for %s: %v", file, err)
	}
	imports := make([]string, 0, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	return imports
}

func selectorPath(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		prefix := selectorPath(typed.X)
		if prefix == "" {
			return typed.Sel.Name
		}
		return prefix + "." + typed.Sel.Name
	default:
		return ""
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func rel(t *testing.T, root string, file string) string {
	t.Helper()
	relPath, err := filepath.Rel(root, file)
	if err != nil {
		return file
	}
	return filepath.ToSlash(relPath)
}
