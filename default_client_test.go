package nixplay

import (
	"bytes"
	"context"
	"crypto/md5"
	"image/jpeg"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"testing"

	"github.com/anitschke/go-nixplay/internal/auth"
	"github.com/anitschke/go-nixplay/internal/test-resources/photos"
	"github.com/anitschke/go-nixplay/types"
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

func tempContainer(t *testing.T, client Client, containerType types.ContainerType) Container {
	name := randomName()
	container, err := client.CreateContainer(context.Background(), containerType, name)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := container.Delete(context.Background())
		assert.NoError(t, err)
	})

	return container
}

type containerData struct {
	containerType types.ContainerType
	name          string
	id            types.ID
	photoCount    int64
}

func newContainerData(c Container) (containerData, error) {
	photoCount, err := c.PhotoCount(context.Background())
	if err != nil {
		return containerData{}, err
	}
	name, err := c.Name(context.Background())
	if err != nil {
		return containerData{}, err
	}
	return containerData{
		containerType: c.ContainerType(),
		name:          name,
		id:            c.ID(),
		photoCount:    photoCount,
	}, nil
}

type photoData struct {
	name string
	id   types.ID

	size    int64
	md5Hash types.MD5Hash
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
		id: photo.ID(),
	}

	var err error
	data.name, err = photo.Name(ctx)
	if err != nil {
		return photoData{}, err
	}

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

func addMyUploadsCleanup(t *testing.T, c Client) {
	t.Cleanup(func() {
		deleteAllPhotosFromMyUploads(t, c)
	})
}

func deleteAllPhotosFromMyUploads(t *testing.T, c Client) {
	ctx := context.Background()
	myUploads, err := c.ContainersWithName(ctx, types.AlbumContainerType, "My Uploads")
	require.Len(t, myUploads, 1)
	require.NoError(t, err)

	photos, err := myUploads[0].Photos(ctx)
	require.NoError(t, err)

	for _, p := range photos {
		err := p.Delete(ctx)
		assert.NoError(t, err)
	}
}

