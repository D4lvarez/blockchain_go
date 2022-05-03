package database

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Snapshot [32]byte

type State struct {
	Balances  map[Account]uint
	txMempool []Tx

	dbFile   *os.File
	snapshot Snapshot
}

func NewStateFromDisk() (*State, error) {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Load genesis.json
	genesisFilePath := filepath.Join(cwd, "database", "genesis.json")
	genesis, err := loadGenesis(genesisFilePath)
	if err != nil {
		return nil, err
	}

	// Get balances from genesis
	balances := make(map[Account]uint)
	for account, balance := range genesis.Balances {
		balances[account] = balance
	}

	// Load tx.db
	txDbFile := filepath.Join(cwd, "database", "tx.db")
	f, err := os.OpenFile(txDbFile, os.O_APPEND|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	// Scanner for transactions
	scanner := bufio.NewScanner(f)
	state := &State{
		balances,
		make([]Tx, 0),
		f,
		Snapshot{},
	}

	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		// convert Tx to Struct
		var tx Tx
		err = json.Unmarshal(scanner.Bytes(), &tx)
		if err != nil {
			return nil, err
		}

		// Rebuilt State
		if err := state.apply(tx); err != nil {
			return nil, err
		}
	}

	err = state.doSnapshot()
	if err != nil {
		return nil, err
	}

	return state, nil
}

// Get Last Snapshot
func (s *State) LatestSnapshot() Snapshot {
	return s.snapshot
}

// Add Transactions
func (s *State) Add(tx Tx) error {
	if err := s.apply(tx); err != nil {
		return err
	}

	s.txMempool = append(s.txMempool, tx)

	return nil
}

// Save Txs on Disk
func (s *State) Persist() (Snapshot, error) {
	// Copy of Mempool
	mempool := make([]Tx, len(s.txMempool))
	copy(mempool, s.txMempool)

	for i := 0; i < len(mempool); i++ {
		txJson, err := json.Marshal(mempool[i])
		if err != nil {
			return Snapshot{}, err
		}
		fmt.Printf("Persisting new Tx to disk: \t%s\n", txJson)

		if _, err = s.dbFile.Write(append(txJson, '\n')); err != nil {
			return Snapshot{}, err
		}

		err = s.doSnapshot()
		if err != nil {
			return Snapshot{}, err
		}
		fmt.Printf("New Db Snapshot: %x\n", s.snapshot)

		// Remove Tx saved from mempool
		s.txMempool = s.txMempool[1:]
	}

	return s.snapshot, nil
}

// Close file
func (s *State) Close() error {
	return s.dbFile.Close()
}

// Change and Validate state
func (s *State) apply(tx Tx) error {
	if tx.IsReward() {
		s.Balances[tx.To] += tx.Value
		return nil
	}

	if tx.Value > s.Balances[tx.From] {
		return fmt.Errorf("Insufficent Balance")
	}

	s.Balances[tx.From] -= tx.Value
	s.Balances[tx.To] += tx.Value

	return nil
}

// Create Snapshot
func (s *State) doSnapshot() error {
	_, err := s.dbFile.Seek(0, 0)
	if err != nil {
		return err
	}

	txsData, err := ioutil.ReadAll(s.dbFile)
	if err != nil {
		return err
	}

	s.snapshot = sha256.Sum256(txsData)

	return nil
}
