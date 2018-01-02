package merkle

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/spacemeshos/go-spacemesh/crypto"
	"github.com/spacemeshos/go-spacemesh/log"
)

var InvalidUserDataError = errors.New("expected non-empty k,v for user data")

// store user data (k,v)

func (mt *merkleTreeImp) Put(k, v []byte) error {

	if len(v) == 0 || len(k) == 0 {
		return InvalidUserDataError
	}

	// calc the user value to store in the merkle tree
	var userValue []byte
	if len(v) > 32 {
		// if v is long we persist it in the user db and store a hash to it (its user-db key) in the merkle tree
		err := mt.persistUserValue(v)
		if err != nil {
			return err
		}
		userValue = crypto.Sha256(v)
	} else {
		// v is short - we just store it in the merkle tree
		userValue = v
	}

	// first, attempt to find the value in the tree and return path to where value should be added
	// in the case it is not already in the tree
	res, stack , err := mt.Get(k)

	if res != nil && bytes.Equal(res,v) {
		// value already stored in db
		log.Info("Value already stored in tree")
		return nil
	}

	hexKey := hex.EncodeToString(k)
	log.Info("m Inserting user data for key: %s...", hexKey)

	err = mt.upsert(0, hexKey, userValue, stack)
	if err != nil {
		return err
	}

	if mt.root == nil {
		nodes := stack.toSlice()
		mt.root = nodes[0]
	}

	return nil
}

// Upserts (updates or inserts) (k,v) to the tree
// k: hex-encoded value's key (always abs full path)
// pos: number of nibbles already matched on k to node on top of the stack
// s: tree path from root to where the value should be updated in the tree
// Returns error if failed to upset the v, nil otherwise
func (mt *merkleTreeImp) upsert(pos int, k string, v []byte, s *stack) error {

	// empty tree - add k,v as leaf
	if s.Len() == 0 {
		newLeaf, err := newLeafNodeContainer(k, v)
		if err != nil {
			return err
		}
		s.push(newLeaf)

		// todo: save sll nodes on stack
		return nil
	}

	lastNode := s.pop()

	if lastNode.isLeaf() {
		l := 0
		items := s.toSlice()
		for _, n := range items {
			if n.isBranch() {
				l++
			} else {
				l += len(n.getShortNode().getPath())
			}
		}

		lastNodePath := lastNode.getShortNode().getPath()
		cp := commonPrefix(lastNodePath, k[:l])

		if len(cp) == len(lastNodePath) && pos == len(k) {

			lastNode.getShortNode().setValue(v)
			s.push(lastNode)

			// todo: save all modified nodes in the stack
			return nil
		}
	}

	if lastNode.isBranch() {
		s.push(lastNode)
		if pos < len(k) {
			pos++
			newLeaf, err := newLeafNodeContainer(k[pos:], v)
			if err != nil {
				return err
			}
			s.push(newLeaf)

		} else {
			// todo: set branch node value to v here !!!
		}

		// todo: save all modified nodes in the stack
		return nil
	}

	// lastNode is ext or leaf

	lastNodePath := lastNode.getShortNode().getPath()
	cp := commonPrefix(lastNodePath, k[pos:])
	cpl := len(cp)

	if cpl > 0 {
		key := lastNodePath[:cpl]
		newExtNode, err := newExtNodeContainer(key, []byte{})
		if err != nil {
			return err
		}
		s.push(newExtNode)

		if cpl < len(lastNodePath) {
			lastNodePath = lastNodePath[cpl:]
		} else {
			lastNodePath = ""
		}
		pos += cpl
	}

	newBranch, err := newBranchNodeContainer(nil, nil)
	if err != nil {
		return err
	}
	s.push(newBranch)

	if len(lastNodePath) > 0 {
		branchKey := string(lastNodePath[0])
		lastNodePath = lastNodePath[1:]

		if len(lastNodePath) > 0 || lastNode.isLeaf() {
			// shrink ext or leaf
			lastNode.getShortNode().setPath(lastNodePath)
			newBranch.addBranchChild(branchKey, lastNode)
		} else {
			// remove ext
			newBranch.getBranchNode().setValue(lastNode.getShortNode().getValue())
		}
	} else {
		newBranch.getBranchNode().setValue(lastNode.getShortNode().getValue())
	}

	if pos < len(k) {
		pos++
		// add new leaf to branch node
		newLeaf, err := newLeafNodeContainer(k[pos:], v)
		if err != nil {
			return err
		}
		s.push(newLeaf)
	}

	// todo: save stack

	return nil
}
