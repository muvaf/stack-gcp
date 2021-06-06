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
	"path/filepath"
	"strings"

	"github.com/muvaf/typewriter/pkg/cmd"
	"github.com/muvaf/typewriter/pkg/packages"
	twtypes "github.com/muvaf/typewriter/pkg/types"
	"github.com/muvaf/typewriter/pkg/wrapper"
	"github.com/pkg/errors"

	"github.com/crossplane/provider-gcp/internal/generator/templates"
)

func NewGroup(shortName, longName, apiVersion string) *Group {
	return &Group{
		ShortName:  shortName,
		LongName:   longName,
		APIVersion: apiVersion,
	}
}

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

func NewResources(c *packages.Cache, remotePackagePath, localPkgPath string, group Group) *Resources {
	return &Resources{
		cache:             c,
		LocalPackagePath:  localPkgPath,
		RemotePackagePath: remotePackagePath,
		Group:             group,
	}
}

type Resources struct {
	cache             *packages.Cache
	LocalPackagePath  string
	RemotePackagePath string
	Group             Group
}

func (r *Resources) GenerateCRDFile(googleGroupName, googleResourceName string) ([]byte, error) {
	localPkgPath, localPkgName := r.LocalPackagePath, r.LocalPackagePath[strings.LastIndex(r.LocalPackagePath, "/")+1:]
	remotePkgPath := filepath.Join(r.RemotePackagePath, googleGroupName)
	file := wrapper.NewFile(localPkgName, templates.CRDTypesTemplate,
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"))

	localPkg, err := r.cache.GetPackage(localPkgPath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load local package: %s", localPkg)
	}

	paramsName := types.NewTypeName(token.NoPos, localPkg.Types, fmt.Sprintf("%sParameters", googleResourceName), nil)
	remoteNamed, err := r.cache.GetType(remotePkgPath, googleResourceName)
	if err != nil {
		panic(err)
	}

	pt := cmd.Type{
		Imports:   file.Imports,
		Cache:     r.cache,
		Generator: twtypes.NewMerger(paramsName, []*types.Named{remoteNamed}),
	}
	paramsStr, err := pt.Run()
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate params struct")
	}

	// TODO(muvaf): We need a way to to figure out which fields are update-able
	// which are not and don't repeat all fields in both spec and status.

	observationName := types.NewTypeName(token.NoPos, localPkg.Types, fmt.Sprintf("%sObservation", googleResourceName), nil)
	ot := cmd.Type{
		Imports:   file.Imports,
		Cache:     r.cache,
		Generator: twtypes.NewMerger(observationName, []*types.Named{remoteNamed}),
	}
	observationStr, err := ot.Run()
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate observation struct")
	}

	input := map[string]interface{}{
		"CRD": map[string]string{
			"Kind": strings.Title(googleResourceName),
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
