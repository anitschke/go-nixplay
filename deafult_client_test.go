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

func TestDefaultClient_InitialAlbums(t *testing.T) {
	client := testClient()

	containers, err := client.Containers(context.Background(), AlbumContainerType)
	assert.NoError(t, err)

	// By default every nixplay account seems to have one album that can be
	// deleted but seems to come back automatically. This album is the
	// ${username}@mynixplay.com album. So we will check that it exists
	assert.Len(t, containers, 1)
	assert.Contains(t, containers[0].Name, "@mynixplay.com")
}
