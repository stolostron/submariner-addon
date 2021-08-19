package testing

import (
	"errors"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime"
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
