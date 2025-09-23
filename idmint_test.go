package idmint_test

import (
	"encoding/json"
	"testing"

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
			name: "kind with underscores",

			kind:  "user_profile",
			value: "12345",

			expectedError: &idmint.IDKindContainsUnderscoresError{},
		},
		{
			name: "empty value",

			kind:  "user",
			value: "",

			expectedError: &idmint.IDValueEmptyError{},
		},
		{
			name: "value with underscores",

			kind:  "user",
			value: "value_with_underscore",

			expectedError: &idmint.IDValueContainsUnderscoresError{},
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
		idAsString := idKind + "_" + idValue

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

			idAsString: "_12345",

			expectedError: &idmint.IDKindEmptyError{},
		},
		{
			name: "fails on empty value part",

			idAsString: "user_",

			expectedError: &idmint.IDValueEmptyError{},
		},
		{
			name: "fails on value with extra underscores",

			idAsString: "user_value_with_underscores",

			expectedError: &idmint.IDValueContainsUnderscoresError{},
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

		idAsString := id.Kind() + "_" + idValue
		assert.Equal(t, idAsString, id.String())
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()

		var id idmint.ID
		assert.Equal(t, "idmint.InvalidID", id.String())
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
		idAsString := idKind + "_" + idValue

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
		idAsJSON := `"` + idKind + "_" + idValue + `"`

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

			idAsJSON: `user_12345`,

			expectedError: &json.UnmarshalTypeError{},
		},
		{
			name: "invalid id format",

			idAsJSON: `"user12345"`,

			expectedError: &idmint.InvalidIDFormatError{},
		},
		{
			name: "empty id kind",

			idAsJSON: `"_12345"`,

			expectedError: &idmint.IDKindEmptyError{},
		},
		{
			name: "empty id value",

			idAsJSON: `"user_"`,

			expectedError: &idmint.IDValueEmptyError{},
		},
		{
			name: "empty id value",

			idAsJSON: `"user_value_with_underscores"`,

			expectedError: &idmint.IDValueContainsUnderscoresError{},
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
