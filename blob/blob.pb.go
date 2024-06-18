// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: blob/blob.proto

package blob

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Blob (named after binary large object) is a chunk of data submitted by a user
// to be published to the Celestia blockchain. The data of a Blob is published
// to a namespace and is encoded into shares based on the format specified by
// share_version.
type Blob struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NamespaceId      []byte `protobuf:"bytes,1,opt,name=namespace_id,json=namespaceId,proto3" json:"namespace_id,omitempty"`
	Data             []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	ShareVersion     uint32 `protobuf:"varint,3,opt,name=share_version,json=shareVersion,proto3" json:"share_version,omitempty"`
	NamespaceVersion uint32 `protobuf:"varint,4,opt,name=namespace_version,json=namespaceVersion,proto3" json:"namespace_version,omitempty"`
	// Signer is sdk.AccAddress that paid for this blob. This field is optional
	// and can only be used when share_version is set to 1.
	Signer []byte `protobuf:"bytes,5,opt,name=signer,proto3" json:"signer,omitempty"`
}

func (x *Blob) Reset() {
	*x = Blob{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blob_blob_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Blob) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Blob) ProtoMessage() {}

func (x *Blob) ProtoReflect() protoreflect.Message {
	mi := &file_blob_blob_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Blob.ProtoReflect.Descriptor instead.
func (*Blob) Descriptor() ([]byte, []int) {
	return file_blob_blob_proto_rawDescGZIP(), []int{0}
}

func (x *Blob) GetNamespaceId() []byte {
	if x != nil {
		return x.NamespaceId
	}
	return nil
}

func (x *Blob) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Blob) GetShareVersion() uint32 {
	if x != nil {
		return x.ShareVersion
	}
	return 0
}

func (x *Blob) GetNamespaceVersion() uint32 {
	if x != nil {
		return x.NamespaceVersion
	}
	return 0
}

func (x *Blob) GetSigner() []byte {
	if x != nil {
		return x.Signer
	}
	return nil
}

// BlobTx wraps an encoded sdk.Tx with a second field to contain blobs of data.
// The raw bytes of the blobs are not signed over, instead we verify each blob
// using the relevant MsgPayForBlobs that is signed over in the encoded sdk.Tx.
type BlobTx struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tx     []byte  `protobuf:"bytes,1,opt,name=tx,proto3" json:"tx,omitempty"`
	Blobs  []*Blob `protobuf:"bytes,2,rep,name=blobs,proto3" json:"blobs,omitempty"`
	TypeId string  `protobuf:"bytes,3,opt,name=type_id,json=typeId,proto3" json:"type_id,omitempty"`
}

func (x *BlobTx) Reset() {
	*x = BlobTx{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blob_blob_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BlobTx) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlobTx) ProtoMessage() {}

func (x *BlobTx) ProtoReflect() protoreflect.Message {
	mi := &file_blob_blob_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlobTx.ProtoReflect.Descriptor instead.
func (*BlobTx) Descriptor() ([]byte, []int) {
	return file_blob_blob_proto_rawDescGZIP(), []int{1}
}

func (x *BlobTx) GetTx() []byte {
	if x != nil {
		return x.Tx
	}
	return nil
}

func (x *BlobTx) GetBlobs() []*Blob {
	if x != nil {
		return x.Blobs
	}
	return nil
}

func (x *BlobTx) GetTypeId() string {
	if x != nil {
		return x.TypeId
	}
	return ""
}

// IndexWrapper adds index metadata to a transaction. This is used to track
// transactions that pay for blobs, and where the blobs start in the square.
type IndexWrapper struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tx           []byte   `protobuf:"bytes,1,opt,name=tx,proto3" json:"tx,omitempty"`
	ShareIndexes []uint32 `protobuf:"varint,2,rep,packed,name=share_indexes,json=shareIndexes,proto3" json:"share_indexes,omitempty"`
	TypeId       string   `protobuf:"bytes,3,opt,name=type_id,json=typeId,proto3" json:"type_id,omitempty"`
}

func (x *IndexWrapper) Reset() {
	*x = IndexWrapper{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blob_blob_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *IndexWrapper) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IndexWrapper) ProtoMessage() {}

