/*
Copyright 2020 The Knative Authors

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
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	// resource validation types

	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	extravalidation "knative.dev/serving/pkg/webhook"

	// config validation constructors

	defaultconfig "knative.dev/serving/pkg/apis/config"
)

var serviceValidation = defaulting.NewCallback(
	extravalidation.ValidateService, webhook.Create, webhook.Update)

var defaultingCallbacks = map[schema.GroupVersionKind]defaulting.Callback{
	servingv1.SchemeGroupVersion.WithKind("Service"): defaulting.NewCallback(
		func(ctx context.Context, u *unstructured.Unstructured) error {
			val, found, err := unstructured.NestedFieldNoCopy(u.Object, "spec", "template", "spec", "containers")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("containers not found")
			}

			containers, ok := val.([]interface{})
			if !ok {
				return errors.New("containers not an array")
			}

			for _, c := range containers {
				container := c.(map[string]interface{})
				if _, ok := container["imagePullPolicy"]; !ok {
					container["imagePullPolicy"] = "Always"
				}
			}

			unstructured.SetNestedSlice(u.Object, containers, "spec", "template", "spec", "containers")
			return nil
		},
		webhook.Create, webhook.Update,
	),
}

var validationCallbacks = map[schema.GroupVersionKind]validation.Callback{
	servingv1.SchemeGroupVersion.WithKind("Service"): validation.NewCallback(
		func(ctx context.Context, u *unstructured.Unstructured) error {
			val, found, err := unstructured.NestedFieldNoCopy(u.Object, "spec", "template", "spec", "containers")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("containers not found")
			}

			containers, ok := val.([]interface{})
			if !ok {
				return errors.New("containers not an array")
			}

			for _, c := range containers {
				container := c.(map[string]interface{})
				if container["imagePullPolicy"].(string) != "Always" {
					return errors.New("imagePullPolicy must be 'Always'")
				}
			}
			return nil
		},
		webhook.Create, webhook.Update,
	),
}

func newDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"always.pull.webhook.serving.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		nil,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		store.ToContext,

		// Whether to disallow unknown fields.
		true,

		defaultingCallbacks,
	)
}

func newValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"always.pull.validation.webhook.serving.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate.
		nil,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		store.ToContext,

		// Whether to disallow unknown fields.
		true,

		// Extra validating callbacks to be applied to resources.
		validationCallbacks,
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: webhook.NameFromEnv(),
		Port:        webhook.PortFromEnv(8443),
		SecretName:  "always-pull-webhook-certs",
	})

	sharedmain.WebhookMainWithContext(ctx, "always-pull-webhook",
		certificates.NewController,
		newDefaultingAdmissionController,
		newValidationAdmissionController,
	)
}
