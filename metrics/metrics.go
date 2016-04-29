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
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type (
	engineFailure string
	registryOp    string
)

const (
	Namespace = "fleet"

	MachineAway     engineFailure = "machine_away"
	RunFailure      engineFailure = "run"
	ScheduleFailure engineFailure = "schedule"
	Get             registryOp    = "get"
	Set             registryOp    = "set"
	GetAll          registryOp    = "get_all"
)

var (
	leaderGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "engine",
		Name:      "leader_start_time",
		Help:      "Start time of becoming an engine leader since epoch in seconds.",
	})

	engineTaskCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "engine",
		Name:      "task_count_total",
		Help:      "Counter of engine schedule tasks.",
	}, []string{"type"})

	engineTaskFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "engine",
		Name:      "task_failure_count_total",
		Help:      "Counter of engine schedule task failures.",
	}, []string{"type"})

	engineReconcileCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "engine",
		Name:      "reconcile_count_total",
		Help:      "Counter of reconcile rounds.",
	})

	engineReconcileDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "engine",
		Name:      "reconcile_duration_second",
		Help:      "Histogram of time (in seconds) each schedule round takes.",
	})

	engineReconcileFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "engine",
		Name:      "reconcile_failure_count_total",
		Help:      "Counter of scheduling failures.",
	}, []string{"type"})

	registryOpCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "registry",
		Name:      "operation_count_total",
		Help:      "Counter of registry operations.",
	}, []string{"type"})

	registryOpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "registry",
		Name:      "operation_duration_second",
		Help:      "Histogram of time (in seconds) each schedule round takes.",
	}, []string{"ops"})

	registryOpFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "registry",
		Name:      "operation_failed_count_total",
		Help:      "Counter of failed registry operations.",
	}, []string{"type"})
)

func init() {
	prometheus.MustRegister(leaderGauge)
	prometheus.MustRegister(engineTaskCount)
	prometheus.MustRegister(engineTaskFailureCount)
	prometheus.MustRegister(engineReconcileCount)
	prometheus.MustRegister(engineReconcileFailureCount)
}

func ReportEngineLeader() {
	epoch := time.Now().Unix()
	leaderGauge.Add(float64(epoch))
}
func ReportEngineTask(task string) {
	task = strings.ToLower(task)
	engineTaskCount.WithLabelValues(string(task)).Inc()
}
func ReportEngineTaskFailure(task string) {
	task = strings.ToLower(task)
	engineTaskFailureCount.WithLabelValues(string(task)).Inc()
}
func ReportEngineReconcileSuccess(start time.Time) {
	engineReconcileCount.Inc()
	engineReconcileDuration.Observe(float64(time.Since(start)) / float64(time.Second))
}
func ReportEngineReconcileFailure(reason engineFailure) {
	engineReconcileFailureCount.WithLabelValues(string(reason)).Inc()
}
func ReportRegistryOpSuccess(op registryOp, start time.Time) {
	registryOpCount.WithLabelValues(string(op)).Inc()
	registryOpDuration.WithLabelValues(string(op)).Observe(float64(time.Since(start)) / float64(time.Second))
}
func ReportRegistryOpFailure(op registryOp) {
	registryOpFailureCount.WithLabelValues(string(op)).Inc()
}
