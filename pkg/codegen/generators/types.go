package generators

import (
	"bytes"

	"github.com/lerenn/asyncapi-codegen/pkg/asyncapi"
)

// TypesGenerator is a code generator for types that will generate all schemas
// contained in an asyncapi specification to golang structures code.
type TypesGenerator struct {
	asyncapi.Specification
}

// Generate will create a new types code generator.
func (tg TypesGenerator) Generate() (string, error) {
	tmplt, err := loadTemplate(
		typesTemplatePath,
		anyTemplatePath,
		messageTemplatePath,
		parameterTemplatePath,
	)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err := tmplt.Execute(buf, tg); err != nil {
		return "", err
	}

	return buf.String(), nil
}
