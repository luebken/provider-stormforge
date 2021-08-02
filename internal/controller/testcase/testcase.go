/*
Copyright 2020 The Crossplane Authors.

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

package testcase

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/luebken/provider-stormforge/apis/load/v1alpha1"
	apisv1alpha1 "github.com/luebken/provider-stormforge/apis/v1alpha1"
)

const (
	errNotMyType    = "managed resource is not a TestCase custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

type forgeApiResponse struct {
	ForgeApiResponseData []forgeApiResponseData `json:"data"`
}
type forgeApiResponseData struct {
	Id         string                         `json:"id"`
	Attributes forgeApiResponseDataAttributes `json:"attributes"`
}
type forgeApiResponseDataAttributes struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Org   string
}

type forge struct {
	jwtToken string
}

func NewForge(jwtToken string) (forge, error) {
	result := &forge{
		jwtToken: jwtToken,
	}
	return *result, nil
}
func (f *forge) ping() error {
	//TODO f.jwtToken
	cmd := exec.Command("forge", "ping")
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	fmt.Println(string(stdout))
	return nil
}
func (f *forge) exists(org string, name string) (bool, error) {
	cmd := exec.Command("forge", "--output", "json", "test-case", "list", org)
	stdout, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
		return false, err
	}

	var r forgeApiResponse
	err = json.Unmarshal(stdout, &r)
	if err != nil {
		fmt.Println(err.Error())
		return false, err
	}

	for _, element := range r.ForgeApiResponseData {
		if element.Attributes.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (f *forge) create(org string, name string) error {
	cmd := exec.Command("forge", "test-case", "create", org+"/"+name, "examples/sample/loadtest.mjs") //TODO real test-case
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	fmt.Println("create: " + string(stdout))
	return nil
}

// Setup adds a controller that reconciles TestCase managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(v1alpha1.TestCaseGroupKind)

	o := controller.Options{
		RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.TestCaseGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o).
		For(&v1alpha1.TestCase{}).
		Complete(r)
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube  client.Client
	usage resource.Tracker
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {

	fmt.Printf("MDL Connect\n")

	cr, ok := mg.(*v1alpha1.TestCase)
	if !ok {
		return nil, errors.New(errNotMyType)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	fmt.Printf("MDL pc.Spec.Credentials.data: %+v\n", string(data))

	forge, err := NewForge(string(data))
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}
	forge.ping()

	return &external{forge: forge}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	forge forge
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	testCase, ok := mg.(*v1alpha1.TestCase)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotMyType)
	}

	exists, _ := c.forge.exists(testCase.Spec.ForProvider.Org, testCase.Spec.ForProvider.Name)

	// These fmt statements should be removed in the real implementation.
	fmt.Printf("MDL Observing: %+v\n", testCase)
	fmt.Printf("MDL Observing TestCase Exists: %+v\n", exists)

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: exists,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: true,

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.TestCase)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotMyType)
	}

	fmt.Printf("MDL Creating: %+v\n", cr)
	err := c.forge.create(cr.Spec.ForProvider.Org, cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.TestCase)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotMyType)
	}

	fmt.Printf("MDL Updating: %+v\n", cr)

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.TestCase)
	if !ok {
		return errors.New(errNotMyType)
	}

	fmt.Printf("MDL Deleting: %+v", cr)

	return nil
}
