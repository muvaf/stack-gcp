/*
Copyright 2021 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generator

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/pkg/errors"

	"github.com/muvaf/typewriter/pkg/packages"
	"github.com/muvaf/typewriter/pkg/traverser"
)

func NewProducerFn(c *packages.Cache, im *packages.Imports) *ProducerFn {
	return &ProducerFn{
		cache:   c,
		imports: im,
	}
}

type ProducerFn struct {
	cache   *packages.Cache
	imports *packages.Imports
}

func (li *ProducerFn) Generate(sourceType, targetType *types.Named) (string, error) {
	tr := traverser.NewGeneric(li.imports)
	printer := traverser.NewPrinter(li.imports, tr)
	name := fmt.Sprintf("generate%s", strings.Title(targetType.Obj().Name()))
	producerStr, err := printer.Print(name, sourceType, targetType, nil)
	if err != nil {
		return "", errors.Wrap(err, "cannot print producer function")
	}
	return producerStr, nil
}
