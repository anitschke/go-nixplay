package nixplay

import (
	"context"
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
		err := client.DeleteContainer(context.Background(), container)
		assert.NoError(t, err)
	})

	return container
}

// sanitizePhotoURL clears out portions of the photo URL that can change over
// time so we can directly compare photo objects to each other during testing.
func sanitizePhotoURL(photoURL string) string {
	photoURL = removeSignatureRegexp.ReplaceAllString(photoURL, "")
	photoURL = removeExpiresRegexp.ReplaceAllString(photoURL, "")
	return photoURL
}

func sanitizePhotosURL(photos []Photo) {
	for i, _ := range photos {
		photos[i].URL = sanitizePhotoURL(photos[i].URL)
	}
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
				assert.Contains(t, containers[0].Name, "@mynixplay.com")
				return []string{containers[0].Name}
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
					names = append(names, c.Name)
					assert.Equal(t, c.ContainerType, PlaylistContainerType)
					isEmailPlaylist := strings.HasSuffix(c.Name, "@mynixplay.com")
					if isEmailPlaylist {
						emailPlaylistName = c.Name
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
			assert.Equal(t, container, Container{})

			//////////////////////////
			// Create
			//////////////////////////
			newContainer, err := client.CreateContainer(ctx, tc.containerType, newName)
			assert.NoError(t, err)
			assert.Equal(t, newContainer.Name, newName)
			assert.Equal(t, newContainer.ContainerType, tc.containerType)

			//////////////////////////
			// List
			//////////////////////////
			containers, err = client.Containers(ctx, tc.containerType)
			assert.NoError(t, err)

			getNamesAndCheckContainerType := func(containers []Container) []string {
				names := []string{}
				for _, c := range containers {
					names = append(names, c.Name)
					assert.Equal(t, c.ContainerType, tc.containerType)
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
			err = client.DeleteContainer(ctx, newContainer)
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
			assert.Equal(t, container, Container{})
		})
	}
}

// xxx finish test
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
			photos, err := client.Photos(ctx, container)
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
				p, err := client.AddPhoto(ctx, container, tp.Name, file, AddPhotoOptions{})
				require.NoError(t, err)
				//xxx test that the md5 hash is correct
				//xxx test that the size is correct
				assert.Equal(t, p.Name, tp.Name)
				addedPhotos = append(addedPhotos, p)
			}
			sanitizePhotosURL(addedPhotos)

			//////////////////////////
			// List
			//////////////////////////
			photos, err = client.Photos(ctx, container)
			sanitizePhotosURL(photos)
			assert.NoError(t, err)
			assert.Len(t, photos, len(addedPhotos))
			assert.ElementsMatch(t, photos, addedPhotos)

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
				err := client.DeletePhoto(ctx, p, tc.deleteScope)
				if tc.expDeleteError != nil {
					assert.ErrorIs(t, err, tc.expDeleteError)
					return
				}
				assert.NoError(t, err)

				expPhotos := addedPhotos[i+1:]
				photos, err := client.Photos(ctx, container)
				sanitizePhotosURL(photos)
				assert.NoError(t, err)
				assert.Len(t, photos, len(expPhotos))
				assert.ElementsMatch(t, photos, expPhotos)

				//xxx also check get
			}

			//////////////////////////
			// List
			//////////////////////////
			photos, err = client.Photos(ctx, container)
			assert.NoError(t, err)
			assert.Empty(t, photos)

			//////////////////////////
			// Get
			//////////////////////////
			//xxx TODO
		})
	}
}
