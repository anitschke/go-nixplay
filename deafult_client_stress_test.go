package nixplay

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"strconv"
	"sync"
	"testing"

	"github.com/anitschke/go-nixplay/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultClient_Photos_Stress
//
// The point of this test is to upload a large number of photos to make sure
// there isn't any issue with pagination and what not.
func TestDefaultClient_Photos_Stress(t *testing.T) {
	const stressTestPhotoCount = 120
	const maxConcurrentRequests = 20

	assert.Greater(t, stressTestPhotoCount, photoPageSize)

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

	//////////////////////////
	// Generate photos
	//////////////////////////

	type testPhoto struct {
		name string
		data bytes.Buffer
	}

	allTestPhotos := make([]testPhoto, 0, stressTestPhotoCount)
	photoSize := image.Rectangle{
		Max: image.Point{1, stressTestPhotoCount - 1},
	}
	photoPalette := color.Palette{color.Black, color.White}
	for i := 0; i < stressTestPhotoCount; i++ {
		name := strconv.Itoa(i) + ".png"
		img := image.NewPaletted(photoSize, photoPalette)
		img.Set(0, i, color.White)
		var b bytes.Buffer
		err := png.Encode(&b, img)
		require.NoError(t, err)

		allTestPhotos = append(allTestPhotos,
			testPhoto{
				name: name,
				data: b,
			},
		)
	}

	for _, tc := range tests {
		t.Run(string(tc.containerType), func(t *testing.T) {
			ctx := context.Background()
			client := testClient()

			// create temporary container for testing
			container := tempContainer(t, client, tc.containerType)

			// create request chan that limits number of concurrent requeists.
			// Put a stuct in the chan to request to be able to make a request
			// and pull off the chan when done with the request. see
			// https://stackoverflow.com/a/25306241
			requestC := make(chan struct{}, maxConcurrentRequests)
			doRequest := func(f func()) {
				requestC <- struct{}{}
				defer func() { <-requestC }()
				f()
			}

			//////////////////////////
			// List
			//////////////////////////
			photos, err := container.Photos(ctx)
			assert.NoError(t, err)
			assert.Empty(t, photos)

			//////////////////////////
			// Add
			//////////////////////////
			var wg sync.WaitGroup
			wg.Add(len(allTestPhotos))
			for _, tp := range allTestPhotos {
				go func(tp testPhoto) {
					defer wg.Done()
					addPhoto := func() {
						_, err := container.AddPhoto(ctx, tp.name, &tp.data, AddPhotoOptions{})
						require.NoError(t, err)
					}
					doRequest(addPhoto)
				}(tp)
			}
			wg.Wait()

			//////////////////////////
			// List
			//////////////////////////
			container.ResetCache()
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Len(t, photos, len(allTestPhotos))

			//////////////////////////
			// Validate ID uniqueness
			//////////////////////////
			idMap := make(map[types.ID]struct{}, len(photos))
			for _, p := range photos {
				idMap[p.ID()] = struct{}{}
			}
			assert.Equal(t, len(idMap), len(photos))

			//////////////////////////
			// Download
			//////////////////////////
			downloadedPhotos := make([]testPhoto, len(photos))
			wg.Add(len(photos))
			for i, p := range photos {
				go func(i int, p Photo) {
					defer wg.Done()
					downloadPhoto := func() {
						name, err := p.Name(ctx)
						assert.NoError(t, err)

						r, err := p.Open(ctx)
						require.NoError(t, err)
						defer func() {
							assert.NoError(t, r.Close())
						}()

						var buff bytes.Buffer
						_, err = io.Copy(&buff, r)
						assert.NoError(t, err)

						downloadedPhotos[i] = testPhoto{
							name: name,
							data: buff,
						}
					}
					doRequest(downloadPhoto)
				}(i, p)
			}
			wg.Wait()
			assert.ElementsMatch(t, downloadedPhotos, allTestPhotos)

			//////////////////////////
			// Delete
			//////////////////////////
			for _, p := range photos {
				err := p.Delete(ctx)
				assert.NoError(t, err)
			}

			//////////////////////////
			// List
			//////////////////////////
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Empty(t, photos)

			//////////////////////////
			// Clear Cache and List
			//////////////////////////
			container.ResetCache()
			photos, err = container.Photos(ctx)
			assert.NoError(t, err)
			assert.Empty(t, photos)
		})
	}
}
