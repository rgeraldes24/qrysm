// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.23.3
// source: proto/qrysm/v1alpha1/withdrawals.proto

package zond

import (
	reflect "reflect"
	sync "sync"

	github_com_theQRL_qrysm_v4_consensus_types_primitives "github.com/theQRL/qrysm/v4/consensus-types/primitives"
	_ "github.com/theQRL/qrysm/v4/proto/zond/ext"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type DilithiumToExecutionChange struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ValidatorIndex      github_com_theQRL_qrysm_v4_consensus_types_primitives.ValidatorIndex `protobuf:"varint,1,opt,name=validator_index,json=validatorIndex,proto3" json:"validator_index,omitempty" cast-type:"github.com/theQRL/qrysm/v4/consensus-types/primitives.ValidatorIndex"`
	FromDilithiumPubkey []byte                                                               `protobuf:"bytes,2,opt,name=from_dilithium_pubkey,json=fromDilithiumPubkey,proto3" json:"from_dilithium_pubkey,omitempty" ssz-size:"2592"`
	ToExecutionAddress  []byte                                                               `protobuf:"bytes,3,opt,name=to_execution_address,json=toExecutionAddress,proto3" json:"to_execution_address,omitempty" ssz-size:"20"`
}

func (x *DilithiumToExecutionChange) Reset() {
	*x = DilithiumToExecutionChange{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_prysm_v1alpha1_withdrawals_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DilithiumToExecutionChange) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DilithiumToExecutionChange) ProtoMessage() {}

func (x *DilithiumToExecutionChange) ProtoReflect() protoreflect.Message {
	mi := &file_proto_prysm_v1alpha1_withdrawals_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DilithiumToExecutionChange.ProtoReflect.Descriptor instead.
func (*DilithiumToExecutionChange) Descriptor() ([]byte, []int) {
	return file_proto_prysm_v1alpha1_withdrawals_proto_rawDescGZIP(), []int{0}
}

func (x *DilithiumToExecutionChange) GetValidatorIndex() github_com_theQRL_qrysm_v4_consensus_types_primitives.ValidatorIndex {
	if x != nil {
		return x.ValidatorIndex
	}
	return github_com_theQRL_qrysm_v4_consensus_types_primitives.ValidatorIndex(0)
}

func (x *DilithiumToExecutionChange) GetFromDilithiumPubkey() []byte {
	if x != nil {
		return x.FromDilithiumPubkey
	}
	return nil
}

func (x *DilithiumToExecutionChange) GetToExecutionAddress() []byte {
	if x != nil {
		return x.ToExecutionAddress
	}
	return nil
}

type SignedDilithiumToExecutionChange struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Message   *DilithiumToExecutionChange `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	Signature []byte                      `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty" ssz-size:"4595"`
}

func (x *SignedDilithiumToExecutionChange) Reset() {
	*x = SignedDilithiumToExecutionChange{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_prysm_v1alpha1_withdrawals_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignedDilithiumToExecutionChange) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignedDilithiumToExecutionChange) ProtoMessage() {}

