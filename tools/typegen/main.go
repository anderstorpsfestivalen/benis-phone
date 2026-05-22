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

	serviceSchemas := make([]ServiceSchemaInfo, 0, len(services))
	for _, svc := range services {
		args, err := parseServiceArgs(svc.Dir)
		if err != nil {
			fatal("parse %s args: %v", svc.Name, err)
		}
		fields, err := parseServiceTemplate(svc.Dir)
		if err != nil {
			fatal("parse %s template: %v", svc.Name, err)
		}
		serviceSchemas = append(serviceSchemas, ServiceSchemaInfo{
			Name:           svc.Name,
			Args:           args,
			TemplateFields: fields,
		})
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
	if err := writeServicesTS(serviceSchemas); err != nil {
		fatal("write services.ts: %v", err)
	}

	fmt.Fprintf(os.Stderr, "typegen: wrote %d structs and %d services to %s\n", len(filtered), len(serviceSchemas), outDir)
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

// ServiceInfo names a registered service and the on-disk directory of its
// package (e.g. {Name: "traintimes", Dir: "extensions/services/train"}).
type ServiceInfo struct {
	Name string
	Dir  string
}

// ArgSchema mirrors one field of a service's `Args` struct after tag parsing.
type ArgSchema struct {
	Name        string // toml-style lowercase
	Type        string // "string" | "number" | "boolean"
	Required    bool
	Description string
	Default     string
}

// TemplateField is one accessor path inside a service's `TemplateData` value,
// flattened from nested structs (e.g. ".Current.TempC").
type TemplateField struct {
	Path        string
	Type        string // "string" | "number" | "boolean" | "slice" | "struct"
	Description string
}

// ServiceSchemaInfo bundles everything the UI needs about one service.
type ServiceSchemaInfo struct {
	Name           string
	Args           []ArgSchema
	TemplateFields []TemplateField
}

func parseServiceRegistry(path string) ([]ServiceInfo, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	// alias → import path (e.g. "train" → ".../extensions/services/train")
	aliasToPath := map[string]string{}
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			// Default alias is the last path segment.
			alias = p
			if i := strings.LastIndex(alias, "/"); i >= 0 {
				alias = alias[i+1:]
			}
		}
		aliasToPath[alias] = p
	}

	var infos []ServiceInfo
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
				name, err := strconv.Unquote(key.Value)
				if err != nil {
					continue
				}
				selector := resolvePackageSelector(kv.Value)
				if selector == "" {
					continue
				}
				importPath, ok := aliasToPath[selector]
				if !ok {
					continue
				}
				// Convert the import path back to a repo-relative dir.
				dir := importPath
				const prefix = "github.com/anderstorpsfestivalen/benis-phone/"
				if strings.HasPrefix(dir, prefix) {
					dir = strings.TrimPrefix(dir, prefix)
				}
				infos = append(infos, ServiceInfo{Name: name, Dir: dir})
			}
		}
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// resolvePackageSelector pulls the package alias out of expressions like
// `&weather.Weather{}` or `weather.Weather{}` — returning "weather".
func resolvePackageSelector(expr ast.Expr) string {
	if u, ok := expr.(*ast.UnaryExpr); ok {
		expr = u.X
	}
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return ""
	}
	sel, ok := cl.Type.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}

// parseServiceArgs reads the `Args` struct from a service package and
// returns its fields as ArgSchema entries. A missing Args type returns an
// empty slice (no args).
func parseServiceArgs(dir string) ([]ArgSchema, error) {
	types, err := parsePackageStructs(dir)
	if err != nil {
		return nil, err
	}
	st, ok := types["Args"]
	if !ok {
		return nil, fmt.Errorf("%s: missing `type Args struct`", dir)
	}
	var out []ArgSchema
	for _, field := range st.Fields.List {
		for _, n := range field.Names {
			if !n.IsExported() {
				continue
			}
			tsType, _ := goTypeToTS(field.Type)
			argType := "string"
			switch tsType {
			case "number":
				argType = "number"
			case "boolean":
				argType = "boolean"
			}
			name := tomlOrLower(field.Tag, n.Name)
			required, def, desc := parseSchemaTags(field.Tag)
			out = append(out, ArgSchema{
				Name:        name,
				Type:        argType,
				Required:    required,
				Default:     def,
				Description: desc,
			})
		}
	}
	return out, nil
}

// parseServiceTemplate flattens `TemplateData` from a service package into a
// list of dot-paths usable in a Go text/template.
func parseServiceTemplate(dir string) ([]TemplateField, error) {
	types, err := parsePackageStructs(dir)
	if err != nil {
		return nil, err
	}
	st, ok := types["TemplateData"]
	if !ok {
		return nil, fmt.Errorf("%s: missing `type TemplateData struct`", dir)
	}
	var out []TemplateField
	walkTemplateStruct(st, types, "", 0, &out)
	return out, nil
}

const maxTemplateDepth = 3

