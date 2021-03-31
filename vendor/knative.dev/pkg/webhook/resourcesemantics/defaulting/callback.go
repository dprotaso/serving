package defaulting

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/pkg/webhook"
)

// Callback is a generic function to be called by a consumer of validation
type Callback struct {
	// function is the callback to be invoked
	function func(ctx context.Context, unstructured *unstructured.Unstructured) error

	// supportedVerbs are the verbs supported for the callback.
	// The function will only be called on these actions.
	supportedVerbs map[webhook.Operation]struct{}
}

// NewCallback creates a new callback function to be invoked on supported verbs.
func NewCallback(function func(context.Context, *unstructured.Unstructured) error, supportedVerbs ...webhook.Operation) Callback {
	m := make(map[webhook.Operation]struct{})
	for _, op := range supportedVerbs {
		if _, has := m[op]; has {
			panic("duplicate verbs not allowed")
		}
		m[op] = struct{}{}
	}
	return Callback{function: function, supportedVerbs: m}
}
