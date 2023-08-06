# go-nixplay
[![Go
Reference](https://pkg.go.dev/badge/github.com/anitschke/go-nixplay.svg)](https://pkg.go.dev/github.com/anitschke/go-nixplay)
![GitHub release (latest
SemVer)](https://img.shields.io/github/v/release/anitschke/go-nixplay))
[![CI](https://github.com/anitschke/go-nixplay/actions/workflows/ci.yaml/badge.svg)](https://github.com/anitschke/go-nixplay/actions/workflows/ci.yaml)
[![Go Report
Card](https://goreportcard.com/badge/github.com/anitschke/go-nixplay)](https://goreportcard.com/report/github.com/anitschke/go-nixplay)

`go-nixplay` is an **unofficial** library for uploading and downloading photos
to/from the [Nixplay](https://www.nixplay.com/) cloud based picture frame
service. Note that since Nixplay does not publicly document any of their APIs
they could change APIs with no warning potentially breaking this library. 

## Usage
This project is intended to simply be a library for communicating with Nixplay
and not as a full application. My eventual goal is to integrate this library
into [rclone](https://rclone.org/) as a cloud backend in order to implement a
flexible way of syncing photos from the local file system or virtually any cloud
storage provider to Nixplay.

For info on using the library see the go [doc reference
page](https://pkg.go.dev/github.com/anitschke/go-nixplay) or see
[tests](./default_client_test.go) for an example.

## Capabilities
* List albums and playlists
* Get basic info about albums and playlists such as name and photo count
* Add and delete albums and playlists
* List photos within an album or playlist
* Get basic info about photos such as name, size, MD5 hash
* Upload new photos
* Delete existing photos

## Caching
My experience has been that the HTTP calls to get data about albums and photos
tends to be on the slow side. So this library will only make calls to get data
about items when it is requested and will cache data that is received as part of
the request in the event that the data is requested again. The cache of
albums/playlists can be cleared by doing `client.ResetCache()` and the cache of
photos within an individual album/playlist can be cleared by doing
`container.ResetCache()`. Cached data such as name, size, MD5 hash for an
individual item can not be cleared, to get updated data for that item reset that
parents cache and re-request that item and associated data.

## Limitations

### Nixplay Meta Model
Before we get into real limitations it is useful to discuss the meta model (or
at least my interpretation of it) of how photos are stored in Nixplay. To start
with every photo is contained/owned by an album. Nixplay does not allow photos
with duplicate content within the same album. However nixplay does allow photos
with duplicate content to exist in different albums. There is also no constraint
of name uniqueness for photos within an album.

Photos within a playlist are not contained/owned by the playlist but these are
rather just associations to photos that are actually contained/owned by an
album. Nixplay *unfortunately* allows a playlist to contain multiple
associations to the same photo within an album, ie in a playlist you could have
10 copies the same photo in the same album.

### Photo Addition/Delete is not "Atomic"
The biggest fallout from the above meta model is that addition and deletion of
photos is not "atomic", as in adding or deleting the photo may result in it
getting added or deleted from another container.

For example:
* If you add a photo to a playlist it will automatically upload the photo to the
  "My Uploads" album and then associate that photo to the playlist you added the
  photo to.
* If you delete a photo from a album it will also remove the photo from any
  playlists that the photo was associated to.
* When deleting a photo from a playlist we will use APIs to only remove photos
  from that playlist in order to avoid affecting other playlists the photo may
  be a part of. This is probably expected based on everything mentioned above,
  but it is worth explicitly mentioning because this can result in "leaking"
  photos. If I add a photo to a playlist and then immediately delete the photo
  it results in the photo being "leaked" in the "My Uploads" folder. This
  probably isn't a big issue, but if you were to use these APIs to frequently
  add/remove mass number of photos you could leak enough photos to hit [10GB
  free storage
  quota](https://web.archive.org/web/20230401125711/https://support.nixplay.com/hc/en-us/articles/360015748892-How-is-storage-being-used-on-the-Nixplay-Cloud-and-on-Nixplay-Frames-).


Note that the caching mentioned in the [Caching](#caching) does not take this
into account. If you add or delete a photo from one container you need to reset
the cache in other containers if you want to guarantee that you have the correct
state for those containers.

### Multiple Copies of Photos in Playlist
One of the goals of this library was to provide a reference to the photo that is
uploaded to Nixplay. The difficulty with this is that Nixplay does not return
any such reference to describe the ID of the photo upload, only that if upload
succeeded or failed. So to achieve this goal when a photo is upload this library
stores the MD5 hash of the photo and can use this information to differentiate
the photo from others within the container. This works well for photos within
albums as Nixplay doesn't allow photos with duplicate content in the same album
(as mentioned above).

However as I mentioned above Nixplay DOES allow duplicate copies of photos with
in a playlist. For now it is recommended that you avoid uploading duplicate
copies of photos to a playlist as this will likely result in unexpected
behavior. At some point I may look into resolving this issue but I doubt many
people will run into this issue/limitation.

### Name Encoding
//xxx add comment about encoding of names

## Testing
This library contains tests to ensure that all APIs are working correctly. To
make this possible a test Nixplay account needs to be used. At the start of
testing this account needs to have the default empty configuration, it should
have two empty playlists `${username}@mynixplay.com` and `Favorites`, it should
have two empty albums `${username}@mynixplay.com` and `My Uploads`. DO NOT use a
real nixplay account you care about for testing as this may remove photos you
care about.

The credentials for test account to be used for testing should be specified by
using the `GO_NIXPLAY_TEST_ACCOUNT_USERNAME` and
`GO_NIXPLAY_TEST_ACCOUNT_PASSWORD` environment variables.

When running tests the `-p 1` flag must be passed to `go test` to disable
running tests in parallel as having multiple tests attempting to add/remove
albums/playlists/photos at the same time as some tests look at all
albums/playlists to lock down the APIs use to add/remove containers.

For example to run all tests
```bash
export GO_NIXPLAY_TEST_ACCOUNT_USERNAME="YOUR_USERNAME_HERE"
export GO_NIXPLAY_TEST_ACCOUNT_PASSWORD="YOUR_PASSWORD_HERE"
go test -p 1 -v ./...
```

This library runs these tests via GitHub Actions to ensure there are no bugs
introduced in PRs. To do this the above mentioned environment variables are
injected in to the testing environment by using [encrypted
secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets#creating-encrypted-secrets-for-an-environment)
that are tied to the `test` environment. There is a good writeup of this
workflow
[here](https://dev.to/petrsvihlik/using-environment-protection-rules-to-secure-secrets-when-building-external-forks-with-pullrequesttarget-hci).

## Acknowledgements

Thanks to [andrewjjenkins](https://github.com/andrewjjenkins) for doing the
initial reverse engineering of Nixplay's APIs as part of
[andrewjjenkins/picsync](https://github.com/andrewjjenkins/picsync).
 