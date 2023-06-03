package types

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

//xxx doc 
type ContainerType string

const (
	AlbumContainerType    = ContainerType("album")
	PlaylistContainerType = ContainerType("playlist")
)

var (
	ErrInvalidContainerType = errors.New("invalid container type")
)

//xxx doc
type ID [IDSize]byte
const IDSize = sha256.Size


type MD5Hash [md5.Size]byte

func (hash *MD5Hash) UnmarshalText(data []byte) error {
	if hex.DecodedLen(len(data)) != md5.Size {
		return fmt.Errorf("invalid md5 hash length")
	}
	_, err := hex.Decode(hash[:], data)
	if err != nil {
		return fmt.Errorf("failed to decode md5 hash: %w", err)
	}
	return nil
}
