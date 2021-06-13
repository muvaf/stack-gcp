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

func (c *Conversions) GenerateConversionsFile(localPkgPath, localPkgName, specTypePath, statusTypePath, gcpTypePath string) ([]byte, error) {
	specType, err := c.cache.GetTypeWithFullPath(specTypePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get spec type: %s", specTypePath)
	}
	statusType, err := c.cache.GetTypeWithFullPath(statusTypePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get status type: %s", specTypePath)
	}
	gcpType, err := c.cache.GetTypeWithFullPath(gcpTypePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get target gcp type: %s", gcpTypePath)
	}
	file := wrapper.NewFile(localPkgPath, localPkgName, templates.ConversionsTemplate,
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"))

	lateInitGen := NewLateInitializeFn(c.cache, file.Imports)
	lateInitFn, err := lateInitGen.Generate(specType, gcpType)
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate late initialize function")
	}

	isUpToDateGen := NewIsUpToDateFn(c.cache, file.Imports)
	isUpToDateFn, err := isUpToDateGen.Generate(specType, gcpType)
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate isUpToDate function")
	}

	produceTargetGen := NewProducerFn(c.cache, file.Imports)
	produceGCPStructFn, err := produceTargetGen.Generate(specType, gcpType)
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate producer for spec to target GCP struct")
	}
	produceObservationFn, err := produceTargetGen.Generate(gcpType, statusType)
	if err != nil {
		return nil, errors.Wrap(err, "cannot generate producer for target GCP struct to status")
	}

	input := map[string]interface{}{
		"LateInitializeFn":     lateInitFn,
		"ProduceGCPStructFn":   produceGCPStructFn,
		"ProduceObservationFn": produceObservationFn,
		"IsUpToDateFn":         isUpToDateFn,
	}
	b, err := file.Wrap(input)
	if err != nil {
		return nil, errors.Wrap(err, "cannot wrap conversions file")
	}
	return b, nil
}

func NewController(c *packages.Cache) *Controller {
	return &Controller{
		cache: c,
	}
}

type Controller struct {
	cache *packages.Cache
}

func (c *Controller) GenerateControllerFile(localPkgPath, localPkgName, group, kind, version string) ([]byte, error) {
	file := wrapper.NewFile(localPkgPath, localPkgName, templates.ControllerTemplate,
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"))

	input := map[string]interface{}{
		"CRD": map[string]string{
			"Group":      group,
			"Version":    version,
			"Kind":       kind,
			"KindLower":  strings.ToLower(kind),
			"GroupLower": strings.ToLower(group),
		},
	}
	b, err := file.Wrap(input)
	if err != nil {
		return nil, errors.Wrap(err, "cannot wrap controller file")
	}
	return b, nil
}
