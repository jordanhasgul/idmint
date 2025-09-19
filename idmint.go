package idmint

import (
	"encoding/json"
	"errors"
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

func (id *ID) Equal(jd ID) bool {
	return id.kind == jd.kind &&
		id.value == jd.value
}

func (id *ID) Kind() string {
	return id.kind
}

func (id *ID) Value() string {
	return id.value
}

func (id *ID) String() string {
	return id.Kind() + "_" + id.Value()
}

var _ json.Marshaler = (*ID)(nil)

func (id *ID) MarshalJSON() ([]byte, error) {
	var (
		idAsString = id.String()
		idAsJSON   = []byte(`"` + idAsString + `"`)
	)
	return idAsJSON, nil
}

var _ json.Unmarshaler = (*ID)(nil)

func (id *ID) UnmarshalJSON(data []byte) error {
	var idAsString string

	err := json.Unmarshal(data, &idAsString)
	if err != nil {
		return fmt.Errorf("unmarshalling json to string: %w", err)
	}

	idKindAndValue := strings.SplitN(idAsString, "_", 2)
	if len(idKindAndValue) != 2 {
		return errors.New("invalid id format, expected '<kind>_<value>'")
	}

	idKind := idKindAndValue[0]
	if idKind == "" {
		return errors.New("id 'kind' is empty")
	}

	idValue := idKindAndValue[1]
	if idValue == "" {
		return errors.New("id 'value' is empty")
	}

	id.kind = idKind
	id.value = idValue

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

func NewMinter(workerID uint64, configurers ...Configurer) *Minter {
	minter := &Minter{
		workerID: workerID,
	}
	for _, configurer := range configurers {
		configurer.configure(minter)
	}

	return minter
}

func (m *Minter) Mint(kind string) (ID, error) {
	m.once.Do(m.initialise)

	if m.workerID > maxWorkerID {
		errorStr := fmt.Sprintf("worker ID greater than %d", maxWorkerID)
		return ID{}, errors.New(errorStr)
	}

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

	return ID{
		value: encodedValue,
		kind:  kind,
	}, nil
}

func (m *Minter) canMint(now time.Time) (uint64, error) {
	if now.Before(m.startOfTime) {
		return 0, errors.New("before startOfTime")
	}

	if now.After(m.endOfTime) {
		return 0, errors.New("overflow")
	}

	if now.Before(m.lastMintedAt) {
		return 0, errors.New("before lastMintedAt timestamp")
	}

	sequenceNumber := m.getNextSequenceNumber(now)
	if sequenceNumber > maxSequenceNumber {
		return 0, errors.New("sequence number greater than max sequence")
	}

	return sequenceNumber, nil
}

func (m *Minter) getNextSequenceNumber(now time.Time) uint64 {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.sequenceNumber++
	if now.After(m.lastMintedAt) {
		m.lastMintedAt = now
		m.sequenceNumber = 0
	}

	return m.sequenceNumber
}

const (
	numTimeBits = uint64(42)
	maxTime     = (1 << numTimeBits) - 1

	numWorkerIDBits = uint64(10)
	maxWorkerID     = (1 << numWorkerIDBits) - 1

	numSequenceNumberBits = uint64(12)
	maxSequenceNumber     = (1 << numSequenceNumberBits) - 1
)

func (m *Minter) doMint(now time.Time, sequenceNumber uint64) uint64 {
	var rawValue uint64

	rawValue |= uint64(now.Sub(m.startOfTime).
		Milliseconds())
	offset := 64 - numTimeBits
	rawValue <<= offset

	rawValue |= m.workerID
	offset -= numWorkerIDBits
	rawValue <<= offset

	rawValue |= sequenceNumber
	offset -= numSequenceNumberBits
	rawValue <<= offset

	return rawValue
}

var defaultEpoch = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).
	Truncate(time.Millisecond)

func (m *Minter) initialise() {
	if m.startOfTime.IsZero() {
		m.startOfTime = defaultEpoch
	}
	m.endOfTime = m.startOfTime.Add(maxTime * time.Millisecond).
		Truncate(time.Millisecond)

	timer := m.timer
	if timer == nil {
		timer = TimeFunc(time.Now)
	}
	m.timer = TimeFunc(func() time.Time {
		now := timer.Time().
			Truncate(time.Millisecond)
		return now
	})

	m.lastMintedAt = m.timer.Time()

	if m.encoder == nil {
		noOpEncoder := func(s string) (string, error) {
			return s, nil
		}
		m.encoder = EncoderFunc(noOpEncoder)
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