func TestDefaultClient_Containers(t *testing.T) {

	auth, err := auth.TestAccountAuth()
	require.NoError(t, err)

	type testData struct {
		containerType           types.ContainerType
		verifyInitialContainers func(containers []Container) (initialContainerNames []string)
	}

	tests := []testData{
		{
			containerType: types.AlbumContainerType,
			verifyInitialContainers: func(containers []Container) []string {
				// By default every nixplay account seems to have two albums.
				// This album is the ${username}@mynixplay.com album. The other
				// is a "My Uploads" album.
				assert.Len(t, containers, 2)

				var names []string
				for _, c := range containers {
					name, err := c.Name(context.Background())
					assert.NoError(t, err)
					names = append(names, name)
				}

				expNames := []string{auth.Username + "@mynixplay.com", "My Uploads"}
				assert.ElementsMatch(t, names, expNames)

				return names
			},
		},
		{
			containerType: types.PlaylistContainerType,
			verifyInitialContainers: func(containers []Container) []string {
				// By default every nixplay account seems to have two playlists.
				// These are a playlist for the @mynixplay.com email address and
				// a favorites playlist.
				assert.Len(t, containers, 2)

				var names []string
				for _, c := range containers {
					name, err := c.Name(context.Background())
					assert.NoError(t, err)
					names = append(names, name)
				}

				expNames := []string{auth.Username + "@mynixplay.com", "Favorites"}
				assert.ElementsMatch(t, names, expNames)

				return names
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
			containers, err = client.ContainersWithName(ctx, tc.containerType, newName)
			assert.NoError(t, err)
			assert.Len(t, containers, 0)

			//////////////////////////
			// Create
			//////////////////////////
			newContainer, err := client.CreateContainer(ctx, tc.containerType, newName)
			assert.NoError(t, err)
			name, err := newContainer.Name(ctx)
			assert.NoError(t, err)
			assert.Equal(t, name, newName)
			assert.Equal(t, newContainer.ContainerType(), tc.containerType)

			photoCount, err := newContainer.PhotoCount(ctx)
			assert.NoError(t, err)
			assert.Equal(t, photoCount, int64(0))

			newContainerD, err := newContainerData(newContainer)
			assert.NoError(t, err)

			//////////////////////////
			// List
			//////////////////////////
			containers, err = client.Containers(ctx, tc.containerType)
			assert.NoError(t, err)

			getNamesAndCheckContainerType := func(containers []Container) []string {
				names := []string{}
				for _, c := range containers {
					name, err := c.Name(ctx)
					assert.NoError(t, err)
					names = append(names, name)
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
			containers, err = client.ContainersWithName(ctx, tc.containerType, newName)
			require.Len(t, containers, 1)
			assert.NoError(t, err)

			containerD, err := newContainerData(containers[0])
			assert.NoError(t, err)
			assert.Equal(t, containerD, newContainerD)

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
			containers, err = client.ContainersWithName(ctx, tc.containerType, newName)
			assert.NoError(t, err)
			assert.Len(t, containers, 0)

			//////////////////////////
			// Reset Cache and List
			//////////////////////////
			client.ResetCache()
			containers, err = client.Containers(ctx, tc.containerType)
			assert.NoError(t, err)
			assert.Len(t, containers, len(initialContainerNames))
			names = getNamesAndCheckContainerType(containers)
			assert.ElementsMatch(t, names, initialContainerNames)

			//////////////////////////
			// Reset Cache and Get
			//////////////////////////
			client.ResetCache()
			containers, err = client.ContainersWithName(ctx, tc.containerType, newName)
			assert.NoError(t, err)
			assert.Len(t, containers, 0)

		})
	}
}

func TestDefaultClient_DuplicateContainerName(t *testing.T) {
	// See comments in ContainersWithName for details about how nixplay support
	// for duplicate containers with same name.

	type testData struct {
		containerType types.ContainerType
	}

	tests := []testData{
		{
			containerType: types.AlbumContainerType,
		},
		{
			containerType: types.PlaylistContainerType,
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.containerType), func(t *testing.T) {
			ctx := context.Background()
			client := testClient()

			name := randomName()

			//////////////////////////
			// List
			//////////////////////////
			containers, err := client.ContainersWithName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Empty(t, containers)

			container, err := client.ContainerWithUniqueName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Nil(t, container)

			//////////////////////////
			// Create First Container
			//////////////////////////
			container1, err := client.CreateContainer(ctx, tc.containerType, name)
			assert.NoError(t, err)

			actName, err := container1.Name(ctx)
			assert.NoError(t, err)
			assert.Equal(t, actName, name)

			actUniqueName1, err := container1.NameUnique(ctx)
			assert.NoError(t, err)
			assert.Equal(t, actUniqueName1, name)

			containers, err = client.ContainersWithName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Equal(t, containers, []Container{container1})

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Equal(t, container, container1)

			//////////////////////////
			// Create Second Container
			//////////////////////////
			container2, err := client.CreateContainer(ctx, tc.containerType, name)
			assert.NoError(t, err)

			assert.NotEqual(t, container1.ID(), container2.ID())

			actName, err = container2.Name(ctx)
			assert.NoError(t, err)
			assert.Equal(t, actName, name)

			actName, err = container1.Name(ctx)
			assert.NoError(t, err)
			assert.Equal(t, actName, name)

			actUniqueName2, err := container2.NameUnique(ctx)
			assert.NoError(t, err)
			assert.NotEqual(t, actUniqueName2, name)

			actUniqueName1, err = container1.NameUnique(ctx)
			assert.NoError(t, err)
			assert.NotEqual(t, actUniqueName1, name)

			assert.NotEqual(t, actUniqueName1, actUniqueName2)

			getContainerIDs := func(containers []Container) []types.ID {
				ids := make([]types.ID, 0, len(containers))
				for _, c := range containers {
					ids = append(ids, c.ID())
				}
				return ids
			}

			containers, err = client.ContainersWithName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.ElementsMatch(t, containers, []Container{container1, container2})
			assert.ElementsMatch(t, getContainerIDs(containers), getContainerIDs([]Container{container1, container2}))

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Nil(t, container)

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, actUniqueName1)
			assert.NoError(t, err)
			assert.Equal(t, container.ID(), container1.ID())

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, actUniqueName2)
			assert.NoError(t, err)
			assert.Equal(t, container.ID(), container2.ID())

			client.ResetCache()

			containers, err = client.ContainersWithName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.ElementsMatch(t, getContainerIDs(containers), getContainerIDs([]Container{container1, container2}))

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Nil(t, container)

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, actUniqueName1)
			assert.NoError(t, err)
			assert.Equal(t, container.ID(), container1.ID())

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, actUniqueName2)
			assert.NoError(t, err)
			assert.Equal(t, container.ID(), container2.ID())

			//////////////////////////
			// Delete Second Container
			//////////////////////////
			assert.NoError(t, container2.Delete(ctx))

			actName, err = container1.Name(ctx)
			assert.NoError(t, err)
			assert.Equal(t, actName, name)

			actUniqueName1, err = container1.NameUnique(ctx)
			assert.NoError(t, err)
			assert.Equal(t, actUniqueName1, name)

			containers, err = client.ContainersWithName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.ElementsMatch(t, getContainerIDs(containers), getContainerIDs([]Container{container1}))

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Equal(t, container.ID(), container1.ID())

			//////////////////////////
			// Delete First Container
			//////////////////////////
			assert.NoError(t, container1.Delete(ctx))

			containers, err = client.ContainersWithName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Empty(t, containers)

			client.ResetCache()

			containers, err = client.ContainersWithName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Empty(t, containers)

			container, err = client.ContainerWithUniqueName(ctx, tc.containerType, name)
			assert.NoError(t, err)
			assert.Nil(t, container)
		})
	}
}

