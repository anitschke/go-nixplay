package nixplay

import (
	"context"
	"crypto/md5"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/anitschke/go-nixplay/auth"
	"github.com/anitschke/go-nixplay/test-resources/photos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	removeSignatureRegexp = regexp.MustCompile("Signature=[^&]*&")
	removeExpiresRegexp   = regexp.MustCompile("Expires=[^&]*&")
)

func testClient() *DefaultClient {
	authorization, err := auth.TestAccountAuth()
	if err != nil {
		panic(err)
	}
	client, err := NewDefaultClient(context.Background(), authorization, DefaultClientOptions{})
	if err != nil {
		panic(err)
	}
	return client
}

func randomName() string {
	return strconv.FormatUint(rand.Uint64(), 36)
}

func tempContainer(t *testing.T, client Client, containerType ContainerType) Container {
	name := randomName()
	container, err := client.CreateContainer(context.Background(), containerType, name)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := container.Delete(context.Background())
		assert.NoError(t, err)
	})

	return container
}

type photoData struct {
	name string
	id   ID

	size    int64
	md5Hash MD5Hash
	url     string
}

// sanitizePhotoURL clears out portions of the photo URL that can change over
// time so we can directly compare photo objects to each other during testing.
func sanitizePhotoURL(photoURL string) string {
	photoURL = removeSignatureRegexp.ReplaceAllString(photoURL, "")
	photoURL = removeExpiresRegexp.ReplaceAllString(photoURL, "")
	return photoURL
}

func newPhotoData(photo Photo) (photoData, error) {
	ctx := context.Background()
	data := photoData{
		name: photo.Name(),
		id:   photo.ID(),
	}

	var err error
	data.size, err = photo.Size(ctx)
	if err != nil {
		return photoData{}, err
	}

	data.md5Hash, err = photo.MD5Hash(ctx)
	if err != nil {
		return photoData{}, err
	}

	data.url, err = photo.URL(ctx)
	if err != nil {
		return photoData{}, err
	}
	data.url = sanitizePhotoURL(data.url)

	return data, nil
}

func photoDataSlice(photos []Photo) ([]photoData, error) {
	data := make([]photoData, 0, len(photos))
	for _, p := range photos {
		d, err := newPhotoData(p)
		if err != nil {
			return nil, err
		}
		data = append(data, d)
	}
	return data, nil
}

func TestDefaultClient_Containers_ListGetCreateListGetDeleteListGet(t *testing.T) {
	type testData struct {
		containerType           ContainerType
		verifyInitialContainers func(containers []Container) (initialContainerNames []string)
	}

	tests := []testData{
		{
			containerType: AlbumContainerType,
			verifyInitialContainers: func(containers []Container) []string {
				// By default every nixplay account seems to have one album that can be
				// deleted but seems to come back automatically. This album is the
				// ${username}@mynixplay.com album. So we will check that it exists
				assert.Len(t, containers, 1)
				assert.Contains(t, containers[0].Name(), "@mynixplay.com")
				return []string{containers[0].Name()}
			},
		},
		{
			containerType: PlaylistContainerType,
			verifyInitialContainers: func(containers []Container) []string {
				// By default every nixplay account seems to have two playlists that can not
				// be deleted. These are a playlist for the @mynixplay.com email address and
				// a favorites playlist.
				assert.Len(t, containers, 2)
				var names []string
				var foundEmailPlaylist bool
				var emailPlaylistName string
				for _, c := range containers {
					names = append(names, c.Name())
					assert.Equal(t, c.ContainerType(), PlaylistContainerType)
					isEmailPlaylist := strings.HasSuffix(c.Name(), "@mynixplay.com")
					if isEmailPlaylist {
						emailPlaylistName = c.Name()
					}
					foundEmailPlaylist = foundEmailPlaylist || isEmailPlaylist
				}
				assert.Contains(t, names, "Favorites")
				assert.True(t, foundEmailPlaylist)

				return []string{"Favorites", emailPlaylistName}
			},
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.containerType), func(t *testing.T) {
			ctx := context.Background()
			client := testClient()

			//////////////////////////
			// List
			//////////////////////////
			containers, err := client.Containers(ctx, tc.containerType)
			assert.NoError(t, err)

			initialContainerNames := tc.verifyInitialContainers(containers)

			//////////////////////////
			// Get
			//////////////////////////
			newName := "MyNewContainer"
			container, err := client.Container(ctx, tc.containerType, newName)
			assert.ErrorIs(t, err, ErrContainerNotFound)
			assert.Equal(t, container, nil)

			//////////////////////////
			// Create
			//////////////////////////
			newContainer, err := client.CreateContainer(ctx, tc.containerType, newName)
			assert.NoError(t, err)
			assert.Equal(t, newContainer.Name(), newName)
			assert.Equal(t, newContainer.ContainerType(), tc.containerType)

			//////////////////////////
			// List
			//////////////////////////
			containers, err = client.Containers(ctx, tc.containerType)
			assert.NoError(t, err)

			getNamesAndCheckContainerType := func(containers []Container) []string {
				names := []string{}
				for _, c := range containers {
					names = append(names, c.Name())
					assert.Equal(t, c.ContainerType(), tc.containerType)
				}
				return names
			}

			names := getNamesAndCheckContainerType(containers)
			assert.Len(t, containers, len(initialContainerNames)+1)
			expNames := append([]string{newName}, initialContainerNames...)
			assert.ElementsMatch(t, names, expNames)

			//////////////////////////
			// Get
			//////////////////////////
			container, err = client.Container(ctx, tc.containerType, newName)
			assert.NoError(t, err)
			assert.Equal(t, container, newContainer)

			//////////////////////////
			// Delete
			//////////////////////////
			newContainer.Delete(context.Background())
			assert.NoError(t, err)

			//////////////////////////
			// List
			//////////////////////////
			containers, err = client.Containers(ctx, tc.containerType)
			assert.NoError(t, err)
			assert.Len(t, containers, len(initialContainerNames))
			names = getNamesAndCheckContainerType(containers)
			assert.ElementsMatch(t, names, initialContainerNames)

			//////////////////////////
			// Get
			//////////////////////////
			container, err = client.Container(ctx, tc.containerType, newName)
			assert.ErrorIs(t, err, ErrContainerNotFound)
			assert.Equal(t, container, nil)
		})
	}
}

