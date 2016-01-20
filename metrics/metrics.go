// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type (
	engineFailure string
	operation     string
)

const (
	Namespace = "fleet"

	MachineLeft engineFailure = "machine_left"
	UnitRun     engineFailure = "unable_run_unit"
	JobInactive engineFailure = "job_inactive"

	Get    operation = "get"
	Set    operation = "set"
	GetAll operation = "get_all"
)

var (
	engineReconcileFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "engine",
		Name:      "schedule_failure_count_total",
		Help:      "Counter of scheduling failures.",
	}, []string{"type"})

	engineReconcileDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "engine",
		Name:      "reconcile_duration_second",
		Help:      "Historgram of time (in seconds) each reconcile round takes.",
	})

	agentReconcileDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "agent",
		Name:      "reconcile_duration_second",
		Help:      "Historgram of time (in seconds) each reconcile round takes.",
	})
	// agentReconcileCount

	registryCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "registry",
		Name:      "operation_count_total",
		Help:      "Counter of registry operations.",
	}, []string{"type"})

	registryFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "registry",
		Name:      "operation_failed_count_total",
		Help:      "Counter of failed registry operations.",
	}, []string{"type"})

	registryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "registry",
		Name:      "operation_duration_second",
		Help:      "Historgram of time (in seconds) each schedule round takes.",
	}, []string{"ops"})
)

func init() {
	prometheus.MustRegister(engineScheduleFailureCount)
	prometheus.MustRegister(engineReconcileDuration)
	prometheus.MustRegister(registryDuration)
	prometheus.MustRegister(registryCount)
	prometheus.MustRegister(registryFailureCount)
}

func EngineReconcileFailure(reason engineFailure) {
	engineReconcileFailureCount.WithLabelValues(string(reason)).Inc()
}

func EngineReconcileDuration(start time.Time) {
	engineReconcileDuration.Observe(float64(time.Since(start)) / float64(time.Second))
}

func AgentReconcileDuration(start time.Time) {
	agentReconcileDuration.Observe(float64(time.Since(start)) / float64(time.Second))
}

func RegistrySucces(ops operation, start time.Time) {
	registryDuration.WithLabelValues(string(ops)).Observe(float64(time.Since(start)) / float64(time.Second))
	registryCount.WithLabelValues(string(ops)).Inc()
}

func RegistryFailure(ops operation) {
	registryFailureCount.WithLabelValues(string(ops)).Inc()
}
