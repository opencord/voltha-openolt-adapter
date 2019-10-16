/*
 * Copyright 2018-present Open Networking Foundation

 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at

 * http://www.apache.org/licenses/LICENSE-2.0

 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package adaptercore

import (
	"errors"
	"reflect"
	"testing"
)

func getTranisitions() map[Trigger]Transition {
	transitions := make(map[Trigger]Transition)
	transition := Transition{
		previousState: []DeviceState{1},
		currentState:  1,
	}
	transitions[1] = transition
	return transitions
}
func getTranisitionsAfter() map[Trigger]Transition {
	transitions := make(map[Trigger]Transition)
	transition := Transition{
		previousState: []DeviceState{1},
		currentState:  1,
		after: []TransitionHandler{func() error {
			return nil
		}, func() error {
			return errors.New("transition error")
		}},
	}
	transitions[1] = transition
	return transitions
}
func getTranisitionsBefore() map[Trigger]Transition {
	transitions := make(map[Trigger]Transition)
	transition := Transition{
		previousState: []DeviceState{1},
		currentState:  1,
		before: []TransitionHandler{func() error {
			return nil
		}, func() error {
			return errors.New("transition error")
		}},
	}
	transitions[1] = transition
	return transitions
}
func TestNewTransitionMap(t *testing.T) {
	type args struct {
		dh *DeviceHandler
	}
	tests := []struct {
		name string
		args args
		want *TransitionMap
	}{
		{"NewTransitionMap-1", args{newMockDeviceHandler()}, &TransitionMap{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewTransitionMap(tt.args.dh); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("NewTransitionMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransitionMap_Handle(t *testing.T) {
	type fields struct {
		transitions        map[Trigger]Transition
		currentDeviceState DeviceState
	}
	type args struct {
		trigger Trigger
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"Handle-1", fields{getTranisitions(), 0}, args{1}},
		{"Handle-2", fields{getTranisitions(), 1}, args{1}},
		{"Handle-3", fields{getTranisitionsBefore(), 1}, args{1}},
		{"Handle-4", fields{getTranisitionsAfter(), 1}, args{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tMap := &TransitionMap{
				transitions:        tt.fields.transitions,
				currentDeviceState: tt.fields.currentDeviceState,
			}
			tMap.Handle(tt.args.trigger)
		})
	}
}

func TestTransitionMap_isValidTransition(t *testing.T) {
	type fields struct {
		transitions        map[Trigger]Transition
		currentDeviceState DeviceState
	}
	type args struct {
		trigger Trigger
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{"isValidTransition-1", fields{getTranisitions(), 1}, args{1},
			true},
		{"isValidTransition-2", fields{getTranisitions(), 0}, args{1},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tMap := &TransitionMap{
				transitions:        tt.fields.transitions,
				currentDeviceState: tt.fields.currentDeviceState,
			}
			if got := tMap.isValidTransition(tt.args.trigger); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("isValidTransition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_funcName(t *testing.T) {
	type args struct {
		f interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"FuncName-1", args{newMockDeviceHandler()}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := funcName(tt.args.f); got != tt.want {
				t.Errorf("funcName() = %v, want %v", got, tt.want)
			}
		})
	}
}
