package database

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type State struct {
	Balances  map[Account]uint
	txMempool []Tx
	dbFile    *os.File
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
	}

	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		// convert Tx to Struct
		var tx Tx
		json.Unmarshal(scanner.Bytes(), &tx)

		// Rebuilt State
		if err := state.apply(tx); err != nil {
			return nil, err
		}
	}

	return state, nil
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
func (s *State) Persist() error {
	// Copy of Mempool
	mempool := make([]Tx, len(s.txMempool))
	copy(mempool, s.txMempool)

	for i := 0; i < len(mempool); i++ {
		txJson, err := json.Marshal(mempool[i])
		if err != nil {
			return err
		}

		if _, err = s.dbFile.Write(append(txJson, '\n')); err != nil {
			return err
		}

		// Remove Tx saved from mempool
		s.txMempool = s.txMempool[1:]
	}

	return nil
}

// Close file
func (s *State) Close() {
	s.dbFile.Close()
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
