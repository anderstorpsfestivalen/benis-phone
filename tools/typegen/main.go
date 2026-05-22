// typegen walks core/functions/ and extensions/services/services.go and
// emits TypeScript interfaces, Zod schemas, and the service registry as
// string-literal unions into ui/src/generated/.
//
// Run via `go generate ./...` (a //go:generate directive lives in
// core/functions/doc.go) or directly with `go run ./tools/typegen`.
//
// Field naming mirrors BurntSushi/toml: a `toml:"foo"` tag wins, otherwise
// the lowercased Go field name is used. Unexported fields and fields with
// `toml:"-"` are skipped.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	functionsDir = "core/functions"
	servicesFile = "extensions/services/services.go"
	outDir       = "ui/src/generated"
)

type Field struct {
	Name      string // TS field name (toml tag or lowercased Go)
	GoName    string // Original Go field name
	TSType    string
	Optional  bool
	IsPointer bool
}

type Struct struct {
	Name   string
	Fields []Field
	Doc    string
}

// skipFields lists struct fields that exist on the Go side for runtime use
// (e.g. hydrated lookup maps) but are not part of the editable config model
// the UI / TOML sees. Keyed by "StructName.FieldGoName".
var skipFields = map[string]bool{
	"Definition.Functions": true, // hydrated lookup map; built from UnsortedFunctions at load time
}

func main() {
	root := findRepoRoot()
	if root == "" {
		fatal("could not locate repo root (looked for go.mod)")
	}
	if err := os.Chdir(root); err != nil {
		fatal("chdir: %v", err)
	}

	structs, err := parseStructs(functionsDir)
	if err != nil {
		fatal("parse structs: %v", err)
	}

	// Only emit the structs we actually need in the UI. The IVR runtime has
	// internal/unexported state (Queue's chans/timers, ServiceResponse) we
	// don't want to leak into TS — restrict to the data-model surface.
	wanted := map[string]bool{
		"Definition": true,
		"General":    true,
		"SIPConfig":  true,
		"Fn":         true,
		"Action":     true,
		"LiveFeed":   true,
		"Prefix":     true,
		"TTS":        true,
		"File":       true,
		"RandomFile": true,
		"Service":    true,
		"Gate":       true,
		"Recording":  true,
		"Queue":      true,
		"QueuePrompt": true,
		"Playable":    true,
	}
	filtered := make([]Struct, 0, len(wanted))
	for _, s := range structs {
		if wanted[s.Name] {
			filtered = append(filtered, s)
		}
	}
	// Stable order: dependencies first, then the rest alphabetically. We
	// just sort alphabetically — TS doesn't need forward-declarations for
	// interfaces.
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })

	services, err := parseServiceRegistry(servicesFile)
	if err != nil {
		fatal("parse services: %v", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal("mkdir out: %v", err)
	}

	if err := writeConfigTS(filtered); err != nil {
		fatal("write config.ts: %v", err)
	}
	if err := writeSchemasTS(filtered); err != nil {
		fatal("write schemas.ts: %v", err)
	}
	if err := writeServicesTS(services); err != nil {
		fatal("write services.ts: %v", err)
	}

	fmt.Fprintf(os.Stderr, "typegen: wrote %d structs and %d services to %s\n", len(filtered), len(services), outDir)
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func parseStructs(dir string) ([]Struct, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var structs []Struct
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					continue
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					structs = append(structs, structFromAST(ts.Name.Name, st))
				}
			}
		}
	}
	return structs, nil
}

func structFromAST(name string, st *ast.StructType) Struct {
	s := Struct{Name: name}
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			// Embedded — skip; none in our wanted set.
			continue
		}
		for _, n := range field.Names {
			if !n.IsExported() {
				continue
			}
			if skipFields[name+"."+n.Name] {
				continue
			}
			tomlName, skip := parseTomlTag(field.Tag)
			if skip {
				continue
			}
			tsType, isPtr := goTypeToTS(field.Type)
			if tsType == "" {
				// Unsupported type (channel, func, etc.) — skip silently.
				continue
			}
			fieldName := tomlName
			if fieldName == "" {
				fieldName = strings.ToLower(n.Name)
			}
			s.Fields = append(s.Fields, Field{
				Name:      fieldName,
				GoName:    n.Name,
				TSType:    tsType,
				Optional:  isPtr,
				IsPointer: isPtr,
			})
		}
	}
	return s
}

