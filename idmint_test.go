package idmint_test

import (
	"encoding/json"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/jordanhasgul/idmint"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewID(t *testing.T) {
	t.Parallel()

	t.Run("valid kind and value", func(t *testing.T) {
		t.Parallel()

		idKind := "user"
		idValue := "12345"

		id, err := idmint.NewID(idKind, idValue)
		require.NoError(t, err)

		assert.Equal(t, idKind, id.Kind())
		assert.Equal(t, idValue, id.Value())
	})

	testCases := []struct {
		name string

		kind  string
		value string

		expectedError error
	}{
		{
			name: "empty kind",

			kind:  "",
			value: "12345",

			expectedError: &idmint.IDKindEmptyError{},
		},
		{
			name: "kind with colons",

			kind:  "user:profile",
			value: "12345",

			expectedError: &idmint.IDKindContainsColonsError{},
		},
		{
			name: "empty value",

			kind:  "user",
			value: "",

			expectedError: &idmint.IDValueEmptyError{},
		},
		{
			name: "value with colons",

			kind:  "user",
			value: "value:with:underscore",

			expectedError: &idmint.IDValueContainsColonsError{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := idmint.NewID(testCase.kind, testCase.value)
			require.Error(t, err)
			assert.ErrorAs(t, err, &testCase.expectedError)
		})
	}
}

func TestParseID(t *testing.T) {
	t.Parallel()

	t.Run("valid id string", func(t *testing.T) {
		t.Parallel()

		idKind := "user"
		idValue := "12345"
		idAsString := idKind + ":" + idValue

		id, err := idmint.ParseID(idAsString)
		require.NoError(t, err)

		assert.Equal(t, idKind, id.Kind())
		assert.Equal(t, idValue, id.Value())
	})

	t.Run("invalid id string", func(t *testing.T) {
		t.Parallel()

		idKind := "user"
		idValue := "12345"
		idAsString := idKind + idValue

		_, err := idmint.ParseID(idAsString)
		require.Error(t, err)

		var ife *idmint.InvalidIDFormatError
		require.ErrorAs(t, err, &ife)
		assert.Equal(t, idAsString, ife.ID())
	})

	testCases := []struct {
		name string

		idAsString string

		expectedError error
	}{
		{
			name: "fails on empty kind part",

			idAsString: ":12345",

			expectedError: &idmint.IDKindEmptyError{},
		},
		{
			name: "fails on empty value part",

			idAsString: "user:",

			expectedError: &idmint.IDValueEmptyError{},
		},
		{
			name: "fails on value with extra underscores",

			idAsString: "user:value:with:underscores",

			expectedError: &idmint.IDValueContainsColonsError{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := idmint.ParseID(testCase.idAsString)
			require.Error(t, err)
			assert.ErrorAs(t, err, &testCase.expectedError)
		})
	}
}

func TestID_String(t *testing.T) {
	t.Parallel()

	t.Run("valid id", func(t *testing.T) {
		t.Parallel()

		idKind := "user"
		idValue := "12345"

		id, err := idmint.NewID(idKind, idValue)
		require.NoError(t, err)

		idAsString := id.Kind() + ":" + idValue
		assert.Equal(t, idAsString, id.String())
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()

		var id idmint.ID
		assert.Equal(t, "InvalidID", id.String())
	})
}

func TestID_Equal(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string

		id1 idmint.ID
		id2 idmint.ID

		expectedEqual bool
	}{
		{
			name: "equal zero ids",

			id1: idmint.ID{},
			id2: idmint.ID{},

			expectedEqual: true,
		},
		{
			name: "equal valid ids",

			id1: idmint.MustNewID("user", "12345"),
			id2: idmint.MustNewID("user", "12345"),

			expectedEqual: true,
		},
		{
			name: "valid ids, different kinds",

			id1: idmint.MustNewID("user", "12345"),
			id2: idmint.MustNewID("order", "12345"),

			expectedEqual: false,
		},
		{
			name: "valid ids, different values",

			id1: idmint.MustNewID("user", "12345"),
			id2: idmint.MustNewID("user", "67890"),

			expectedEqual: false,
		},
		{
			name: "valid id and invalid id",

			id1: idmint.MustNewID("user", "12345"),
			id2: idmint.ID{},

			expectedEqual: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			equal := testCase.id1.Equal(testCase.id2)
			assert.Equal(t, testCase.expectedEqual, equal)
		})
	}
}

