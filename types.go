package nixplay

import (
	"crypto/md5"
	"crypto/sha256"
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

const IDSize = sha256.Size

type ID [IDSize]byte

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

// xxx remove
type ContainerOLD struct {
	ContainerType ContainerType
	Name          string
	ID            uint64
	PhotoCount    uint64
}

// xxx remove
type PhotoOLD struct {
	Name string

	// xxx it seems I have been having a lot of issues around ID. Playlist
	// photos don't seem to have the same sort of ID that we have for albums and
	// we don't know what the ID of a photo is once it has been uploaded. So I
	// think I need to change this and use my own ID. Perhaps do something like
	// do a hash combine of MD5Hash and the album ID for the id since we know
	// both of those at upload time AND nixplay doesn't allow the same photo in
	// an album more than once.
	ID   uint64
	Size uint64

	// xxx The MD5 hash is returned in album list but technically not returned in
	// the photos for the playlist, BUT the hash does happened to be encoded in
	// the urls for the playlist so so we can extract it from there.
	MD5Hash MD5Hash
	URL     string

	// xxx doc info needed for delete
	// xxx populate this data on get and create
	parentContainerType     ContainerType
	internalPlaylistMediaID string
}
