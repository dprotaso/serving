package main

import "time"

const (
	// ProbePath is the name of a path that activator, autoscaler and
	// prober(used by KIngress generally) use for health check.
	ProbePath = "/healthz"

	// ProbeHeaderName is the name of a header that can be added to
	// requests to probe the knative networking layer.  Requests
	// with this header will not be passed to the user container or
	// included in request metrics.
	ProbeHeaderName = "K-Network-Probe"

	// ProxyHeaderName is the name of an internal header that activator
	// uses to mark requests going through it.
	ProxyHeaderName = "K-Proxy-Request"

	// HashHeaderName is the name of an internal header that Ingress controller
	// uses to find out which version of the networking config is deployed.
	HashHeaderName = "K-Network-Hash"

	// HashHeaderValue is the value that must appear in the HashHeaderName
	// header in order for our network hash to be injected.
	HashHeaderValue = "override"

	// OriginalHostHeader is used to avoid Istio host based routing rules
	// in Activator.
	// The header contains the original Host value that can be rewritten
	// at the Queue proxy level back to be a host header.
	OriginalHostHeader = "K-Original-Host"

	// ConfigName is the name of the configmap containing all
	// customizations for networking features.
	ConfigName = "config-network"

	// DeprecatedDefaultIngressClassKey  Please use DefaultIngressClassKey instead.
	DeprecatedDefaultIngressClassKey = "clusteringress.class"

	// DefaultIngressClassKey is the name of the configuration entry
	// that specifies the default Ingress.
	DefaultIngressClassKey = "ingress.class"

	// DefaultCertificateClassKey is the name of the configuration entry
	// that specifies the default Certificate.
	DefaultCertificateClassKey = "certificate.class"

	// IstioIngressClassName value for specifying knative's Istio
	// Ingress reconciler.
	IstioIngressClassName = "istio.ingress.networking.knative.dev"

	// CertManagerCertificateClassName value for specifying Knative's Cert-Manager
	// Certificate reconciler.
	CertManagerCertificateClassName = "cert-manager.certificate.networking.knative.dev"

	// DomainTemplateKey is the name of the configuration entry that
	// specifies the golang template string to use to construct the
	// Knative service's DNS name.
	DomainTemplateKey = "domainTemplate"

	// TagTemplateKey is the name of the configuration entry that
	// specifies the golang template string to use to construct the
	// hostname for a Route's tag.
	TagTemplateKey = "tagTemplate"

	// RolloutDurationKey is the name of the configuration entry
	// that specifies the default duration of the configuration rollout.
	RolloutDurationKey = "rolloutDuration"

	// KubeProbeUAPrefix is the user agent prefix of the probe.
	// Since K8s 1.8, prober requests have
	//   User-Agent = "kube-probe/{major-version}.{minor-version}".
	KubeProbeUAPrefix = "kube-probe/"

	// KubeletProbeHeaderName is the name of the header supplied by kubelet
	// probes.  Istio with mTLS rewrites probes, but their probes pass a
	// different user-agent.  So we augment the probes with this header.
	KubeletProbeHeaderName = "K-Kubelet-Probe"

	// DefaultDomainTemplate is the default golang template to use when
	// constructing the Knative Route's Domain(host)
	DefaultDomainTemplate = "{{.Name}}.{{.Namespace}}.{{.Domain}}"

	// DefaultTagTemplate is the default golang template to use when
	// constructing the Knative Route's tag names.
	DefaultTagTemplate = "{{.Tag}}-{{.Name}}"

	// AutocreateClusterDomainClaimsKey is the key for the
	// AutocreateClusterDomainClaims property.
	AutocreateClusterDomainClaimsKey = "autocreateClusterDomainClaims"

	// AutoTLSKey is the name of the configuration entry
	// that specifies enabling auto-TLS or not.
	AutoTLSKey = "autoTLS"

	// HTTPProtocolKey is the name of the configuration entry that
	// specifies the HTTP endpoint behavior of Knative ingress.
	HTTPProtocolKey = "httpProtocol"

	// UserAgentKey is the constant for header "User-Agent".
	UserAgentKey = "User-Agent"

	// ActivatorUserAgent is the user-agent header value set in probe requests sent
	// from activator.
	ActivatorUserAgent = "Knative-Activator-Probe"

	// QueueProxyUserAgent is the user-agent header value set in probe requests sent
	// from queue-proxy.
	QueueProxyUserAgent = "Knative-Queue-Proxy-Probe"

	// IngressReadinessUserAgent is the user-agent header value
	// set in probe requests for Ingress status.
	IngressReadinessUserAgent = "Knative-Ingress-Probe"

	// AutoscalingUserAgent is the user-agent header value set in probe
	// requests sent by autoscaling implementations.
	AutoscalingUserAgent = "Knative-Autoscaling-Probe"

	// TagHeaderName is the name of the header entry which has a tag name as value.
	// The tag name specifies which route was expected to be chosen by Ingress.
	TagHeaderName = "Knative-Serving-Tag"

	// DefaultRouteHeaderName is the name of the header entry
	// identifying whether a request is routed via the default route or not.
	// It has one of the string value "true" or "false".
	DefaultRouteHeaderName = "Knative-Serving-Default-Route"

	// TagHeaderBasedRoutingKey is the name of the configuration entry
	// that specifies enabling tag header based routing or not.
	TagHeaderBasedRoutingKey = "tagHeaderBasedRouting"

	// ProtoAcceptContent is the content type to be used when autoscaler scrapes metrics from the QP
	ProtoAcceptContent = "application/protobuf"

	// FlushInterval controls the time when we flush the connection in the
	// reverse proxies (Activator, QP).
	// NB: having it equal to 0 is a problem for streaming requests
	// since the data won't be transferred in chunks less than 4kb, if the
	// reverse proxy fails to detect streaming (gRPC, e.g.).
	FlushInterval = 20 * time.Millisecond

	// VisibilityLabelKey is the label to indicate visibility of Route
	// and KServices.  It can be an annotation too but since users are
	// already using labels for domain, it probably best to keep this
	// consistent.
	VisibilityLabelKey = "networking.knative.dev/visibility"
)
