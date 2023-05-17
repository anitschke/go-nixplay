package nixplay

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
)

type ContainerType string

const (
	AlbumContainerType    = ContainerType("album")
	PlaylistContainerType = ContainerType("playlist")
)

var (
	ErrInvalidContainerType = errors.New("invalid container type")
)

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

// xxx doc
type Container struct {
	ContainerType ContainerType
	Name          string
	ID            uint64
	PhotoCount    uint64 //xxx can I support this?
}

// xxx doc
type Photo struct {
	Name string
	ID   uint64
	Size uint64

	// xxx The MD5 hash is returned in album list but technically not returned in
	// the photos for the playlist, BUT the hash does happened to be encoded in
	// the urls for the playlist so so we can extract it from there.
	MD5Hash MD5Hash
	URL     string
}
