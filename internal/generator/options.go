package generator

import (
	"go/types"
	"strings"
)

// OmitemptyAdder adds omitempty tag to all fields. It assumes only json tag is
// defined.
type OmitemptyAdder struct{}

func (OmitemptyAdder) Filter(field *types.Var, tag string) (*types.Var, string) {
	if !strings.Contains(tag, "json") {
		return field, tag
	}
	return field, tag[:strings.LastIndex(tag, `"`)] + `,omitempty"`
}
