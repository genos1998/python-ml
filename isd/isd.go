package isd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	metricutil "github.com/argoproj/argo-rollouts/utils/metric"
	timeutil "github.com/argoproj/argo-rollouts/utils/time"
)

const (
	ProviderType                          = "ISD"
	configIdLookupURLFormat               = `%s/autopilot/api/v3/registerCanary`
	scoreurl_format                       = `%s/autopilot/canaries/%s`
	httpConnectionTimeout   time.Duration = 15 * time.Second
	jobPayloadFormat                      = `{
        "application": "%s",
        "canaryConfig": {
                "lifetimeHours": %s,
                "canaryHealthCheckHandler": {
                                "minimumCanaryResultScore": %s
                                },
                "canarySuccessCriteria": {
                            "canaryResultScore": %s
                                }
                },
        "canaryDeployments": [
                    {
                    "canaryStartTimeMs": %s,
                    "baselineStartTimeMs": %s
                    }
          ]
    }`
)

type Provider struct {
	logCtx log.Entry
	client http.Client
}

// Type indicates provider is a ISD provider
func (p *Provider) Type() string {
	return ProviderType
}

// GetMetadata returns any additional metadata which needs to be stored & displayed as part of the metrics result.
func (p *Provider) GetMetadata(metric v1alpha1.Metric) map[string]string {
	return nil
}

