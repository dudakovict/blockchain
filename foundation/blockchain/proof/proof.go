package proof

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"time"

	acc "github.com/dudakovict/blockchain/foundation/blockchain/account"
	"github.com/dudakovict/blockchain/foundation/blockchain/block"
	"github.com/dudakovict/blockchain/foundation/blockchain/merkle"
	"github.com/dudakovict/blockchain/foundation/blockchain/signature"
	"github.com/dudakovict/blockchain/foundation/blockchain/transaction"
)

// POWArgs represents the set of arguments required to run POW.
type POWArgs struct {
	BeneficiaryID acc.AccountID
	Difficulty    uint16
	MiningReward  uint64
	PrevBlock     block.Block
	StateRoot     string
	Trans         []transaction.BlockTx
}

// POW constructs a new Block and performs the work to find a nonce that
// solves the cryptographic POW puzzle.
func POW(ctx context.Context, args POWArgs) (block.Block, error) {

	// When mining the first block, the previous block's hash will be zero.
	prevBlockHash := signature.ZeroHash
	if args.PrevBlock.Header.Number > 0 {
		prevBlockHash = args.PrevBlock.Hash()
	}

	// Construct a merkle tree from the transaction for this block. The root
	// of this tree will be part of the block to be mined.
	tree, err := merkle.NewTree(args.Trans)
	if err != nil {
		return block.Block{}, err
	}

	// Construct the block to be mined.
	b := block.Block{
		Header: block.BlockHeader{
			Number:        args.PrevBlock.Header.Number + 1,
			PrevBlockHash: prevBlockHash,
			TimeStamp:     uint64(time.Now().UTC().UnixMilli()),
			BeneficiaryID: args.BeneficiaryID,
			Difficulty:    args.Difficulty,
			MiningReward:  args.MiningReward,
			StateRoot:     args.StateRoot,
			TransRoot:     tree.RootHex(),
			Nonce:         0,
		},
		MerkleTree: tree,
	}

	// Peform the proof of work mining operation.
	if err := performPOW(ctx, b); err != nil {
		return block.Block{}, err
	}

	return b, nil
}

// performPOW does the work of mining to find a valid hash for a specified
// block. Pointer semantics are being used since a nonce is being discovered.
func performPOW(ctx context.Context, b block.Block) error {
	// Choose a random starting point for the nonce. After this, the nonce
	// will be incremented by 1 until a solution is found by us or another node.
	nBig, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return ctx.Err()
	}
	b.Header.Nonce = nBig.Uint64()

	// Loop until we or another node finds a solution for the next block.
	var attempts uint64
	for {
		attempts++
		// Did we timeout trying to solve the problem.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Hash the block and check if we have solved the puzzle.
		hash := b.Hash()
		if !isHashSolved(b.Header.Difficulty, hash) {
			b.Header.Nonce++
			continue
		}

		return nil
	}
}

// isHashSolved checks the hash to make sure it complies with
// the POW rules. We need to match a difficulty number of 0's.
func isHashSolved(difficulty uint16, hash string) bool {
	const match = "0x00000000000000000"

	if len(hash) != 66 {
		return false
	}

	difficulty += 2
	return hash[:difficulty] == match[:difficulty]
}
