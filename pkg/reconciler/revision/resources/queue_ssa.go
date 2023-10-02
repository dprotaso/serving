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
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	applycore "k8s.io/client-go/applyconfigurations/core/v1"
	pkgnet "knative.dev/networking/pkg/apis/networking"
	netheader "knative.dev/networking/pkg/http/header"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/system"
	apicfg "knative.dev/serving/pkg/apis/config"
	"knative.dev/serving/pkg/apis/serving"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"knative.dev/serving/pkg/deployment"
	"knative.dev/serving/pkg/networking"
	"knative.dev/serving/pkg/queue"
	"knative.dev/serving/pkg/queue/readiness"
	"knative.dev/serving/pkg/reconciler/revision/config"
)

func makeQueueProxy(rev *v1.Revision, cfg *config.Config) (*applycore.ContainerApplyConfiguration, error) {
	configName := rev.Labels[serving.ConfigurationLabelKey]
	if owner := metav1.GetControllerOf(rev); owner != nil && owner.Kind == "Configuration" {
		configName = owner.Name
	}
	servingPort := queueHTTPPort
	if rev.GetProtocol() == pkgnet.ProtocolH2C {
		servingPort = queueHTTP2Port
	}

	timeout := int64(0)
	if rev.Spec.TimeoutSeconds != nil {
		timeout = *rev.Spec.TimeoutSeconds
	}
	responseStartTimeout := int64(0)
	if rev.Spec.ResponseStartTimeoutSeconds != nil {
		responseStartTimeout = *rev.Spec.ResponseStartTimeoutSeconds
	}
	idleTimeout := int64(0)
	if rev.Spec.IdleTimeoutSeconds != nil {
		idleTimeout = *rev.Spec.IdleTimeoutSeconds
	}

	var loggingLevel string
	if ll, ok := cfg.Logging.LoggingLevel["queueproxy"]; ok {
		loggingLevel = ll.String()
	}

	var userProbeJSON string
	readinessProbe := applycore.Probe()

	container := rev.Spec.GetContainer()
	userPort := getUserPort(rev)
	if container.ReadinessProbe != nil {
		probePort := userPort
		if container.ReadinessProbe.HTTPGet != nil && container.ReadinessProbe.HTTPGet.Port.IntValue() != 0 {
			probePort = container.ReadinessProbe.HTTPGet.Port.IntVal
		}
		if container.ReadinessProbe.TCPSocket != nil && container.ReadinessProbe.TCPSocket.Port.IntValue() != 0 {
			probePort = container.ReadinessProbe.TCPSocket.Port.IntVal
		}
		if container.ReadinessProbe.GRPC != nil && container.ReadinessProbe.GRPC.Port > 0 {
			probePort = container.ReadinessProbe.GRPC.Port
		}

		// The activator attempts to detect readiness itself by checking the Queue
		// Proxy's health endpoint rather than waiting for Kubernetes to check and
		// propagate the Ready state. We encode the original probe as JSON in an
		// environment variable for this health endpoint to use.
		userProbe := container.ReadinessProbe.DeepCopy()
		applyReadinessProbeDefaultsForExec(userProbe, probePort)

		var err error
		userProbeJSON, err = readiness.EncodeProbe(userProbe)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize readiness probe: %w", err)
		}
		if container.ReadinessProbe.InitialDelaySeconds != 0 {
			readinessProbe.WithInitialDelaySeconds(container.ReadinessProbe.InitialDelaySeconds)
		}
		if container.ReadinessProbe.TimeoutSeconds != 0 {
			readinessProbe.WithTimeoutSeconds(container.ReadinessProbe.TimeoutSeconds)
		}
		if container.ReadinessProbe.PeriodSeconds != 0 {
			readinessProbe.WithPeriodSeconds(container.ReadinessProbe.PeriodSeconds)
		}
		if container.ReadinessProbe.SuccessThreshold != 0 {
			readinessProbe.WithSuccessThreshold(container.ReadinessProbe.SuccessThreshold)
		}
		if container.ReadinessProbe.FailureThreshold != 0 {
			readinessProbe.WithFailureThreshold(container.ReadinessProbe.FailureThreshold)
		}

		readinessProbe.WithHTTPGet(applycore.HTTPGetAction().
			WithPort(intstr.FromInt(int(servingPort.ContainerPort))).
			WithHTTPHeaders(applycore.HTTPHeader().WithName(netheader.ProbeKey).WithValue(queue.Name)),
		)
	}

	c := applycore.Container().
		WithName(QueueContainerName).
		WithImage(cfg.Deployment.QueueSidecarImage).
		WithReadinessProbe(readinessProbe).
		WithResources(createQueueResourcesSSA(rev, cfg)).
		WithSecurityContext(applycore.SecurityContext().
			WithAllowPrivilegeEscalation(false).
			WithReadOnlyRootFilesystem(true).
			WithRunAsNonRoot(true).
			WithCapabilities(applycore.Capabilities().
				WithDrop("ALL"),
			).
			WithSeccompProfile(applycore.SeccompProfile().
				WithType(corev1.SeccompProfileTypeRuntimeDefault),
			),
		).
		WithPorts(
			applycore.ContainerPort().WithName(v1.QueueAdminPortName).WithContainerPort(networking.QueueAdminPort),
			applycore.ContainerPort().WithName(v1.AutoscalingQueueMetricsPortName).WithContainerPort(networking.AutoscalingQueueMetricsPort),
			applycore.ContainerPort().WithName(v1.UserQueueMetricsPortName).WithContainerPort(networking.UserQueueMetricsPort),
		).
		WithEnv(
			applycore.EnvVar().WithName("SERVING_NAMESPACE").WithValue(rev.Namespace),
			applycore.EnvVar().WithName("SERVING_REVISION").WithValue(rev.Name),
			applycore.EnvVar().WithName("SERVING_CONFIGURATION").WithValue(configName),
			applycore.EnvVar().WithName("SERVING_SERVICE").WithValue(rev.Labels[serving.ServiceLabelKey]),
			applycore.EnvVar().WithName("QUEUE_SERVING_PORT").WithValue(strconv.Itoa(int(servingPort.ContainerPort))),
			applycore.EnvVar().WithName("QUEUE_SERVING_TLS_PORT").WithValue(strconv.Itoa(int(queueHTTP2Port.ContainerPort))),
			applycore.EnvVar().WithName("CONTAINER_CONCURRENCY").WithValue(strconv.Itoa(int(rev.Spec.GetContainerConcurrency()))),
			applycore.EnvVar().WithName("REVISION_TIMEOUT_SECONDS").WithValue(strconv.Itoa(int(timeout))),
			applycore.EnvVar().WithName("REVISION_RESPONSE_START_TIMEOUT_SECONDS").WithValue(strconv.Itoa(int(responseStartTimeout))),
			applycore.EnvVar().WithName("REVISION_IDLE_TIMEOUT_SECONDS").WithValue(strconv.Itoa(int(idleTimeout))),
			applycore.EnvVar().WithName("SERVING_POD").WithValueFrom(
				applycore.EnvVarSource().WithFieldRef(applycore.ObjectFieldSelector().WithFieldPath("metadata.name")),
			),
			applycore.EnvVar().WithName("SERVING_POD_IP").WithValueFrom(
				applycore.EnvVarSource().WithFieldRef(applycore.ObjectFieldSelector().WithFieldPath("status.podIP")),
			),
			applycore.EnvVar().WithName("HOST_IP").WithValueFrom(
				applycore.EnvVarSource().WithFieldRef(applycore.ObjectFieldSelector().WithFieldPath("status.hostIP")),
			),
			applycore.EnvVar().WithName("SERVING_LOGGING_CONFIG").WithValue(cfg.Logging.LoggingConfig),
			applycore.EnvVar().WithName("SERVING_LOGGING_LEVEL").WithValue(loggingLevel),
			applycore.EnvVar().WithName("SERVING_REQUEST_LOG_TEMPLATE").WithValue(cfg.Observability.RequestLogTemplate),
			applycore.EnvVar().WithName("SERVING_ENABLE_REQUEST_LOG").WithValue(strconv.FormatBool(cfg.Observability.EnableRequestLog)),
			applycore.EnvVar().WithName("SERVING_ENABLE_PROBE_REQUEST_LOG").WithValue(strconv.FormatBool(cfg.Observability.EnableProbeRequestLog)),
			applycore.EnvVar().WithName("SERVING_REQUEST_METRICS_BACKEND").WithValue(cfg.Observability.RequestMetricsBackend),
			applycore.EnvVar().WithName("SERVING_REQUEST_METRICS_REPORTING_PERIOD_SECONDS").WithValue(
				strconv.Itoa(cfg.Observability.RequestMetricsReportingPeriodSeconds),
			),
			applycore.EnvVar().WithName("TRACING_CONFIG_BACKEND").WithValue(string(cfg.Tracing.Backend)),
			applycore.EnvVar().WithName("TRACING_CONFIG_ZIPKIN_ENDPOINT").WithValue(string(cfg.Tracing.ZipkinEndpoint)),
			applycore.EnvVar().WithName("TRACING_CONFIG_DEBUG").WithValue(strconv.FormatBool(cfg.Tracing.Debug)),
			applycore.EnvVar().WithName("TRACING_CONFIG_SAMPLE_RATE").WithValue(fmt.Sprint(cfg.Tracing.SampleRate)),

			applycore.EnvVar().WithName("USER_PORT").WithValue(strconv.Itoa(int(userPort))),
			applycore.EnvVar().WithName(system.NamespaceEnvKey).WithValue(system.Namespace()),
			applycore.EnvVar().WithName(metrics.DomainEnv).WithValue(metrics.Domain()),

			applycore.EnvVar().WithName("ENABLE_PROFILING").WithValue(strconv.FormatBool(cfg.Observability.EnableProfiling)),
			applycore.EnvVar().WithName("METRICS_COLLECTOR_ADDRESS").WithValue(cfg.Observability.MetricsCollectorAddress),

			applycore.EnvVar().WithName("SERVING_READINESS_PROBE").WithValue(userProbeJSON),
			applycore.EnvVar().WithName("ENABLE_HTTP2_AUTO_DETECTION").WithValue(strconv.FormatBool(cfg.Features.AutoDetectHTTP2 == apicfg.Enabled)),
			applycore.EnvVar().WithName("ROOT_CA").WithValue(cfg.Deployment.QueueSidecarRootCA),
		)

	if cfg.Observability.EnableProfiling {
		c.WithPorts(
			applycore.ContainerPort().WithName(profilingPortName).WithContainerPort(profiling.ProfilingPort),
		)
	}

	return c, nil
}

