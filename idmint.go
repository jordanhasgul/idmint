// Package idmint is a Go module for minting unique, time-sortable IDs in
// a distributed system without coordination between nodes.
package idmint

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ID represents an immutable identifier for a kind of 'resource'.
type ID struct {
	kind  string
	value uint64
}

// NewID returns a new ID from a kind and a value. However, if the kind or
// the value is invalid (as specified by ID.Valid), an error is returned.
func NewID(kind string, value uint64) (ID, error) {
	id := ID{
		kind:  kind,
		value: value,
	}
	err := id.Valid()
	if err != nil {
		return ID{}, fmt.Errorf("validating id: %w", err)
	}

	return id, nil
}

// MustNewID is like NewID but panics if creation fails.
func MustNewID(kind string, value uint64) ID {
	id, err := NewID(kind, value)
	if err != nil {
		err = fmt.Errorf("creating id: %w", err)
		panic(err)
	}

	return id
}

// InvalidIDFormatError indicates that a string to be parsed as an ID is not
// of the format "<kind>:<value>".
type InvalidIDFormatError struct {
	idAsString string
}

func (e InvalidIDFormatError) Error() string {
	errorString := "want id to have format '<kind>:<value>' but got '%s'"
	return fmt.Sprintf(errorString, e.idAsString)
}

func (e InvalidIDFormatError) ID() string {
	return e.idAsString
}

// IDValueParseError indicates that the value part of an ID string could not be
// parsed as a uint64.
type IDValueParseError struct {
	valueAsString string
	parseErr      error
}

func (e IDValueParseError) Value() string {
	return e.valueAsString
}

func (e IDValueParseError) Error() string {
	errorString := "parsing value '%s': %s"
	return fmt.Sprintf(errorString, e.valueAsString, e.parseErr)
}

func (e IDValueParseError) Unwrap() error {
	return e.parseErr
}

// ParseID parses a string of the format "<kind>:<value>" into an ID. However,
// if the string format is invalid or the kind or the value is invalid (as
// specified by ID.Valid), an error is returned.
func ParseID(idAsString string) (ID, error) {
	kind, valueAsString, found := strings.Cut(idAsString, ":")
	if !found {
		return ID{}, &InvalidIDFormatError{
			idAsString: idAsString,
		}
	}

	const (
		base10    = 10
		bitSize64 = 64
	)
	value, err := strconv.ParseUint(valueAsString, base10, bitSize64)
	if err != nil {
		return ID{}, &IDValueParseError{
			valueAsString: valueAsString,
			parseErr:      err,
		}
	}

	id, err := NewID(kind, value)
	if err != nil {
		return ID{}, fmt.Errorf("creating id: %w", err)
	}

	return id, nil
}

// MustParseID is like ParseID but panics if parsing fails.
func MustParseID(idAsString string) ID {
	id, err := ParseID(idAsString)
	if err != nil {
		err = fmt.Errorf("parsing id: %w", err)
		panic(err)
	}

	return id
}

// Kind returns the kind part of the ID.
func (id ID) Kind() string {
	return id.kind
}

// Value returns the value part of the ID.
func (id ID) Value() uint64 {
	return id.value
}

// IDKindEmptyError indicates an ID's kind was empty.
type IDKindEmptyError struct{}

func (e IDKindEmptyError) Error() string {
	return "id kind is empty"
}

// IDKindContainsColonsError indicates an ID's kind contains colons.
type IDKindContainsColonsError struct{}

func (e IDKindContainsColonsError) Error() string {
	return "id kind contains colons"
}

// Valid returns an error if the ID's kind or the ID's value was empty or
// contains colons.
func (id ID) Valid() error {
	kind := id.Kind()
	if kind == "" {
		return &IDKindEmptyError{}
	}

	if strings.Contains(kind, ":") {
		return &IDKindContainsColonsError{}
	}

	return nil
}

var _ fmt.Stringer = (*ID)(nil)

const invalidIDString = "InvalidID"

// String returns the string representation of the ID in the format
// "<kind>:<value>". However, if the ID is invalid, it returns "InvalidID".
func (id ID) String() string {
	err := id.Valid()
	if err != nil {
		return invalidIDString
	}

	return id.string()
}

func (id ID) string() string {
	const base10 = 10
	var (
		kind           = id.Kind()
		value          = id.Value()
		formattedValue = strconv.FormatUint(value, base10)
	)
	return kind + ":" + formattedValue
}

// Equal returns a boolean denoting whether two IDs are equal by comparing
// their kinds and values.
func (id ID) Equal(jd ID) bool {
	return id.Kind() == jd.Kind() &&
		id.Value() == jd.Value()
}

var _ json.Marshaler = (*ID)(nil)