func TestDefaultClient_Photos(t *testing.T) {
	type testData struct {
		containerType types.ContainerType
	}

	tests := []testData{
		{
			containerType: types.AlbumContainerType,
		},
		{
			containerType: types.PlaylistContainerType,
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.containerType), func(t *testing.T) {
			ctx := context.Background()
			client := testClient()
			addMyUploadsCleanup(t, client)

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
			photoNames := make([]string, 0, len(allTestPhotos))
			photoIDs := make([]types.ID, 0, len(allTestPhotos))
			for i, tp := range allTestPhotos {
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
				md5Hash := *(*types.MD5Hash)(hasher.Sum(nil))

				actSize, err := p.Size(ctx)
				assert.NoError(t, err)
				actMD5, err := p.MD5Hash(ctx)
				assert.NoError(t, err)

				name, err := p.Name(ctx)
				assert.NoError(t, err)
				assert.Equal(t, name, tp.Name)

				assert.Equal(t, actSize, tp.Size)
				assert.Equal(t, actMD5, md5Hash)

				photoCount, err := container.PhotoCount(ctx)
				assert.NoError(t, err)
				assert.Equal(t, photoCount, int64(i+1))

				addedPhotos = append(addedPhotos, p)
				photoNames = append(photoNames, tp.Name)
				photoIDs = append(photoIDs, p.ID())
			}
			addedPhotoData, err := photoDataSlice(addedPhotos)
			assert.NoError(t, err)

			//////////////////////////
			// Validate ID uniqueness
			//////////////////////////
			idMap := make(map[types.ID]struct{}, len(addedPhotoData))
			for _, d := range addedPhotoData {
				idMap[d.id] = struct{}{}
			}
			assert.Equal(t, len(idMap), len(addedPhotoData))

			//////////////////////////
			// List
			//////////////////////////
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Len(t, photos, len(addedPhotos))

			photosData, err := photoDataSlice(photos)
			assert.NoError(t, err)
			assert.ElementsMatch(t, photosData, addedPhotoData)

			photoCount, err := container.PhotoCount(ctx)
			assert.NoError(t, err)
			assert.Equal(t, photoCount, int64(len(addedPhotos)))

			//////////////////////////
			// Reset Cache And List
			//////////////////////////
			container.ResetCache()
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Len(t, photos, len(addedPhotos))

			photosData, err = photoDataSlice(photos)
			assert.NoError(t, err)
			assert.ElementsMatch(t, photosData, addedPhotoData)

			photoCount, err = container.PhotoCount(ctx)
			assert.NoError(t, err)
			assert.Equal(t, photoCount, int64(len(addedPhotos)))

			//////////////////////////
			// Get Photo By ID
			//////////////////////////
			for i, id := range photoIDs {
				pWithId, err := container.PhotoWithID(ctx, id)
				assert.NoError(t, err)
				pWithIdData, err := newPhotoData(pWithId)
				assert.NoError(t, err)
				assert.Equal(t, addedPhotoData[i], pWithIdData)
			}

			//////////////////////////
			// Get Photos By Name
			//////////////////////////
			for i, name := range photoNames {
				photosWithName, err := container.PhotosWithName(ctx, name)
				assert.NoError(t, err)
				require.Len(t, photosWithName, 1)
				pWithNameData, err := newPhotoData(photosWithName[0])
				assert.NoError(t, err)
				assert.Equal(t, addedPhotoData[i], pWithNameData)
			}

			//////////////////////////
			// Download
			//////////////////////////
			for i, p := range addedPhotos {
				tp := allTestPhotos[i]

				var downloadedPhotoBytes bytes.Buffer
				func() {
					r, err := p.Open(ctx)
					require.NoError(t, err)
					defer func() {
						err := r.Close()
						assert.NoError(t, err)
					}()
					bytesCopied, err := io.Copy(&downloadedPhotoBytes, r)
					require.NoError(t, err)
					assert.Equal(t, bytesCopied, tp.Size)
				}()

				var localPhotoBytes bytes.Buffer
				func() {
					r, err := tp.Open()
					require.NoError(t, err)
					defer func() {
						err := r.Close()
						assert.NoError(t, err)
					}()
					bytesCopied, err := io.Copy(&localPhotoBytes, r)
					require.NoError(t, err)
					assert.Equal(t, bytesCopied, tp.Size)
				}()

				assert.Equal(t, downloadedPhotoBytes.Bytes(), localPhotoBytes.Bytes())

				// Validate that both of the buffers are actually valid jpeg
				// images
				_, err := jpeg.Decode(&downloadedPhotoBytes)
				assert.NoError(t, err)
				_, err = jpeg.Decode(&localPhotoBytes)
				assert.NoError(t, err)
			}

			//////////////////////////
			// Delete
			//////////////////////////
			for i, p := range addedPhotos {
				err := p.Delete(ctx)
				assert.NoError(t, err)

				expPhotoData := addedPhotoData[i+1:]
				photos, err := container.Photos(ctx)
				assert.NoError(t, err)
				assert.Len(t, photos, len(addedPhotos)-i-1)

				photosData, err = photoDataSlice(photos)
				assert.NoError(t, err)
				assert.ElementsMatch(t, photosData, expPhotoData)

				photoCount, err := container.PhotoCount(ctx)
				assert.NoError(t, err)
				assert.Equal(t, photoCount, int64(len(addedPhotos)-i-1))
			}

			//////////////////////////
			// List
			//////////////////////////
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Empty(t, photos)

			photoCount, err = container.PhotoCount(ctx)
			assert.NoError(t, err)
			assert.Equal(t, photoCount, int64(0))

			//////////////////////////
			// Get Photo By ID
			//////////////////////////
			for _, id := range photoIDs {
				pWithId, err := container.PhotoWithID(ctx, id)
				assert.NoError(t, err)
				assert.Nil(t, pWithId)
			}

			//////////////////////////
			// Get Photos By Name
			//////////////////////////
			for _, name := range photoNames {
				photosWithName, err := container.PhotosWithName(ctx, name)
				assert.NoError(t, err)
				assert.Empty(t, photosWithName)
			}
		})
	}
}

