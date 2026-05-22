// Package functions defines the IVR config data model (TOML-driven).
//
// The TypeScript editor in /ui consumes a code-generated mirror of these
// structs. To regenerate after changing any struct here, run from the
// repo root:
//
//	go generate ./...
//
// CI fails if the generated files in ui/src/generated/ are out of date.
package functions

//go:generate go run ../../tools/typegen
