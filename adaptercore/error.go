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
	"strings"
)

// ErrFields specifies as set of name/values pairs to be associated with error
type ErrFields map[string]interface{}

func (f ErrFields) copy() ErrFields {
	dst := make(ErrFields)
	for k, v := range f {
		dst[k] = v
	}
	return dst
}

func (f ErrFields) merge(src ErrFields) ErrFields {
	dst := make(ErrFields)
	for k, v := range f {
		dst[k] = v
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ErrAdapter represents a basic adapter error that combines an name, field set
// and wrapped error
type ErrAdapter struct {
	name    string
	fields  ErrFields
	wrapped error
}

// NewErrAdapter constructs a new error with the given values
func NewErrAdapter(name string, fields ErrFields, wrapped error) *ErrAdapter {
	return &ErrAdapter{
		name:    name,
		fields:  fields.copy(),
		wrapped: wrapped,
	}
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
type ErrInvalidValue ErrAdapter

// NewErrInvalidValue constructs a new error based on the given values
func NewErrInvalidValue(fields ErrFields, wrapped error) *ErrInvalidValue {
	return &ErrInvalidValue{
		name:    "invalid-value",
		fields:  fields.copy(),
		wrapped: wrapped,
	}
}

// Error returns a string representation of the error
func (e *ErrInvalidValue) Error() string {
	return (*ErrAdapter)(e).Error()
}

// ErrNotFound represents an error condition when a value can not be located
// given a field set of criteria
type ErrNotFound ErrAdapter

// NewErrNotFound constructs a new error based on the given values
func NewErrNotFound(target string, fields ErrFields, wrapped error) *ErrNotFound {
	return &ErrNotFound{
		name:    "not-found",
		fields:  fields.merge(ErrFields{"target": target}),
		wrapped: wrapped,
	}
}

// Error returns a string representation of the error
func (e *ErrNotFound) Error() string {
	return (*ErrAdapter)(e).Error()
}

// ErrPersistence representation an error condition when a persistence operation
// did not succeed
type ErrPersistence ErrAdapter

// NewErrPersistence constructs a new error based on the given values
func NewErrPersistence(operation, entityType string, ID uint32, fields ErrFields, wrapped error) *ErrPersistence {
	return &ErrPersistence{
		name: "unable-to-persist",
		fields: fields.merge(ErrFields{
			"operation":   operation,
			"entity-type": entityType,
			"id":          fmt.Sprintf("0x%x", ID)}),
		wrapped: wrapped,
	}
}

// Error returns a string representation of the error
func (e *ErrPersistence) Error() string {
	return (*ErrAdapter)(e).Error()
}

// ErrFlowOp represents an error condition when a flow operation to a device did
// not succeed
type ErrFlowOp ErrAdapter

// NewErrFlowOp constructs a new error based on the given values
func NewErrFlowOp(operation string, ID uint32, fields ErrFields, wrapped error) *ErrPersistence {
	return &ErrPersistence{
		name: "unable-to-perform-flow-operation",
		fields: fields.merge(ErrFields{
			"operation": operation,
			"id":        fmt.Sprintf("0x%x", ID)}),
		wrapped: wrapped,
	}
}

// Error returns a string representation of the error
func (e *ErrFlowOp) Error() string {
	return (*ErrAdapter)(e).Error()
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
