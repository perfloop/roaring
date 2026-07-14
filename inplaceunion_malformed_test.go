package roaring

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunContainer16InplaceUnionDeserializedReceiverNormalizes(t *testing.T) {
	t.Run("SingleIncomingRun", func(t *testing.T) {
		receiver := malformedInplaceUnionReceiver(t)
		receiver.Or(inPlaceUnionIncomingBitmap([]interval16{newInterval16Range(30, 30)}))

		want := appendInplaceUnionRange(nil, 10, 20)
		want = appendInplaceUnionRange(want, 30, 30)
		want = appendInplaceUnionRange(want, 50, 60)
		assertInplaceUnionBitmap(t, receiver, want)

		receiver.Or(inPlaceUnionIncomingBitmap([]interval16{newInterval16Range(65, 65)}))
		want = appendInplaceUnionRange(want, 65, 65)
		assertInplaceUnionBitmap(t, receiver, want)
	})

	t.Run("MultipleIncomingRuns", func(t *testing.T) {
		receiver := malformedInplaceUnionReceiver(t)
		receiver.Or(inPlaceUnionIncomingBitmap([]interval16{
			newInterval16Range(70, 72),
			newInterval16Range(80, 82),
		}))

		want := appendInplaceUnionRange(nil, 10, 20)
		want = appendInplaceUnionRange(want, 50, 60)
		want = appendInplaceUnionRange(want, 70, 72)
		want = appendInplaceUnionRange(want, 80, 82)
		assertInplaceUnionBitmap(t, receiver, want)
	})

	t.Run("FastOr", func(t *testing.T) {
		receiver := malformedInplaceUnionReceiver(t)
		result := FastOr(receiver, inPlaceUnionIncomingBitmap([]interval16{newInterval16Range(30, 30)}))
		require.Error(t, receiver.Validate())

		want := appendInplaceUnionRange(nil, 10, 20)
		want = appendInplaceUnionRange(want, 30, 30)
		want = appendInplaceUnionRange(want, 50, 60)
		assertInplaceUnionBitmap(t, result, want)
	})
}

func TestRunContainer16InplaceUnionDeserializedWrappedRunNormalizes(t *testing.T) {
	encoded := NewBitmap()
	encoded.highlowcontainer.appendContainer(0, &runContainer16{iv: []interval16{
		{start: 65530, length: 20},
		newInterval16Range(500, 510),
	}}, false)
	serialized, err := encoded.ToBytes()
	require.NoError(t, err)

	receiver := NewBitmap()
	_, err = receiver.ReadFrom(bytes.NewReader(serialized))
	require.NoError(t, err)
	receiver.Or(inPlaceUnionIncomingBitmap([]interval16{newInterval16Range(600, 600)}))

	want := appendInplaceUnionRange(nil, 500, 510)
	want = appendInplaceUnionRange(want, 600, 600)
	require.Equal(t, want, receiver.ToArray())
	require.NoError(t, receiver.Validate())
}

func malformedInplaceUnionReceiver(t *testing.T) *Bitmap {
	t.Helper()

	encoded := NewBitmap()
	encoded.highlowcontainer.appendContainer(0, &runContainer16{iv: []interval16{
		newInterval16Range(10, 20),
		newInterval16Range(50, 60),
	}}, false)
	serialized, err := encoded.ToBytes()
	require.NoError(t, err)
	copy(serialized[len(serialized)-8:], []byte{50, 0, 10, 0, 10, 0, 10, 0})

	receiver := NewBitmap()
	_, err = receiver.ReadFrom(bytes.NewReader(serialized))
	require.NoError(t, err)
	return receiver
}

func inPlaceUnionIncomingBitmap(intervals []interval16) *Bitmap {
	bitmap := NewBitmap()
	bitmap.highlowcontainer.appendContainer(0, &runContainer16{iv: intervals}, false)
	return bitmap
}

func assertInplaceUnionBitmap(t *testing.T, bitmap *Bitmap, want []uint32) {
	t.Helper()

	require.True(t, bitmap.Contains(55))
	require.Equal(t, want, bitmap.ToArray())
	require.NoError(t, bitmap.Validate())
}

func appendInplaceUnionRange(values []uint32, first, last uint32) []uint32 {
	for value := first; value <= last; value++ {
		values = append(values, value)
	}
	return values
}
