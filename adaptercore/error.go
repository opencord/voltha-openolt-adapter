/*
 * Copyright 2020-present Open Networking Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package adaptercore provides the utility for olt devices, flows and statistics
package adaptercore

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"strings"
)

func copy(src log.Fields) log.Fields {
	dst := make(log.Fields)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func merge(one, two log.Fields) log.Fields {
	dst := make(log.Fields)
	for k, v := range one {
		dst[k] = v
	}
	for k, v := range two {
		dst[k] = v
	}
	return dst
}

// LogAndReturn logs the error at the specified log level and then returns the
// error
func LogAndReturn(level int, err error, message ...string) error {
	fields := log.Debugw
	plain := log.Debug
	switch level {
	case log.InfoLevel:
		fields = log.Infow
		plain = log.Info
	case log.WarnLevel:
		fields = log.Warnw
		plain = log.Warn
	case log.ErrorLevel:
		fields = log.Errorw
		plain = log.Error
	case log.FatalLevel:
		fields = log.Fatalw
		plain = log.Fatal
	}
	var buf strings.Builder
	if len(message) > 0 {
		buf.WriteString(strings.Join(message, " "))
		buf.WriteString(": ")
	}
	if info, ok := err.(ErrorInformation); ok {
		buf.WriteString(info.Name())
		if len(message) > 0 {
			buf.WriteString(":")
			for _, m := range message {
				buf.WriteString(" ")
				buf.WriteString(m)
			}
		}
		local := info.Fields()
		if info.Unwrap() != nil {
			local = merge(info.Fields(), log.Fields{"wrapped": info.Unwrap()})
		}
		fields(buf.String(), local)
	} else {
		if len(message) > 0 {
			fields(buf.String(), log.Fields{"error": err})
		} else {
			plain(err)
		}
	}
	return err
}

// ErrorInformation exposes the common information about an adapter error
type ErrorInformation interface {
	Name() string
	Fields() log.Fields
	Unwrap() error
}

// ErrAdapter represents a basic adapter error that combines an name, field set
// and wrapped error
type ErrAdapter struct {
	name    string
	fields  log.Fields
	wrapped error
}

// NewErrAdapter constructs a new error with the given values
func NewErrAdapter(name string, fields log.Fields, wrapped error) *ErrAdapter {
	return &ErrAdapter{
		name:    name,
		fields:  copy(fields),
		wrapped: wrapped,
	}
}

// Name returns the error name
func (e *ErrAdapter) Name() string {
	return e.name
}

// Fields returns the fields associated with the error
func (e *ErrAdapter) Fields() log.Fields {
	return e.fields
}

// Unwrap returns the wrapped or nested error
func (e *ErrAdapter) Unwrap() error {
	return e.wrapped
}

// Error returns a string representation of the error
func (e *ErrAdapter) Error() string {
	var buf strings.Builder
	buf.WriteString(e.name)
	if len(e.fields) > 0 {
		if val, err := json.Marshal(e.fields); err == nil {
			buf.WriteString(": [")
			buf.WriteString(string(val))
			buf.WriteString("]")
		}
	}
	if e.wrapped != nil {
		buf.WriteString(": ")
		buf.WriteString(e.wrapped.Error())
	}
	return buf.String()
}

// ErrInvalidValue represents an error condition with given value is not able to
// be processed
type ErrInvalidValue struct {
	ErrAdapter
}

// NewErrInvalidValue constructs a new error based on the given values
func NewErrInvalidValue(fields log.Fields, wrapped error) *ErrInvalidValue {
	return &ErrInvalidValue{
		ErrAdapter{
			name:    "invalid-value",
			fields:  copy(fields),
			wrapped: wrapped,
		},
	}
}

// ErrNotFound represents an error condition when a value can not be located
// given a field set of criteria
type ErrNotFound struct {
	ErrAdapter
}

// NewErrNotFound constructs a new error based on the given values
func NewErrNotFound(target string, fields log.Fields, wrapped error) *ErrNotFound {
	return &ErrNotFound{
		ErrAdapter{
			name:    "not-found",
			fields:  merge(fields, log.Fields{"target": target}),
			wrapped: wrapped,
		},
	}
}

// ErrPersistence representation an error condition when a persistence operation
// did not succeed
type ErrPersistence struct {
	ErrAdapter
}

// NewErrPersistence constructs a new error based on the given values
func NewErrPersistence(operation, entityType string, ID uint32, fields log.Fields, wrapped error) *ErrPersistence {
	return &ErrPersistence{
		ErrAdapter{
			name: "unable-to-persist",
			fields: merge(fields, log.Fields{
				"operation":   operation,
				"entity-type": entityType,
				"id":          fmt.Sprintf("0x%x", ID)}),
			wrapped: wrapped,
		},
	}
}

// ErrCommunication representation an error condition when an interprocess
// message communication fails
type ErrCommunication struct {
	ErrAdapter
}

// NewErrCommunication constructs a new error based on the given values
func NewErrCommunication(operation string, fields log.Fields, wrapped error) *ErrCommunication {
	return &ErrCommunication{
		ErrAdapter{
			name: "failed-communication",
			fields: merge(fields, log.Fields{
				"operation": operation}),
			wrapped: wrapped,
		},
	}
}

// ErrFlowOp represents an error condition when a flow operation to a device did
// not succeed
type ErrFlowOp struct {
	ErrAdapter
}

// NewErrFlowOp constructs a new error based on the given values
func NewErrFlowOp(operation string, ID uint32, fields log.Fields, wrapped error) *ErrPersistence {
	return &ErrPersistence{
		ErrAdapter{
			name: "unable-to-perform-flow-operation",
			fields: merge(fields, log.Fields{
				"operation": operation,
				"id":        fmt.Sprintf("0x%x", ID)}),
			wrapped: wrapped,
		},
	}
}

// ErrTimeout represents an error condition when the deadline for performing an
// operation has been exceeded
type ErrTimeout struct {
	ErrAdapter
}

// NewErrTimeout constructs a new error based on the given values
func NewErrTimeout(operation string, fields log.Fields, wrapped error) *ErrTimeout {
	return &ErrTimeout{
		ErrAdapter{
			name:    "operation-timed-out",
			fields:  merge(fields, log.Fields{"operation": operation}),
			wrapped: wrapped,
		},
	}
}

var (
	// ErrNotImplemented error returned when an unimplemented method is
	// invoked
	ErrNotImplemented = errors.New("not-implemented")

	// ErrInvalidPortRange error returned when a given port is not in the
	// valid range
	ErrInvalidPortRange = errors.New("invalid-port-range")

	// ErrStateTransition error returned when a state transition is fails
	ErrStateTransition = errors.New("state-transition")

	// ErrResourceManagerInstantiating error returned when an unexpected
	// condition occcurs while instantiating the resource manager
	ErrResourceManagerInstantiating = errors.New("resoure-manager-instantiating")
)