func TestID_MarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("marshal a valid id", func(t *testing.T) {
		t.Parallel()

		idKind := "user"
		idValue := "12345"
		idAsString := idKind + ":" + idValue

		id, err := idmint.NewID(idKind, idValue)
		require.NoError(t, err)

		idAsJSON, err := json.Marshal(id)
		require.NoError(t, err)

		assert.JSONEq(t, `"`+idAsString+`"`, string(idAsJSON))
	})

	t.Run("marshal invalid id", func(t *testing.T) {
		t.Parallel()

		var id idmint.ID
		_, err := json.Marshal(id)
		require.Error(t, err)
	})
}

func TestID_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("valid id", func(t *testing.T) {
		t.Parallel()

		idKind := "user"
		idValue := "12345"
		idAsJSON := `"` + idKind + ":" + idValue + `"`

		var id idmint.ID
		err := json.Unmarshal([]byte(idAsJSON), &id)
		require.NoError(t, err)

		expectedID, err := idmint.NewID(idKind, idValue)
		require.NoError(t, err)
		assert.Equal(t, expectedID, id)
	})

	testCases := []struct {
		name string

		idAsJSON string

		expectedError error
	}{
		{
			name: "non-string type json value",

			idAsJSON: `user:12345`,

			expectedError: &json.UnmarshalTypeError{},
		},
		{
			name: "invalid id format",

			idAsJSON: `"user12345"`,

			expectedError: &idmint.InvalidIDFormatError{},
		},
		{
			name: "empty id kind",

			idAsJSON: `":12345"`,

			expectedError: &idmint.IDKindEmptyError{},
		},
		{
			name: "empty id value",

			idAsJSON: `"user:"`,

			expectedError: &idmint.IDValueEmptyError{},
		},
		{
			name: "empty id value",

			idAsJSON: `"user:value:with:underscores"`,

			expectedError: &idmint.IDValueContainsColonsError{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var id idmint.ID
			err := json.Unmarshal([]byte(testCase.idAsJSON), &id)
			require.Error(t, err)

			assert.ErrorAs(t, err, &testCase.expectedError)
		})
	}
}

func TestNewMinter(t *testing.T) {
	t.Parallel()

	t.Run("valid worker id", func(t *testing.T) {
		t.Parallel()

		minter, err := idmint.NewMinter(0)
		require.NoError(t, err)
		assert.NotNil(t, minter)
	})

	t.Run("invalid worker id", func(t *testing.T) {
		t.Parallel()

		const maxWorkerIDPlusOne = uint64(1 << 10)

		minter, err := idmint.NewMinter(maxWorkerIDPlusOne)
		require.Error(t, err)

		var tle *idmint.WorkerIDTooLargeError
		assert.ErrorAs(t, err, &tle)
		assert.Nil(t, minter)
	})
}

type controllableTimer struct {
	now time.Time
}

var _ idmint.Timer = (*controllableTimer)(nil)

func (c *controllableTimer) Time() time.Time {
	return c.now
}

func (c *controllableTimer) SetTimeTo(time time.Time) {
	c.now = time
}

func (c *controllableTimer) MoveTimeBy(duration time.Duration) {
	c.now = c.now.Add(duration)
}

