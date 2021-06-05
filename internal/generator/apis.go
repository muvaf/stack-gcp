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
	"go/token"
	"go/types"
	"strings"

	"github.com/muvaf/typewriter/pkg/cmd"
	"github.com/muvaf/typewriter/pkg/packages"
	twtypes "github.com/muvaf/typewriter/pkg/types"
	"github.com/muvaf/typewriter/pkg/wrapper"
	"github.com/pkg/errors"

	"github.com/crossplane/provider-gcp/internal/generator/templates"
)

type Group struct {
	ShortName  string
	LongName   string
	APIVersion string
}

func (g *Group) GenerateDocFile() ([]byte, error) {
	input := map[string]interface{}{
		"Group": g,
	}
	doc := wrapper.NewFile(g.APIVersion, templates.DocFileTemplate,
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"))
	b, err := doc.Wrap(input)
	if err != nil {
		return nil, errors.Wrap(err, "cannot wrap doc file")
	}
	return b, nil
}

func (g *Group) GenerateGroupVersionFile() ([]byte, error) {
	input := map[string]interface{}{
		"Group": g,
	}
	gv := wrapper.NewFile(g.APIVersion, templates.GroupVersionInfoTemplate,
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"))
	b, err := gv.Wrap(input)
	if err != nil {
		return nil, errors.Wrap(err, "cannot wrap groupversion file")
	}
	return b, nil
}

type CRD struct {
	Cache              *packages.Cache
	LocalPackagePath   string
	GoogleGroupName    string
	GoogleResourceName string
	Group              Group
}

func (c *CRD) GenerateCRDFile() ([]byte, error) {
	localPkgPath, localPkgName := c.LocalPackagePath, c.LocalPackagePath[strings.LastIndex(c.LocalPackagePath, "/")+1:]
	remotePkgPath := fmt.Sprintf("github.com/GoogleCloudPlatform/declarative-resource-client-library/services/google/%s", c.GoogleGroupName)
	file := wrapper.NewFile(localPkgName, templates.CRDTypesTemplate,
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"))

	localPkg, err := c.Cache.GetPackage(localPkgPath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load local package: %s", localPkg)
	}

	paramsName := types.NewTypeName(token.NoPos, localPkg.Types, fmt.Sprintf("%sParameters", c.GoogleResourceName), nil)
	remoteNamed, err := c.Cache.GetType(remotePkgPath, c.GoogleResourceName)
	if err != nil {
		panic(err)
	}

	paramsMerger := twtypes.NewMerger(paramsName, []*types.Named{remoteNamed})
	pt := cmd.Type{
		Imports:   file.Imports,
		Cache:     c.Cache,
		Generator: paramsMerger,
	}
	paramsStr, err := pt.Run()
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate params struct")
	}

	// TODO(muvaf): We need a way to to figure out which fields are update-able
	// which are not and don't repeat all fields in both spec and status.

	observationName := types.NewTypeName(token.NoPos, localPkg.Types, fmt.Sprintf("%sObservation", c.GoogleResourceName), nil)
	observationMerger := twtypes.NewMerger(observationName, []*types.Named{remoteNamed})
	ot := cmd.Type{
		Imports:   file.Imports,
		Cache:     c.Cache,
		Generator: observationMerger,
	}
	observationStr, err := ot.Run()
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate observation struct")
	}

	input := map[string]interface{}{
		"CRD": map[string]string{
			"Kind": strings.Title(c.GoogleResourceName),
		},
		"Types": map[string]string{
			"Parameters":  paramsStr,
			"Observation": observationStr,
		},
	}
	b, err := file.Wrap(input)
	if err != nil {
		return nil, errors.Wrap(err, "cannot wrap crd file")
	}
	return b, nil
}
