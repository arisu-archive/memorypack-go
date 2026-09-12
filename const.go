package memorypack

// MemoryPack format constants.
const (
	// Format version.
	MemoryPackFormatVersion byte = 0x07

	// Collection header constants.
	NullCollection int32 = -1 // 0xFFFFFFFF

	// Object header constants.
	WideTag     byte = 250 // For Union, wide tag
	ReferenceID byte = 250 // For circular references
	Reserved1   byte = 250
	Reserved2   byte = 251
	Reserved3   byte = 252
	Reserved4   byte = 253
	Reserved5   byte = 254
	NullObject  byte = 255 // 0xFF

	// Depth constants.
	MaxDepth = 1000

	// DefaultCollectionLimit bounds element-wise decoding work. A Reader's
	// CollectionLimit can be raised for larger trusted collections. Byte slices
	// are instead bounded by the input buffer because they need no element loop.
	DefaultCollectionLimit = 1 << 20
)

const (
	int16Size             = 2
	int32Size             = 4
	int64Size             = 8
	bitsPerByte           = 8
	initialWriterCapacity = 128
)
