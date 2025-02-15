// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

// AWSPlatformStatusApplyConfiguration represents a declarative configuration of the AWSPlatformStatus type for use
// with apply.
type AWSPlatformStatusApplyConfiguration struct {
	Region                  *string                                    `json:"region,omitempty"`
	ServiceEndpoints        []AWSServiceEndpointApplyConfiguration     `json:"serviceEndpoints,omitempty"`
	ResourceTags            []AWSResourceTagApplyConfiguration         `json:"resourceTags,omitempty"`
	CloudLoadBalancerConfig *CloudLoadBalancerConfigApplyConfiguration `json:"cloudLoadBalancerConfig,omitempty"`
}

// AWSPlatformStatusApplyConfiguration constructs a declarative configuration of the AWSPlatformStatus type for use with
// apply.
func AWSPlatformStatus() *AWSPlatformStatusApplyConfiguration {
	return &AWSPlatformStatusApplyConfiguration{}
}

// WithRegion sets the Region field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Region field is set to the value of the last call.
func (b *AWSPlatformStatusApplyConfiguration) WithRegion(value string) *AWSPlatformStatusApplyConfiguration {
	b.Region = &value
	return b
}

// WithServiceEndpoints adds the given value to the ServiceEndpoints field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the ServiceEndpoints field.
func (b *AWSPlatformStatusApplyConfiguration) WithServiceEndpoints(values ...*AWSServiceEndpointApplyConfiguration) *AWSPlatformStatusApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithServiceEndpoints")
		}
		b.ServiceEndpoints = append(b.ServiceEndpoints, *values[i])
	}
	return b
}

// WithResourceTags adds the given value to the ResourceTags field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the ResourceTags field.
func (b *AWSPlatformStatusApplyConfiguration) WithResourceTags(values ...*AWSResourceTagApplyConfiguration) *AWSPlatformStatusApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithResourceTags")
		}
		b.ResourceTags = append(b.ResourceTags, *values[i])
	}
	return b
}

// WithCloudLoadBalancerConfig sets the CloudLoadBalancerConfig field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CloudLoadBalancerConfig field is set to the value of the last call.
func (b *AWSPlatformStatusApplyConfiguration) WithCloudLoadBalancerConfig(value *CloudLoadBalancerConfigApplyConfiguration) *AWSPlatformStatusApplyConfiguration {
	b.CloudLoadBalancerConfig = value
	return b
}