func parseTomlTag(tag *ast.BasicLit) (name string, skip bool) {
	if tag == nil {
		return "", false
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return "", false
	}
	t := reflect.StructTag(raw).Get("toml")
	if t == "" {
		return "", false
	}
	parts := strings.Split(t, ",")
	if parts[0] == "-" {
		return "", true
	}
	return parts[0], false
}

func goTypeToTS(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string", false
		case "bool":
			return "boolean", false
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float32", "float64":
			return "number", false
		default:
			// Assume named struct in same package
			return t.Name, false
		}
	case *ast.StarExpr:
		inner, _ := goTypeToTS(t.X)
		if inner == "" {
			return "", false
		}
		return inner + " | null", true
	case *ast.ArrayType:
		inner, _ := goTypeToTS(t.Elt)
		if inner == "" {
			return "", false
		}
		return inner + "[]", false
	case *ast.MapType:
		k, _ := goTypeToTS(t.Key)
		v, _ := goTypeToTS(t.Value)
		if k == "" || v == "" {
			return "", false
		}
		return fmt.Sprintf("Record<%s, %s>", k, v), false
	case *ast.SelectorExpr:
		// e.g. template.Template — not part of our data model
		return "", false
	case *ast.InterfaceType, *ast.ChanType, *ast.FuncType:
		return "", false
	}
	return "", false
}

func parseServiceRegistry(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 || vs.Names[0].Name != "ServiceRegistry" {
				continue
			}
			if len(vs.Values) == 0 {
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.BasicLit)
				if !ok || key.Kind != token.STRING {
					continue
				}
				s, err := strconv.Unquote(key.Value)
				if err != nil {
					continue
				}
				names = append(names, s)
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

func writeConfigTS(structs []Struct) error {
	var b strings.Builder
	writeHeader(&b)
	b.WriteString("\n")

	for _, s := range structs {
		b.WriteString(fmt.Sprintf("export interface %s {\n", s.Name))
		for _, f := range s.Fields {
			opt := ""
			if f.Optional {
				opt = "?"
			}
			b.WriteString(fmt.Sprintf("  %s%s: %s;\n", quoteFieldIfNeeded(f.Name), opt, f.TSType))
		}
		b.WriteString("}\n\n")
	}

	// Discriminated-union helper for Action. Kept hand-maintained but
	// derived from the same Action interface so it can't drift in shape —
	// only in which fields we treat as the discriminator.
	b.WriteString(actionVariantSnippet)

	return os.WriteFile(filepath.Join(outDir, "config.ts"), []byte(b.String()), 0o644)
}

func writeSchemasTS(structs []Struct) error {
	var b strings.Builder
	writeHeader(&b)
	b.WriteString("\nimport { z } from \"zod\";\n\n")

	// Forward-declare lazy refs so cyclic types (Definition -> Fn -> Action -> ...) work.
	for _, s := range structs {
		b.WriteString(fmt.Sprintf("export const %sSchema: z.ZodType<unknown> = z.lazy(() => z.object({\n", lowerFirst(s.Name)))
		for _, f := range s.Fields {
			b.WriteString(fmt.Sprintf("  %s: %s,\n", quoteFieldIfNeeded(f.Name), zodFor(f)))
		}
		b.WriteString("}).passthrough());\n\n")
	}

	return os.WriteFile(filepath.Join(outDir, "schemas.ts"), []byte(b.String()), 0o644)
}

func writeServicesTS(names []string) error {
	var b strings.Builder
	writeHeader(&b)
	b.WriteString("\n")
	if len(names) == 0 {
		b.WriteString("export const SERVICE_NAMES = [] as const;\n")
		b.WriteString("export type ServiceName = never;\n")
		return os.WriteFile(filepath.Join(outDir, "services.ts"), []byte(b.String()), 0o644)
	}
	b.WriteString("export const SERVICE_NAMES = [\n")
	for _, n := range names {
		b.WriteString(fmt.Sprintf("  %q,\n", n))
	}
	b.WriteString("] as const;\n\n")
	b.WriteString("export type ServiceName = (typeof SERVICE_NAMES)[number];\n")
	return os.WriteFile(filepath.Join(outDir, "services.ts"), []byte(b.String()), 0o644)
}

func writeHeader(b *strings.Builder) {
	b.WriteString("// CODE GENERATED BY tools/typegen — DO NOT EDIT.\n")
	b.WriteString("// Regenerate with `go generate ./...` from the repo root.\n")
}

func zodFor(f Field) string {
	// Resolve top-level primitives or known struct refs.
	t := f.TSType
	// Optional / pointer
	pointer := f.IsPointer

	var base string
	switch {
	case t == "string":
		base = "z.string()"
	case t == "number":
		base = "z.number()"
	case t == "boolean":
		base = "z.boolean()"
	case strings.HasPrefix(t, "Record<"):
		// Record<string, string> -> z.record(z.string(), z.string())
		inner := strings.TrimSuffix(strings.TrimPrefix(t, "Record<"), ">")
		parts := strings.SplitN(inner, ", ", 2)
		if len(parts) == 2 {
			base = fmt.Sprintf("z.record(%s, %s)", zodForTypeStr(parts[0]), zodForTypeStr(parts[1]))
		} else {
			base = "z.record(z.string(), z.unknown())"
		}
	case strings.HasSuffix(t, "[]"):
		inner := strings.TrimSuffix(t, "[]")
		base = fmt.Sprintf("z.array(%s)", zodForTypeStr(inner))
	case strings.HasSuffix(t, " | null"):
		inner := strings.TrimSuffix(t, " | null")
		base = fmt.Sprintf("%s.nullable()", zodForTypeStr(inner))
	default:
		base = lowerFirst(t) + "Schema"
	}

	if pointer && !strings.HasSuffix(t, " | null") {
		base += ".nullable()"
	}
	base += ".optional()"
	return base
}

func zodForTypeStr(t string) string {
	t = strings.TrimSpace(t)
	if strings.HasSuffix(t, " | null") {
		inner := strings.TrimSuffix(t, " | null")
		return zodForTypeStr(inner) + ".nullable()"
	}
	if strings.HasSuffix(t, "[]") {
		return fmt.Sprintf("z.array(%s)", zodForTypeStr(strings.TrimSuffix(t, "[]")))
	}
	if strings.HasPrefix(t, "Record<") {
		inner := strings.TrimSuffix(strings.TrimPrefix(t, "Record<"), ">")
		parts := strings.SplitN(inner, ", ", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("z.record(%s, %s)", zodForTypeStr(parts[0]), zodForTypeStr(parts[1]))
		}
	}
	switch t {
	case "string":
		return "z.string()"
	case "number":
		return "z.number()"
	case "boolean":
		return "z.boolean()"
	}
	return lowerFirst(t) + "Schema"
}

// lowerFirst lowercases the leading run of capitals so common Go names
// produce readable JS identifiers: TTS -> tts, SIPConfig -> sipConfig,
// LiveFeed -> liveFeed.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	n := 0
	for n < len(runes) && unicode.IsUpper(runes[n]) {
		n++
	}
	switch {
	case n == 0:
		return s
	case n == 1, n == len(runes):
		for i := 0; i < n; i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
	default:
		for i := 0; i < n-1; i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
	}
	return string(runes)
}

