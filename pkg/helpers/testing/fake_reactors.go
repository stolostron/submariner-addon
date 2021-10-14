package testing

import (
	"errors"
	"sync"
	"sync/atomic"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/testing"
)

type FailingReactor struct {
	fail atomic.Value
}

func (r *FailingReactor) Fail(v bool) {
	r.fail.Store(v)
}

func FailOnAction(f *testing.Fake, resource, verb string, customErr error, autoReset bool) *FailingReactor {
	r := &FailingReactor{}
	r.fail.Store(true)

	retErr := customErr
	if retErr == nil {
		retErr = errors.New("fake error")
	}

	chain := []testing.Reactor{&testing.SimpleReactor{
		Verb:     verb,
		Resource: resource,
		Reaction: func(action testing.Action) (bool, runtime.Object, error) {
			if r.fail.Load().(bool) {
				if autoReset {
					r.fail.Store(false)
				}

				return true, nil, retErr
			}

			return false, nil, nil
		}}}
	f.ReactionChain = append(chain, f.ReactionChain...)

	return r
}

func ConflictOnUpdateReactor(f *testing.Fake, resource string) {
	reactors := f.ReactionChain[0:]
	resourceVersion := "100"
	state := sync.Map{}

	chain := []testing.Reactor{&testing.SimpleReactor{
		Verb:     "get",
		Resource: resource,
		Reaction: func(action testing.Action) (bool, runtime.Object, error) {
			obj, err := propagate(action, reactors)
			if obj != nil {
				m, _ := meta.Accessor(obj)
				_, ok := state.Load(m.GetName())
				if ok {
					m.SetResourceVersion(resourceVersion)
				}
			}

			return true, obj, err
		}}, &testing.SimpleReactor{
		Verb:     "update",
		Resource: resource,
		Reaction: func(action testing.Action) (bool, runtime.Object, error) {
			updateAction := action.(testing.UpdateActionImpl)
			m, _ := meta.Accessor(updateAction.Object)

			_, ok := state.Load(m.GetName())
			if !ok {
				state.Store(m.GetName(), true)

				return true, nil, apierrors.NewConflict(schema.GroupResource{}, "", errors.New("fake conflict"))
			}

			if m.GetResourceVersion() != resourceVersion {
				return true, nil, apierrors.NewConflict(schema.GroupResource{}, "", errors.New("fake conflict"))
			}

			state.Delete(m.GetName())

			return false, nil, nil
		}}}
	f.ReactionChain = append(chain, f.ReactionChain...)
}

func propagate(action testing.Action, toReactors []testing.Reactor) (runtime.Object, error) {
	for _, reactor := range toReactors {
		if !reactor.Handles(action) {
			continue
		}

		handled, ret, err := reactor.React(action)
		if !handled {
			continue
		}

		return ret, err
	}

	return nil, errors.New("action not handled")
}
