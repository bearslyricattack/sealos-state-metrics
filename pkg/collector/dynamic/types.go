// Package dynamic provides a generic dynamic client-based collector framework for monitoring CRDs.
package dynamic

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EventType represents the type of event that occurred
type EventType string

const (
	// EventAdd indicates a resource was added
	EventAdd EventType = "add"
	// EventUpdate indicates a resource was updated
	EventUpdate EventType = "update"
	// EventDelete indicates a resource was deleted
	EventDelete EventType = "delete"
)

// EventHandler is the callback interface for resource events
type EventHandler interface {
	// OnAdd is called when a resource is added
	OnAdd(obj *unstructured.Unstructured)

	// OnUpdate is called when a resource is updated
	OnUpdate(oldObj, newObj *unstructured.Unstructured)

	// OnDelete is called when a resource is deleted
	OnDelete(obj *unstructured.Unstructured)
}

// EventHandlerFuncs is a helper struct that implements EventHandler interface with function fields
type EventHandlerFuncs struct {
	AddFunc    func(obj *unstructured.Unstructured)
	UpdateFunc func(oldObj, newObj *unstructured.Unstructured)
	DeleteFunc func(obj *unstructured.Unstructured)
}

// OnAdd calls AddFunc if set
func (e EventHandlerFuncs) OnAdd(obj *unstructured.Unstructured) {
	if e.AddFunc != nil {
		e.AddFunc(obj)
	}
}

// OnUpdate calls UpdateFunc if set
func (e EventHandlerFuncs) OnUpdate(oldObj, newObj *unstructured.Unstructured) {
	if e.UpdateFunc != nil {
		e.UpdateFunc(oldObj, newObj)
	}
}

// OnDelete calls DeleteFunc if set
func (e EventHandlerFuncs) OnDelete(obj *unstructured.Unstructured) {
	if e.DeleteFunc != nil {
		e.DeleteFunc(obj)
	}
}
