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

package ingress

import (
	"flag"
	"knative.dev/pkg/test/v2"
	"testing"
)

type Conformance struct {
	test.BaseConfig
	IngressClass string
}

func (c *Conformance) AddFlags(fs *flag.FlagSet) {
	c.Context.AddFlags(fs)

	flag.StringVar(&c.IngressClass,
		"ingress.class",
		"",
		"Set this flag to the ingress class to test against.")
}

// RunConformance will run ingress conformance tests
//
// Depending on the options it may test alpha and beta features
func RunConformance(tt *testing.T, c *Conformance) {
	t := test.NewContext(t, c)

	t.Stable("basics", TestBasics)
	t.Stable("basics/http2", TestBasicsHTTP2)

	t.Stable("grpc", TestGRPC)
	t.Stable("grpc/split", TestGRPCSplit)

	t.Stable("headers/pre-split", TestPreSplitSetHeaders)
	t.Stable("headers/post-split", TestPostSplitSetHeaders)

	t.Stable("hosts/multiple", TestMultipleHosts)

	t.Stable("dispatch/path", TestPath)
	t.Stable("dispatch/percentage", TestPercentage)
	t.Stable("dispatch/path_and_percentage", TestPathAndPercentageSplit)

	t.Stable("retry", TestRetry)
	t.Stable("timeout", TestTimeout)

	t.Stable("tls", TestIngressTLS)
	t.Stable("update", TestUpdate)

	t.Stable("visibility", TestVisibility)
	t.Stable("visibility/split", TestVisibilitySplit)
	t.Stable("visibility/path", TestVisibilityPath)

	t.Stable("websocket", TestWebsocket)
	t.Stable("websocket/split", TestWebsocketSplit)

	// Add your conformance test for alpha features
	t.Alpha("headers/tags", TestTagHeaders)
}
