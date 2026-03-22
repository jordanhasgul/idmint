package idmint_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/jordanhasgul/idmint"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMinter(t *testing.T) {
	t.Parallel()

	t.Run("minimum valid worker id", func(t *testing.T) {
		t.Parallel()

		minter, err := idmint.NewMinter(0)
		require.NoError(t, err)
		assert.NotNil(t, minter)
	})

	t.Run("maximum valid worker id", func(t *testing.T) {
		t.Parallel()

		const maxWorkerID = uint64(1<<10 - 1)

		minter, err := idmint.NewMinter(maxWorkerID)
		require.NoError(t, err)
		assert.NotNil(t, minter)
	})

	t.Run("invalid worker id", func(t *testing.T) {
		t.Parallel()

		const maxWorkerIDPlusOne = uint64(1 << 10)

		minter, err := idmint.NewMinter(maxWorkerIDPlusOne)
		require.Error(t, err)

		var widtle *idmint.WorkerIDTooLargeError
		assert.ErrorAs(t, err, &widtle)
		assert.Nil(t, minter)
	})
}

type controllableClock struct {
	now time.Time
}

var _ idmint.Clock = (*controllableClock)(nil)

func (c *controllableClock) Now() time.Time {
	return c.now
}

func (c *controllableClock) SetTimeTo(time time.Time) {
	c.now = time
}

func (c *controllableClock) MoveTimeBy(duration time.Duration) {
	c.now = c.now.Add(duration)
}

func TestMinter_Mint(t *testing.T) {
	t.Parallel()

	t.Run("id values increase when minted at the same point in time", func(t *testing.T) {
		t.Parallel()

		clock := &controllableClock{
			now: time.Now(),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		id1, err := minter.Mint()
		require.NoError(t, err)
		require.NotZero(t, id1)

		id2, err := minter.Mint()
		require.NoError(t, err)
		require.NotZero(t, id2)

		id3, err := minter.Mint()
		require.NoError(t, err)
		require.NotZero(t, id3)

		assert.Greater(t, id3, id2)
		assert.Greater(t, id2, id1)
	})

	t.Run("id values increase as time moves forward", func(t *testing.T) {
		t.Parallel()

		clock := &controllableClock{
			now: time.Now(),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		id1, err := minter.Mint()
		require.NoError(t, err)
		require.NotZero(t, id1)

		clock.MoveTimeBy(time.Millisecond)

		id2, err := minter.Mint()
		require.NoError(t, err)
		require.NotZero(t, id2)

		clock.MoveTimeBy(time.Millisecond)

		id3, err := minter.Mint()
		require.NoError(t, err)
		require.NotZero(t, id3)

		assert.Greater(t, id3, id2)
		assert.Greater(t, id2, id1)
	})

	t.Run("cannot mint when time moves backwards", func(t *testing.T) {
		t.Parallel()

		startOfMintingTime := time.Now()
		clock := &controllableClock{
			now: startOfMintingTime.Add(1 * time.Millisecond),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
			idmint.WithStartOfMintingTime(startOfMintingTime),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		id1, err := minter.Mint()
		require.NoError(t, err)
		require.NotZero(t, id1)

		clock.MoveTimeBy(-1 * time.Millisecond)

		id2, err := minter.Mint()
		require.Error(t, err)

		var tmbe *idmint.TimeMovedBackwardsError
		assert.ErrorAs(t, err, &tmbe)
		assert.Zero(t, id2)
	})

	t.Run("cannot mint when sequence number overflows", func(t *testing.T) {
		t.Parallel()

		clock := &controllableClock{
			now: time.Now(),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		for range 1 << 12 {
			id, err := minter.Mint()
			require.NoError(t, err)
			require.NotZero(t, id)
		}

		id, err := minter.Mint()
		require.Error(t, err)

		var sntle *idmint.SequenceNumberTooLargeError
		assert.ErrorAs(t, err, &sntle)
		assert.Zero(t, id)
	})

	t.Run("sequence resets when time advances", func(t *testing.T) {
		t.Parallel()

		clock := &controllableClock{
			now: time.Now(),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		for range 1 << 12 {
			id, err := minter.Mint()
			require.NoError(t, err)
			require.NotZero(t, id)
		}

		_, err = minter.Mint()
		require.Error(t, err)

		var sntle *idmint.SequenceNumberTooLargeError
		require.ErrorAs(t, err, &sntle)

		clock.MoveTimeBy(time.Millisecond)

		id, err := minter.Mint()
		require.NoError(t, err)
		assert.NotZero(t, id)
	})

	t.Run("can mint at exactly the start of minting time", func(t *testing.T) {
		t.Parallel()

		startOfMintingTime := time.Now()
		clock := &controllableClock{
			now: startOfMintingTime,
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
			idmint.WithStartOfMintingTime(startOfMintingTime),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		_, err = minter.Mint()
		assert.NoError(t, err)
	})

	t.Run("cannot mint when time is before the start of time", func(t *testing.T) {
		t.Parallel()

		startOfMintingTime := time.Now()
		clock := &controllableClock{
			now: startOfMintingTime.Add(-1 * time.Millisecond),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
			idmint.WithStartOfMintingTime(startOfMintingTime),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		id, err := minter.Mint()
		assert.Error(t, err)

		var ctbsomte *idmint.CurrentTimeBeforeStartOfMintingTimeError
		assert.ErrorAs(t, err, &ctbsomte)
		assert.Zero(t, id)
	})

	t.Run("can mint at exactly the end of minting time", func(t *testing.T) {
		t.Parallel()

		const maxTimestamp = (1 << 42) - 1

		startOfMintingTime := time.Now()
		clock := &controllableClock{
			now: startOfMintingTime.Add(maxTimestamp * time.Millisecond),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
			idmint.WithStartOfMintingTime(startOfMintingTime),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		_, err = minter.Mint()
		assert.NoError(t, err)
	})

	t.Run("cannot mint when time is after the end of time", func(t *testing.T) {
		t.Parallel()

		const maxTimestampPlusOne = 1 << 42

		startOfMintingTime := time.Now()
		clock := &controllableClock{
			now: startOfMintingTime.Add(maxTimestampPlusOne * time.Millisecond),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithClock(clock),
			idmint.WithStartOfMintingTime(startOfMintingTime),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		id, err := minter.Mint()
		assert.Error(t, err)

		var ctaeomte *idmint.CurrentTimeAfterEndOfMintingTimeError
		assert.ErrorAs(t, err, &ctaeomte)
		assert.Zero(t, id)
	})

	t.Run("different worker ids produce different values", func(t *testing.T) {
		t.Parallel()

		clock := &controllableClock{
			now: time.Now(),
		}

		minter1, err := idmint.NewMinter(0, idmint.WithClock(clock))
		require.NoError(t, err)
		require.NotNil(t, minter1)

		minter2, err := idmint.NewMinter(1, idmint.WithClock(clock))
		require.NoError(t, err)
		require.NotNil(t, minter2)

		id1, err := minter1.Mint()
		require.NoError(t, err)
		require.NotZero(t, id1)

		id2, err := minter2.Mint()
		require.NoError(t, err)
		require.NotZero(t, id2)

		assert.NotEqual(t, id1, id2)
	})
}

func BenchmarkMinter_Mint(b *testing.B) {
	b.ReportAllocs()

	minter, err := idmint.NewMinter(0)
	require.NoError(b, err)
	require.NotNil(b, minter)

	id, err := minter.Mint()
	require.NoError(b, err)
	require.NotZero(b, id)

	b.ResetTimer()
	for b.Loop() {
		id, _ = minter.Mint()
	}

	runtime.KeepAlive(id)
}
