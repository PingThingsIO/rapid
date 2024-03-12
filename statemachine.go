// Copyright 2019 Gregory Petrosyan <gregory.petrosyan@gmail.com>
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package rapid

import (
	"fmt"
	"reflect"
	"testing"
)

const (
	actionLabel       = "action"
	validActionTries  = 100 // hack, but probably good enough for now
	noValidActionsMsg = "can't find a valid (non-skipped) action"
)

// Repeat executes a random sequence of actions (often called a "state machine" test).
//
// Check, if set, is ran initially once and after every action. It should contain
// invariant checks.
//
// For complex state machines, it can be more convenient to specify actions as
// methods of a special state machine type. In this case, [StateMachineActions]
// can be used to create an actions generator from state machine methods using reflection.
func (t *T) Repeat(actions *Generator[StateMachineAction], check func(*T)) {
	t.Helper()

	steps := flags.steps
	if testing.Short() {
		steps /= 2
	}

	repeat := newRepeat(-1, -1, float64(steps), "Repeat")
	sm := stateMachine{
		actions: actions,
	}

	if check != nil {
		check(t)
	}
	t.failOnError()
	for repeat.more(t.s) {
		ok := sm.executeAction(t)
		if ok {
			if check != nil {
				check(t)
			}
			t.failOnError()
		} else {
			repeat.reject()
		}
	}
}

type smActionName interface {
	ActionName() string
}

type StateMachineAction struct {
	Name string
	Func func(*T)
}

func (sa StateMachineAction) ActionName() string {
	return sa.Name
}

func (sa StateMachineAction) GoString() string {
	return fmt.Sprintf("%q", sa.Name)
}

// StateMachineActions creates an actions generator for [*T.Repeat]
// from methods of sm using reflection.
func StateMachineActions(sm interface{}) *Generator[StateMachineAction] {
	var (
		v = reflect.ValueOf(sm)
		t = v.Type()
		n = t.NumMethod()
	)

	var actions []StateMachineAction
	for i := 0; i < n; i++ {
		name := t.Method(i).Name
		m, ok := v.Method(i).Interface().(func(*T))
		if ok {
			actions = append(actions, StateMachineAction{
				Name: name,
				Func: m,
			})
		}
	}

	assertf(len(actions) > 0, "state machine of type %v has no actions specified", t)

	return SampledFrom(actions)
}

type stateMachine struct {
	actions *Generator[StateMachineAction]
}

func (sm *stateMachine) executeAction(t *T) bool {
	t.Helper()

	for n := 0; n < validActionTries; n++ {
		i := t.s.beginGroup(actionLabel, false)
		action := sm.actions.Draw(t, "action")
		invalid, skipped := runAction(t, action)
		t.s.endGroup(i, false)

		if skipped {
			continue
		} else {
			return !invalid
		}
	}

	panic(stopTest(noValidActionsMsg))
}

func runAction(t *T, action StateMachineAction) (invalid bool, skipped bool) {
	defer func(draws int) {
		if r := recover(); r != nil {
			if _, ok := r.(invalidData); ok {
				invalid = true
				skipped = t.draws == draws
			} else {
				panic(r)
			}
		}
	}(t.draws)

	action.Func(t)
	t.failOnError()

	return false, false
}
