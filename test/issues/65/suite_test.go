package asyncapi_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/znas-io/asyncapi-codegen/pkg/asyncapi"
	"github.com/znas-io/asyncapi-codegen/pkg/codegen/generators"
)

func TestSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

type Suite struct {
	suite.Suite
}

func (suite *Suite) TestExtensionsWithSchema() {
	// Set specification
	spec := asyncapi.Specification{
		Components: asyncapi.Components{
			Schemas: map[string]*asyncapi.Schema{
				"flag": {
					Type:       asyncapi.MessageTypeIsInteger.String(),
					Extensions: asyncapi.Extensions{ExtGoType: "uint8"},
				},
			},
		},
	}

	// Generate code and test result
	res, err := generators.TypesGenerator{Specification: spec}.Generate()
	suite.Require().NoError(err)
	suite.Require().True(regexp.MustCompile("FlagSchema +uint8").Match([]byte(res)))
}

func (suite *Suite) TestExtensionsWithObjectProperty() {
	// Set specification
	spec := asyncapi.Specification{
		Components: asyncapi.Components{
			Schemas: map[string]*asyncapi.Schema{
				asyncapi.MessageTypeIsObject.String(): {
					Type: asyncapi.MessageTypeIsObject.String(),
					Properties: map[string]*asyncapi.Schema{
						"flag": {
							Type:       asyncapi.MessageTypeIsInteger.String(),
							Extensions: asyncapi.Extensions{ExtGoType: "uint8"},
						},
					},
					Required: []string{"flag"},
				},
			},
		},
	}

	// Generate code and test result
	res, err := generators.TypesGenerator{Specification: spec}.Generate()
	suite.Require().NoError(err)
	suite.Require().True(regexp.MustCompile("Flag +uint8").Match([]byte(res)))
}

func (suite *Suite) TestExtensionsWithArrayItem() {
	// Set specification
	spec := asyncapi.Specification{
		Components: asyncapi.Components{
			Schemas: map[string]*asyncapi.Schema{
				"flags": {
					Type: asyncapi.MessageTypeIsArray.String(),
					Items: &asyncapi.Schema{
						Type:       asyncapi.MessageTypeIsInteger.String(),
						Extensions: asyncapi.Extensions{ExtGoType: "uint8"},
					},
				},
			},
		},
	}

	// Generate code and test result
	res, err := generators.TypesGenerator{Specification: spec}.Generate()
	suite.Require().NoError(err)
	suite.Require().True(regexp.MustCompile(`FlagsSchema +\[\]uint8`).Match([]byte(res)))
}

func (suite *Suite) TestExtensionsWithObjectPropertyAndTypeFromPackage() {
	// Set specification
	spec := asyncapi.Specification{
		Components: asyncapi.Components{
			Schemas: map[string]*asyncapi.Schema{
				asyncapi.MessageTypeIsObject.String(): {
					Type: asyncapi.MessageTypeIsObject.String(),
					Properties: map[string]*asyncapi.Schema{
						"flag": {
							Type:       asyncapi.MessageTypeIsInteger.String(),
							Extensions: asyncapi.Extensions{ExtGoType: "mypackage.Flag"},
						},
					},
					Required: []string{"flag"},
				},
			},
		},
	}

	// Generate code and test result
	res, err := generators.TypesGenerator{Specification: spec}.Generate()
	suite.Require().NoError(err)
	suite.Require().True(regexp.MustCompile(`Flag +mypackage.Flag`).Match([]byte(res)))
}
