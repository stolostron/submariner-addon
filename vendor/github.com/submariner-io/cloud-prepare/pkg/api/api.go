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
package api

// Reporter is responsible for reporting back on the progress of the cloud preparation
type Reporter interface {
	// Started will report that an operation started on the cloud
	Started(message string, args ...interface{})

	// Succeeded will report that the last operation on the cloud has succeeded
	Succeeded(message string, args ...interface{})

	// Failed will report that the last operation on the cloud has failed
	Failed(errs ...error)
}

// PortSpec is a specification of port+protocol to open
type PortSpec struct {
	Port     uint16
	Protocol string
}

type PrepareForSubmarinerInput struct {
	// List of ports to open inside the cluster for proper communication between Submariner services
	InternalPorts []PortSpec

	// List of ports to open externally so that Submariner can reach and be reached by other Submariners
	PublicPorts []PortSpec

	// Amount of gateways that are being deployed
	//
	// 0 = Deploy one gateway per public subnet (Default if not specified)
	//
	// 1-* = Deploy the amount of gateways requested (May fail if there aren't enough public subnets)
	Gateways int
}

// Cloud is a potential cloud for installing Submariner on
type Cloud interface {
	// PrepareForSubmariner will prepare the cloud for Submariner to operate on
	PrepareForSubmariner(input PrepareForSubmarinerInput, reporter Reporter) error

	// CleanupAfterSubmariner will clean up the cloud after Submariner is removed
	CleanupAfterSubmariner(reporter Reporter) error
}
