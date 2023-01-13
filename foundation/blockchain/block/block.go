package block

import (
	"errors"
	"fmt"
	"time"

	acc "github.com/dudakovict/blockchain/foundation/blockchain/account"
	"github.com/dudakovict/blockchain/foundation/blockchain/merkle"
	"github.com/dudakovict/blockchain/foundation/blockchain/signature"
	"github.com/dudakovict/blockchain/foundation/blockchain/transaction"
)

// ErrChainForked is returned from validateNextBlock if another node's chain
// is two or more blocks ahead of ours.
var ErrChainForked = errors.New("blockchain forked, start resync")

// BlockHeader represents common information required for each block.
type BlockHeader struct {
	Number        uint64        `json:"number"`
	PrevBlockHash string        `json:"prev_block_hash"`
	TimeStamp     uint64        `json:"timestamp"`
	BeneficiaryID acc.AccountID `json:"beneficiary"`
	Difficulty    uint16        `json:"difficulty"`
	MiningReward  uint64        `json:"mining_reward"`
	StateRoot     string        `json:"state_root"` // Ethereum: Represents a hash of the accounts and their balances.
	TransRoot     string        `json:"trans_root"`
	Nonce         uint64        `json:"nonce"`
}

// Block represents a group of transactions batched together.
type Block struct {
	Header     BlockHeader
	MerkleTree *merkle.Tree[transaction.BlockTx]
}

func New(blockHeader BlockHeader, trans []transaction.BlockTx) (Block, error) {
	tree, err := merkle.NewTree(trans)
	if err != nil {
		return Block{}, err
	}

	block := Block{
		Header:     blockHeader,
		MerkleTree: tree,
	}

	return block, nil
}

func (b Block) Hash() string {
	if b.Header.Number == 0 {
		return signature.ZeroHash
	}

	return signature.Hash(b.Header)
}

func (b Block) ValidateBlock(previousBlock Block, stateRoot string) error {
	nextNumber := previousBlock.Header.Number + 1
	if b.Header.Number >= (nextNumber + 2) {
		return ErrChainForked
	}

	if b.Header.Difficulty < previousBlock.Header.Difficulty {
		return fmt.Errorf("block difficulty is less than previous block difficulty, parent %d, block %d", previousBlock.Header.Difficulty, b.Header.Difficulty)
	}

	if b.Header.Number != nextNumber {
		return fmt.Errorf("this block is not the next number, got %d, exp %d", b.Header.Number, nextNumber)
	}

	if b.Header.PrevBlockHash != previousBlock.Hash() {
		return fmt.Errorf("parent block hash doesn't match our known parent, got %s, exp %s", b.Header.PrevBlockHash, previousBlock.Hash())
	}

	if previousBlock.Header.TimeStamp > 0 {
		parentTime := time.Unix(int64(previousBlock.Header.TimeStamp), 0)
		blockTime := time.Unix(int64(b.Header.TimeStamp), 0)
		if blockTime.Before(parentTime) {
			return fmt.Errorf("block timestamp is before parent block, parent %s, block %s", parentTime, blockTime)
		}
	}

	return nil
}
