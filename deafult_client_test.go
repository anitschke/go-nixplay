package nixplay

import (
	"context"
	"strings"
	"testing"

	"github.com/anitschke/go-nixplay/auth"
	"github.com/stretchr/testify/assert"
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
			containers, err := client.Containers(context.Background(), tc.containerType)
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
			containers, err = client.Containers(context.Background(), tc.containerType)
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
