/*
 * HCS API
 *
 * No description provided (generated by Swagger Codegen https://github.com/swagger-api/swagger-codegen)
 *
 * API version: 2.1
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package hcsschema

type DeviceType string

const (
	ClassGUID        DeviceType = "ClassGuid"
	DeviceInstanceID DeviceType = "DeviceInstance"
	GPUMirror        DeviceType = "GpuMirror"
)

type Device struct {
	//  The type of device to assign to the container.
	Type DeviceType `json:"Type,omitempty"`
	//  The interface class guid of the device interfaces to assign to the  container.  Only used when Type is ClassGuid.
	InterfaceClassGuid string `json:"InterfaceClassGuid,omitempty"`
	//  The location path of the device to assign to the container.  Only used when Type is DeviceInstanceID.
	LocationPath string `json:"LocationPath,omitempty"`
}
