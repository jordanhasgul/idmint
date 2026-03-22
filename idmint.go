// Package idmint is a Go module for minting unique, time-sortable IDs in a
// distributed system without coordination between nodes.
package idmint

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// Minter mints unique, time-sortable, IDs whose 64-bit value is composed of:
//
//   - 42 bits representing the time, in milliseconds since the start, when
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

	clock              Clock
	startOfMintingTime int64
	endOfMintingTime   int64

	lock sync.Mutex

	lastMintedAt   int64
	sequenceNumber uint64
}

// Configurer configures the behaviour of a Minter.
type Configurer interface {
	configure(m *Minter)
}

// WorkerIDTooLargeError indicates that the worker ID was larger than the
// maximum value.
type WorkerIDTooLargeError struct {
	workerID uint64
}

func (e WorkerIDTooLargeError) Error() string {
	errorString := "worker id '%d' is larger than %d"
	return fmt.Sprintf(errorString, e.workerID, maxWorkerID)
}

// NewMinter returns a new Minter with the provided worker ID, that has been
// configured by applying the supplied Configurer's.
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
	minter.once.Do(minter.initialise)

	return minter, nil
}

const (
	numBitsForTimestamp = uint64(42)
	maxTimestamp        = (1 << numBitsForTimestamp) - 1

	numBitsForWorkerID = uint64(10)
	maxWorkerID        = (1 << numBitsForWorkerID) - 1

	numBitsForSequenceNumber = uint64(12)
	maxSequenceNumber        = (1 << numBitsForSequenceNumber) - 1
)

var defaultStartOfTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).
	UnixMilli()

func (m *Minter) initialise() {
	if m.startOfMintingTime == 0 {
		m.startOfMintingTime = defaultStartOfTime
	}
	m.endOfMintingTime = m.startOfMintingTime + maxTimestamp

	if m.clock == nil {
		m.clock = systemClock{}
	}

	m.lastMintedAt = m.startOfMintingTime
}

// CurrentTimeBeforeStartOfMintingTimeError indicates that the current time is before
// the start of time.
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

// CurrentTimeAfterEndOfMintingTimeError indicates that the current time is
// after the end of time.
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

// TimeMovedBackwardsError indicates that the clock has moved backwards since
// the last ID was minted.
type TimeMovedBackwardsError struct {
	durationMovedBackwards time.Duration
}

func (e TimeMovedBackwardsError) Error() string {
	return "time moved backwards by " + e.durationMovedBackwards.String()
}

func (e TimeMovedBackwardsError) Duration() time.Duration {
	return e.durationMovedBackwards
}

// SequenceNumberTooLargeError indicates that a Minter has minted more than
// 4096 IDs in the same millisecond.
type SequenceNumberTooLargeError struct{}

func (e SequenceNumberTooLargeError) Error() string {
	return "sequence number is greater than " + strconv.Itoa(maxSequenceNumber)
}

// Mint returns a unique, time-sortable, 64-bit unsigned integer.
func (m *Minter) Mint() (uint64, error) {
	m.once.Do(m.initialise)

	m.lock.Lock()
	defer m.lock.Unlock()

	now := m.clock.Now().
		UnixMilli()

	sequenceNumber, err := m.canMint(now)
	if err != nil {
		return 0, fmt.Errorf("attempting to mint: %w", err)
	}

	id := m.doMint(now, sequenceNumber)
	return id, nil
}

func (m *Minter) canMint(now int64) (uint64, error) {
	if now < m.startOfMintingTime {
		return 0, &CurrentTimeBeforeStartOfMintingTimeError{
			currentTime:        time.UnixMilli(now),
			startOfMintingTime: time.UnixMilli(m.startOfMintingTime),
		}
	}

	if now > m.endOfMintingTime {
		return 0, &CurrentTimeAfterEndOfMintingTimeError{
			currentTime:      time.UnixMilli(now),
			endOfMintingTime: time.UnixMilli(m.endOfMintingTime),
		}
	}

	if now < m.lastMintedAt {
		durationMovedBackwards := m.lastMintedAt - now
		return 0, &TimeMovedBackwardsError{
			durationMovedBackwards: time.Duration(durationMovedBackwards) * time.Millisecond,
		}
	}

	sequenceNumber := m.getNextSequenceNumber(now)
	if sequenceNumber > maxSequenceNumber {
		return 0, &SequenceNumberTooLargeError{}
	}

	return sequenceNumber, nil
}

func (m *Minter) getNextSequenceNumber(now int64) uint64 {
	if now > m.lastMintedAt {
		m.lastMintedAt = now
		m.sequenceNumber = 0
	}

	sequenceNumber := m.sequenceNumber
	m.sequenceNumber++

	return sequenceNumber
}

func (m *Minter) doMint(now int64, sequenceNumber uint64) uint64 {
	var id uint64

	id |= uint64(now - m.startOfMintingTime)

	id <<= numBitsForWorkerID
	id |= m.workerID

	id <<= numBitsForSequenceNumber
	id |= sequenceNumber

	return id
}

// Clock retrieves the current time.
type Clock interface {
	Now() time.Time
}

// ClockFunc is an adapter that allows us to use ordinary functions as a Clock.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time {
	return f()
}

type withClockConfigurer struct {
	clock Clock
}

func (c *withClockConfigurer) configure(m *Minter) {
	m.clock = c.clock
}

// WithClock returns a Configurer that sets a custom Clock on a Minter.
func WithClock(clock Clock) Configurer {
	return &withClockConfigurer{clock: clock}
}

type systemClock struct{}

var _ Clock = (*systemClock)(nil)

func (s systemClock) Now() time.Time {
	return time.Now()
}

type withStartOfMintingTimeConfigurer struct {
	startOfMintingTime int64
}

func (c *withStartOfMintingTimeConfigurer) configure(m *Minter) {
	m.startOfMintingTime = c.startOfMintingTime
}

// WithStartOfMintingTime returns a Configurer that the start of minting time
// on a Minter.
func WithStartOfMintingTime(startOfMintingTime time.Time) Configurer {
	return &withStartOfMintingTimeConfigurer{
		startOfMintingTime: startOfMintingTime.UnixMilli(),
	}
}
