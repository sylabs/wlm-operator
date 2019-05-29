// Copyright (c) 2019 Sylabs, Inc. All rights reserved.
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

// Code generated by main. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/sylabs/slurm-operator/pkg/operator/apis/slurm/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeSlurmJobs implements SlurmJobInterface
type FakeSlurmJobs struct {
	Fake *FakeSlurmV1alpha1
	ns   string
}

var slurmjobsResource = schema.GroupVersionResource{Group: "slurm.sylabs.io", Version: "v1alpha1", Resource: "slurmjobs"}

var slurmjobsKind = schema.GroupVersionKind{Group: "slurm.sylabs.io", Version: "v1alpha1", Kind: "SlurmJob"}

// Get takes name of the slurmJob, and returns the corresponding slurmJob object, and an error if there is any.
func (c *FakeSlurmJobs) Get(name string, options v1.GetOptions) (result *v1alpha1.SlurmJob, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(slurmjobsResource, c.ns, name), &v1alpha1.SlurmJob{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SlurmJob), err
}

// List takes label and field selectors, and returns the list of SlurmJobs that match those selectors.
func (c *FakeSlurmJobs) List(opts v1.ListOptions) (result *v1alpha1.SlurmJobList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(slurmjobsResource, slurmjobsKind, c.ns, opts), &v1alpha1.SlurmJobList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.SlurmJobList{ListMeta: obj.(*v1alpha1.SlurmJobList).ListMeta}
	for _, item := range obj.(*v1alpha1.SlurmJobList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested slurmJobs.
func (c *FakeSlurmJobs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(slurmjobsResource, c.ns, opts))

}

// Create takes the representation of a slurmJob and creates it.  Returns the server's representation of the slurmJob, and an error, if there is any.
func (c *FakeSlurmJobs) Create(slurmJob *v1alpha1.SlurmJob) (result *v1alpha1.SlurmJob, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(slurmjobsResource, c.ns, slurmJob), &v1alpha1.SlurmJob{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SlurmJob), err
}

// Update takes the representation of a slurmJob and updates it. Returns the server's representation of the slurmJob, and an error, if there is any.
func (c *FakeSlurmJobs) Update(slurmJob *v1alpha1.SlurmJob) (result *v1alpha1.SlurmJob, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(slurmjobsResource, c.ns, slurmJob), &v1alpha1.SlurmJob{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SlurmJob), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeSlurmJobs) UpdateStatus(slurmJob *v1alpha1.SlurmJob) (*v1alpha1.SlurmJob, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(slurmjobsResource, "status", c.ns, slurmJob), &v1alpha1.SlurmJob{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SlurmJob), err
}

// Delete takes name of the slurmJob and deletes it. Returns an error if one occurs.
func (c *FakeSlurmJobs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(slurmjobsResource, c.ns, name), &v1alpha1.SlurmJob{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeSlurmJobs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(slurmjobsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.SlurmJobList{})
	return err
}

// Patch applies the patch and returns the patched slurmJob.
func (c *FakeSlurmJobs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.SlurmJob, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(slurmjobsResource, c.ns, name, pt, data, subresources...), &v1alpha1.SlurmJob{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SlurmJob), err
}