func (x *IndexWrapper) ProtoReflect() protoreflect.Message {
	mi := &file_blob_blob_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IndexWrapper.ProtoReflect.Descriptor instead.
func (*IndexWrapper) Descriptor() ([]byte, []int) {
	return file_blob_blob_proto_rawDescGZIP(), []int{2}
}

func (x *IndexWrapper) GetTx() []byte {
	if x != nil {
		return x.Tx
	}
	return nil
}

func (x *IndexWrapper) GetShareIndexes() []uint32 {
	if x != nil {
		return x.ShareIndexes
	}
	return nil
}

func (x *IndexWrapper) GetTypeId() string {
	if x != nil {
		return x.TypeId
	}
	return ""
}

var File_blob_blob_proto protoreflect.FileDescriptor

var file_blob_blob_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x62, 0x6c, 0x6f, 0x62, 0x2f, 0x62, 0x6c, 0x6f, 0x62, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x08, 0x70, 0x6b, 0x67, 0x2e, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0xa7, 0x01, 0x0a, 0x04,
	0x42, 0x6c, 0x6f, 0x62, 0x12, 0x21, 0x0a, 0x0c, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x6e, 0x61, 0x6d, 0x65,
	0x73, 0x70, 0x61, 0x63, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x23, 0x0a, 0x0d, 0x73,
	0x68, 0x61, 0x72, 0x65, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0d, 0x52, 0x0c, 0x73, 0x68, 0x61, 0x72, 0x65, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e,
	0x12, 0x2b, 0x0a, 0x11, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x5f, 0x76, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x10, 0x6e, 0x61, 0x6d,
	0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x16, 0x0a,
	0x06, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x06, 0x73,
	0x69, 0x67, 0x6e, 0x65, 0x72, 0x22, 0x57, 0x0a, 0x06, 0x42, 0x6c, 0x6f, 0x62, 0x54, 0x78, 0x12,
	0x0e, 0x0a, 0x02, 0x74, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x74, 0x78, 0x12,
	0x24, 0x0a, 0x05, 0x62, 0x6c, 0x6f, 0x62, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0e,
	0x2e, 0x70, 0x6b, 0x67, 0x2e, 0x62, 0x6c, 0x6f, 0x62, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x05,
	0x62, 0x6c, 0x6f, 0x62, 0x73, 0x12, 0x17, 0x0a, 0x07, 0x74, 0x79, 0x70, 0x65, 0x5f, 0x69, 0x64,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x74, 0x79, 0x70, 0x65, 0x49, 0x64, 0x22, 0x5c,
	0x0a, 0x0c, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x57, 0x72, 0x61, 0x70, 0x70, 0x65, 0x72, 0x12, 0x0e,
	0x0a, 0x02, 0x74, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x74, 0x78, 0x12, 0x23,
	0x0a, 0x0d, 0x73, 0x68, 0x61, 0x72, 0x65, 0x5f, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x65, 0x73, 0x18,
	0x02, 0x20, 0x03, 0x28, 0x0d, 0x52, 0x0c, 0x73, 0x68, 0x61, 0x72, 0x65, 0x49, 0x6e, 0x64, 0x65,
	0x78, 0x65, 0x73, 0x12, 0x17, 0x0a, 0x07, 0x74, 0x79, 0x70, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x74, 0x79, 0x70, 0x65, 0x49, 0x64, 0x42, 0x27, 0x5a, 0x25,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x65, 0x6c, 0x65, 0x73,
	0x74, 0x69, 0x61, 0x6f, 0x72, 0x67, 0x2f, 0x67, 0x6f, 0x2d, 0x73, 0x71, 0x75, 0x61, 0x72, 0x65,
	0x2f, 0x62, 0x6c, 0x6f, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_blob_blob_proto_rawDescOnce sync.Once
	file_blob_blob_proto_rawDescData = file_blob_blob_proto_rawDesc
)

func file_blob_blob_proto_rawDescGZIP() []byte {
	file_blob_blob_proto_rawDescOnce.Do(func() {
		file_blob_blob_proto_rawDescData = protoimpl.X.CompressGZIP(file_blob_blob_proto_rawDescData)
	})
	return file_blob_blob_proto_rawDescData
}

var file_blob_blob_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_blob_blob_proto_goTypes = []any{
	(*Blob)(nil),         // 0: pkg.blob.Blob
	(*BlobTx)(nil),       // 1: pkg.blob.BlobTx
	(*IndexWrapper)(nil), // 2: pkg.blob.IndexWrapper
}
var file_blob_blob_proto_depIdxs = []int32{
	0, // 0: pkg.blob.BlobTx.blobs:type_name -> pkg.blob.Blob
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_blob_blob_proto_init() }
func file_blob_blob_proto_init() {
	if File_blob_blob_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_blob_blob_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Blob); i {
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
		file_blob_blob_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*BlobTx); i {
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
		file_blob_blob_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*IndexWrapper); i {
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
			RawDescriptor: file_blob_blob_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_blob_blob_proto_goTypes,
		DependencyIndexes: file_blob_blob_proto_depIdxs,
		MessageInfos:      file_blob_blob_proto_msgTypes,
	}.Build()
	File_blob_blob_proto = out.File
	file_blob_blob_proto_rawDesc = nil
	file_blob_blob_proto_goTypes = nil
	file_blob_blob_proto_depIdxs = nil
}
