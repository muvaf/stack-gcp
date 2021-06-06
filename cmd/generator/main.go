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

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"

	"github.com/crossplane/provider-gcp/internal/generator"
	"github.com/muvaf/typewriter/pkg/packages"
)

var cli GeneratorCLI

func main() {
	kongCtx := kong.Parse(&cli)
	ctx := context.WithValue(context.TODO(), "debug", &cli.Debug)
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	kongCtx.FatalIfErrorf(kongCtx.Run())
}

type GeneratorCLI struct {
	Debug       bool           `kong:"help:'Enable debug mode. This will disable all linters'"`
	Group       GroupCmd       `kong:"cmd,help:'Generate group files.'"`
	Crds        CrdsCmd        `kong:"cmd,help:'Generate CRD files.'"`
	Controllers ControllersCmd `kong:"cmd,help:'Generate controller files.'"`
}

const (
	GoogleDCLPackagePath = "github.com/GoogleCloudPlatform/declarative-resource-client-library/services/google"

	APISFolderPath        = "apis"
	ControllersFolderPath = "pkg/controller"

	DocFileName              = "zz_doc.go"
	GroupVersionInfoFileName = "zz_groupversion_info.go"
	CRDTypesFileNameFmt      = "zz_%s_types.go"
	ConversionsFileName      = "zz_conversions.go"
)

type GroupCmd struct {
	ShortName  string `kong:"required,help:'Single word name of the group, such as container.'"`
	APIVersion string `kong:"required,help:'API version of the group, such as v1alpha1'"`
}

func (g *GroupCmd) Run(ctx context.Context) error {
	group := &generator.Group{
		ShortName:  g.ShortName,
		LongName:   g.ShortName + ".gcp.crossplane.io",
		APIVersion: g.APIVersion,
	}
	docContent, err := group.GenerateDocFile()
	if err != nil {
		return errors.Wrap(err, "doc file cannot be generated")
	}
	folderPath := filepath.Join(APISFolderPath, g.ShortName, g.APIVersion)
	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot create new folder: %s", folderPath)
	}
	docFilePath := filepath.Join(APISFolderPath, g.ShortName, g.APIVersion, DocFileName)
	if err := os.RemoveAll(docFilePath); err != nil {
		return errors.Wrapf(err, "cannot delete doc file: %s", docFilePath)
	}
	if err := WriteFile(docFilePath, docContent, os.ModePerm, !*ctx.Value("debug").(*bool)); err != nil {
		return errors.Wrap(err, "cannot write doc file")
	}
	gvContent, err := group.GenerateGroupVersionFile()
	if err != nil {
		return errors.Wrap(err, "group version file cannot be generated")
	}
	gvFilePath := filepath.Join(APISFolderPath, g.ShortName, g.APIVersion, GroupVersionInfoFileName)
	if err := os.RemoveAll(gvFilePath); err != nil {
		return errors.Wrapf(err, "cannot delete groupversion_info file: %s", gvFilePath)
	}
	if err := WriteFile(gvFilePath, gvContent, os.ModePerm, !*ctx.Value("debug").(*bool)); err != nil {
		return errors.Wrap(err, "cannot write doc file")
	}
	return nil
}

type CrdsCmd struct {
	GoogleGroupName string
	APIVersion      string
	//Exclude         []string
	Include []string
}

func (c *CrdsCmd) Run(ctx context.Context) error {
	localPkgPath := filepath.Join(APISFolderPath, c.GoogleGroupName, c.APIVersion)
	absLocalPkgPath, err := filepath.Abs(localPkgPath)
	if err != nil {
		return errors.Wrapf(err, "cannot calculate absolute path of local package: %s", localPkgPath)
	}
	if err := os.MkdirAll(localPkgPath, os.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot create new folder: %s", localPkgPath)
	}
	cache := packages.NewCache()
	list := c.Include
	apiGroup := generator.Group{
		ShortName:  c.GoogleGroupName,
		LongName:   c.GoogleGroupName + ".gcp.crossplane.io",
		APIVersion: c.APIVersion,
	}
	resourcesGen := generator.NewResources(cache, GoogleDCLPackagePath, absLocalPkgPath, apiGroup)
	for _, resourceName := range list {
		content, err := resourcesGen.GenerateCRDFile(c.GoogleGroupName, resourceName)
		if err != nil {
			return errors.Wrapf(err, "cannot generate crd file for %s", resourceName)
		}
		crdFilePath := filepath.Join(APISFolderPath, apiGroup.ShortName, apiGroup.APIVersion, fmt.Sprintf(CRDTypesFileNameFmt, strings.ToLower(resourceName)))
		if err := os.RemoveAll(crdFilePath); err != nil {
			return errors.Wrapf(err, "cannot delete crd file: %s", crdFilePath)
		}
		if err := WriteFile(crdFilePath, content, os.ModePerm, !*ctx.Value("debug").(*bool)); err != nil {
			return errors.Wrapf(err, "cannot write crd file: %s", crdFilePath)
		}
	}
	return nil
}

type ControllersCmd struct {
	GoogleGroupName string
	APIVersion      string
	//Exclude         []string
	Include []string
}

func (c *ControllersCmd) Run(ctx context.Context) error {
	sourcePkgPath := filepath.Join(APISFolderPath, c.GoogleGroupName, c.APIVersion)
	absSourcePkgPath, err := filepath.Abs(sourcePkgPath)
	if err != nil {
		return errors.Wrapf(err, "cannot calculate absolute path of local package: %s", sourcePkgPath)
	}
	cache := packages.NewCache()
	list := c.Include
	conversionsGen := generator.NewConversions(cache)
	for _, resourceName := range list {
		sourceTypePath := fmt.Sprintf("%s.%sParameters", absSourcePkgPath, strings.Title(resourceName))
		targetTypePath := fmt.Sprintf("%s.%s", filepath.Join(GoogleDCLPackagePath, c.GoogleGroupName), strings.Title(resourceName))
		content, err := conversionsGen.GenerateConversionsFile(sourceTypePath, targetTypePath)
		if err != nil {
			return errors.Wrapf(err, "cannot generate conversions file for %s", resourceName)
		}
		conversionsFileFolder := filepath.Join(ControllersFolderPath, c.GoogleGroupName, strings.ToLower(resourceName))
		if err := os.MkdirAll(conversionsFileFolder, os.ModePerm); err != nil {
			return errors.Wrapf(err, "cannot create new folder: %s", conversionsFileFolder)
		}
		conversionsFilePath := filepath.Join(conversionsFileFolder, ConversionsFileName)
		if err := os.RemoveAll(conversionsFilePath); err != nil {
			return errors.Wrapf(err, "cannot delete conversions file: %s", conversionsFilePath)
		}
		if err := WriteFile(conversionsFilePath, content, os.ModePerm, !*ctx.Value("debug").(*bool)); err != nil {
			return errors.Wrapf(err, "cannot write conversions file: %s", conversionsFilePath)
		}
	}
	return nil
}

func WriteFile(name string, data []byte, perm os.FileMode, goimports bool) error {
	out := bytes.NewBuffer(data)
	if goimports {
		outb := bytes.NewBuffer([]byte{})
		shellCmd := exec.Command("goimports")
		shellCmd.Stdin = bytes.NewBuffer(data)
		shellCmd.Stdout = outb
		if err := shellCmd.Run(); err != nil {
			return errors.Wrap(err, "goimports failed")
		}
		out = outb
	}
	return errors.Wrap(os.WriteFile(name, out.Bytes(), perm), "cannot write file")
}