func TestMinter_Mint(t *testing.T) {
	t.Parallel()

	t.Run("id value gets encoded", func(t *testing.T) {
		t.Parallel()

		encoder := idmint.EncoderFunc(func(s string) (string, error) {
			return "encoded(" + s + ")", nil
		})
		minter, err := idmint.NewMinter(0, idmint.WithEncoder(encoder))
		require.NoError(t, err)
		require.NotNil(t, minter)

		id, err := minter.Mint("user")
		require.NoError(t, err)

		pattern := regexp.MustCompile("encoded\\(.*\\)")
		assert.Regexp(t, pattern, id.String())
	})

	t.Run("id values increase in the same timestamp", func(t *testing.T) {
		t.Parallel()

		timer := &controllableTimer{
			now: time.Now(),
		}
		minter, err := idmint.NewMinter(0, idmint.WithTimer(timer))
		require.NoError(t, err)
		require.NotNil(t, minter)

		id1, err := minter.Mint("user")
		require.NoError(t, err)
		assert.NotNil(t, id1)

		id2, err := minter.Mint("user")
		require.NoError(t, err)
		assert.NotNil(t, id2)

		id3, err := minter.Mint("user")
		require.NoError(t, err)
		assert.NotNil(t, id3)

		assert.Greater(t, id3.Value(), id2.Value())
		assert.Greater(t, id2.Value(), id1.Value())
	})

	t.Run("id values increase as time moves forward", func(t *testing.T) {
		t.Parallel()

		timer := &controllableTimer{
			now: time.Now(),
		}
		minter, err := idmint.NewMinter(0, idmint.WithTimer(timer))
		require.NoError(t, err)
		require.NotNil(t, minter)

		id1, err := minter.Mint("user")
		require.NoError(t, err)
		require.NotZero(t, id1)

		timer.MoveTimeBy(time.Millisecond)

		id2, err := minter.Mint("user")
		require.NoError(t, err)
		require.NotZero(t, id2)

		timer.MoveTimeBy(time.Millisecond)

		id3, err := minter.Mint("user")
		require.NoError(t, err)
		require.NotZero(t, id3)

		assert.Greater(t, id3.Value(), id2.Value())
		assert.Greater(t, id2.Value(), id1.Value())
	})

	t.Run("cannot mint when time is before the start of time", func(t *testing.T) {
		t.Parallel()

		epoch := time.Now()
		timer := &controllableTimer{
			now: epoch.Add(-1 * time.Millisecond),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithTimer(timer),
			idmint.WithEpoch(epoch),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		id, err := minter.Mint("user")
		assert.Error(t, err)

		var bsomte *idmint.CurrentTimeBeforeStartOfMintingTimeError
		assert.ErrorAs(t, err, &bsomte)
		assert.Zero(t, id)
	})

	t.Run("cannot mint when time is after the end of time", func(t *testing.T) {
		t.Parallel()

		const maxTimestampPlusOne = 1 << 42

		epoch := time.Now()
		timer := &controllableTimer{
			now: epoch.Add(maxTimestampPlusOne * time.Millisecond),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithTimer(timer),
			idmint.WithEpoch(epoch),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		id, err := minter.Mint("user")
		assert.Error(t, err)

		var aeomte *idmint.CurrentTimeAfterEndOfMintingTimeError
		assert.ErrorAs(t, err, &aeomte)
		assert.Zero(t, id)
	})

	t.Run("cannot mint when time moves backwards", func(t *testing.T) {
		t.Parallel()

		epoch := time.Now()
		timer := &controllableTimer{
			now: epoch.Add(1 * time.Millisecond),
		}
		minter, err := idmint.NewMinter(
			0,
			idmint.WithTimer(timer),
			idmint.WithEpoch(epoch),
		)
		require.NoError(t, err)
		require.NotNil(t, minter)

		id1, err := minter.Mint("user")
		require.NoError(t, err)
		require.NotNil(t, id1)

		timer.MoveTimeBy(-1 * time.Millisecond)

		id2, err := minter.Mint("user")
		require.Error(t, err)

		var tmbe *idmint.TimeMovedBackwardsError
		assert.ErrorAs(t, err, &tmbe)
		assert.Zero(t, id2)
	})

	t.Run("cannot mint when sequence number overflows", func(t *testing.T) {
		t.Parallel()

		timer := &controllableTimer{
			now: time.Now(),
		}
		minter, err := idmint.NewMinter(0, idmint.WithTimer(timer))
		require.NoError(t, err)
		require.NotNil(t, minter)

		for range 1 << 12 {
			id, err := minter.Mint("user")
			require.NoError(t, err)
			require.NotNil(t, id)
		}

		id, err := minter.Mint("user")
		require.Error(t, err)

		var sntle *idmint.SequenceNumberTooLargeError
		assert.ErrorAs(t, err, &sntle)
		assert.Zero(t, id)
	})
}

func BenchmarkMinter_Mint(b *testing.B) {
	b.ReportAllocs()

	now := time.Now()
	minter, err := idmint.NewMinter(0, idmint.WithEpoch(now))
	require.NoError(b, err)
	require.NotNil(b, minter)

	id, err := minter.Mint("user")
	require.NoError(b, err)
	require.NotZero(b, id)

	b.ResetTimer()
	for b.Loop() {
		id, _ = minter.Mint("user")
	}

	runtime.KeepAlive(id)
}