func TestDefaultClient_SamePhotoInTwoContainers(t *testing.T) {
	// This test exists because I have had some issues with trying to upload the
	// same image into two different playlists, I think because under the hood
	// they are uploaded into the same "My Uploads" album and nixplay doesn't
	// allow multiple photos with the same content to exist in the same album.

	type testData struct {
		containerType types.ContainerType
	}

	tests := []testData{
		{
			containerType: types.AlbumContainerType,
		},
		{
			containerType: types.PlaylistContainerType,
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.containerType), func(t *testing.T) {
			ctx := context.Background()
			client := testClient()
			addMyUploadsCleanup(t, client)

			// create temporary container1 for testing
			container1 := tempContainer(t, client, tc.containerType)
			container2 := tempContainer(t, client, tc.containerType)
			allTestPhotos, err := photos.AllPhotos()
			require.NoError(t, err)
			photoToUpload := allTestPhotos[0]

			//////////////////////////
			// Add To First Container
			//////////////////////////
			upload := func(c Container) Photo {
				file, err := photoToUpload.Open()
				require.NoError(t, err)
				defer file.Close()
				p, err := c.AddPhoto(ctx, photoToUpload.Name, file, AddPhotoOptions{})
				require.NoError(t, err)
				return p
			}

			p1 := upload(container1)

			//////////////////////////
			// Add To Second Container
			//////////////////////////
			p2 := upload(container2)

			//////////////////////////
			// Validate ID uniqueness
			//////////////////////////
			assert.NotEqual(t, p1.ID(), p2.ID())

			//////////////////////////
			// Verify the photos really are in both containers
			//////////////////////////
			container1.ResetCache()
			container2.ResetCache()

			p1Check, err := container1.PhotoWithID(ctx, p1.ID())
			assert.NoError(t, err)
			require.NotNil(t, p1Check)
			assert.Equal(t, p1Check.ID(), p1.ID())

			p2Check, err := container2.PhotoWithID(ctx, p2.ID())
			assert.NoError(t, err)
			require.NotNil(t, p2Check)
			assert.Equal(t, p2Check.ID(), p2.ID())

			//////////////////////////
			// Download Photo in First Container
			//////////////////////////
			var localPhotoBytes bytes.Buffer
			func() {
				r, err := photoToUpload.Open()
				require.NoError(t, err)
				defer func() {
					err := r.Close()
					assert.NoError(t, err)
				}()
				bytesCopied, err := io.Copy(&localPhotoBytes, r)
				require.NoError(t, err)
				assert.Equal(t, bytesCopied, photoToUpload.Size)
			}()

			verifyDownload := func(p Photo) {
				var downloadedPhotoBytes bytes.Buffer
				r, err := p.Open(ctx)
				require.NoError(t, err)
				defer func() {
					err := r.Close()
					assert.NoError(t, err)
				}()
				bytesCopied, err := io.Copy(&downloadedPhotoBytes, r)
				require.NoError(t, err)
				assert.Equal(t, bytesCopied, photoToUpload.Size)

				assert.Equal(t, downloadedPhotoBytes.Bytes(), localPhotoBytes.Bytes())
			}
			verifyDownload(p1)

			//////////////////////////
			// Download Photo in Second Container
			//////////////////////////
			verifyDownload(p2)

			//////////////////////////
			// Delete
			//////////////////////////
			// Delete from the photos in the containers should happen
			// individually. If we delete the photo from container1 the photo
			// should remain in container2
			err = p1.Delete(ctx)
			assert.NoError(t, err)

			container2.ResetCache()
			p2Check, err = container2.PhotoWithID(ctx, p2.ID())
			assert.NoError(t, err)
			require.NotNil(t, p2Check)
			assert.Equal(t, p2Check.ID(), p2.ID())

			err = p2.Delete(ctx)
			assert.NoError(t, err)

			container1.ResetCache()
			container2.ResetCache()

			p1Check, err = container1.PhotoWithID(ctx, p1.ID())
			assert.NoError(t, err)
			require.Nil(t, p1Check)

			p2Check, err = container2.PhotoWithID(ctx, p2.ID())
			assert.NoError(t, err)
			require.Nil(t, p2Check)
		})
	}
}

