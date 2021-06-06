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
	"go/types"

	"github.com/pkg/errors"

	"github.com/muvaf/typewriter/pkg/packages"
	"github.com/muvaf/typewriter/pkg/traverser"
)

const (
	LateInitializeFuncTmpl = `
// {{ .FunctionName }} late initializes the {{ .ATypeName }} with the information
// from given {{ .BTypeName }} and reports whether any change has been made.
func {{ .FunctionName }}(a {{ .ATypeName }}, b {{ .BTypeName }}) bool {
  li := {{ .RuntimeResourceImportAlias }}NewLateInitializer()
  {{ .Statements }}
  return li.IsChanged()
}`
	LateInitializeMapTmpl = `
if len({{ .AFieldPath }}) == 0 && len({{ .BFieldPath }}) != 0 {
  {{ .AFieldPath }} = make({{ .TypeA }}, len({{ .BFieldPath }}))
  for {{ .Key }} := range {{ .BFieldPath }} {
    {{ .Statements }}
  }
}`
	LateInitializeSliceTmpl = `
if len({{ .AFieldPath }}) == 0 && len({{ .BFieldPath }}) != 0 {
  {{ .AFieldPath }} = make({{ .TypeA }}, len({{ .BFieldPath }}))
  for {{ .Index }} := range {{ .BFieldPath }} {
    {{ .Statements }}
  }
}`
	LateInitializePointerTmpl = `
if {{ .BFieldPath }} != nil {
  if {{ .AFieldPath }} == nil {
    {{ .AFieldPath }} = new({{ .NonPointerTypeA }})
  }
  {{ .Statements }}
}`
)

var (
	LateInitializeBasicPtrTmpl = map[types.BasicKind]string{
		types.Bool:   "\n{{ .AFieldPath }} = li.LateInitializeBoolPtr({{ .AFieldPath }}, {{ .BFieldPath }})",
		types.String: "\n{{ .AFieldPath }} = li.LateInitializeStringPtr({{ .AFieldPath }}, {{ .BFieldPath }})",
		types.Int64:  "\n{{ .AFieldPath }} = li.LateInitializeInt64Ptr({{ .AFieldPath }}, {{ .BFieldPath }})",
	}
)

func NewLateInitializeFn(c *packages.Cache, im *packages.Imports) *LateInitializeFn {
	return &LateInitializeFn{
		cache:   c,
		imports: im,
	}
}

type LateInitializeFn struct {
	cache   *packages.Cache
	imports *packages.Imports
}

func (li *LateInitializeFn) Generate(sourceTypePath, targetTypePath string) (string, error) {
	sourceType, err := li.cache.GetTypeWithFullPath(sourceTypePath)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get source type: %s", sourceTypePath)
	}
	targetType, err := li.cache.GetTypeWithFullPath(targetTypePath)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get target type: %s", targetTypePath)
	}
	tr := traverser.NewGeneric(li.imports,
		traverser.WithBasicPointerTemplate(LateInitializeBasicPtrTmpl),
		traverser.WithPointerTemplate(LateInitializePointerTmpl),
		traverser.WithMapTemplate(LateInitializeMapTmpl),
		traverser.WithSliceTemplate(LateInitializeSliceTmpl),
	)
	printer := traverser.NewPrinter(li.imports, tr,
		traverser.WithTemplate(LateInitializeFuncTmpl))
	lateInitStr, err := printer.Print("lateInitialize", sourceType, targetType, map[string]interface{}{
		"RuntimeResourceImportAlias": li.imports.UsePackage("github.com/crossplane/crossplane-runtime/pkg/resource"),
	})
	if err != nil {
		return "", errors.Wrap(err, "cannot print lateInitialize function")
	}
	return lateInitStr, nil
}
