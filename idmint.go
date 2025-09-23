package idmint

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ID struct {
	kind  string
	value string
}

func NewID(kind, value string) (ID, error) {
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

func MustNewID(kind, value string) ID {
	id, err := NewID(kind, value)
	if err != nil {
		err = fmt.Errorf("creating id: %w", err)
		panic(err)
	}

	return id
}

func ParseID(idAsString string) (ID, error) {
	kind, value, found := strings.Cut(idAsString, "_")
	if !found {
		return ID{}, &InvalidIDFormatError{idAsString: idAsString}
	}

	id, err := NewID(kind, value)
	if err != nil {
		return ID{}, fmt.Errorf("creating id: %w", err)
	}

	return id, nil
}

func MustParseID(idAsString string) ID {
	id, err := ParseID(idAsString)
	if err != nil {
		err = fmt.Errorf("parsing id: %v", err)
		panic(err)
	}

	return id
}

func (id ID) Kind() string {
	return id.kind
}

func (id ID) Value() string {
	return id.value
}

type IDKindEmptyError struct{}

func (e IDKindEmptyError) Error() string {
	return "id kind is empty"
}

type IDKindContainsUnderscoresError struct{}

func (e IDKindContainsUnderscoresError) Error() string {
	return "id kind contains underscores"
}

type IDValueEmptyError struct{}

func (e IDValueEmptyError) Error() string {
	return "id value is empty"
}

type IDValueContainsUnderscoresError struct{}

func (e IDValueContainsUnderscoresError) Error() string {
	return "id value contains underscores"
}

func (id ID) Valid() error {
	if id.Kind() == "" {
		return &IDKindEmptyError{}
	}

	if strings.Contains(id.Kind(), "_") {
		return &IDKindContainsUnderscoresError{}
	}

	if id.Value() == "" {
		return &IDValueEmptyError{}
	}

	if strings.Contains(id.Value(), "_") {
		return &IDValueContainsUnderscoresError{}
	}

	return nil
}

var _ fmt.Stringer = (*ID)(nil)

const invalidIDString = "idmint.InvalidID"

func (id ID) String() string {
	err := id.Valid()
	if err != nil {
		return invalidIDString
	}

	return id.Kind() + "_" + id.Value()
}

func (id ID) Equal(jd ID) bool {
	return id.Kind() == jd.Kind() &&
		id.Value() == jd.Value()
}

var _ json.Marshaler = (*ID)(nil)

func (id ID) MarshalJSON() ([]byte, error) {
	err := id.Valid()
	if err != nil {
		return nil, fmt.Errorf("validating id: %w", err)
	}

	idAsJSON := []byte(`"` + id.Kind() + `_` + id.Value() + `"`)
	return idAsJSON, nil
}

type InvalidIDFormatError struct {
	idAsString string
}

func (e InvalidIDFormatError) Error() string {
	errorString := "want id to have format '<kind>_<value>' but got '%s'"
	return fmt.Sprintf(errorString, e.idAsString)
}

func (e InvalidIDFormatError) ID() string {
	return e.idAsString
}

var _ json.Unmarshaler = (*ID)(nil)

func (id *ID) UnmarshalJSON(idAsJson []byte) error {
	var idAsString string

	err := json.Unmarshal(idAsJson, &idAsString)
	if err != nil {
		return fmt.Errorf("unmarshalling to string: %w", err)
	}

	parsedID, err := ParseID(idAsString)
	if err != nil {
		return fmt.Errorf("parsing id: %w", err)
	}

	*id = parsedID

	return nil
}

type Minter struct {
	once sync.Once

	workerID uint64

	timer       Timer
	startOfTime time.Time
	endOfTime   time.Time

	lock sync.Mutex

	lastMintedAt   time.Time
	sequenceNumber uint64

	encoder Encoder
}

type Configurer interface {
	configure(m *Minter)
}

type WorkerIDTooLargeError struct {
	workerID uint64
}

func (e WorkerIDTooLargeError) Error() string {
	errorString := "worker id '%d' is greater than %d"
	return fmt.Sprintf(errorString, e.workerID, maxWorkerID)
}

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

func (m *Minter) Mint(kind string) (ID, error) {
	m.once.Do(m.initialise)

	now := m.timer.Time()
	sequenceNumber, err := m.canMint(now)
	if err != nil {
		return ID{}, fmt.Errorf("checking if we can mint: %w", err)
	}

	rawValue := m.doMint(now, sequenceNumber)

	const base10 int = 10
	formattedValue := strconv.FormatUint(rawValue, base10)

	encodedValue, err := m.encoder.Encode(formattedValue)
	if err != nil {
		return ID{}, fmt.Errorf("encoding value: %w", err)
	}

	id, err := NewID(kind, encodedValue)
	if err != nil {
		return ID{}, fmt.Errorf("creating id: %w", err)
	}

	return id, nil
}

type CurrentTimeBeforeStartOfTimeError struct {
	currentTime time.Time
	startOfTime time.Time
}

func (e CurrentTimeBeforeStartOfTimeError) Error() string {
	var (
		currentTimeAsString = e.currentTime.Format(time.RFC3339)
		startOfTimeAsString = e.startOfTime.Format(time.RFC3339)

		errorStr = "current time '%s' is before start of time '%s'"
	)
	return fmt.Sprintf(errorStr, currentTimeAsString, startOfTimeAsString)
}

type CurrentTimeAfterEndOfTimeError struct {
	currentTime time.Time
	endOfTime   time.Time
}

func (e CurrentTimeAfterEndOfTimeError) Error() string {
	var (
		currentTimeAsString = e.currentTime.Format(time.RFC3339)
		endOfTimeAsString   = e.endOfTime.Format(time.RFC3339)

		errorString = "current time '%s' is after end of time '%s'"
	)
	return fmt.Sprintf(errorString, currentTimeAsString, endOfTimeAsString)
}

type TimeMovedBackwardsError struct {
	durationMovedBackwards time.Duration
}

func (e TimeMovedBackwardsError) Duration() time.Duration {
	return e.durationMovedBackwards
}

func (e TimeMovedBackwardsError) Error() string {
	return "time moved backwards by " + e.durationMovedBackwards.String()
}

type SequenceNumberTooLargeError struct{}

func (e SequenceNumberTooLargeError) Error() string {
	return "sequence number is greater than " + strconv.Itoa(maxSequenceNumber)
}

func (m *Minter) canMint(now time.Time) (uint64, error) {
	if now.Before(m.startOfTime) {
		return 0, &CurrentTimeBeforeStartOfTimeError{
			currentTime: now,
			startOfTime: m.startOfTime,
		}
	}

	if now.After(m.endOfTime) {
		return 0, &CurrentTimeAfterEndOfTimeError{
			currentTime: now,
			endOfTime:   m.endOfTime,
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

	rawValue |= uint64(now.Sub(m.startOfTime).
		Milliseconds())

	rawValue <<= numBitsForWorkerID
	rawValue |= m.workerID

	rawValue <<= numBitsForSequenceNumber
	rawValue |= sequenceNumber

	return rawValue
}

var defaultStartOfTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func (m *Minter) initialise() {
	if m.startOfTime.IsZero() {
		m.startOfTime = defaultStartOfTime
	}
	m.startOfTime = m.startOfTime.Truncate(time.Millisecond)
	m.endOfTime = m.startOfTime.Add(maxTimestamp * time.Millisecond)

	if m.timer == nil {
		m.timer = stdTimer{}
	}
	m.timer = truncatingTimer{
		timer:    m.timer,
		duration: time.Millisecond,
	}

	m.lastMintedAt = m.timer.Time()

	if m.encoder == nil {
		m.encoder = noOpEncoder{}
	}
}

type Timer interface {
	Time() time.Time
}

type TimeFunc func() time.Time

func (f TimeFunc) Time() time.Time {
	return f()
}

type withTimerConfigurer struct {
	timer Timer
}

func (c *withTimerConfigurer) configure(m *Minter) {
	m.timer = c.timer
}

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
	m.startOfTime = c.epoch
}

func WithEpoch(epoch time.Time) Configurer {
	return &withEpochConfigurer{epoch: epoch}
}

type Encoder interface {
	Encode(s string) (string, error)
}

type EncoderFunc func(s string) (string, error)

func (f EncoderFunc) Encode(s string) (string, error) {
	return f(s)
}

type withEncoderConfigurer struct {
	encoder Encoder
}

func (c *withEncoderConfigurer) configure(m *Minter) {
	m.encoder = c.encoder
}

func WithEncoder(encoder Encoder) Configurer {
	return &withEncoderConfigurer{encoder: encoder}
}

type noOpEncoder struct{}

func (e noOpEncoder) Encode(s string) (string, error) {
	return s, nil
}