func TestDefaultClient_DuplicatePhotoNameInSameContainer(t *testing.T) {
	type testData struct {
		containerType types.ContainerType
	}

	tests := []testData{
		{
			containerType: types.AlbumContainerType,
		},
		{
			containerType: types.PlaylistContainerType,
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.containerType), func(t *testing.T) {
			ctx := context.Background()
			client := testClient()
			addMyUploadsCleanup(t, client)

			// create temporary container for testing
			container := tempContainer(t, client, tc.containerType)
			allTestPhotos, err := photos.AllPhotos()
			require.NoError(t, err)

			// We only need to test this with at few photos
			nonUniqueName := "nonUniqueName.jpg"
			allTestPhotos = allTestPhotos[:3]
			assert.Len(t, allTestPhotos, 3)

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
			for i, tp := range allTestPhotos {
				file, err := tp.Open()
				require.NoError(t, err)
				defer file.Close()
				p, err := container.AddPhoto(ctx, nonUniqueName, file, AddPhotoOptions{})
				require.NoError(t, err)

				name, err := p.Name(ctx)
				assert.NoError(t, err)
				assert.Equal(t, name, nonUniqueName)

				// If this is the first time the loop is running then there are
				// no other photos with this name so unique name should return
				// the same thing as name
				if i == 0 {
					uniqueName, err := p.NameUnique(ctx)
					assert.NoError(t, err)
					assert.Equal(t, uniqueName, nonUniqueName)
				}

				addedPhotos = append(addedPhotos, p)
			}

			// Now double check now that all photos have a unique name
			var uniqueNames []string
			uniqueNameSet := make(map[string]struct{})
			for _, p := range addedPhotos {
				uniqueName, err := p.NameUnique(ctx)
				assert.NoError(t, err)
				assert.NotEqual(t, uniqueName, nonUniqueName)
				uniqueNames = append(uniqueNames, uniqueName)
				uniqueNameSet[uniqueName] = struct{}{}
			}
			assert.Len(t, uniqueNameSet, 3)

			//////////////////////////
			// Get Photos By Name
			//////////////////////////
			photosWithName, err := container.PhotosWithName(ctx, nonUniqueName)
			assert.NoError(t, err)
			require.Len(t, photosWithName, 3)

			//////////////////////////
			// Get Photos By UniqueName
			//////////////////////////
			for i, uName := range uniqueNames {
				p, err := container.PhotoWithUniqueName(ctx, uName)
				assert.NoError(t, err)
				require.Equal(t, p, addedPhotos[i])
			}

			//////////////////////////
			// Delete
			//////////////////////////
			err = addedPhotos[0].Delete(ctx)
			assert.NoError(t, err)
			err = addedPhotos[1].Delete(ctx)
			assert.NoError(t, err)

			// Now that we have deleted 2 of the 3 photos with the same name the
			// third should no longer need a unique name.
			name, err := addedPhotos[2].Name(ctx)
			assert.NoError(t, err)
			assert.Equal(t, name, nonUniqueName)

			uName, err := addedPhotos[2].NameUnique(ctx)
			assert.NoError(t, err)
			assert.Equal(t, uName, nonUniqueName)

			photos, err = container.PhotosWithName(ctx, nonUniqueName)
			assert.NoError(t, err)
			assert.Len(t, photos, 1)
			assert.Equal(t, addedPhotos[2].ID(), photos[0].ID())

			p, err := container.PhotoWithUniqueName(ctx, nonUniqueName)
			assert.NoError(t, err)
			assert.Equal(t, addedPhotos[2].ID(), p.ID())

			for _, p := range addedPhotos {
				err := p.Delete(ctx)
				assert.NoError(t, err)
			}

			//////////////////////////
			// List
			//////////////////////////
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Empty(t, photos)
		})
	}
}

