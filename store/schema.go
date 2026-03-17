package store

import (
	_ "embed"

	"github.com/dpopsuev/origami/connectors/sqlite"
)

//go:embed testdata/schema.yaml
var schemaData []byte

// LoadSchema parses the embedded reference schema. This is a fallback for
// tests and backward compatibility; production binaries should inject their
// own schema via WithStoreSchema in the fold-generated main.go.
func LoadSchema() (*sqlite.Schema, error) {
	return sqlite.ParseSchema(schemaData)
}
