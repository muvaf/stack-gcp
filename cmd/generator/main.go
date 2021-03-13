package main

import (
	"bytes"
	"fmt"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/muvaf/typewriter/pkg/scanner"
	"github.com/pkg/errors"

	"github.com/muvaf/typewriter/pkg/packages"

	"github.com/muvaf/typewriter/pkg/wrapper"
	gpackages "golang.org/x/tools/go/packages"
)

const (
	LoadMode = gpackages.NeedName | gpackages.NeedFiles | gpackages.NeedImports | gpackages.NeedDeps | gpackages.NeedTypes | gpackages.NeedSyntax
)

const FileName = "apis/database/experimental/zz_database.go"

func main() {
	// temporary so that we can re-run and generate everything. otherwise, it skips
	// the types that already exist
	_ = os.RemoveAll(FileName)
	localDynamoDBPkg := "/Users/monus/go/src/github.com/crossplane/provider-gcp/apis/database/experimental"
	remoteDynamoDBPkg := "google.golang.org/api/sqladmin/v1beta4"
	pkgs, err := gpackages.Load(&gpackages.Config{Mode: LoadMode}, localDynamoDBPkg, remoteDynamoDBPkg)
	if err != nil {
		panic(err)
	}
	fmt.Println("package loading completed")
	var aPkg *types.Package
	var bPkg *types.Package
	for _, p := range pkgs {
		if p.Name == "experimental" {
			aPkg = p.Types
		}
		if p.Name == "sqladmin" {
			bPkg = p.Types
		}
	}
	if aPkg == nil {
		panic("local package could not be read")
	}
	if bPkg == nil {
		panic("remote package could not be read")
	}
	if err := CRD(aPkg, bPkg); err != nil {
		panic(err)
	}
}

func CRD(p1, p2 *types.Package) error {
	im := packages.NewMap("database")
	im.Imports["github.com/crossplane/crossplane-runtime/apis/common/v1"] = "xpv1"
	im.Imports["k8s.io/apimachinery/pkg/apis/meta/v1"] = "metav1"
	im.Imports["k8s.io/apimachinery/pkg/runtime/schema"] = "schema"
	fl := wrapper.NewFile("experimental", "cmd/generator/templates/crd.go.tmpl",
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"),
		wrapper.WithImports(im),
	)
	b := p2.Scope().Lookup("DatabaseInstance").Type().(*types.Named)

	rc := scanner.NewRemoteCalls(p2.Scope(),
		scanner.WithCreateInputs("DatabaseInstance"),
		scanner.WithReadOutputs("DatabaseInstance"),
	)
	inputNamed, inputMarkers := rc.AggregatedInput(types.NewTypeName(token.NoPos, types.NewPackage("apis/database/experimental", "experimental"), "DatabaseParameters", nil))
	tp := scanner.NewTypePrinter(b.Obj().Pkg().Path(), inputNamed, inputMarkers.Print(), fl.Imports, p1.Scope())
	tp.Parse()
	aggInput, err := tp.Print("DatabaseParameters")
	if err != nil {
		return errors.Wrap(err, "type printing failed")
	}
	outputNamed, outputMarkers := rc.AggregatedOutput(types.NewTypeName(token.NoPos, types.NewPackage("apis/database/experimental", "experimental"), "DatabaseObservation", nil))
	tp = scanner.NewTypePrinter(b.Obj().Pkg().Path(), outputNamed, outputMarkers.Print(), fl.Imports, p1.Scope())
	tp.Parse()
	aggOutput, err := tp.Print("DatabaseObservation")
	if err != nil {
		return errors.Wrap(err, "type printing failed")
	}

	in := map[string]interface{}{
		"CRD": map[string]string{
			"Kind": "Database",
		},
		"Types": map[string]string{
			"AggregatedInput":  aggInput,
			"AggregatedOutput": aggOutput,
		},
	}
	file, err := fl.Wrap(in)
	if err != nil {
		return err
	}
	fb := bytes.NewBuffer(file)
	cmd := exec.Command("goimports")
	cmd.Stdin = fb
	outb := &bytes.Buffer{}
	cmd.Stdout = outb
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "goimports failed")
	}
	if err := ioutil.WriteFile(FileName, outb.Bytes(), os.ModePerm); err != nil {
		return err
	}
	return nil
}
