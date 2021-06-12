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
	cli.cache = packages.NewCache()
	kongCtx.FatalIfErrorf(kongCtx.Run())
}

type GeneratorCLI struct {
	Debug       bool           `kong:"help:'Enable debug mode. This will disable all linters'"`
	Group       GroupCmd       `kong:"cmd,help:'Generate group files.'"`
	Crds        CrdsCmd        `kong:"cmd,help:'Generate CRD files.'"`
	Controllers ControllersCmd `kong:"cmd,help:'Generate controller files.'"`
	Full        FullCmd        `kong:"cmd,help:'Generate all necessary files.'"`

	cache *packages.Cache
}

const (
	GoogleDCLPackagePath = "github.com/GoogleCloudPlatform/declarative-resource-client-library/services/google"

	APISFolderPath        = "apis"
	ControllersFolderPath = "pkg/controller"

	DocFileName              = "zz_doc.go"
	GroupVersionInfoFileName = "zz_groupversion_info.go"
	CRDTypesFileNameFmt      = "zz_%s_types.go"
	ConversionsFileName      = "zz_conversions.go"
	ControllerFileName       = "zz_controller.go"
)

type FullCmd struct {
	Group    string   `kong:"required,help:'Name of the group, such as container.'"`
	Version  string   `kong:"required,help:'Version of the group, such as v1alpha1'"`
	KindList []string `kong:"required,help:'List of resource names to be generated, such as Cluster.'"`
}

func (f *FullCmd) Run() error {
	gvFolderPath := filepath.Join(APISFolderPath, f.Group, f.Version)
	if err := os.RemoveAll(gvFolderPath); err != nil {
		return errors.Wrapf(err, "cannot delete group folder: %s", gvFolderPath)
	}
	group := &GroupCmd{
		Name:    f.Group,
		Version: f.Version,
	}
	if err := group.Run(); err != nil {
		return errors.Wrap(err, "cannot generate group")
	}
	crds := &CrdsCmd{
		GoogleGroupName: f.Group,
		Version:         f.Version,
		Include:         f.KindList,
	}
	if err := crds.Run(); err != nil {
		return errors.Wrap(err, "cannot generate crds")
	}
	// temp hack to get the group package compiling so that controllers generators
	// can read it.
	genCmd := exec.Command("make", "generate")
	if err := genCmd.Run(); err != nil {
		return errors.Wrap(err, "cannot run make generate")
	}
	controllers := &ControllersCmd{
		GoogleGroupName: f.Group,
		Version:         f.Version,
		Include:         f.KindList,
	}
	return errors.Wrap(controllers.Run(), "cannot generate controllers")
}

type GroupCmd struct {
	Name    string `kong:"required,help:'Name of the group, such as container.'"`
	Version string `kong:"required,help:'Version of the group, such as v1alpha1'"`
}

func (g *GroupCmd) Run() error {
	group := &generator.Group{
		ShortName:  g.Name,
		LongName:   g.Name + ".gcp.crossplane.io",
		APIVersion: g.Version,
	}
	docContent, err := group.GenerateDocFile()
	if err != nil {
		return errors.Wrap(err, "doc file cannot be generated")
	}
	folderPath := filepath.Join(APISFolderPath, g.Name, g.Version)
	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot create new folder: %s", folderPath)
	}
	docFilePath := filepath.Join(APISFolderPath, g.Name, g.Version, DocFileName)
	if err := os.RemoveAll(docFilePath); err != nil {
		return errors.Wrapf(err, "cannot delete doc file: %s", docFilePath)
	}
	if err := WriteFile(docFilePath, docContent, os.ModePerm, !cli.Debug); err != nil {
		return errors.Wrap(err, "cannot write doc file")
	}
	gvContent, err := group.GenerateGroupVersionFile()
	if err != nil {
		return errors.Wrap(err, "group version file cannot be generated")
	}
	gvFilePath := filepath.Join(APISFolderPath, g.Name, g.Version, GroupVersionInfoFileName)
	if err := os.RemoveAll(gvFilePath); err != nil {
		return errors.Wrapf(err, "cannot delete groupversion_info file: %s", gvFilePath)
	}
	if err := WriteFile(gvFilePath, gvContent, os.ModePerm, !cli.Debug); err != nil {
		return errors.Wrap(err, "cannot write doc file")
	}
	return nil
}

type CrdsCmd struct {
	GoogleGroupName string
	Version         string
	//Exclude         []string
	Include []string
}

