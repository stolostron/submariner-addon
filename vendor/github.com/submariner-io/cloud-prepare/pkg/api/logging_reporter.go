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
package api

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/errors"
)

type loggingReporter struct{}

func NewLoggingReporter() Reporter {
	return &loggingReporter{}
}

func (r *loggingReporter) Started(message string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(message, args...))
}

func (r *loggingReporter) Succeeded(message string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(message, args...))
}

func (r *loggingReporter) Failed(errs ...error) {
	fmt.Println(errors.NewAggregate(errs).Error())
}
