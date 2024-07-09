/*
Copyright 2023 The K8sGPT Authors.
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

package analyzer

import (
	"fmt"

	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type HpaAnalyzer struct{}

func (HpaAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {

	kind := "HorizontalPodAutoscaler"

	AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	list, err := a.Client.GetClient().AutoscalingV2().HorizontalPodAutoscalers(a.Namespace).List(a.Context, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var preAnalysis = map[string]common.PreAnalysis{}

	for _, hpa := range list.Items {
		var failures []common.Failure

	
		//check the error from status field
		conditions := hpa.Status.Conditions
		for _, condition := range conditions {
			if condition.Status != "True" {
				failures = append(failures, common.Failure{
					Text:      condition.Message,
					Sensitive: []common.Sensitive{},
				})
			}
		}
		if len(failures) > 0 {
			preAnalysis[fmt.Sprintf("%s/%s", hpa.Namespace, hpa.Name)] = common.PreAnalysis{
				HorizontalPodAutoscalers: hpa,
				FailureDetails:           failures,
			}
			AnalyzerErrorsMetric.WithLabelValues(kind, hpa.Name, hpa.Namespace).Set(float64(len(failures)))
		}

	}

	for key, value := range preAnalysis {
		var currentAnalysis = common.Result{
			Kind:  kind,
			Name:  key,
			Error: value.FailureDetails,
		}

		parent, found := util.GetParent(a.Client, value.HorizontalPodAutoscalers.ObjectMeta)
		if found {
			currentAnalysis.ParentObject = parent
		}
		a.Results = append(a.Results, currentAnalysis)
	}

	return a.Results, nil
}
