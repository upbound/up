// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"sort"
	"strings"
	"time"

	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	queryv1alpha2 "github.com/upbound/up-sdk-go/apis/query/v1alpha2"
)

type Tree struct {
	AutoCollapse bool

	root *tview.TreeNode
}

func NewTree() Tree {
	return Tree{
		root: tview.NewTreeNode(""),
	}
}

func (t *Tree) Root() *tview.TreeNode {
	return t.root
}

func (t *Tree) Update(objs []queryv1alpha2.QueryResponseObject) {
	t.update(t.root, objs, 0)
}

func (t *Tree) update(parent *tview.TreeNode, respObjs []queryv1alpha2.QueryResponseObject, level int) []*Object { // nolint:gocyclo // TODO: split up
	existing := map[string]*tview.TreeNode{}
	for _, n := range parent.GetChildren() {
		obj := n.GetReference().(*Object)
		existing[obj.Id] = n
	}

	for _, o := range respObjs {
		if o.Object == nil {
			continue // should never happen
		}

		gv, _, _ := unstructured.NestedString(o.Object.Object, "apiVersion")
		ss := strings.SplitN(gv, "/", 2)
		group := ""
		if len(ss) == 2 {
			group = ss[0]
		}
		kind, _, _ := unstructured.NestedString(o.Object.Object, "kind")
		ns, _, _ := unstructured.NestedString(o.Object.Object, "metadata", "namespace")
		name, _, _ := unstructured.NestedString(o.Object.Object, "metadata", "name")
		creationString, _, _ := unstructured.NestedString(o.Object.Object, "metadata", "creationTimestamp")
		creationTimestamp, err := time.Parse(time.RFC3339, creationString)
		if err != nil {
			continue // should never happen as the kube API is type-safe
		}
		deletionString, _, _ := unstructured.NestedString(o.Object.Object, "metadata", "deletionTimestamp")
		var deletionTimestamp time.Time
		if deletionString != "" {
			if deletionTimestamp, err = time.Parse(time.RFC3339, deletionString); err != nil {
				continue // should never happen as the kube API is type-safe
			}
		}
		obj := &Object{
			Group:     group,
			Kind:      kind,
			Id:        o.ID,
			Namespace: ns,
			Name:      name,
			ControlPlane: ControlPlane{
				Namespace: o.ControlPlane.Namespace,
				Name:      o.ControlPlane.Name,
			},
			CreationTimestamp: creationTimestamp,
			DeletionTimestamp: deletionTimestamp,
			JSON:              *o.Object,
		}

		n, ok := existing[o.ID]
		if ok {
			old := n.GetReference().(*Object)
			obj.Synced = old.Synced
			obj.Ready = old.Ready
		} else {
			n = tview.NewTreeNode("title")
			parent.AddChild(n)
		}

		n.SetText(obj.Title())
		n.SetReference(obj)

		// open or close condition intervals
		if synced := obj.JSON.GetCondition(xpv1.TypeSynced); synced.Status == corev1.ConditionTrue {
			if len(obj.Synced) == 0 || !obj.Synced[len(obj.Synced)-1].To.IsZero() {
				obj.Synced = append(obj.Synced, Interval{From: synced.LastTransitionTime.Time})
			}
		} else {
			if len(obj.Synced) > 0 && obj.Synced[len(obj.Synced)-1].To.IsZero() {
				obj.Synced[len(obj.Synced)-1].To = synced.LastTransitionTime.Time
			}
		}
		if ready := obj.JSON.GetCondition(xpv1.TypeReady); ready.Status == corev1.ConditionTrue {
			if len(obj.Ready) == 0 || !obj.Ready[len(obj.Ready)-1].To.IsZero() {
				obj.Ready = append(obj.Ready, Interval{From: ready.LastTransitionTime.Time})
			}
		} else {
			if len(obj.Ready) > 0 && obj.Ready[len(obj.Ready)-1].To.IsZero() {
				obj.Ready[len(obj.Ready)-1].To = ready.LastTransitionTime.Time
			}
		}

		// translate events
		obj.Events = nil
		for _, respEv := range o.Relations["events"].Objects {
			var ev Event

			ev.Message, _, _ = unstructured.NestedString(respEv.Object.Object, "message")
			ev.Type, _, _ = unstructured.NestedString(respEv.Object.Object, "type")
			count, _, _ := unstructured.NestedInt64(respEv.Object.Object, "count")
			ev.Count = int(count)
			ts, _, _ := unstructured.NestedString(respEv.Object.Object, "lastTimestamp")
			if err := ev.LastTimestamp.Unmarshal([]byte(ts)); err != nil {
				continue // ignore this event
			}

			obj.Events = append(obj.Events, ev)
		}
		sort.Sort(EventsOrder(obj.Events))

		obj.Children = t.update(n, o.Relations["resources"].Objects, level+1)

		delete(existing, o.ID)
	}

	for _, n := range existing {
		parent.RemoveChild(n)
	}

	// auto-collapse when ready. NEEDS WORK.
	if false {
		hide := true
		for _, n := range parent.GetChildren() {
			ref := n.GetReference()
			if ref == nil {
				continue
			}
			obj := ref.(*Object)
			synced := obj.JSON.GetCondition(xpv1.TypeSynced)
			isSynced := synced.Status == corev1.ConditionTrue
			ready := obj.JSON.GetCondition(xpv1.TypeReady)
			isReady := ready.Status == corev1.ConditionTrue

			if !isSynced || !isReady || level == 0 {
				hide = false
				break
			}
		}
		if hide {
			parent.Collapse()
		} else {
			parent.Expand()
		}
	}

	sort.Sort(ObjectsOrder(parent.GetChildren()))

	objs := make([]*Object, len(parent.GetChildren()))
	for i, n := range parent.GetChildren() {
		objs[i] = n.GetReference().(*Object)
	}

	return objs
}