func quoteFieldIfNeeded(name string) string {
	if name == "" {
		return `""`
	}
	for i, r := range name {
		if i == 0 && !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return strconv.Quote(name)
		}
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return strconv.Quote(name)
		}
	}
	return name
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "typegen: "+format+"\n", args...)
	os.Exit(1)
}

// actionVariantSnippet is appended verbatim to config.ts. It models the
// "exactly one action kind per Action" invariant from action.Type() in a
// way the editor can switch on. The Action interface above stays flat to
// match the Go struct / TOML roundtrip — the variant type is a UI
// convenience layered on top.
const actionVariantSnippet = `export type ActionKind =
  | "dst"
  | "file"
  | "randomfile"
  | "tts"
  | "srv"
  | "dispatcher"
  | "transfer"
  | "hangup"
  | "record"
  | "dtmf"
  | "livefeed"
  | "clear";

export const ACTION_KINDS: readonly ActionKind[] = [
  "dst",
  "file",
  "randomfile",
  "tts",
  "srv",
  "dispatcher",
  "transfer",
  "hangup",
  "record",
  "dtmf",
  "livefeed",
  "clear",
];

/** Inspect an Action and report which kind it currently represents,
 *  mirroring core/functions/action.go:Action.Type(). */
export function actionKind(a: Action): ActionKind | null {
  if (a.dst) return "dst";
  if (a.file && a.file.src) return "file";
  if (a.randomfile && a.randomfile.folder) return "randomfile";
  if (a.tts && a.tts.msg) return "tts";
  if (a.srv && a.srv.dst) return "srv";
  if (a.dispatcher) return "dispatcher";
  if (a.transfer) return "transfer";
  if (a.hangup) return "hangup";
  if (a.record) return "record";
  if (a.dtmf) return "dtmf";
  if (a.livefeed) return "livefeed";
  if (a.clear) return "clear";
  return null;
}
`
