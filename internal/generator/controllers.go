package generator

import (
	"strings"

	"github.com/muvaf/typewriter/pkg/packages"
	"github.com/muvaf/typewriter/pkg/wrapper"
	"github.com/pkg/errors"

	"github.com/crossplane/provider-gcp/internal/generator/templates"
)

func NewConversions(c *packages.Cache) *Conversions {
	return &Conversions{
		cache: c,
	}
}

type Conversions struct {
	cache *packages.Cache
}

func (c *Conversions) GenerateConversionsFile(sourceTypePath, targetTypePath string) ([]byte, error) {
	targetPkgName := targetTypePath[strings.LastIndex(targetTypePath, ".")+1:]
	file := wrapper.NewFile(strings.ToLower(targetPkgName), templates.ConversionsTemplate,
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"))
	lateInitGen := NewLateInitializeFn(c.cache, file.Imports)
	lateInitFn, err := lateInitGen.Generate(sourceTypePath, targetTypePath)
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate late initialize function")
	}
	input := map[string]interface{}{
		"LateInitializeFn": lateInitFn,
	}
	b, err := file.Wrap(input)
	if err != nil {
		return nil, errors.Wrap(err, "cannot wrap conversions file")
	}
	return b, nil
}
