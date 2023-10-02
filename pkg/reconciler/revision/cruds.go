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

package revision

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyapps "k8s.io/client-go/applyconfigurations/apps/v1"

	caching "knative.dev/caching/pkg/apis/caching/v1alpha1"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	autoscalingv1alpha1 "knative.dev/serving/pkg/apis/autoscaling/v1alpha1"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"knative.dev/serving/pkg/networking"
	"knative.dev/serving/pkg/reconciler/revision/config"
	"knative.dev/serving/pkg/reconciler/revision/resources"
)

func (c *Reconciler) createDeployment(ctx context.Context, rev *v1.Revision) (*appsv1.Deployment, error) {
	cfgs := config.FromContext(ctx)

	deployment, err := resources.MakeDeploymentSSA(rev, cfgs)

	if err != nil {
		return nil, fmt.Errorf("failed to make deployment: %w", err)
	}

	return c.kubeclient.AppsV1().Deployments(*deployment.Namespace).Apply(ctx, deployment, metav1.ApplyOptions{
		FieldManager: "controller",
	})
}

func (c *Reconciler) createSecret(ctx context.Context, ns *corev1.Namespace) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            networking.ServingCertName,
			Namespace:       ns.Name,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ns, corev1.SchemeGroupVersion.WithKind("Namespace"))},
			Labels: map[string]string{
				networking.ServingCertName + "-ctrl": "data-plane-user",
			},
		},
	}
	return c.kubeclient.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func (c *Reconciler) checkAndUpdateDeployment(ctx context.Context, rev *v1.Revision, have *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logging.FromContext(ctx)
	cfgs := config.FromContext(ctx)

	wantConfig, err := resources.MakeDeploymentSSA(rev, cfgs)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment: %w", err)
	}

	haveConfig, err := applyapps.ExtractDeployment(have, "controller")
	if err != nil {
		return nil, fmt.Errorf("failed to extract deployment", err)
	}

	// If the spec we want is the spec we have, then we're good.
	if equality.Semantic.DeepEqual(haveConfig, wantConfig) {
		logger.Info("deployments are the same")
		return have, nil
	} else {
		// If what comes back from the update (with defaults applied by the API server) is the same
		// as what we have then nothing changed.
		diff, err := kmp.SafeDiff(haveConfig, wantConfig)
		if err != nil {
			return nil, err
		}
		logger.Info("Reconciling deployment diff (-desired, +observed): ", diff)
	}

	return c.kubeclient.AppsV1().Deployments(*wantConfig.Namespace).Apply(ctx, wantConfig, metav1.ApplyOptions{
		FieldManager: "controller",
	})
}

func (c *Reconciler) createImageCache(ctx context.Context, rev *v1.Revision, containerName, imageDigest string) (*caching.Image, error) {
	image := resources.MakeImageCache(rev, containerName, imageDigest)
	return c.cachingclient.CachingV1alpha1().Images(image.Namespace).Create(ctx, image, metav1.CreateOptions{})
}

func (c *Reconciler) createPA(ctx context.Context, rev *v1.Revision) (*autoscalingv1alpha1.PodAutoscaler, error) {
	pa := resources.MakePA(rev)
	return c.client.AutoscalingV1alpha1().PodAutoscalers(pa.Namespace).Create(ctx, pa, metav1.CreateOptions{})
}
