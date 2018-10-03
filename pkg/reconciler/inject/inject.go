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
package inject

import (
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/record"
)

type (
	logger interface {
		InjectLogger(*zap.SugaredLogger)
	}

	eventRecorder interface {
		InjectEventRecorder(record.EventRecorder)
	}

	objectTracker interface {
		InjectObjectTracker(tracker.Interface)
	}
)

func LoggerInto(l *zap.SugaredLogger, i interface{}) {
	if o, ok := i.(logger); ok {
		o.InjectLogger(l)
	}
}

func EventRecorderInto(r record.EventRecorder, i interface{}) {
	if o, ok := i.(eventRecorder); ok {
		o.InjectEventRecorder(r)
	}
}

func ObjectTrackerInto(t tracker.Interface, i interface{}) {
	if o, ok := i.(objectTracker); ok {
		o.InjectObjectTracker(t)
	}
}
