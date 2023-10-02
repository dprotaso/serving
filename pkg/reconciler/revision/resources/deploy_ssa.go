/*
Copyright 2023 The Knative Authors

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

package resources

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	applyapps "k8s.io/client-go/applyconfigurations/apps/v1"
	applycore "k8s.io/client-go/applyconfigurations/core/v1"
	applymeta "k8s.io/client-go/applyconfigurations/meta/v1"

	netheader "knative.dev/networking/pkg/http/header"
	apiconfig "knative.dev/serving/pkg/apis/config"
	"knative.dev/serving/pkg/apis/serving"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"knative.dev/serving/pkg/networking"
	"knative.dev/serving/pkg/queue"
	"knative.dev/serving/pkg/reconciler/revision/config"
	"knative.dev/serving/pkg/reconciler/revision/resources/names"
)

func MakeDeploymentSSA(rev *v1.Revision, cfg *config.Config) (*applyapps.DeploymentApplyConfiguration, error) {
	podSpec, err := makePodSpecSSA(rev, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create PodSpec: %w", err)
	}

	labels := makeLabels(rev)
	annotations := makeAnnotations(rev)

	spec := applyapps.DeploymentSpec().
		WithTemplate(applycore.PodTemplateSpec().
			WithLabels(labels).
			WithAnnotations(annotations).
			WithSpec(podSpec),
		).
		WithSelector(applymeta.LabelSelector().
			WithMatchLabels(map[string]string{
				serving.RevisionUID: string(rev.GetUID()),
			}),
		).
		WithStrategy(applyapps.DeploymentStrategy().
			WithType(appsv1.RollingUpdateDeploymentStrategyType).
			WithRollingUpdate(applyapps.RollingUpdateDeployment().
				WithMaxUnavailable(intstr.FromInt(0)),
			),
		)

	progressDeadline := int32(cfg.Deployment.ProgressDeadline.Seconds())
	_, pdAnn, pdFound := serving.ProgressDeadlineAnnotation.Get(rev.Annotations)
	if pdFound {
		// Ignore errors and no error checking because already validated in webhook.
		pd, _ := time.ParseDuration(pdAnn)
		progressDeadline = int32(pd.Seconds())
	}

	spec.WithProgressDeadlineSeconds(progressDeadline)

	// replicaCount := cfg.Autoscaler.InitialScale
	// _, ann, found := autoscaling.InitialScaleAnnotation.Get(rev.Annotations)
	// if found {
	// 	// Ignore errors and no error checking because already validated in webhook.
	// 	rc, _ := strconv.ParseInt(ann, 10, 32)
	// 	replicaCount = int32(rc)
	// }

	// spec.WithReplicas(replicaCount)

	d := applyapps.Deployment(
		names.Deployment(rev),
		rev.Namespace,
	).
		WithLabels(labels).
		WithAnnotations(annotations).
		WithOwnerReferences(
			applymeta.OwnerReference().
				WithAPIVersion(rev.GetGroupVersionKind().GroupVersion().String()).
				WithBlockOwnerDeletion(true).
				WithController(true).
				WithKind(rev.GetGroupVersionKind().Kind).
				WithUID(rev.GetUID()).
				WithName(rev.GetName()),
		).
		WithSpec(spec)

	return d, nil
}

func makePodSpecSSA(rev *v1.Revision, cfg *config.Config) (*applycore.PodSpecApplyConfiguration, error) {
	queueContainer, err := makeQueueProxy(rev, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue-proxy container: %w", err)
	}

	template, err := revPodSpec(rev)
	if err != nil {
		return nil, fmt.Errorf("failed to convert rev pod spec: %w", err)
	}

	updateUserContainers(rev, template)
	template.WithContainers(queueContainer)

	podInfoFeature, podInfoExists := rev.Annotations[apiconfig.QueueProxyPodInfoFeatureKey]

	if cfg.Features.QueueProxyMountPodInfo == apiconfig.Enabled ||
		(cfg.Features.QueueProxyMountPodInfo == apiconfig.Allowed &&
			podInfoExists &&
			strings.EqualFold(podInfoFeature, string(apiconfig.Enabled))) {

		queueContainer.WithVolumeMounts(podInfoVolumeMountSSA)
		template.WithVolumes(podInfoVolumeSSA)
	}

	if len(cfg.Deployment.QueueSidecarTokenAudiences) > 0 {
		audiences := cfg.Deployment.QueueSidecarTokenAudiences.List()
		sort.Strings(audiences)

		projection := applycore.ProjectedVolumeSource()
		tokenVolume := applycore.Volume().
			WithName("knative-token-volume").
			WithProjected(projection)

		for _, aud := range audiences {
			if aud == "" {
				continue
			}

			projection.WithSources(applycore.VolumeProjection().
				WithServiceAccountToken(applycore.ServiceAccountTokenProjection().
					WithPath(aud).
					WithAudience(aud).
					WithExpirationSeconds(3600),
				))
		}
		if len(projection.Sources) > 0 {
			template.WithVolumes(tokenVolume)
			queueContainer.WithVolumeMounts(applycore.VolumeMount().
				WithName(*tokenVolume.Name).
				WithMountPath(queue.TokenDirectory),
			)
		}
	}

	if cfg.Network.InternalTLSEnabled() {
		queueContainer.WithVolumeMounts(applycore.VolumeMount().
			WithMountPath(queue.CertDirectory).
			WithName(certVolumeName).
			WithReadOnly(true),
		)

		template.WithVolumes(applycore.Volume().
			WithName(certVolumeName).
			WithSecret(applycore.SecretVolumeSource().
				WithSecretName(networking.ServingCertName),
			),
		)
	}

	if cfg.Observability.EnableVarLogCollection {
		template.WithVolumes(applycore.Volume().
			WithName("knative-var-log").
			WithEmptyDir(applycore.EmptyDirVolumeSource()),
		)

		for _, container := range template.Containers {
			if *container.Name == QueueContainerName {
				continue
			}

			container.
				WithVolumeMounts(applycore.VolumeMount().
					WithName("knative-var-log").
					WithMountPath("/var/log").
					WithSubPathExpr("$(K_INTERNAL_POD_NAMESPACE)_$(K_INTERNAL_POD_NAME)_"+*container.Name),
				).
				WithEnv(
					applycore.EnvVar().WithName("K_INTERNAL_POD_NAME").WithValueFrom(
						applycore.EnvVarSource().WithFieldRef(applycore.ObjectFieldSelector().WithFieldPath("metdata.name")),
					),
					applycore.EnvVar().WithName("K_INTERNAL_POD_NAMESPACE").WithValueFrom(
						applycore.EnvVarSource().WithFieldRef(applycore.ObjectFieldSelector().WithFieldPath("metdata.namespace")),
					),
				)
		}
	}

	return template, nil
}

func updateUserContainers(rev *v1.Revision, spec *applycore.PodSpecApplyConfiguration) {
	for i := range spec.Containers {
		if len(rev.Status.ContainerStatuses) != 0 {
			if rev.Status.ContainerStatuses[i].ImageDigest != "" {
				spec.Containers[i].WithImage(rev.Status.ContainerStatuses[i].ImageDigest)
			}
		}

		if len(rev.Spec.PodSpec.Containers[i].Ports) != 0 || len(rev.Spec.PodSpec.Containers) == 1 {
			updateServingContainer(rev, &spec.Containers[i])
		} else {
			updateSidecarContainer(rev, &spec.Containers[i])
		}
	}

}

func updateServingContainer(rev *v1.Revision, spec *applycore.ContainerApplyConfiguration) {
	updateSidecarContainer(rev, spec)

	userPort := getUserPort(rev)
	userPortStr := strconv.Itoa(int(userPort))

	// clear out existing ports
	spec.Ports = nil
	spec.
		WithPorts(applycore.ContainerPort().
			WithName(v1.UserPortName).
			WithContainerPort(userPort),
		).
		WithEnv(
			applycore.EnvVar().WithName("PORT").WithValue(userPortStr),
		)

	if spec.ReadinessProbe != nil {
		if spec.ReadinessProbe.HTTPGet != nil || spec.ReadinessProbe.TCPSocket != nil || spec.ReadinessProbe.GRPC != nil {
			// HTTP, TCP and gRPC ReadinessProbes are executed by the queue-proxy directly against the
			// user-container instead of via kubelet.
			spec.ReadinessProbe = nil
		}
	}

	if p := spec.LivenessProbe; p != nil {
		switch {
		case p.HTTPGet != nil:
			p.HTTPGet.WithPort(intstr.FromInt(int(userPort)))

			// With mTLS enabled, Istio rewrites probes, but doesn't spoof the kubelet
			// user agent, so we need to inject an extra header to be able to distinguish
			// between probes and real requests.
			p.HTTPGet.WithHTTPHeaders(
				applycore.HTTPHeader().WithName(netheader.KubeletProbeKey).WithValue(queue.Name),
			)
		case p.TCPSocket != nil:
			p.TCPSocket.WithPort(intstr.FromInt(int(userPort)))
		case p.GRPC != nil:
			p.GRPC.WithPort(userPort)
		}
	}
}

func updateSidecarContainer(rev *v1.Revision, spec *applycore.ContainerApplyConfiguration) {
	spec.
		WithLifecycle(applycore.Lifecycle().
			WithPreStop(applycore.LifecycleHandler().
				WithHTTPGet(applycore.HTTPGetAction().
					WithPort(intstr.FromInt(networking.QueueAdminPort)).
					WithPath(queue.RequestQueueDrainPath),
				),
			),
		).
		WithEnv(
			applycore.EnvVar().
				WithName(knativeServiceEnvVariableKey).
				WithValue(rev.Labels[serving.ServiceLabelKey]),
			applycore.EnvVar().
				WithName(knativeConfigurationEnvVariableKey).
				WithValue(rev.Labels[serving.ConfigurationLabelKey]),
			applycore.EnvVar().
				WithName(knativeRevisionEnvVariableKey).
				WithValue(rev.Name),
		).
		WithStdin(false).
		WithTTY(false)

	if spec.TerminationMessagePolicy == nil {
		spec.WithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError)
	}
}

var (
	podInfoVolumeSSA = applycore.Volume().
				WithName("pod-info").
				WithDownwardAPI(applycore.DownwardAPIVolumeSource().
					WithItems(applycore.DownwardAPIVolumeFile().
						WithPath(queue.PodInfoAnnotationsFilename).
						WithFieldRef(applycore.ObjectFieldSelector().WithFieldPath("metadata.annotations")),
			),
		)

	podInfoVolumeMountSSA = applycore.VolumeMount().
				WithName(*podInfoVolumeSSA.Name).
				WithMountPath(queue.PodInfoDirectory).
				WithReadOnly(true)
)

func revPodSpec(rev *v1.Revision) (*applycore.PodSpecApplyConfiguration, error) {
	bytes, err := json.Marshal(rev.Spec.PodSpec)
	if err != nil {
		return nil, err
	}

	spec := applycore.PodSpec()
	err = json.Unmarshal(bytes, spec)
	if err != nil {
		return nil, err
	}
	return spec, nil
}