func TestDefaultClient_Photo_ListAddListGetDownloadDeleteListGet(t *testing.T) {
	type testData struct {
		name           string
		containerType  ContainerType
		deleteScope    DeleteScope
		expDeleteError error
	}

	tests := []testData{
		{
			name:           "AlbumContainerScope",
			containerType:  AlbumContainerType,
			deleteScope:    ContainerDeleteScope,
			expDeleteError: nil,
		},
		{
			name:           "AlbumGlobalScope",
			containerType:  AlbumContainerType,
			deleteScope:    GlobalDeleteScope,
			expDeleteError: errGlobalDeleteScopeNotForAlbums,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := testClient()

			// create temporary container for testing
			container := tempContainer(t, client, tc.containerType)
			allTestPhotos, err := photos.AllPhotos()
			require.NoError(t, err)

			//////////////////////////
			// List
			//////////////////////////
			photos, err := container.Photos(ctx)
			assert.NoError(t, err)
			assert.Empty(t, photos)

			//////////////////////////
			// Add
			//////////////////////////
			addedPhotos := make([]Photo, 0, len(allTestPhotos))
			for _, tp := range allTestPhotos {
				file, err := tp.Open()
				require.NoError(t, err)
				defer file.Close()
				p, err := container.AddPhoto(ctx, tp.Name, file, AddPhotoOptions{})
				require.NoError(t, err)

				// open the file a second time and get the hash
				fileForHash, err := tp.Open()
				require.NoError(t, err)
				defer fileForHash.Close()
				hasher := md5.New()
				io.Copy(hasher, fileForHash)
				md5Hash := MD5Hash(hasher.Sum(nil))

				actSize, err := p.Size(ctx)
				assert.NoError(t, err)
				actMD5, err := p.MD5Hash(ctx)
				assert.NoError(t, err)

				assert.Equal(t, p.Name(), tp.Name)
				assert.Equal(t, actSize, tp.Size)
				assert.Equal(t, actMD5, md5Hash)
				addedPhotos = append(addedPhotos, p)
			}
			addedPhotoData, err := photoDataSlice(addedPhotos)
			assert.NoError(t, err)

			//////////////////////////
			// List
			//////////////////////////
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Len(t, photos, len(addedPhotos))

			photosData, err := photoDataSlice(photos)
			assert.NoError(t, err)
			assert.ElementsMatch(t, photosData, addedPhotoData)

			//////////////////////////
			// Get
			//////////////////////////
			// xxx TODO

			//////////////////////////
			// Download
			//////////////////////////
			// xxx TODO

			//////////////////////////
			// Delete
			//////////////////////////
			for i, p := range addedPhotos {
				err := p.Delete(ctx, tc.deleteScope)
				if tc.expDeleteError != nil {
					assert.ErrorIs(t, err, tc.expDeleteError)
					return
				}
				assert.NoError(t, err)

				expPhotoData := addedPhotoData[i+1:]
				photos, err := container.Photos(ctx)
				assert.NoError(t, err)
				assert.Len(t, photos, len(addedPhotos)-i-1)

				photosData, err = photoDataSlice(photos)
				assert.NoError(t, err)
				assert.ElementsMatch(t, photosData, expPhotoData)

				//xxx also check get
			}

			//////////////////////////
			// List
			//////////////////////////
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Empty(t, photos)

			//////////////////////////
			// Get
			//////////////////////////
			//xxx TODO
		})
	}
}
