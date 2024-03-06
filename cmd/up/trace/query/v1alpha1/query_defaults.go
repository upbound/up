// Copyright 2024 Upbound Inc
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

package v1alpha1

func (q *SpaceQuery) Default() {
	if q.Name == "" {
		q.Name = "default"
	}
	if q.Spec != nil {
		q.Spec.Default()
	}
}

func (q *GroupQuery) Default() {
	if q.Name == "" {
		q.Name = "default"
	}
	if q.Spec != nil {
		if q.Spec.Filter.ControlPlane.Namespace == "" {
			q.Spec.Filter.ControlPlane.Namespace = q.Namespace
		}

		q.Spec.Default()
	}
}

func (q *Query) Default() {
	if q.Spec != nil {
		if q.Spec.Filter.ControlPlane.Namespace == "" {
			q.Spec.Filter.ControlPlane.Namespace = q.Namespace
		}
		if q.Spec.Filter.ControlPlane.Name == "" {
			q.Spec.Filter.ControlPlane.Name = q.Name
		}

		q.Spec.Default()
	}
}

func (s *QuerySpec) Default() {
	if len(s.Order) == 0 {
		// default order list on top-level
		s.Order = []QueryOrder{{}}
	}

	s.QueryResources.Default()
}

func (r *QueryResources) Default() {
	if r.Limit == 0 {
		r.Limit = 100
	}

	for i := range r.Order {
		// default existing orders, but not the whole list
		r.Order[i].Default()
	}

	if r.Objects != nil {
		for name, sub := range r.Objects.Relations {
			sub.Default()
			r.Objects.Relations[name] = sub
		}
	}
}

func (r *QueryOrder) Default() {
	zero := QueryOrder{}
	if *r == zero {
		r.CreationTimestamp = "desc"
	}
}
