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
	"strings"

	"github.com/pkg/errors"

	"github.com/muvaf/typewriter/pkg/packages"
	"github.com/muvaf/typewriter/pkg/traverser"
)

const (
	IsUpToDateFuncTmpl = `
// {{ .FunctionName }} compares a {{ .ATypeName }} with {{ .BTypeName }} and
// reports whether there is any difference with between matching fields.
func {{ .FunctionName }}(a {{ .ATypeName }}, b {{ .BTypeName }}) bool {
  {{ .Statements }}
  return true
}`
	IsUpToDateMapTmpl = `
if len({{ .AFieldPath }}) != len({{ .BFieldPath }}) {
  return false
}
for {{ .Key }} := range {{ .BFieldPath }} {
{{ .Statements }}
}`
	IsUpToDateSliceTmpl = `
if len({{ .AFieldPath }}) != len({{ .BFieldPath }}) {
  return false
}
for {{ .Index }} := range {{ .BFieldPath }} {
{{ .Statements }}
}`
	IsUpToDatePointerTmpl = `
if ({{ .AFieldPath }} == nil) != ({{ .BFieldPath }} == nil) {
  return false
}
if {{ .BFieldPath }} != nil && {{ .AFieldPath }} != nil {
{{ .Statements }}
}`
	IsUpToDateBasicTmpl = "\nif {{ .AFieldPath }} != {{ .BFieldPath }} {\nreturn false\n}"
)

var (
	IsUpToDateBasicPtrTmpl = map[types.BasicKind]string{
		types.Bool:   "\nif <pkg-placeholder>BoolValue({{ .AFieldPath }}) != <pkg-placeholder>BoolValue({{ .BFieldPath }}) {\nreturn false\n}",
		types.String: "\nif <pkg-placeholder>StringValue({{ .AFieldPath }}) != <pkg-placeholder>StringValue({{ .BFieldPath }}) {\nreturn false\n}",
		types.Int64:  "\nif <pkg-placeholder>Int64Value({{ .AFieldPath }}) != <pkg-placeholder>Int64Value({{ .BFieldPath }}) {\nreturn false\n}",
	}
)

func NewIsUpToDateFn(c *packages.Cache, im *packages.Imports) *IsUpToDateFn {
	return &IsUpToDateFn{
		cache:   c,
		imports: im,
	}
}

type IsUpToDateFn struct {
	cache   *packages.Cache
	imports *packages.Imports
}

func (li *IsUpToDateFn) Generate(sourceType, targetType *types.Named) (string, error) {
	clientsPkg := li.imports.UsePackage("github.com/crossplane/provider-gcp/pkg/clients")
	// TODO(muvaf): This is a temporary workaround. Printer accepts extra inputs
	// only for its high level execution. Traversers do not accept any extra input
	// yet.
	basicPtrTmpl := map[types.BasicKind]string{}
	for k, v := range IsUpToDateBasicPtrTmpl {
		basicPtrTmpl[k] = strings.ReplaceAll(v, "<pkg-placeholder>", clientsPkg)
	}
	basicTmpl := map[types.BasicKind]string{}
	for i := 1; i < 26; i++ {
		basicTmpl[types.BasicKind(i)] = IsUpToDateBasicTmpl
	}
	tr := traverser.NewGeneric(li.imports,
		traverser.WithBasicPointerTemplate(basicPtrTmpl),
		traverser.WithBasicTemplate(basicTmpl),
		traverser.WithPointerTemplate(IsUpToDatePointerTmpl),
		traverser.WithMapTemplate(IsUpToDateMapTmpl),
		traverser.WithSliceTemplate(IsUpToDateSliceTmpl),
	)
	printer := traverser.NewPrinter(li.imports, tr,
		traverser.WithTemplate(IsUpToDateFuncTmpl))
	isUpToDateStr, err := printer.Print("isUpToDate", sourceType, targetType, nil)
	if err != nil {
		return "", errors.Wrap(err, "cannot print isUpToDate function")
	}
	return isUpToDateStr, nil
}
