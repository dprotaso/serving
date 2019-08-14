/*
Copyright 2018 The Knative Authors

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

package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/serving/pkg/apis/serving"
	"knative.dev/serving/pkg/apis/serving/v1alpha1"
	listers "knative.dev/serving/pkg/client/serving/listers/serving/v1alpha1"
)

// MakeRevision creates a revision object from configuration.
//
// TODO(dprotaso) remove once we switch to internal versions
func MakeRevision(config *v1alpha1.Configuration) *v1alpha1.Revision {
	// Start from the ObjectMeta/Spec inlined in the Configuration resources.
	rev := &v1alpha1.Revision{
		ObjectMeta: config.Spec.GetTemplate().ObjectMeta,
		Spec:       config.Spec.GetTemplate().Spec,
	}
	// Populate the Namespace and Name.
	rev.Namespace = config.Namespace

	if rev.Name == "" {
		rev.GenerateName = config.Name + "-"
	}

	UpdateRevisionLabels(rev, config)
	UpdateRevisionAnnotations(rev, config)

	// Populate OwnerReferences so that deletes cascade.
	rev.OwnerReferences = append(rev.OwnerReferences, *kmeta.NewControllerRef(config))

	return rev
}

// UpdateRevisionLabels sets the revisions labels given a Configuration.
func UpdateRevisionLabels(rev *v1alpha1.Revision, config *v1alpha1.Configuration) {
	if rev.Labels == nil {
		rev.Labels = make(map[string]string)
	}

	for _, key := range []string{
		serving.ConfigurationLabelKey,
		serving.ServiceLabelKey,
		serving.ConfigurationGenerationLabelKey,
	} {
		rev.Labels[key] = RevisionLabelValueForKey(key, config)
	}
}

// UpdateRevisionAnnotations sets the revisions annotations given a Configuration's updater annotation.
func UpdateRevisionAnnotations(rev *v1alpha1.Revision, config *v1alpha1.Configuration) {
	if rev.Annotations == nil {
		rev.Annotations = make(map[string]string)
	}

	// Populate the CreatorAnnotation from configuration.
	cans := config.GetAnnotations()
	if c, ok := cans[serving.UpdaterAnnotation]; ok {
		rev.Annotations[serving.CreatorAnnotation] = c
	}
}

// RevisionLabelValueForKey returns the label value for the given key.
func RevisionLabelValueForKey(key string, config *v1alpha1.Configuration) string {
	switch key {
	case serving.ConfigurationLabelKey:
		return config.Name
	case serving.ServiceLabelKey:
		return config.Labels[serving.ServiceLabelKey]
	case serving.ConfigurationGenerationLabelKey:
		return fmt.Sprintf("%d", config.Generation)
	}

	return ""
}

// CheckNameAvailability checks that if the named Revision specified by the Configuration
// is available (not found), exists (but matches), or exists with conflict (doesn't match).
func CheckNameAvailability(config *v1alpha1.Configuration, lister listers.RevisionLister) (*v1alpha1.Revision, error) {
	// If config.Spec.GetTemplate().Name is set, then we can directly look up
	// the revision by name.
	name := config.Spec.GetTemplate().Name
	if name == "" {
		return nil, nil
	}
	errConflict := errors.NewAlreadyExists(v1alpha1.Resource("revisions"), name)

	rev, err := lister.Revisions(config.Namespace).Get(name)
	if errors.IsNotFound(err) {
		// Does not exist, we must be good!
		// note: for the name to change the generation must change.
		return nil, err
	} else if err != nil {
		return nil, err
	} else if !metav1.IsControlledBy(rev, config) {
		// If the revision isn't controller by this configuration, then
		// do not use it.
		return nil, errConflict
	}

	// Check the generation on this revision.
	generationKey := serving.ConfigurationGenerationLabelKey
	expectedValue := RevisionLabelValueForKey(generationKey, config)
	if rev.Labels != nil && rev.Labels[generationKey] == expectedValue {
		return rev, nil
	}
	// We only require spec equality because the rest is immutable and the user may have
	// annotated or labeled the Revision (beyond what the Configuration might have).
	if !equality.Semantic.DeepEqual(config.Spec.GetTemplate().Spec, rev.Spec) {
		return nil, errConflict
	}
	return rev, nil
}
