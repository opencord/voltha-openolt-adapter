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
	"strings"

	"github.com/opencord/voltha-lib-go/v7/pkg/log"
)

const (
	defaultLogAndReturnLevel = log.ErrorLevel
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

// LoggableError defined functions that can be used to log an object
type LoggableError interface {
	error
	Log() error
	LogAt(log.LogLevel) error
}

// ErrAdapter represents a basic adapter error that combines an name, field set
// and wrapped error
type ErrAdapter struct {
	name    string
	fields  log.Fields
	wrapped error
}

// NewErrAdapter constructs a new error with the given values
func NewErrAdapter(name string, fields log.Fields, wrapped error) LoggableError {
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
	ctx := context.Background()
	var buf strings.Builder
	_, er := buf.WriteString(e.name)
	if er != nil {
		logger.Error(ctx, er)
	}
	if len(e.fields) > 0 {
		if val, err := json.Marshal(e.fields); err == nil {
			_, er = buf.WriteString(fmt.Sprintf(": [%s]", string(val)))
			if er != nil {
				logger.Error(ctx, er)
			}
		}
	}
	if e.wrapped != nil {
		_, er = buf.WriteString(fmt.Sprintf(": %s", e.wrapped.Error()))
		if er != nil {
			logger.Error(ctx, er)
		}
	}
	return buf.String()
}

// Log logs the error at the default level for log and return
func (e *ErrAdapter) Log() error {
	return e.LogAt(defaultLogAndReturnLevel)
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrAdapter) LogAt(level log.LogLevel) error {
	loggerFunc := logger.Debugw
	switch level {
	case log.InfoLevel:
		loggerFunc = logger.Infow
	case log.WarnLevel:
		loggerFunc = logger.Warnw
	case log.ErrorLevel:
		loggerFunc = logger.Errorw
	case log.FatalLevel:
		loggerFunc = logger.Fatalw
	}
	local := e.fields
	if e.wrapped != nil {
		local = merge(e.fields, log.Fields{"wrapped": e.wrapped})
	}
	loggerFunc(context.Background(), e.name, local)
	return e
}

// ErrInvalidValue represents an error condition with given value is not able to
// be processed
type ErrInvalidValue struct {
	ErrAdapter
}

// NewErrInvalidValue constructs a new error based on the given values
func NewErrInvalidValue(fields log.Fields, wrapped error) LoggableError {
	return &ErrInvalidValue{
		ErrAdapter{
			name:    "invalid-value",
			fields:  copy(fields),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrInvalidValue) Log() error {
	_ = e.ErrAdapter.Log()
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrInvalidValue) LogAt(level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(level)
	return e
}

// ErrNotFound represents an error condition when a value can not be located
// given a field set of criteria
type ErrNotFound struct {
	ErrAdapter
}

// NewErrNotFound constructs a new error based on the given values
func NewErrNotFound(target string, fields log.Fields, wrapped error) LoggableError {
	return &ErrNotFound{
		ErrAdapter{
			name:    "not-found",
			fields:  merge(fields, log.Fields{"target": target}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrNotFound) Log() error {
	_ = e.ErrAdapter.Log()
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrNotFound) LogAt(level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(level)
	return e
}

// ErrPersistence representation an error condition when a persistence operation
// did not succeed
type ErrPersistence struct {
	ErrAdapter
}

// NewErrPersistence constructs a new error based on the given values
func NewErrPersistence(operation, entityType string, ID uint64, fields log.Fields, wrapped error) LoggableError {
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

// Log logs the error at the default level for log and return
func (e *ErrPersistence) Log() error {
	_ = e.ErrAdapter.Log()
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrPersistence) LogAt(level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(level)
	return e
}

// ErrCommunication representation an error condition when an interprocess
// message communication fails
type ErrCommunication struct {
	ErrAdapter
}

// NewErrCommunication constructs a new error based on the given values
func NewErrCommunication(operation string, fields log.Fields, wrapped error) LoggableError {
	return &ErrCommunication{
		ErrAdapter{
			name: "failed-communication",
			fields: merge(fields, log.Fields{
				"operation": operation}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrCommunication) Log() error {
	_ = e.ErrAdapter.Log()
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrCommunication) LogAt(level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(level)
	return e
}

// ErrFlowOp represents an error condition when a flow operation to a device did
// not succeed
type ErrFlowOp struct {
	ErrAdapter
}

// NewErrFlowOp constructs a new error based on the given values
func NewErrFlowOp(operation string, ID uint64, fields log.Fields, wrapped error) LoggableError {
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

// Log logs the error at the default level for log and return
func (e *ErrFlowOp) Log() error {
	_ = e.ErrAdapter.Log()
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrFlowOp) LogAt(level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(level)
	return e
}

// ErrGroupOp represents an error condition when a group operation to a device did
// not succeed
type ErrGroupOp struct {
	ErrAdapter
}

// NewErrGroupOp constructs a new error based on the given values
func NewErrGroupOp(operation string, ID uint32, fields log.Fields, wrapped error) LoggableError {
	return &ErrPersistence{
		ErrAdapter{
			name: "unable-to-perform-group-operation",
			fields: merge(fields, log.Fields{
				"operation": operation,
				"id":        fmt.Sprintf("0x%x", ID)}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrGroupOp) Log() error {
	_ = e.ErrAdapter.Log()
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrGroupOp) LogAt(level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(level)
	return e
}

// ErrTimeout represents an error condition when the deadline for performing an
// operation has been exceeded
type ErrTimeout struct {
	ErrAdapter
}

// NewErrTimeout constructs a new error based on the given values
func NewErrTimeout(operation string, fields log.Fields, wrapped error) LoggableError {
	return &ErrTimeout{
		ErrAdapter{
			name:    "operation-timed-out",
			fields:  merge(fields, log.Fields{"operation": operation}),
			wrapped: wrapped,
		},
	}
}

// Log logs the error at the default level for log and return
func (e *ErrTimeout) Log() error {
	_ = e.ErrAdapter.Log()
	return e
}

// LogAt logs the error at the specified level and then returns the error
func (e *ErrTimeout) LogAt(level log.LogLevel) error {
	_ = e.ErrAdapter.LogAt(level)
	return e
}

var (
	// ErrNotImplemented error returned when an unimplemented method is
	// invoked
	ErrNotImplemented = NewErrAdapter("not-implemented", nil, nil)

	// ErrInvalidPortRange error returned when a given port is not in the
	// valid range
	ErrInvalidPortRange = NewErrAdapter("invalid-port-range", nil, nil)

	// ErrStateTransition error returned when a state transition is fails
	ErrStateTransition = NewErrAdapter("state-transition", nil, nil)

	// ErrResourceManagerInstantiating error returned when an unexpected
	// condition occcurs while instantiating the resource manager
	ErrResourceManagerInstantiating = NewErrAdapter("resoure-manager-instantiating", nil, nil)

	// ErrFlowManagerInstantiating error returned when an unexpected
	// condition occcurs while instantiating the flow manager
	ErrFlowManagerInstantiating = NewErrAdapter("flow-manager-instantiating", nil, nil)

	// ErrGroupManagerInstantiating error returned when an unexpected
	// condition occcurs while instantiating the group manager
	ErrGroupManagerInstantiating = NewErrAdapter("group-manager-instantiating", nil, nil)
)
