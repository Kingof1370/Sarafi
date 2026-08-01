package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"velyxora/packages/types"
)

// JournalAction defines recorded operations in our sequence log
type JournalAction string

const (
	ActionSubmit JournalAction = "SUBMIT"
	ActionCancel JournalAction = "CANCEL"
)

// JournalRecord maps exact operations to restore book deterministically
type JournalRecord struct {
	Sequence int64         `json:"sequence"`
	Action   JournalAction `json:"action"`
	Order    *types.Order  `json:"order,omitempty"`
	OrderID  string        `json:"order_id,omitempty"`
}

// RecoveryEngine manages disk-persisted sequence journals and replays
type RecoveryEngine struct {
	mu          sync.Mutex
	filepath    string
	sequenceNum int64
}

// NewRecoveryEngine initializes the sequence file writer
func NewRecoveryEngine(path string) *RecoveryEngine {
	return &RecoveryEngine{
		filepath: path,
	}
}

// WriteSubmit writes a submit order action into the sequence journal
func (re *RecoveryEngine) WriteSubmit(order *types.Order) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.sequenceNum++
	record := JournalRecord{
		Sequence: re.sequenceNum,
		Action:   ActionSubmit,
		Order:    order,
	}

	return re.writeRecord(record)
}

// WriteCancel writes a cancellation action into the sequence journal
func (re *RecoveryEngine) WriteCancel(orderID string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.sequenceNum++
	record := JournalRecord{
		Sequence: re.sequenceNum,
		Action:   ActionCancel,
		OrderID:  orderID,
	}

	return re.writeRecord(record)
}

// ReplayJournal loads journal sequence file and replays events on the Matcher
func (re *RecoveryEngine) ReplayJournal(matcher *Matcher) (int, error) {
	re.mu.Lock()
	defer re.mu.Unlock()

	file, err := os.Open(re.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // no journal file yet, safe bypass
		}
		return 0, fmt.Errorf("failed to open recovery journal: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	count := 0

	for decoder.More() {
		var record JournalRecord
		if err := decoder.Decode(&record); err != nil {
			return count, fmt.Errorf("failed to decode journal record: %w", err)
		}

		if record.Action == ActionSubmit {
			matcher.MatchOrder(record.Order)
		} else if record.Action == ActionCancel {
			matcher.CancelOrder(record.OrderID)
		}
		count++
	}

	return count, nil
}

func (re *RecoveryEngine) writeRecord(record JournalRecord) error {
	file, err := os.OpenFile(re.filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open sequence journal for write: %w", err)
	}
	defer file.Close()

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal journal record: %w", err)
	}

	_, err = file.Write(append(payload, '\n'))
	if err != nil {
		return fmt.Errorf("failed to append journal log line: %w", err)
	}

	return nil
}
