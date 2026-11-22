// Package goverter is the project-wide whitelist of conversion helpers: each
// var re-exports one function, and every Convert pulls the whole set with a
// single `// goverter:extend {{ .Project }}/pkg/goverter:.*`. Add a var here to
// share a new helper. See docs/conventions/dto-conversion.md for overrides.
package goverter

import (
	"github.com/ribora/convert/numeric"
	"github.com/ribora/convert/proto"
	"github.com/ribora/convert/temporal"
)

// Temporal — project default is RFC3339 (no nanoseconds).
var (
	StringToTime       = temporal.StringToTime
	TimeToString       = temporal.TimeToString
	StringPtrToTimePtr = temporal.StringPtrToTimePtr
	TimePtrToStringPtr = temporal.TimePtrToStringPtr

	Int64ToTime       = temporal.Int64ToTime
	TimeToInt64       = temporal.TimeToInt64
	Int64MillisToTime = temporal.Int64MillisToTime
	TimeToInt64Millis = temporal.TimeToInt64Millis
	Int64PtrToTimePtr = temporal.Int64PtrToTimePtr
	TimePtrToInt64Ptr = temporal.TimePtrToInt64Ptr
)

// Numeric
var (
	IntToInt64         = numeric.IntToInt64
	Int64ToInt         = numeric.Int64ToInt
	Int32ToInt64       = numeric.Int32ToInt64
	Int64ToInt32       = numeric.Int64ToInt32
	IntToInt32         = numeric.IntToInt32
	Int32ToInt         = numeric.Int32ToInt
	IntPtrToInt64Ptr   = numeric.IntPtrToInt64Ptr
	Int64PtrToIntPtr   = numeric.Int64PtrToIntPtr
	Int32PtrToInt64Ptr = numeric.Int32PtrToInt64Ptr
	Int64PtrToInt32Ptr = numeric.Int64PtrToInt32Ptr
	IntPtrToInt32Ptr   = numeric.IntPtrToInt32Ptr
	Int32PtrToIntPtr   = numeric.Int32PtrToIntPtr
)

// Protobuf well-known types
var (
	TimestampToTime       = proto.TimestampToTime
	TimeToTimestamp       = proto.TimeToTimestamp
	TimestampPtrToTimePtr = proto.TimestampPtrToTimePtr
	TimePtrToTimestampPtr = proto.TimePtrToTimestampPtr

	DurationToDuration       = proto.DurationToDuration
	DurationToProto          = proto.DurationToProto
	DurationPtrToDurationPtr = proto.DurationPtrToDurationPtr
	DurationPtrToProtoPtr    = proto.DurationPtrToProtoPtr

	StringValueToString    = proto.StringValueToString
	StringToStringValue    = proto.StringToStringValue
	StringValueToStringPtr = proto.StringValueToStringPtr
	StringPtrToStringValue = proto.StringPtrToStringValue

	Int32ValueToInt32    = proto.Int32ValueToInt32
	Int32ToInt32Value    = proto.Int32ToInt32Value
	Int64ValueToInt64    = proto.Int64ValueToInt64
	Int64ToInt64Value    = proto.Int64ToInt64Value
	BoolValueToBool      = proto.BoolValueToBool
	BoolToBoolValue      = proto.BoolToBoolValue
	DoubleValueToFloat64 = proto.DoubleValueToFloat64
	Float64ToDoubleValue = proto.Float64ToDoubleValue
)

// Project-specific helpers go below when needed (money, localized, etc.).