func TestDefaultClient_UnusualFileNames(t *testing.T) {
	// TestDefaultClient_UnusualNames tests that we are able to handle a series
	// of unusual names for the names of containers and images. The expectation
	// is that if Nixplay is not able to handle these names then the names will
	// be encoded so that on a round trip go-nixplay still sees them as the same
	// name that was originally provided.

	type testData struct {
		description string
		name        string
	}

	// This list of encoding decoding was originally pulled from the rclone
	// encoding decode test suite and more cases where added such as unicode
	// characters.
	tests := []testData{
		{"number", "1234"},
		{"quote", `"`},
		{"backslash", `\`},
		{"control chars", "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0A\x0B\x0C\x0D\x0E\x0F\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1A\x1B\x1C\x1D\x1E\x1F\x7F"},
		{"dot", "."},
		{"dot dot", ".."},
		{"punctuation", "!\"#$%&'()*+,-./\\:;<=>?@[\\]^_`{|}~"},
		{"leading trailing space", " space "},
		{"leading trailing tilde", "~tilde~"},
		{"leading trailing quote", `"quote"`},
		{"leading trailing backslash", `\backslash\`},
		{"leading trailing CR", "\rCR\r"},
		{"leading trailing LF", "\nLF\n"},
		{"leading trailing HT", "\tHT\t"},
		{"leading trailing VT", "\vVT\v"},
		{"leading trailing dot", ".dot."},
		{"invalid UTF-8", "invalid utf-8\xfe"},
		{"URL encoding", "test%46.txt"},
		{"Japanese Kanji", "\u6f22\u5b57"},                                                  // Some Kanji from https://en.wikipedia.org/wiki/Kanji
		{"Emoji", "\U0001f60a"},                                                             // SMILING FACE WITH SMILING EYES emoji
		{"Full Width Characters", "\uff26\uff55\uff4c\uff4c\uff37\uff49\uff44\uff54\uff48"}, // "FullWidth" in full width characters
	}

	containerTypes := []types.ContainerType{
		types.AlbumContainerType,
		types.PlaylistContainerType,
	}

	ctx := context.Background()
	client := testClient()

	for _, ct := range containerTypes {
		t.Run(string(ct), func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.description, func(t *testing.T) {
					container, err := client.CreateContainer(ctx, ct, tt.name)
					require.NoError(t, err)
					assert.NotNil(t, container)

					actName, err := container.Name(ctx)
					assert.NoError(t, err)
					assert.Equal(t, actName, tt.name)

					client.ResetCache()
					containersFromSearch, err := client.ContainersWithName(ctx, ct, tt.name)
					require.NoError(t, err)
					require.Len(t, containersFromSearch, 1)
					actName, err = containersFromSearch[0].Name(ctx)
					assert.NoError(t, err)
					assert.Equal(t, actName, tt.name)

					err = container.Delete(ctx)
					assert.NoError(t, err)
				})
			}
		})
	}

	// We will only test uploading files to albums with the odd names because
	// uploading to playlists should work the same way.
	container := tempContainer(t, client, types.AlbumContainerType)

	allTestPhotos, err := photos.AllPhotos()
	require.NoError(t, err)
	photoToUpload := allTestPhotos[0]

	t.Run("photo", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.description, func(t *testing.T) {
				r, err := photoToUpload.Open()
				require.NoError(t, err)
				defer func() {
					err := r.Close()
					assert.NoError(t, err)
				}()

				photoName := tt.name + ".jpg"

				photo, err := container.AddPhoto(ctx, photoName, r, AddPhotoOptions{})
				require.NoError(t, err)

				defer func() {
					err = photo.Delete(ctx)
					assert.NoError(t, err)
				}()

				actName, err := photo.Name(ctx)
				assert.NoError(t, err)
				assert.Equal(t, actName, photoName)

				actUniqueName, err := photo.NameUnique(ctx)
				assert.NoError(t, err)
				assert.Equal(t, actUniqueName, photoName)

				container.ResetCache()
				photosFromNameSearch, err := container.PhotosWithName(ctx, photoName)
				require.NoError(t, err)
				require.NotEmpty(t, photosFromNameSearch)
				require.Len(t, photosFromNameSearch, 1)
				photoFromNameSearch := photosFromNameSearch[0]

				actName, err = photoFromNameSearch.Name(ctx)
				assert.NoError(t, err)
				assert.Equal(t, actName, photoName)

				actUniqueName, err = photoFromNameSearch.NameUnique(ctx)
				assert.NoError(t, err)
				assert.Equal(t, actUniqueName, photoName)

				photoFromUniqueNameSearch, err := container.PhotoWithUniqueName(ctx, photoName)
				assert.NoError(t, err)
				assert.NotNil(t, photoFromUniqueNameSearch)
				assert.Equal(t, photoFromUniqueNameSearch, photoFromNameSearch)
			})
		}
	})
}
