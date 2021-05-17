// +build !ignore_autogenerated

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWS) DeepCopyInto(out *AWS) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWS.
func (in *AWS) DeepCopy() *AWS {
	if in == nil {
		return nil
	}
	out := new(AWS)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GatewayConfig) DeepCopyInto(out *GatewayConfig) {
	*out = *in
	out.AWS = in.AWS
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GatewayConfig.
func (in *GatewayConfig) DeepCopy() *GatewayConfig {
	if in == nil {
		return nil
	}
	out := new(GatewayConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagedClusterInfo) DeepCopyInto(out *ManagedClusterInfo) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagedClusterInfo.
func (in *ManagedClusterInfo) DeepCopy() *ManagedClusterInfo {
	if in == nil {
		return nil
	}
	out := new(ManagedClusterInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubmarinerConfig) DeepCopyInto(out *SubmarinerConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubmarinerConfig.
func (in *SubmarinerConfig) DeepCopy() *SubmarinerConfig {
	if in == nil {
		return nil
	}
	out := new(SubmarinerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SubmarinerConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubmarinerConfigList) DeepCopyInto(out *SubmarinerConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SubmarinerConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubmarinerConfigList.
func (in *SubmarinerConfigList) DeepCopy() *SubmarinerConfigList {
	if in == nil {
		return nil
	}
	out := new(SubmarinerConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SubmarinerConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubmarinerConfigSpec) DeepCopyInto(out *SubmarinerConfigSpec) {
	*out = *in
	if in.CredentialsSecret != nil {
		in, out := &in.CredentialsSecret, &out.CredentialsSecret
		*out = new(v1.LocalObjectReference)
		**out = **in
	}
	out.SubscriptionConfig = in.SubscriptionConfig
	out.ImagePullSpecs = in.ImagePullSpecs
	out.GatewayConfig = in.GatewayConfig
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubmarinerConfigSpec.
func (in *SubmarinerConfigSpec) DeepCopy() *SubmarinerConfigSpec {
	if in == nil {
		return nil
	}
	out := new(SubmarinerConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubmarinerConfigStatus) DeepCopyInto(out *SubmarinerConfigStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.ManagedClusterInfo = in.ManagedClusterInfo
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubmarinerConfigStatus.
func (in *SubmarinerConfigStatus) DeepCopy() *SubmarinerConfigStatus {
	if in == nil {
		return nil
	}
	out := new(SubmarinerConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubmarinerImagePullSpecs) DeepCopyInto(out *SubmarinerImagePullSpecs) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubmarinerImagePullSpecs.
func (in *SubmarinerImagePullSpecs) DeepCopy() *SubmarinerImagePullSpecs {
	if in == nil {
		return nil
	}
	out := new(SubmarinerImagePullSpecs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubscriptionConfig) DeepCopyInto(out *SubscriptionConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubscriptionConfig.
func (in *SubscriptionConfig) DeepCopy() *SubscriptionConfig {
	if in == nil {
		return nil
	}
	out := new(SubscriptionConfig)
	in.DeepCopyInto(out)
	return out
}