func isNil(i interface{}) bool {
	return i == nil
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

// Run queries kayentd for the metric
func (p *Provider) Run(run *v1alpha1.AnalysisRun, metric v1alpha1.Metric) v1alpha1.Measurement {

	var canaryStartTime string
	var baselineStartTime string
	var canary_Id string

	startTime := timeutil.MetaNow()
	newMeasurement := v1alpha1.Measurement{
		StartedAt: &startTime,
	}

	configIdLookupURL := fmt.Sprintf(configIdLookupURLFormat, metric.Provider.ISD.Gate_url)

	if metric.Provider.ISD.Canary_start_time == "" && metric.Provider.ISD.Baseline_start_time == "" && metric.Provider.ISD.LifetimeHours == "" {
		err := errors.New("invalid Response")
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	/* Future use where start time are not given only lifetimeHours
	if metric.Provider.ISD.Canary_start_time == "" && metric.Provider.ISD.Baseline_start_time == "" {
		tm := time.Now()
		canaryStartTime = fmt.Sprintf("%d", tm.UnixNano()/int64(time.Millisecond))
		baselineStartTime = fmt.Sprintf("%d", tm.UnixNano()/int64(time.Millisecond))
		fmt.Printf("\nTime: %s, %s\n", canaryStartTime, baselineStartTime)
	} else {
		ts_start, _ := time.Parse(time.RFC3339, metric.Provider.ISD.Canary_start_time)   //make a time object for canary start time
		canaryStartTime = fmt.Sprintf("%d", ts_start.UnixNano()/int64(time.Millisecond)) //convert ISO to epoch

		ts_start, _ = time.Parse(time.RFC3339, metric.Provider.ISD.Baseline_start_time)    //make a time object for baseline start time
		baselineStartTime = fmt.Sprintf("%d", ts_start.UnixNano()/int64(time.Millisecond)) //convert ISO to epoch
	} */

	ts_start, _ := time.Parse(time.RFC3339, metric.Provider.ISD.Canary_start_time)   //make a time object for canary start time
	canaryStartTime = fmt.Sprintf("%d", ts_start.UnixNano()/int64(time.Millisecond)) //convert ISO to epoch

	ts_start, _ = time.Parse(time.RFC3339, metric.Provider.ISD.Baseline_start_time)    //make a time object for baseline start time
	baselineStartTime = fmt.Sprintf("%d", ts_start.UnixNano()/int64(time.Millisecond)) //convert ISO to epoch
	//If lifetimeHours not given
	if metric.Provider.ISD.LifetimeHours == "" {
		ts_start, _ := time.Parse(time.RFC3339, metric.Provider.ISD.Canary_start_time)
		ts_end, _ := time.Parse(time.RFC3339, metric.Provider.ISD.End_time)
		ts_difference := ts_end.Sub(ts_start)
		min, _ := time.ParseDuration(ts_difference.String())
		metric.Provider.ISD.LifetimeHours = fmt.Sprintf("%v", roundFloat(min.Minutes()/60, 1))
	}

	jobPayload := fmt.Sprintf(jobPayloadFormat, metric.Provider.ISD.Application, metric.Provider.ISD.LifetimeHours, fmt.Sprintf("%d", metric.Provider.ISD.Threshold.Marginal), fmt.Sprintf("%d", metric.Provider.ISD.Threshold.Pass), canaryStartTime, baselineStartTime) //Make the payload
	reqBody := strings.NewReader(jobPayload)
	// create a request object
	req, err := http.NewRequest(
		"POST",
		configIdLookupURL,
		reqBody,
	)

	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	// add a request header
	req.Header.Add("x-spinnaker-user", "admin")
	req.Header.Add("Content-Type", "application/json")

	// send an HTTP using `req` object
	res, _ := http.DefaultClient.Do(req)

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	// close response body
	res.Body.Close()

	var canary map[string]interface{}
	checkvalid := json.Valid(data)
	if checkvalid {
		json.Unmarshal(data, &canary)
		canary_Id = fmt.Sprintf("%#v", canary["canaryId"])
		fmt.Printf("\ncanary Id: %s", canary_Id)
	} else {
		err = errors.New("invalid Response")
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}

	scoreURL := fmt.Sprintf(scoreurl_format, metric.Provider.ISD.Gate_url, canary_Id)
	req, _ = http.NewRequest(
		"GET",
		scoreURL,
		nil,
	)
	req.Header.Set("x-spinnaker-user", "admin")
	req.Header.Set("Content-Type", "application/json")

	// Send Request
	res, err = p.client.Do(req)
	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}

	data, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	process := "RUNNING"
	var status map[string]interface{}

	//check till the system has finished running
	for process == "RUNNING" {
		json.Unmarshal(data, &status)
		a, _ := json.MarshalIndent(status["status"], "", "   ")
		json.Unmarshal(a, &status)
		if status["status"] != "RUNNING" {
			process = "COMPLETED"
		} else {

			res, err = p.client.Do(req)
			if err != nil {
				return metricutil.MarkMeasurementError(newMeasurement, err)
			}
			data, err = ioutil.ReadAll(res.Body)
			if err != nil {
				return metricutil.MarkMeasurementError(newMeasurement, err)
			}
		}
	}
	res.Body.Close()

	var result map[string]interface{}
	var final_score map[string]interface{}
	var canary_score string

	checkvalid = json.Valid(data)
	if checkvalid {
		json.Unmarshal(data, &result)
		jsonBytes, _ := json.MarshalIndent(result["canaryResult"], "", "   ")
		json.Unmarshal(jsonBytes, &final_score)
		if isNil(final_score["overallScore"]) {
			canary_score = "0"
		} else {
			canary_score = fmt.Sprintf("%v", final_score["overallScore"])
		}
	} else {
		err = errors.New("invalid Response")
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	fmt.Printf("canary Score: %s", canary_score)
	fmt.Printf("\n-----------------------------\n")
	score, _ := strconv.Atoi(canary_score)
	newMeasurement.Value = canary_score
	newMeasurement.Phase = evaluateResult(score, int(metric.Provider.ISD.Threshold.Pass), int(metric.Provider.ISD.Threshold.Marginal))
	finishTime := timeutil.MetaNow()
	newMeasurement.FinishedAt = &finishTime
	return newMeasurement
}

func evaluateResult(score int, pass int, marginal int) v1alpha1.AnalysisPhase {
	if score >= pass {
		return v1alpha1.AnalysisPhaseSuccessful
	} else if score < pass && score >= marginal {
		return v1alpha1.AnalysisPhaseInconclusive
	} else {
		return v1alpha1.AnalysisPhaseFailed
	}
}

// Resume should not be used the WebMetric provider since all the work should occur in the Run method
func (p *Provider) Resume(run *v1alpha1.AnalysisRun, metric v1alpha1.Metric, measurement v1alpha1.Measurement) v1alpha1.Measurement {
	p.logCtx.Warn("ISD provider should not execute the Resume method")
	return measurement
}

// Terminate should not be used the ISD provider since all the work should occur in the Run method
func (p *Provider) Terminate(run *v1alpha1.AnalysisRun, metric v1alpha1.Metric, measurement v1alpha1.Measurement) v1alpha1.Measurement {
	p.logCtx.Warn("ISD provider should not execute the Terminate method")
	return measurement
}

// GarbageCollect is a no-op for the ISD provider
func (p *Provider) GarbageCollect(run *v1alpha1.AnalysisRun, metric v1alpha1.Metric, limit int) error {
	return nil
}

func NewISDProvider(logCtx log.Entry, client http.Client) *Provider {
	return &Provider{
		logCtx: logCtx,
		client: client,
	}
}

func NewHttpClient() http.Client {

	c := http.Client{
		Timeout: httpConnectionTimeout,
	}

	return c
}
