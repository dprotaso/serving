/*
Copyright 2018 The Knative Authors.

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

package testing

import (
	"context"
	"testing"

	"github.com/knative/pkg/logging"
	"github.com/knative/serving/pkg/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	. "github.com/knative/pkg/logging/testing"
)

type (
	ReconcilerTests       []ReconcilerTest
	ReconcilerInitializer func(reconciler.CommonOptions, []runtime.Object) (reconciler.Reconciler, []FakeClient)
	ReconcilerTest        struct {
		Name    string
		Key     string
		Context context.Context

		// World State
		Failures Failures
		Objects  Objects
		//
		// Expectations
		ExpectedCreates Creates
		ExpectedPatches Patches
		ExpectedUpdates Updates
		ExpectError     bool
	}
)

func (tests ReconcilerTests) Run(t *testing.T, init ReconcilerInitializer) {
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			test.Run(t, init)
		})
	}
}

func (s *ReconcilerTest) Run(t *testing.T, init ReconcilerInitializer) {
	logger := TestLogger(t)

	opts := reconciler.CommonOptions{
		Logger:        TestLogger(t),
		Recorder:      &record.FakeRecorder{},
		ObjectTracker: &NullTracker{},
	}

	reconciler, fakeClients := init(opts, s.Objects)

	clients := setupClientValidations(fakeClients, s.Failures)

	ctx := context.TODO()

	if s.Context != nil {
		ctx = s.Context
	}

	ctx = logging.WithLogger(ctx, logger)

	err := reconciler.Reconcile(ctx, s.Key)

	if (err != nil) != s.ExpectError {
		t.Errorf("Reconcile() error = %v, expected error %v", err, s.ExpectError)
	}

	actions, err := clients.ActionsByVerb()

	if err != nil {
		t.Errorf("error capturing actions by verb: %q", err)
	}

	expectedNamespace, _, _ := cache.SplitMetaNamespaceKey(s.Key)

	assertCreates(t, s.ExpectedCreates, actions.Creates, expectedNamespace)
	assertUpdates(t, s.ExpectedUpdates, actions.Updates, expectedNamespace)
	assertPatches(t, s.ExpectedPatches, actions.Patches, expectedNamespace)
}