// MarshalJSON implements the [encoding/json.Marshaler] interface. It marshals
// the ID into a JSON string in the format "<kind>:<value>". However, an
// error is returned if the ID is invalid.
func (id ID) MarshalJSON() ([]byte, error) {
	err := id.Valid()
	if err != nil {
		return nil, fmt.Errorf("validating id: %w", err)
	}

	idAsJSON := []byte(`"` + id.string() + `"`)
	return idAsJSON, nil
}

var _ json.Unmarshaler = (*ID)(nil)

// UnmarshalJSON implements the [encoding/json.Unmarshaler] interface. It
// unmarshals a JSON string of the format "<kind>:<value>" in to an ID. However,
// an error is returned if the ID is invalid.
func (id *ID) UnmarshalJSON(idAsJSON []byte) error {
	var idAsString string

	err := json.Unmarshal(idAsJSON, &idAsString)
	if err != nil {
		return fmt.Errorf("unmarshaling to string: %w", err)
	}

	parsedID, err := ParseID(idAsString)
	if err != nil {
		return fmt.Errorf("parsing id: %w", err)
	}

	*id = parsedID

	return nil
}

// Minter mints unique, time-sortable, IDs whose 64-bit value is composed of:
//
//   - 42 bits representing the time, in milliseconds since the epoch, when
//     the ID was minted.
//   - 10 bits that uniquely identify the worker that minted the ID.
//   - 12 bits the number of the ID in the sequence of IDs minted that
//     millisecond.
//
// This allows us to have 1024 workers each minting 4096 IDs per millisecond. It
// is safe for concurrent use.
type Minter struct {
	once sync.Once

	workerID uint64

	timer              Timer
	startOfMintingTime time.Time
	endOfMintingTime   time.Time

	lock sync.Mutex

	lastMintedAt   time.Time
	sequenceNumber uint64
}

// Configurer configures the behaviour of a Minter.
type Configurer interface {
	configure(m *Minter)
}

// WorkerIDTooLargeError indicates that the worker identifier was larger
// than 1023.
type WorkerIDTooLargeError struct {
	workerID uint64
}

func (e WorkerIDTooLargeError) Error() string {
	errorString := "worker id '%d' is greater than %d"
	return fmt.Sprintf(errorString, e.workerID, maxWorkerID)
}

// NewMinter returns a new Minter, with the specified worker identifier
// (between 0 and 1023 inclusive), that has been configured by applying
// the supplied Configurer's.
func NewMinter(workerID uint64, configurers ...Configurer) (*Minter, error) {
	if workerID > maxWorkerID {
		return nil, &WorkerIDTooLargeError{workerID: workerID}
	}

	minter := &Minter{
		workerID: workerID,
	}
	for _, configurer := range configurers {
		configurer.configure(minter)
	}

	return minter, nil
}

// Mint returns a unique, time-sortable, ID for a kind of 'resource'. However,
// it returns an error if:
//
//   - The current time is before the start of minting time or after the end of
//     minting time, where the end of minting time = start of minting time + 2^42 - 1
//     milliseconds.
//   - The current time has moved backwards since the last ID was minted.
//   - The minter has minted more than 4096 IDs in the current millisecond.
func (m *Minter) Mint(kind string) (ID, error) {
	m.once.Do(m.initialise)

	now := m.timer.Time()
	sequenceNumber, err := m.canMint(now)
	if err != nil {
		return ID{}, fmt.Errorf("attempting to mint: %w", err)
	}

	value := m.doMint(now, sequenceNumber)

	id, err := NewID(kind, value)
	if err != nil {
		return ID{}, fmt.Errorf("creating id: %w", err)
	}

	return id, nil
}

// CurrentTimeBeforeStartOfMintingTimeError indicates that the current time is before the start of minting time.
type CurrentTimeBeforeStartOfMintingTimeError struct {
	currentTime        time.Time
	startOfMintingTime time.Time
}

func (e CurrentTimeBeforeStartOfMintingTimeError) Error() string {
	var (
		currentTimeAsString        = e.currentTime.Format(time.RFC3339)
		startOfMintingTimeAsString = e.startOfMintingTime.Format(time.RFC3339)

		errorStr = "current time '%s' is before start of minting time '%s'"
	)
	return fmt.Sprintf(errorStr, currentTimeAsString, startOfMintingTimeAsString)
}

// CurrentTimeAfterEndOfMintingTimeError indicates that the current time is after the end of minting time.
type CurrentTimeAfterEndOfMintingTimeError struct {
	currentTime      time.Time
	endOfMintingTime time.Time
}

func (e CurrentTimeAfterEndOfMintingTimeError) Error() string {
	var (
		currentTimeAsString      = e.currentTime.Format(time.RFC3339)
		endOfMintingTimeAsString = e.endOfMintingTime.Format(time.RFC3339)

		errorString = "current time '%s' is after end of minting time '%s'"
	)
	return fmt.Sprintf(errorString, currentTimeAsString, endOfMintingTimeAsString)
}

