// Package database handles all the lower level support for maintaining the
// blockchain in storage and maintaining an in-memory databse of account information.
package database

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	acc "github.com/dudakovict/blockchain/foundation/blockchain/account"
	"github.com/dudakovict/blockchain/foundation/blockchain/block"
	"github.com/dudakovict/blockchain/foundation/blockchain/genesis"
	"github.com/dudakovict/blockchain/foundation/blockchain/signature"
	"github.com/dudakovict/blockchain/foundation/blockchain/transaction"
)

// =============================================================================

// Database manages data related to accounts who have transacted on the blockchain.
type Database struct {
	mu          sync.RWMutex
	genesis     genesis.Genesis
	latestBlock block.Block
	accounts    map[acc.AccountID]acc.Account
}

// New constructs a new database and applies account genesis information and
// reads/writes the blockchain database on disk if a dbPath is provided.
func New(genesis genesis.Genesis) (*Database, error) {
	db := Database{
		genesis:  genesis,
		accounts: make(map[acc.AccountID]acc.Account),
	}

	// Update the database with account balance information from genesis.
	for accountStr, balance := range genesis.Balances {
		accountID, err := acc.ToAccountID(accountStr)
		if err != nil {
			return nil, err
		}
		db.accounts[accountID] = acc.New(accountID, balance)
	}

	return &db, nil
}

// Reset re-initializes the database back to the genesis state.
func (db *Database) Reset() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Initializes the database back to the genesis information.
	db.latestBlock = block.Block{}
	db.accounts = make(map[acc.AccountID]acc.Account)
	for accountStr, balance := range db.genesis.Balances {
		accountID, err := acc.ToAccountID(accountStr)
		if err != nil {
			return err
		}

		db.accounts[accountID] = acc.New(accountID, balance)
	}

	return nil
}

// Remove deletes an account from the database.
func (db *Database) Remove(accountID acc.AccountID) {
	db.mu.Lock()
	defer db.mu.Unlock()

	delete(db.accounts, accountID)
}

// Query retrieves an account from the database.
func (db *Database) Query(accountID acc.AccountID) (acc.Account, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	account, exists := db.accounts[accountID]
	if !exists {
		return acc.Account{}, errors.New("account does not exist")
	}

	return account, nil
}

// Copy makes a copy of the current accounts in the database.
func (db *Database) Copy() map[acc.AccountID]acc.Account {
	db.mu.RLock()
	defer db.mu.RUnlock()

	accounts := make(map[acc.AccountID]acc.Account)
	for accountID, account := range db.accounts {
		accounts[accountID] = account
	}
	return accounts
}

// HashState returns a hash based on the contents of the accounts and
// their balances. This is added to each block and checked by peers.
func (db *Database) HashState() string {
	accounts := make([]acc.Account, 0, len(db.accounts))
	db.mu.RLock()
	{
		for _, account := range db.accounts {
			accounts = append(accounts, account)
		}
	}
	db.mu.RUnlock()

	sort.Sort(acc.ByAccount(accounts))
	return signature.Hash(accounts)
}

// ApplyMiningReward gives the specififed account the mining reward.
func (db *Database) ApplyMiningReward(b block.Block) {
	db.mu.Lock()
	defer db.mu.Unlock()

	account := db.accounts[b.Header.BeneficiaryID]
	account.Balance += b.Header.MiningReward

	db.accounts[b.Header.BeneficiaryID] = account
}

// ApplyTransaction performs the business logic for applying a transaction
// to the database.
func (db *Database) ApplyTransaction(b block.Block, tx transaction.BlockTx) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	{
		// Capture these accounts from the database.
		from, exists := db.accounts[tx.FromID]
		if !exists {
			from = acc.New(tx.FromID, 0)
		}

		to, exists := db.accounts[tx.ToID]
		if !exists {
			to = acc.New(tx.ToID, 0)
		}

		bnfc, exists := db.accounts[b.Header.BeneficiaryID]
		if !exists {
			bnfc = acc.New(b.Header.BeneficiaryID, 0)
		}

		// The account needs to pay the gas fee regardless. Take the
		// remaining balance if the account doesn't hold enough for the
		// full amount of gas. This is the only way to stop bad actors.
		gasFee := tx.GasPrice * tx.GasUnits
		if gasFee > from.Balance {
			gasFee = from.Balance
		}
		from.Balance -= gasFee
		bnfc.Balance += gasFee

		// Make sure these changes get applied.
		db.accounts[tx.FromID] = from
		db.accounts[b.Header.BeneficiaryID] = bnfc

		// Perform basic accounting checks.
		{
			if tx.Nonce != (from.Nonce + 1) {
				return fmt.Errorf("transaction invalid, wrong nonce, got %d, exp %d", tx.Nonce, from.Nonce+1)
			}

			if from.Balance == 0 || from.Balance < (tx.Value+tx.Tip) {
				return fmt.Errorf("transaction invalid, insufficient funds, bal %d, needed %d", from.Balance, (tx.Value + tx.Tip))
			}
		}

		// Update the balances between the two parties.
		from.Balance -= tx.Value
		to.Balance += tx.Value

		// Give the beneficiary the tip.
		from.Balance -= tx.Tip
		bnfc.Balance += tx.Tip

		// Update the nonce for the next transaction check.
		from.Nonce = tx.Nonce

		// Update the final changes to these accounts.
		db.accounts[tx.FromID] = from
		db.accounts[tx.ToID] = to
		db.accounts[b.Header.BeneficiaryID] = bnfc
	}

	return nil
}

// UpdateLatestBlock provides safe access to update the latest block.
func (db *Database) UpdateLatestBlock(b block.Block) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.latestBlock = b
}

// LatestBlock returns the latest block.
func (db *Database) LatestBlock() block.Block {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return db.latestBlock
}
