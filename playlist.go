package nixplay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/anitschke/go-nixplay/httpx"
)

type playlist struct {
	name       string
	id         ID
	photoCount int64

	authClient httpx.Client
	client     httpx.Client
	nixplayID  uint64
}

func newPlaylist(authClient httpx.Client, client httpx.Client, name string, nixplayID uint64, photoCount int64) *playlist {
	var id ID
	binary.LittleEndian.PutUint64(id[:], nixplayID)
	id = sha256.Sum256(id[:])

	return &playlist{
		authClient: authClient,
		client:     client,
		name:       name,
		id:         id,
		nixplayID:  nixplayID,
		photoCount: photoCount,
	}
}

var _ = (Container)((*playlist)(nil))

func (p *playlist) ContainerType() ContainerType {
	return PlaylistContainerType
}

func (p *playlist) Name() string {
	return p.name
}

func (p *playlist) ID() ID {
	return p.id
}

func (p *playlist) PhotoCount(ctx context.Context) (int64, error) {
	return p.photoCount, nil
}

func (p *playlist) Delete(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to delete album: %w", err)
		}
	}()

	url := fmt.Sprintf("https://api.nixplay.com/v3/playlists/%d", p.nixplayID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}
	resp, err := p.authClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err = io.ReadAll(resp.Body); err != nil {
		return err
	}
	if err = httpx.StatusError(resp); err != nil {
		return err
	}
	return nil
}

func (p *playlist) Photos(ctx context.Context) (retPhotos []Photo, err error) {
	panic("not implemented") // xxx: Implement
}

func (p *playlist) AddPhoto(ctx context.Context, name string, r io.Reader, opts AddPhotoOptions) (Photo, error) {
	albumID := uploadContainerID{
		idName: "playlistId",
		id:     strconv.FormatUint(p.nixplayID, 10),
	}

	photoData, err := addPhoto(ctx, p.authClient, p.client, albumID, name, r, opts)
	if err != nil {
		return nil, err
	}

	nixplayPhotoID := uint64(0)
	photoURL := ""

	panic("not implemented") // xxx: Implement playlistPhoto
	return newAlbumPhoto(p, p.authClient, p.client, name, photoData.md5Hash, nixplayPhotoID, photoData.size, photoURL), nil
}
