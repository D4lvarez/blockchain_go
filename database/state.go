package database

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type State struct {
	Balances  map[Account]uint
	txMempool []Tx

	dbFile *os.File

	latestBlockHash Hash
	latestBlock     Block
}

func NewStateFromDisk(dataDir string) (*State, error) {
	// Initialize DataDir
	err := initDataDirIfNotExists(dataDir)
	if err != nil {
		return nil, err
	}

	// Load Genesis
	genesis, err := loadGenesis(getGenesisJsonFilePath(dataDir))
	if err != nil {
		return nil, err
	}

	// Get balances from genesis
	balances := make(map[Account]uint)
	for account, balance := range genesis.Balances {
		balances[account] = balance
	}

	// Load block.db
	f, err := os.OpenFile(getBlocksDbFilePath(dataDir), os.O_APPEND|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	// Scanner for transactions
	scanner := bufio.NewScanner(f)
	state := &State{
		balances,
		make([]Tx, 0),
		f,
		Hash{},
		Block{},
	}

	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		blockFsJson := scanner.Bytes()

		if len(blockFsJson) == 0 {
			break
		}

		var blockFs BlockFS
		err = json.Unmarshal(blockFsJson, &blockFs)
		if err != nil {
			return nil, err
		}

		err = state.applyBlock(blockFs.Value)
		if err != nil {
			return nil, err
		}

		state.latestBlock = blockFs.Value
		state.latestBlockHash = blockFs.Key
	}

	return state, nil
}

// Get Last block Hash
func (s *State) LatestBlockHash() Hash {
	return s.latestBlockHash
}

// Get Last block
func (s *State) LatestBlock() Block {
	return s.latestBlock
}

// Add Blocks
func (s *State) AddBlock(b Block) error {
	for _, tx := range b.TXs {
		if err := s.AddTx(tx); err != nil {
			return err
		}
	}
	return nil
}

// Add Transactions
func (s *State) AddTx(tx Tx) error {
	if err := s.apply(tx); err != nil {
		return err
	}

	s.txMempool = append(s.txMempool, tx)

	return nil
}

// Save Txs on Disk
func (s *State) Persist() (Hash, error) {
	latestBlockHash, err := s.latestBlock.Hash()
	if err != nil {
		return Hash{}, err
	}

	// Create New block
	block := NewBlock(
		latestBlockHash,
		s.latestBlock.Header.Number+1,
		uint64(time.Now().Unix()),
		s.txMempool,
	)

	blockHash, err := block.Hash()
	if err != nil {
		return Hash{}, err
	}

	blockFs := BlockFS{
		blockHash,
		block,
	}

	// Convert To Json
	blockFsJson, err := json.Marshal(blockFs)
	if err != nil {
		return Hash{}, nil
	}

	fmt.Printf("Persisting new Block to disk:\n")
	fmt.Printf("\t%s\n", blockFsJson)

	_, err = s.dbFile.Write(append(blockFsJson, '\n'))
	if err != nil {
		return Hash{}, err
	}

	s.latestBlock = block
	s.latestBlockHash = latestBlockHash
	s.txMempool = []Tx{}

	return latestBlockHash, nil
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

	if s.Balances[tx.From] < tx.Value {
		return fmt.Errorf("Insufficent Balance")
	}

	s.Balances[tx.From] -= tx.Value
	s.Balances[tx.To] += tx.Value

	return nil
}

func (s *State) applyBlock(b Block) error {
	for _, tx := range b.TXs {
		if err := s.apply(tx); err != nil {
			return err
		}
	}
	return nil
}
