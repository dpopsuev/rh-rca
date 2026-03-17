package rca

import "github.com/dpopsuev/origami/schematics/toolkit"

// Thin wrappers delegating to toolkit. Unexported for RCA internal use.

func mapStr(m map[string]any, key string) string     { return toolkit.MapStr(m, key) }
func mapFloat(m map[string]any, key string) float64   { return toolkit.MapFloat(m, key) }
func mapBool(m map[string]any, key string) bool       { return toolkit.MapBool(m, key) }
func mapInt64(m map[string]any, key string) int64     { return toolkit.MapInt64(m, key) }
func mapStrSlice(m map[string]any, key string) []string { return toolkit.MapStrSlice(m, key) }
func mapMap(m map[string]any, key string) map[string]any { return toolkit.MapMap(m, key) }
func mapSlice(m map[string]any, key string) []any     { return toolkit.MapSlice(m, key) }
func asMap(v any) map[string]any                      { return toolkit.AsMap(v) }
