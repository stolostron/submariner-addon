/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package aws

import (
	"errors"
	"fmt"

	"github.com/aws/smithy-go"
)

type notFoundError struct {
	s string
}

func (e notFoundError) Error() string {
	return fmt.Sprintf("%s not found", e.s)
}

func newNotFoundError(msg string, args ...interface{}) error {
	return notFoundError{fmt.Sprintf(msg, args...)}
}

func isNotFoundError(err error) bool {
	var e notFoundError
	return errors.As(err, &e)
}

func appendIfError(errs []error, err error) []error {
	if err == nil {
		return errs
	}

	return append(errs, err)
}

func isAWSError(err error, code string) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		// Has to be checked as string, see https://github.com/aws/aws-sdk-go/issues/3235
		return apiErr.ErrorCode() == code
	}

	return false
}
