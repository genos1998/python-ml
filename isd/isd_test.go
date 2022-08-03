package isd

import (
	"net/http"
	"testing"
	"time"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	httpConnectionTestTimeout time.Duration = 15 * time.Second
)

func TestRunSuccessfully(t *testing.T) {
	// Test Cases
	var tests = []struct {
		metric        v1alpha1.Metric
		expectedPhase v1alpha1.AnalysisPhase
		expectedValue string
	}{
		{
			metric: v1alpha1.Metric{
				Name: "testapp",
				Provider: v1alpha1.MetricProvider{
					ISD: &v1alpha1.ISDMetric{
						Gate_url:            "https://ds312.isd-dev.opsmx.net/",
						Application:         "testapp",
						Baseline_start_time: "2022-07-29T13:15:00Z",
						Canary_start_time:   "2022-07-29T13:15:00Z",
						LifetimeHours:       "0.5",
						Threshold: v1alpha1.ISDThreshold{
							Pass:     80,
							Marginal: 65,
						},
					},
				},
			},
			expectedValue: "100",
			expectedPhase: v1alpha1.AnalysisPhaseSuccessful,
		},
		{
			metric: v1alpha1.Metric{
				Name: "testapp",
				Provider: v1alpha1.MetricProvider{
					ISD: &v1alpha1.ISDMetric{
						Gate_url:            "https://ds312.isd-dev.opsmx.net/",
						Application:         "testapp",
						Baseline_start_time: "2022-08-02T13:15:00Z",
						Canary_start_time:   "2022-08-02T13:15:00Z",
						End_time:            "2022-08-02T13:45:10Z",
						Threshold: v1alpha1.ISDThreshold{
							Pass:     80,
							Marginal: 65,
						},
					},
				},
			},
			expectedValue: "0",
			expectedPhase: v1alpha1.AnalysisPhaseSuccessful,
		},
		{
			metric: v1alpha1.Metric{
				Name: "testapp",
				Provider: v1alpha1.MetricProvider{
					ISD: &v1alpha1.ISDMetric{
						Gate_url:            "https://ds312.isd-dev.opsmx.net/",
						Application:         "testapp",
						Baseline_start_time: "2022-08-02T13:15:00Z",
						Canary_start_time:   "2022-08-02T13:15:00Z",
						LifetimeHours:       "0.5",
						Threshold: v1alpha1.ISDThreshold{
							Pass:     80,
							Marginal: 65,
						},
					},
				},
			},
			expectedValue: "0",
			expectedPhase: v1alpha1.AnalysisPhaseSuccessful,
		},
		/*{
			metric: v1alpha1.Metric{
				Name: "testapp",
				Provider: v1alpha1.MetricProvider{
					ISD: &v1alpha1.ISDMetric{
						Gate_url:      "https://ds312.isd-dev.opsmx.net/",
						Application:   "testapp",
						LifetimeHours: "0.5",
						Threshold: v1alpha1.ISDThreshold{
							Pass:     80,
							Marginal: 65,
						},
					},
				},
			},
			expectedValue: "0",
			expectedPhase: v1alpha1.AnalysisPhaseSuccessful,
		},*/
	}
	for _, test := range tests {
		e := log.NewEntry(log.New())
		c := NewTestHttpClient()
		provider := NewISDProvider(*e, c)
		measurement := provider.Run(newAnalysisRun(), test.metric)
		// Phase specific cases
		switch test.expectedPhase {
		case v1alpha1.AnalysisPhaseSuccessful:
			assert.NotNil(t, measurement.StartedAt)
			assert.Equal(t, test.expectedValue, measurement.Value)
			assert.NotNil(t, measurement.FinishedAt)
		case v1alpha1.AnalysisPhaseFailed:
			assert.NotNil(t, measurement.StartedAt)
			assert.NotNil(t, measurement.FinishedAt)
		}
	}
}

func newAnalysisRun() *v1alpha1.AnalysisRun {
	return &v1alpha1.AnalysisRun{}
}

func NewTestHttpClient() http.Client {
	c := http.Client{
		Timeout: httpConnectionTestTimeout,
	}
	return c
}
