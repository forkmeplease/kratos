/*
 * Ory Identities API
 *
 * This is the API specification for Ory Identities with features such as registration, login, recovery, account verification, profile settings, password reset, identity management, session management, email and sms delivery, and more.
 *
 * API version:
 * Contact: office@ory.sh
 */

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package client

import (
	"encoding/json"
)

// UiNode Nodes are represented as HTML elements or their native UI equivalents. For example, a node can be an `<img>` tag, or an `<input element>` but also `some plain text`.
type UiNode struct {
	Attributes UiNodeAttributes `json:"attributes"`
	// Group specifies which group (e.g. password authenticator) this node belongs to. default DefaultGroup password PasswordGroup oidc OpenIDConnectGroup profile ProfileGroup link LinkGroup code CodeGroup totp TOTPGroup lookup_secret LookupGroup webauthn WebAuthnGroup passkey PasskeyGroup identifier_first IdentifierFirstGroup captcha CaptchaGroup saml SAMLGroup
	Group    string     `json:"group"`
	Messages []UiText   `json:"messages"`
	Meta     UiNodeMeta `json:"meta"`
	// The node's type text Text input Input img Image a Anchor script Script div Division
	Type string `json:"type"`
}

// NewUiNode instantiates a new UiNode object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewUiNode(attributes UiNodeAttributes, group string, messages []UiText, meta UiNodeMeta, type_ string) *UiNode {
	this := UiNode{}
	this.Attributes = attributes
	this.Group = group
	this.Messages = messages
	this.Meta = meta
	this.Type = type_
	return &this
}

// NewUiNodeWithDefaults instantiates a new UiNode object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewUiNodeWithDefaults() *UiNode {
	this := UiNode{}
	return &this
}

// GetAttributes returns the Attributes field value
func (o *UiNode) GetAttributes() UiNodeAttributes {
	if o == nil {
		var ret UiNodeAttributes
		return ret
	}

	return o.Attributes
}

// GetAttributesOk returns a tuple with the Attributes field value
// and a boolean to check if the value has been set.
func (o *UiNode) GetAttributesOk() (*UiNodeAttributes, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Attributes, true
}

// SetAttributes sets field value
func (o *UiNode) SetAttributes(v UiNodeAttributes) {
	o.Attributes = v
}

// GetGroup returns the Group field value
func (o *UiNode) GetGroup() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Group
}

// GetGroupOk returns a tuple with the Group field value
// and a boolean to check if the value has been set.
func (o *UiNode) GetGroupOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Group, true
}

// SetGroup sets field value
func (o *UiNode) SetGroup(v string) {
	o.Group = v
}

// GetMessages returns the Messages field value
func (o *UiNode) GetMessages() []UiText {
	if o == nil {
		var ret []UiText
		return ret
	}

	return o.Messages
}

// GetMessagesOk returns a tuple with the Messages field value
// and a boolean to check if the value has been set.
func (o *UiNode) GetMessagesOk() ([]UiText, bool) {
	if o == nil {
		return nil, false
	}
	return o.Messages, true
}

// SetMessages sets field value
func (o *UiNode) SetMessages(v []UiText) {
	o.Messages = v
}

// GetMeta returns the Meta field value
func (o *UiNode) GetMeta() UiNodeMeta {
	if o == nil {
		var ret UiNodeMeta
		return ret
	}

	return o.Meta
}

// GetMetaOk returns a tuple with the Meta field value
// and a boolean to check if the value has been set.
func (o *UiNode) GetMetaOk() (*UiNodeMeta, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Meta, true
}

// SetMeta sets field value
func (o *UiNode) SetMeta(v UiNodeMeta) {
	o.Meta = v
}

// GetType returns the Type field value
func (o *UiNode) GetType() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Type
}

// GetTypeOk returns a tuple with the Type field value
// and a boolean to check if the value has been set.
func (o *UiNode) GetTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Type, true
}

// SetType sets field value
func (o *UiNode) SetType(v string) {
	o.Type = v
}

func (o UiNode) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if true {
		toSerialize["attributes"] = o.Attributes
	}
	if true {
		toSerialize["group"] = o.Group
	}
	if true {
		toSerialize["messages"] = o.Messages
	}
	if true {
		toSerialize["meta"] = o.Meta
	}
	if true {
		toSerialize["type"] = o.Type
	}
	return json.Marshal(toSerialize)
}

type NullableUiNode struct {
	value *UiNode
	isSet bool
}

func (v NullableUiNode) Get() *UiNode {
	return v.value
}

func (v *NullableUiNode) Set(val *UiNode) {
	v.value = val
	v.isSet = true
}

func (v NullableUiNode) IsSet() bool {
	return v.isSet
}

func (v *NullableUiNode) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableUiNode(val *UiNode) *NullableUiNode {
	return &NullableUiNode{value: val, isSet: true}
}

func (v NullableUiNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableUiNode) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