func walkTemplateStruct(st *ast.StructType, all map[string]*ast.StructType, prefix string, depth int, out *[]TemplateField) {
	for _, field := range st.Fields.List {
		for _, n := range field.Names {
			if !n.IsExported() {
				continue
			}
			path := prefix + "." + n.Name
			_, desc := parseDescTag(field.Tag)
			tsType, isSlice, nested := classifyTemplateType(field.Type)
			*out = append(*out, TemplateField{
				Path:        path,
				Type:        tsType,
				Description: desc,
			})
			if isSlice {
				continue // don't recurse into slice element types (path syntax gets messy)
			}
			if nested != "" && depth+1 < maxTemplateDepth {
				if child, ok := all[nested]; ok {
					walkTemplateStruct(child, all, path, depth+1, out)
				}
			}
		}
	}
}

// classifyTemplateType returns (label, isSlice, nestedStructName).
// nestedStructName is non-empty when the field is a struct defined in the
// same package, signalling that recursion should follow.
func classifyTemplateType(expr ast.Expr) (string, bool, string) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string", false, ""
		case "bool":
			return "boolean", false, ""
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float32", "float64":
			return "number", false, ""
		default:
			// Named type — assume same-package struct; caller looks it up.
			return "struct", false, t.Name
		}
	case *ast.StarExpr:
		return classifyTemplateType(t.X)
	case *ast.ArrayType:
		_, _, nested := classifyTemplateType(t.Elt)
		_ = nested
		return "slice", true, ""
	case *ast.SelectorExpr:
		// External type (e.g. trainannouncement.TrainAnnouncement). We can't
		// see its fields, so surface the path but don't recurse.
		return "struct", false, ""
	}
	return "unknown", false, ""
}

// parsePackageStructs returns the map of struct name → AST node for one
// non-test Go package directory.
func parsePackageStructs(dir string) (map[string]*ast.StructType, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	out := map[string]*ast.StructType{}
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
					out[ts.Name.Name] = st
				}
			}
		}
	}
	return out, nil
}

// parseSchemaTags pulls `schema:"required"`, `schema:"default=auto"` and
// `desc:"..."` out of a struct tag.
func parseSchemaTags(tag *ast.BasicLit) (required bool, def string, desc string) {
	if tag == nil {
		return false, "", ""
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return false, "", ""
	}
	st := reflect.StructTag(raw)
	schema := st.Get("schema")
	for _, part := range strings.Split(schema, ",") {
		part = strings.TrimSpace(part)
		switch {
		case part == "required":
			required = true
		case strings.HasPrefix(part, "default="):
			def = strings.TrimPrefix(part, "default=")
		}
	}
	desc = st.Get("desc")
	return required, def, desc
}

func parseDescTag(tag *ast.BasicLit) (string, string) {
	if tag == nil {
		return "", ""
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return "", ""
	}
	st := reflect.StructTag(raw)
	return "", st.Get("desc")
}

func tomlOrLower(tag *ast.BasicLit, goName string) string {
	if name, _ := parseTomlTag(tag); name != "" {
		return name
	}
	return strings.ToLower(goName)
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

func writeServicesTS(schemas []ServiceSchemaInfo) error {
	var b strings.Builder
	writeHeader(&b)
	b.WriteString(`
export type ArgSchema = {
  name: string;
  type: "string" | "number" | "boolean";
  required: boolean;
  description: string;
  default: string;
};

export type TemplateField = {
  path: string;
  type: "string" | "number" | "boolean" | "slice" | "struct" | "unknown";
  description: string;
};

export type ServiceSchema = {
  args: readonly ArgSchema[];
  templateFields: readonly TemplateField[];
};

`)

	if len(schemas) == 0 {
		b.WriteString("export const SERVICE_SCHEMAS: Record<string, ServiceSchema> = {};\n")
		b.WriteString("export const SERVICE_NAMES = [] as const;\n")
		b.WriteString("export type ServiceName = never;\n")
		return os.WriteFile(filepath.Join(outDir, "services.ts"), []byte(b.String()), 0o644)
	}

	// We emit the schema literals into a helper `_RAW` so `as const` can pin
	// the service-name keys for ServiceName, while the public
	// SERVICE_SCHEMAS export keeps the wider ServiceSchema field types so
	// consumers can iterate args without bumping into narrow literal types.
	b.WriteString("const _RAW = {\n")
	for _, s := range schemas {
		b.WriteString(fmt.Sprintf("  %q: {\n", s.Name))
		// args
		b.WriteString("    args: [")
		if len(s.Args) > 0 {
			b.WriteString("\n")
			for _, a := range s.Args {
				b.WriteString(fmt.Sprintf("      { name: %q, type: %q, required: %t, description: %q, default: %q },\n",
					a.Name, a.Type, a.Required, a.Description, a.Default))
			}
			b.WriteString("    ")
		}
		b.WriteString("],\n")
		// template fields
		b.WriteString("    templateFields: [")
		if len(s.TemplateFields) > 0 {
			b.WriteString("\n")
			for _, f := range s.TemplateFields {
				b.WriteString(fmt.Sprintf("      { path: %q, type: %q, description: %q },\n",
					f.Path, f.Type, f.Description))
			}
			b.WriteString("    ")
		}
		b.WriteString("],\n")
		b.WriteString("  },\n")
	}
	b.WriteString("} as const;\n\n")

	b.WriteString("export type ServiceName = keyof typeof _RAW;\n")
	b.WriteString("export const SERVICE_SCHEMAS: Record<ServiceName, ServiceSchema> = _RAW;\n")
	b.WriteString("export const SERVICE_NAMES = Object.keys(SERVICE_SCHEMAS) as ServiceName[];\n")

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
