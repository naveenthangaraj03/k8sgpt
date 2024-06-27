package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

type ConstraintAnalyzer struct{}

type Violation struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type Status struct {
	Violations []Violation `json:"violations,omitempty"`
}

func InitializeClients() (dynamic.Interface, error) {
	kubeconfigpath := "/root/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigpath)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return dynamicClient, nil
}

type MachineReconciler struct {
	DynClient dynamic.Interface
	Logger    *log.Logger
}

func (m *MachineReconciler) getConstraint(group, version, resource string, ctx context.Context) ([]*unstructured.Unstructured, error) {
	resourceId := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	ResourceClient := m.DynClient.Resource(resourceId)
	Resources, err := ResourceClient.List(ctx, v1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return []*unstructured.Unstructured{}, nil
		}
		m.Logger.Printf("Error listing %s Resources: %s\n", resource, err.Error())
		return nil, err
	}
	var releases []*unstructured.Unstructured
	for _, item := range Resources.Items {
		releases = append(releases, &item)
	}
	return releases, nil
}

func (m *MachineReconciler) getConstraintTemplateNameList(group, version, resource string, ctx context.Context) ([]string, error) {
	resourceId := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	ResourceClient := m.DynClient.Resource(resourceId)
	Resources, err := ResourceClient.List(ctx, v1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil , nil
		}
		m.Logger.Printf("Error listing %s Resources: %s\n", resource, err.Error())
		return nil, err
	}
	var releases []string
	for _, item := range Resources.Items {
		releases = append(releases, item.GetName())
	}
	return releases, nil
}


func (ConstraintAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	dynamicClient, _ := InitializeClients()
	reconciler := &MachineReconciler{
		DynClient: dynamicClient,
		Logger:    log.New(log.Writer(), "MachineReconciler: ", log.Flags()),
	}
	var preAnalysis = map[string]common.PreAnalysis{}
	ctx := context.TODO()
	var constrainttemplate []string
	var name string
	constrainttemplate, _ = reconciler.getConstraintTemplateNameList("templates.gatekeeper.sh","v1","constrainttemplates" , ctx)
	for i:=0; i < len(constrainttemplate); i++ {
		constraintlist, _ := reconciler.getConstraint("constraints.gatekeeper.sh", "v1beta1",constrainttemplate[i] , ctx)
		for _, release := range constraintlist {
			name = release.GetName()
			var failures []common.Failure
			objStatus, _, _ := unstructured.NestedMap(release.UnstructuredContent(), "status")
			tempJSON, _ := json.Marshal(objStatus)
			var tempStatus Status
			_ = json.Unmarshal(tempJSON, &tempStatus)
			for _, violation := range tempStatus.Violations {
				failures = append(failures, common.Failure{
					Text:      violation.Message,
					Sensitive: []common.Sensitive{},
				})
			}
			if len(failures) > 0 {
				preAnalysis[fmt.Sprintf("%s/%s", release.GetKind(), name)] = common.PreAnalysis{
					FailureDetails: failures,
				}
			}
		}
	}
	for key, value := range preAnalysis {
		var currentAnalysis = common.Result{
			Kind:  "GatekeeperConstraint",
			Name:  key,
			Error: value.FailureDetails,
		}
		a.Results = append(a.Results, currentAnalysis)
	}

	return a.Results, nil
}
