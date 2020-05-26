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

//Package olterrors implements functions to manipulate OLT errors
package olterrors

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"strings"
)

const (
	defaultLogAndReturnLevel = log.ErrorLevel
)

func copy(ctx context.Context, src log.Fields) log.Fields {
	dst := make(log.Fields)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func merge(ctx context.Context, one, two log.Fields) log.Fields {
	dst := make(log.Fields)
	for k, v := range one {
		dst[k] = v
	}
	for k, v := range two {
		dst[k] = v
	}
	return dst
}

// LoggableError defined functions that can be used to log an object
type LoggableError interface {
	error
	Log(context.Context) error
	LogAt(context.Context, log.LogLevel) error
}

// ErrAdapter represents a basic adapter error that combines an name, field set
// and wrapped error
type ErrAdapter struct {
	name    string
	fields  log.Fields
	wrapped error
}

// NewErrAdapter constructs a new error with the given values
func NewErrAdapter(ctx context.Context, name string, fields log.Fields, wrapped error) LoggableError {
	return &ErrAdapter{
		name:    name,
		fields:  copy(ctx, fields),
		wrapped: wrapped,
	}
}

// Name returns the error name
func (e *ErrAdapter) Name(ctx context.Context) string {
	return e.name
}

// Fields returns the fields associated with the error
func (e *ErrAdapter) Fields(ctx context.Context) log.Fields {
	return e.fields
}

// Unwrap returns the wrapped or nested error
func (e *ErrAdapter) Unwrap(ctx context.Context) error {
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

// Log logs the error at the default level for log and return
func (e *ErrAdapter) Log(ctx context.Context) error {
	return e.LogAt(ctx, defaultLogAndReturnLevel)
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrAdapter) LogAt(ctx context.Context, level log.LogLevel) error {
	logger := log.Debugw
	switch level {
	case log.InfoLevel:
		logger = log.Infow
	case log.WarnLevel:
		logger = log.Warnw
	case log.ErrorLevel:
		logger = log.Errorw
	case log.FatalLevel:
		logger = log.Fatalw
	}
	local := e.fields
	if e.wrapped != nil {
		local = merge(ctx, e.fields, log.Fields{"wrapped": e.wrapped})
	}
	logger(e.name, local)
	return e
}

// ErrInvalidValue represents an error condition with given value is not able to
// be processed
type ErrInvalidValue struct {
	ErrAdapter
}

// NewErrInvalidValue constructs a new error based on the given values
func NewErrInvalidValue(ctx context.Context, fields log.Fields, wrapped error) LoggableError {
	return &ErrInvalidValue{
		ErrAdapter{
			name:    "invalid-value",
			fields:  copy(ctx, fields),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrInvalidValue) Log(ctx context.Context) error {
	_ = e.ErrAdapter.Log(ctx)
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrInvalidValue) LogAt(ctx context.Context, level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(ctx, level)
	return e
}

// ErrNotFound represents an error condition when a value can not be located
// given a field set of criteria
type ErrNotFound struct {
	ErrAdapter
}

// NewErrNotFound constructs a new error based on the given values
func NewErrNotFound(ctx context.Context, target string, fields log.Fields, wrapped error) LoggableError {
	return &ErrNotFound{
		ErrAdapter{
			name:    "not-found",
			fields:  merge(ctx, fields, log.Fields{"target": target}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrNotFound) Log(ctx context.Context) error {
	_ = e.ErrAdapter.Log(ctx)
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrNotFound) LogAt(ctx context.Context, level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(ctx, level)
	return e
}

// ErrPersistence representation an error condition when a persistence operation
// did not succeed
type ErrPersistence struct {
	ErrAdapter
}

// NewErrPersistence constructs a new error based on the given values
func NewErrPersistence(ctx context.Context, operation, entityType string, ID uint32, fields log.Fields, wrapped error) LoggableError {
	return &ErrPersistence{
		ErrAdapter{
			name: "unable-to-persist",
			fields: merge(ctx, fields, log.Fields{
				"operation":   operation,
				"entity-type": entityType,
				"id":          fmt.Sprintf("0x%x", ID)}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrPersistence) Log(ctx context.Context) error {
	_ = e.ErrAdapter.Log(ctx)
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrPersistence) LogAt(ctx context.Context, level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(ctx, level)
	return e
}

// ErrCommunication representation an error condition when an interprocess
// message communication fails
type ErrCommunication struct {
	ErrAdapter
}

// NewErrCommunication constructs a new error based on the given values
func NewErrCommunication(ctx context.Context, operation string, fields log.Fields, wrapped error) LoggableError {
	return &ErrCommunication{
		ErrAdapter{
			name: "failed-communication",
			fields: merge(ctx, fields, log.Fields{
				"operation": operation}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrCommunication) Log(ctx context.Context) error {
	_ = e.ErrAdapter.Log(ctx)
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrCommunication) LogAt(ctx context.Context, level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(ctx, level)
	return e
}

// ErrFlowOp represents an error condition when a flow operation to a device did
// not succeed
type ErrFlowOp struct {
	ErrAdapter
}

// NewErrFlowOp constructs a new error based on the given values
func NewErrFlowOp(ctx context.Context, operation string, ID uint32, fields log.Fields, wrapped error) LoggableError {
	return &ErrPersistence{
		ErrAdapter{
			name: "unable-to-perform-flow-operation",
			fields: merge(ctx, fields, log.Fields{
				"operation": operation,
				"id":        fmt.Sprintf("0x%x", ID)}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrFlowOp) Log(ctx context.Context) error {
	_ = e.ErrAdapter.Log(ctx)
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrFlowOp) LogAt(ctx context.Context, level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(ctx, level)
	return e
}

// ErrGroupOp represents an error condition when a group operation to a device did
// not succeed
type ErrGroupOp struct {
	ErrAdapter
}

// NewErrGroupOp constructs a new error based on the given values
func NewErrGroupOp(ctx context.Context, operation string, ID uint32, fields log.Fields, wrapped error) LoggableError {
	return &ErrPersistence{
		ErrAdapter{
			name: "unable-to-perform-group-operation",
			fields: merge(ctx, fields, log.Fields{
				"operation": operation,
				"id":        fmt.Sprintf("0x%x", ID)}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrGroupOp) Log(ctx context.Context) error {
	_ = e.ErrAdapter.Log(ctx)
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrGroupOp) LogAt(ctx context.Context, level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(ctx, level)
	return e
}

// ErrTimeout represents an error condition when the deadline for performing an
// operation has been exceeded
type ErrTimeout struct {
	ErrAdapter
}

// NewErrTimeout constructs a new error based on the given values
func NewErrTimeout(ctx context.Context, operation string, fields log.Fields, wrapped error) LoggableError {
	return &ErrTimeout{
		ErrAdapter{
			name:    "operation-timed-out",
			fields:  merge(ctx, fields, log.Fields{"operation": operation}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrTimeout) Log(ctx context.Context) error {
	_ = e.ErrAdapter.Log(ctx)
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrTimeout) LogAt(ctx context.Context, level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(ctx, level)
	return e
}

var (
	// ErrNotImplemented error returned when an unimplemented method is
	// invoked
	ErrNotImplemented = NewErrAdapter(nil, "not-implemented", nil, nil)

	// ErrInvalidPortRange error returned when a given port is not in the
	// valid range
	ErrInvalidPortRange = NewErrAdapter(nil, "invalid-port-range", nil, nil)

	// ErrStateTransition error returned when a state transition is fails
	ErrStateTransition = NewErrAdapter(nil, "state-transition", nil, nil)

	// ErrResourceManagerInstantiating error returned when an unexpected
	// condition occcurs while instantiating the resource manager
	ErrResourceManagerInstantiating = NewErrAdapter(nil, "resoure-manager-instantiating", nil, nil)
)