func createQueueResourcesSSA(rev *v1.Revision, cfg *config.Config) *applycore.ResourceRequirementsApplyConfiguration {
	annotations := rev.Annotations
	userContainer := rev.Spec.GetContainer()
	useDefaults := cfg.Features.QueueProxyResourceDefaults == apicfg.Enabled

	resourceRequests := corev1.ResourceList{}
	resourceLimits := corev1.ResourceList{}

	for _, r := range []struct {
		Name           corev1.ResourceName
		Request        *resource.Quantity
		RequestDefault *resource.Quantity
		Limit          *resource.Quantity
		LimitDefault   *resource.Quantity
	}{{
		Name:           corev1.ResourceCPU,
		Request:        cfg.Deployment.QueueSidecarCPURequest,
		RequestDefault: &deployment.QueueSidecarCPURequestDefault,
		Limit:          cfg.Deployment.QueueSidecarCPULimit,
		LimitDefault:   &deployment.QueueSidecarCPULimitDefault,
	}, {
		Name:           corev1.ResourceMemory,
		Request:        cfg.Deployment.QueueSidecarMemoryRequest,
		RequestDefault: &deployment.QueueSidecarMemoryRequestDefault,
		Limit:          cfg.Deployment.QueueSidecarMemoryLimit,
		LimitDefault:   &deployment.QueueSidecarMemoryLimitDefault,
	}, {
		Name:    corev1.ResourceEphemeralStorage,
		Request: cfg.Deployment.QueueSidecarEphemeralStorageRequest,
		Limit:   cfg.Deployment.QueueSidecarEphemeralStorageLimit,
	}} {
		if r.Request != nil {
			resourceRequests[r.Name] = *r.Request
		} else if useDefaults && r.RequestDefault != nil {
			resourceRequests[r.Name] = *r.RequestDefault
		}
		if r.Limit != nil {
			resourceLimits[r.Name] = *r.Limit
		} else if useDefaults && r.LimitDefault != nil {
			resourceLimits[r.Name] = *r.LimitDefault
		}
	}

	var requestCPU, limitCPU, requestMemory, limitMemory resource.Quantity

	if resourceFraction, ok := fractionFromPercentage(annotations, serving.QueueSidecarResourcePercentageAnnotation); ok {
		if ok, requestCPU = computeResourceRequirements(userContainer.Resources.Requests.Cpu(), resourceFraction, queueContainerRequestCPU); ok {
			resourceRequests[corev1.ResourceCPU] = requestCPU
		}

		if ok, limitCPU = computeResourceRequirements(userContainer.Resources.Limits.Cpu(), resourceFraction, queueContainerLimitCPU); ok {
			resourceLimits[corev1.ResourceCPU] = limitCPU
		}

		if ok, requestMemory = computeResourceRequirements(userContainer.Resources.Requests.Memory(), resourceFraction, queueContainerRequestMemory); ok {
			resourceRequests[corev1.ResourceMemory] = requestMemory
		}

		if ok, limitMemory = computeResourceRequirements(userContainer.Resources.Limits.Memory(), resourceFraction, queueContainerLimitMemory); ok {
			resourceLimits[corev1.ResourceMemory] = limitMemory
		}
	}

	if requestCPU, ok := resourceFromAnnotation(annotations, serving.QueueSidecarCPUResourceRequestAnnotation); ok {
		resourceRequests[corev1.ResourceCPU] = requestCPU
	}

	if limitCPU, ok := resourceFromAnnotation(annotations, serving.QueueSidecarCPUResourceLimitAnnotation); ok {
		resourceLimits[corev1.ResourceCPU] = limitCPU
	}

	if requestMemory, ok := resourceFromAnnotation(annotations, serving.QueueSidecarMemoryResourceRequestAnnotation); ok {
		resourceRequests[corev1.ResourceMemory] = requestMemory
	}

	if limitMemory, ok := resourceFromAnnotation(annotations, serving.QueueSidecarMemoryResourceLimitAnnotation); ok {
		resourceLimits[corev1.ResourceMemory] = limitMemory
	}

	if requestEphmeralStorage, ok := resourceFromAnnotation(annotations, serving.QueueSidecarEphemeralStorageResourceRequestAnnotation); ok {
		resourceRequests[corev1.ResourceEphemeralStorage] = requestEphmeralStorage
	}

	if limitEphemeralStorage, ok := resourceFromAnnotation(annotations, serving.QueueSidecarEphemeralStorageResourceLimitAnnotation); ok {
		resourceLimits[corev1.ResourceEphemeralStorage] = limitEphemeralStorage
	}

	resources := applycore.ResourceRequirements().
		WithRequests(resourceRequests)

	if len(resourceLimits) != 0 {
		resources.WithLimits(resourceLimits)
	}

	return resources
}