// TimeMovedBackwardsError indicates that the time has moved backwards since
// the last ID was minted.
type TimeMovedBackwardsError struct {
	durationMovedBackwards time.Duration
}

func (e TimeMovedBackwardsError) Duration() time.Duration {
	return e.durationMovedBackwards
}

func (e TimeMovedBackwardsError) Error() string {
	return "time moved backwards by " + e.durationMovedBackwards.String()
}

// SequenceNumberTooLargeError indicates that a Minter has minted more than
// 4096 IDs in the same millisecond.
type SequenceNumberTooLargeError struct{}

func (e SequenceNumberTooLargeError) Error() string {
	return "sequence number is greater than " + strconv.Itoa(maxSequenceNumber)
}

func (m *Minter) canMint(now time.Time) (uint64, error) {
	if now.Before(m.startOfMintingTime) {
		return 0, &CurrentTimeBeforeStartOfMintingTimeError{
			currentTime:        now,
			startOfMintingTime: m.startOfMintingTime,
		}
	}

	if now.After(m.endOfMintingTime) {
		return 0, &CurrentTimeAfterEndOfMintingTimeError{
			currentTime:      now,
			endOfMintingTime: m.endOfMintingTime,
		}
	}

	if now.Before(m.lastMintedAt) {
		durationMovedBackwards := m.lastMintedAt.Sub(now)
		return 0, &TimeMovedBackwardsError{durationMovedBackwards: durationMovedBackwards}
	}

	sequenceNumber := m.getNextSequenceNumber(now)
	if sequenceNumber > maxSequenceNumber {
		return 0, &SequenceNumberTooLargeError{}
	}

	return sequenceNumber, nil
}

func (m *Minter) getNextSequenceNumber(now time.Time) uint64 {
	m.lock.Lock()
	defer m.lock.Unlock()

	if now.After(m.lastMintedAt) {
		m.lastMintedAt = now
		m.sequenceNumber = 0
	}

	sequenceNumber := m.sequenceNumber
	m.sequenceNumber++

	return sequenceNumber
}

const (
	numBitsForTimestamp = uint64(42)
	maxTimestamp        = (1 << numBitsForTimestamp) - 1

	numBitsForWorkerID = uint64(10)
	maxWorkerID        = (1 << numBitsForWorkerID) - 1

	numBitsForSequenceNumber = uint64(12)
	maxSequenceNumber        = (1 << numBitsForSequenceNumber) - 1
)

func (m *Minter) doMint(now time.Time, sequenceNumber uint64) uint64 {
	var rawValue uint64

	rawValue |= uint64(now.Sub(m.startOfMintingTime).
		Milliseconds())

	rawValue <<= numBitsForWorkerID
	rawValue |= m.workerID

	rawValue <<= numBitsForSequenceNumber
	rawValue |= sequenceNumber

	return rawValue
}

var defaultStartOfTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func (m *Minter) initialise() {
	if m.startOfMintingTime.IsZero() {
		m.startOfMintingTime = defaultStartOfTime
	}
	m.startOfMintingTime = m.startOfMintingTime.Truncate(time.Millisecond)
	m.endOfMintingTime = m.startOfMintingTime.Add(maxTimestamp * time.Millisecond)

	if m.timer == nil {
		m.timer = stdTimer{}
	}
	m.timer = truncatingTimer{
		timer:    m.timer,
		duration: time.Millisecond,
	}

	m.lastMintedAt = m.timer.Time()
}

// Timer retrieves the current time.
type Timer interface {
	Time() time.Time
}

// TimerFunc is an adapter that allows us to use ordinary functions as a Timer.
type TimerFunc func() time.Time

func (f TimerFunc) Time() time.Time {
	return f()
}

type withTimerConfigurer struct {
	timer Timer
}

func (c *withTimerConfigurer) configure(m *Minter) {
	m.timer = c.timer
}

// WithTimer returns a Configurer that sets a custom Timer on a Minter.
func WithTimer(timer Timer) Configurer {
	return &withTimerConfigurer{timer: timer}
}

type stdTimer struct{}

var _ Timer = (*stdTimer)(nil)

func (s stdTimer) Time() time.Time {
	return time.Now()
}

type truncatingTimer struct {
	timer    Timer
	duration time.Duration
}

func (t truncatingTimer) Time() time.Time {
	return t.timer.Time().Truncate(t.duration)
}

type withEpochConfigurer struct {
	epoch time.Time
}

func (c *withEpochConfigurer) configure(m *Minter) {
	m.startOfMintingTime = c.epoch
}

// WithEpoch returns a Configurer that sets a custom epoch on a Minter.
func WithEpoch(epoch time.Time) Configurer {
	return &withEpochConfigurer{epoch: epoch}
}
