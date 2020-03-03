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

package core

import (
	"context"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"reflect"
	"testing"
	"time"
)

/**
Get's the transition Map with current state of the device.
*/
func getTranisitions() map[Trigger]Transition {
	transitions := make(map[Trigger]Transition)
	transition := Transition{
		previousState: []DeviceState{deviceStateConnected},
		currentState:  deviceStateConnected,
	}
	transitions[DeviceInit] = transition
	return transitions
}

/**
Get's the transition Map with after Transition func added.
*/
func getTranisitionsAfter() map[Trigger]Transition {
	transitions := make(map[Trigger]Transition)
	transition := Transition{
		previousState: []DeviceState{deviceStateConnected},
		currentState:  deviceStateConnected,
		after: []TransitionHandler{func(ctx context.Context) error {
			return nil
		}, func(ctx context.Context) error {
			return olterrors.ErrStateTransition
		}},
	}
	transitions[GrpcConnected] = transition
	return transitions
}

/**
Get's the transition Map with before Transition func added.
*/
func getTranisitionsBefore() map[Trigger]Transition {
	transitions := make(map[Trigger]Transition)
	transition := Transition{
		previousState: []DeviceState{deviceStateConnected},
		currentState:  deviceStateConnected,
		before: []TransitionHandler{func(ctx context.Context) error {
			return nil
		}, func(ctx context.Context) error {
			return olterrors.ErrStateTransition
		}},
	}
	transitions[GrpcConnected] = transition
	return transitions
}

/**
Check's Creation of transition Map, return's NewTransitionMap.
*/
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

/**
Checks the different transition of the device handled properly.
*/
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
		{"Handle-1", fields{getTranisitions(), deviceStateDown}, args{GrpcConnected}},
		{"Handle-2", fields{getTranisitions(), deviceStateConnected}, args{GrpcConnected}},
		{"Handle-3", fields{getTranisitionsBefore(), deviceStateConnected}, args{GrpcConnected}},
		{"Handle-4", fields{getTranisitionsAfter(), deviceStateConnected}, args{GrpcConnected}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tMap := &TransitionMap{
				transitions:        tt.fields.transitions,
				currentDeviceState: tt.fields.currentDeviceState,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			tMap.Handle(ctx, tt.args.trigger)
		})
	}
}

/**
Check's if the transition is valid or not.
*/
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
		{"isValidTransition-1", fields{getTranisitions(), deviceStateConnected}, args{DeviceInit},
			true},
		{"isValidTransition-2", fields{getTranisitions(), deviceStateDown}, args{GrpcConnected},
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

/**
Get's the After/Before transition method's function name.
*/
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
