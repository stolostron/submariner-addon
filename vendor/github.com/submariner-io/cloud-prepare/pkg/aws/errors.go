/*
© 2021 Red Hat, Inc. and others.

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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
)

type notFoundError struct {
	s string
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("%s not found", e.s)
}

func newNotFoundError(msg string, args ...interface{}) error {
	return &notFoundError{fmt.Sprintf(msg, args...)}
}

func isNotFoundError(err error) bool {
	_, ok := err.(*notFoundError)
	return ok
}

type compositeError struct {
	errs []error
}

func (e *compositeError) Error() string {
	errStrings := make([]string, 0, len(e.errs))

	for _, err := range e.errs {
		fmt.Println()

		errStrings = append(errStrings, err.Error())
	}

	return fmt.Sprintf("Encountered %v errors: %s", len(e.errs), strings.Join(errStrings, "; "))
}

func newCompositeError(errs ...error) error {
	return &compositeError{errs: errs}
}

func appendIfError(errs []error, err error) []error {
	if err == nil {
		return errs
	}

	return append(errs, err)
}

func isAWSError(err error, code string) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		// Has to be checked as string, see https://github.com/aws/aws-sdk-go/issues/3235
		return awsErr.Code() == code
	}

	return false
}