func (x *SignedDilithiumToExecutionChange) ProtoReflect() protoreflect.Message {
	mi := &file_proto_prysm_v1alpha1_withdrawals_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignedDilithiumToExecutionChange.ProtoReflect.Descriptor instead.
func (*SignedDilithiumToExecutionChange) Descriptor() ([]byte, []int) {
	return file_proto_prysm_v1alpha1_withdrawals_proto_rawDescGZIP(), []int{1}
}

func (x *SignedDilithiumToExecutionChange) GetMessage() *DilithiumToExecutionChange {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *SignedDilithiumToExecutionChange) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

var File_proto_prysm_v1alpha1_withdrawals_proto protoreflect.FileDescriptor

var file_proto_prysm_v1alpha1_withdrawals_proto_rawDesc = []byte{
	0x0a, 0x26, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2f, 0x76, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x77, 0x69, 0x74, 0x68, 0x64, 0x72, 0x61, 0x77, 0x61,
	0x6c, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14, 0x74, 0x68, 0x65, 0x71, 0x72, 0x6c,
	0x2e, 0x7a, 0x6f, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x1a, 0x1c,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x7a, 0x6f, 0x6e, 0x64, 0x2f, 0x65, 0x78, 0x74, 0x2f, 0x6f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x87, 0x02, 0x0a,
	0x1a, 0x44, 0x69, 0x6c, 0x69, 0x74, 0x68, 0x69, 0x75, 0x6d, 0x54, 0x6f, 0x45, 0x78, 0x65, 0x63,
	0x75, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x12, 0x71, 0x0a, 0x0f, 0x76,
	0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x5f, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x04, 0x42, 0x48, 0x82, 0xb5, 0x18, 0x44, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74, 0x68, 0x65, 0x51, 0x52, 0x4c, 0x2f, 0x71, 0x72, 0x79, 0x73,
	0x6d, 0x2f, 0x76, 0x34, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x65, 0x6e, 0x73, 0x75, 0x73, 0x2d, 0x74,
	0x79, 0x70, 0x65, 0x73, 0x2f, 0x70, 0x72, 0x69, 0x6d, 0x69, 0x74, 0x69, 0x76, 0x65, 0x73, 0x2e,
	0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x52, 0x0e,
	0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x3c,
	0x0a, 0x15, 0x66, 0x72, 0x6f, 0x6d, 0x5f, 0x64, 0x69, 0x6c, 0x69, 0x74, 0x68, 0x69, 0x75, 0x6d,
	0x5f, 0x70, 0x75, 0x62, 0x6b, 0x65, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x08, 0x8a,
	0xb5, 0x18, 0x04, 0x32, 0x35, 0x39, 0x32, 0x52, 0x13, 0x66, 0x72, 0x6f, 0x6d, 0x44, 0x69, 0x6c,
	0x69, 0x74, 0x68, 0x69, 0x75, 0x6d, 0x50, 0x75, 0x62, 0x6b, 0x65, 0x79, 0x12, 0x38, 0x0a, 0x14,
	0x74, 0x6f, 0x5f, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x61, 0x64, 0x64,
	0x72, 0x65, 0x73, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x8a, 0xb5, 0x18, 0x02,
	0x32, 0x30, 0x52, 0x12, 0x74, 0x6f, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x41,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x22, 0x96, 0x01, 0x0a, 0x20, 0x53, 0x69, 0x67, 0x6e, 0x65,
	0x64, 0x44, 0x69, 0x6c, 0x69, 0x74, 0x68, 0x69, 0x75, 0x6d, 0x54, 0x6f, 0x45, 0x78, 0x65, 0x63,
	0x75, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x12, 0x4a, 0x0a, 0x07, 0x6d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x30, 0x2e, 0x74,
	0x68, 0x65, 0x71, 0x72, 0x6c, 0x2e, 0x7a, 0x6f, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x2e, 0x44, 0x69, 0x6c, 0x69, 0x74, 0x68, 0x69, 0x75, 0x6d, 0x54, 0x6f, 0x45,
	0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x52, 0x07,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x26, 0x0a, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61,
	0x74, 0x75, 0x72, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x08, 0x8a, 0xb5, 0x18, 0x04,
	0x34, 0x35, 0x39, 0x35, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x42,
	0x92, 0x01, 0x0a, 0x18, 0x6f, 0x72, 0x67, 0x2e, 0x74, 0x68, 0x65, 0x71, 0x72, 0x6c, 0x2e, 0x7a,
	0x6f, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x42, 0x10, 0x57, 0x69,
	0x74, 0x68, 0x64, 0x72, 0x61, 0x77, 0x61, 0x6c, 0x73, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01,
	0x5a, 0x34, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74, 0x68, 0x65,
	0x51, 0x52, 0x4c, 0x2f, 0x71, 0x72, 0x79, 0x73, 0x6d, 0x2f, 0x76, 0x34, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x3b, 0x7a, 0x6f, 0x6e, 0x64, 0xaa, 0x02, 0x14, 0x54, 0x68, 0x65, 0x51, 0x52, 0x4c, 0x2e,
	0x5a, 0x6f, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xca, 0x02, 0x14,
	0x54, 0x68, 0x65, 0x51, 0x52, 0x4c, 0x5c, 0x5a, 0x6f, 0x6e, 0x64, 0x5c, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_prysm_v1alpha1_withdrawals_proto_rawDescOnce sync.Once
	file_proto_prysm_v1alpha1_withdrawals_proto_rawDescData = file_proto_prysm_v1alpha1_withdrawals_proto_rawDesc
)

func file_proto_prysm_v1alpha1_withdrawals_proto_rawDescGZIP() []byte {
	file_proto_prysm_v1alpha1_withdrawals_proto_rawDescOnce.Do(func() {
		file_proto_prysm_v1alpha1_withdrawals_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_prysm_v1alpha1_withdrawals_proto_rawDescData)
	})
	return file_proto_prysm_v1alpha1_withdrawals_proto_rawDescData
}

var file_proto_prysm_v1alpha1_withdrawals_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto_prysm_v1alpha1_withdrawals_proto_goTypes = []interface{}{
	(*DilithiumToExecutionChange)(nil),       // 0: theqrl.zond.v1alpha1.DilithiumToExecutionChange
	(*SignedDilithiumToExecutionChange)(nil), // 1: theqrl.zond.v1alpha1.SignedDilithiumToExecutionChange
}
var file_proto_prysm_v1alpha1_withdrawals_proto_depIdxs = []int32{
	0, // 0: theqrl.zond.v1alpha1.SignedDilithiumToExecutionChange.message:type_name -> theqrl.zond.v1alpha1.DilithiumToExecutionChange
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_proto_prysm_v1alpha1_withdrawals_proto_init() }
func file_proto_prysm_v1alpha1_withdrawals_proto_init() {
	if File_proto_prysm_v1alpha1_withdrawals_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_prysm_v1alpha1_withdrawals_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DilithiumToExecutionChange); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_prysm_v1alpha1_withdrawals_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignedDilithiumToExecutionChange); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_prysm_v1alpha1_withdrawals_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto_prysm_v1alpha1_withdrawals_proto_goTypes,
		DependencyIndexes: file_proto_prysm_v1alpha1_withdrawals_proto_depIdxs,
		MessageInfos:      file_proto_prysm_v1alpha1_withdrawals_proto_msgTypes,
	}.Build()
	File_proto_prysm_v1alpha1_withdrawals_proto = out.File
	file_proto_prysm_v1alpha1_withdrawals_proto_rawDesc = nil
	file_proto_prysm_v1alpha1_withdrawals_proto_goTypes = nil
	file_proto_prysm_v1alpha1_withdrawals_proto_depIdxs = nil
}