func (c *CrdsCmd) Run() error {
	localPkgPath := filepath.Join(APISFolderPath, c.GoogleGroupName, c.Version)
	absLocalPkgPath, err := filepath.Abs(localPkgPath)
	if err != nil {
		return errors.Wrapf(err, "cannot calculate absolute path of local package: %s", localPkgPath)
	}
	if err := os.MkdirAll(localPkgPath, os.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot create new folder: %s", localPkgPath)
	}
	list := c.Include
	apiGroup := generator.Group{
		ShortName:  c.GoogleGroupName,
		LongName:   c.GoogleGroupName + ".gcp.crossplane.io",
		APIVersion: c.Version,
	}
	resourcesGen := generator.NewResources(cli.cache, GoogleDCLPackagePath, absLocalPkgPath, apiGroup)
	for _, resourceName := range list {
		crdFilePath := filepath.Join(APISFolderPath, apiGroup.ShortName, apiGroup.APIVersion, fmt.Sprintf(CRDTypesFileNameFmt, strings.ToLower(resourceName)))
		if err := os.RemoveAll(crdFilePath); err != nil {
			return errors.Wrapf(err, "cannot delete crd file: %s", crdFilePath)
		}
		content, err := resourcesGen.GenerateCRDFile(c.GoogleGroupName, resourceName)
		if err != nil {
			return errors.Wrapf(err, "cannot generate crd file for %s", resourceName)
		}
		if err := WriteFile(crdFilePath, content, os.ModePerm, !cli.Debug); err != nil {
			return errors.Wrapf(err, "cannot write crd file: %s", crdFilePath)
		}
	}
	return nil
}

type ControllersCmd struct {
	GoogleGroupName string
	Version         string
	//Exclude         []string
	Include []string
}

func (c *ControllersCmd) Run() error {
	sourcePkgPath := filepath.Join(APISFolderPath, c.GoogleGroupName, c.Version)
	absSourcePkgPath, err := filepath.Abs(sourcePkgPath)
	if err != nil {
		return errors.Wrapf(err, "cannot calculate absolute path of local package: %s", sourcePkgPath)
	}
	list := c.Include
	conversionsGen := generator.NewConversions(cli.cache)
	controllerGen := generator.NewController(cli.cache)
	for _, resourceName := range list {
		paramsTypePath := fmt.Sprintf("%s.%sParameters", absSourcePkgPath, strings.Title(resourceName))
		observationTypePath := fmt.Sprintf("%s.%sObservation", absSourcePkgPath, strings.Title(resourceName))
		targetTypePath := fmt.Sprintf("%s.%s", filepath.Join(GoogleDCLPackagePath, c.GoogleGroupName), strings.Title(resourceName))
		conversionsContent, err := conversionsGen.GenerateConversionsFile(paramsTypePath, observationTypePath, targetTypePath)
		if err != nil {
			return errors.Wrapf(err, "cannot generate conversions file for %s", resourceName)
		}
		controllerContent, err := controllerGen.GenerateControllerFile(strings.ToLower(c.GoogleGroupName), strings.Title(resourceName), c.Version)
		if err != nil {
			return errors.Wrapf(err, "cannot generate controller file for %s", resourceName)
		}
		controllerFolderPath := filepath.Join(ControllersFolderPath, c.GoogleGroupName, strings.ToLower(resourceName))
		if err := os.MkdirAll(controllerFolderPath, os.ModePerm); err != nil {
			return errors.Wrapf(err, "cannot create new folder: %s", controllerFolderPath)
		}
		conversionsFilePath := filepath.Join(controllerFolderPath, ConversionsFileName)
		if err := os.RemoveAll(conversionsFilePath); err != nil {
			return errors.Wrapf(err, "cannot delete conversions file: %s", conversionsFilePath)
		}
		if err := WriteFile(conversionsFilePath, conversionsContent, os.ModePerm, !cli.Debug); err != nil {
			return errors.Wrapf(err, "cannot write conversions file: %s", conversionsFilePath)
		}
		controllerFilePath := filepath.Join(controllerFolderPath, ControllerFileName)
		if err := os.RemoveAll(controllerFilePath); err != nil {
			return errors.Wrapf(err, "cannot delete controller file: %s", conversionsFilePath)
		}
		if err := WriteFile(controllerFilePath, controllerContent, os.ModePerm, !cli.Debug); err != nil {
			return errors.Wrapf(err, "cannot write controller file: %s", controllerFilePath)
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
