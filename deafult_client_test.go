package nixplay

import (
	"context"
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

func TestDefaultClient_Albums_ListCreateListDeleteList(t *testing.T) {
	ctx := context.Background()
	client := testClient()

	//////////////////////////
	// List
	//////////////////////////
	containers, err := client.Containers(context.Background(), AlbumContainerType)
	assert.NoError(t, err)

	// By default every nixplay account seems to have one album that can be
	// deleted but seems to come back automatically. This album is the
	// ${username}@mynixplay.com album. So we will check that it exists
	assert.Len(t, containers, 1)
	assert.Contains(t, containers[0].Name, "@mynixplay.com")
	initialName := containers[0].Name

	//////////////////////////
	// Create
	//////////////////////////
	newName := "MyNewContainer"
	newContainer, err := client.CreateContainer(ctx, AlbumContainerType, newName)
	assert.NoError(t, err)
	assert.Equal(t, newContainer.Name, newName)

	//////////////////////////
	// List
	//////////////////////////
	containers, err = client.Containers(ctx, AlbumContainerType)
	assert.NoError(t, err)
	var names []string
	for _, c := range containers {
		names = append(names, c.Name)
	}
	assert.Len(t, containers, 2)
	assert.Contains(t, names, newName)
	assert.Contains(t, names, initialName)

	//////////////////////////
	// Delete
	//////////////////////////
	err = client.DeleteContainer(ctx, newContainer)
	assert.NoError(t, err)

	//////////////////////////
	// List
	//////////////////////////
	containers, err = client.Containers(context.Background(), AlbumContainerType)
	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Equal(t, containers[0].Name, initialName)
}
