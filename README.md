# go-nixplay
[![Go Reference](https://pkg.go.dev/badge/github.com/anitschke/go-nixplay.svg)](https://pkg.go.dev/github.com/anitschke/go-nixplay)

`go-nixplay` is an **unofficial** library for uploading and downloading photos to/from the [Nixplay](https://www.nixplay.com/) cloud based picture frame service. Note that since Nixplay does not publicly document any of their APIs they could change APIs with no warning potentially breaking this library. 

## Usage
This project is intended to simply be a library for communicating with Nixplay and not as a full application. My eventual goal is to integrate this library into [rclone](https://rclone.org/) as a cloud backend in order to implement a flexible way of syncing photos from the local file system or virtually any cloud storage provider to Nixplay.

For info on using the library see the go [doc reference page](https://pkg.go.dev/github.com/anitschke/go-nixplay) or see [tests](./deafult_client_test.go) for an example.

## Capabilities
* List albums and playlists
* Get basic info about albums and playlists such as name and photo count
* Add and delete albums and playlists
* List photos within an album or playlist
* Get basic info about photos such as name, size, MD5 hash
* Upload new photos
* Delete existing photos

## Caching
My experience has been that the HTTP calls to get data about albums and photos tends to be on the slow side. So this library will only make calls to get data about items when it is requested and will cache data that is received as part of the request in the event that the data is requested again. The cache of albums/playlists can be cleared by doing `client.ResetCache()` and the cache of photos within an individual album/playlist can be cleared by doing `container.ResetCache()`. Cached data such as name, size, MD5 hash for an individual item can not be cleared, to get updated data for that item reset that parents cache and re-request that item and associated data.

## Limitations
xxx

## Testing
This library contains tests to ensure that all APIs are working correctly. To make this possible a test Nixplay account needs to be used. At the start of testing this account needs to have the default empty configuration, it should have two empty playlists `${username}@mynixplay.com` and `Favorites`, it should have two empty albums `${username}@mynixplay.com` and `My Uploads`. DO NOT use a real nixplay account you care about for testing as this may remove photos you care about.

The credentials for test account to be used for testing should be specified by using the `GO_NIXPLAY_TEST_ACCOUNT_USERNAME` and `GO_NIXPLAY_TEST_ACCOUNT_PASSWORD` environment variables.

When running tests the `-p 1` flag must be passed to `go test` to disable running tests in parallel as having multiple tests attempting to add/remove albums/playlists/photos at the same time as some tests look at all albums/playlists to lock down the APIs use to add/remove containers.

For example to run all tests
```bash
export GO_NIXPLAY_TEST_ACCOUNT_USERNAME="YOUR_USERNAME_HERE"
export GO_NIXPLAY_TEST_ACCOUNT_PASSWORD="YOUR_PASSWORD_HERE"
go test -p 1 -v ./...
```

This library runs these tests via GitHub Actions to ensure there are no bugs introduced in PRs. To do this the above mentioned environment variables are injected in to the testing environment by using [encrypted secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets#creating-encrypted-secrets-for-an-environment) that are tied to the `test` environment. There is a good writeup of this workflow [here](https://dev.to/petrsvihlik/using-environment-protection-rules-to-secure-secrets-when-building-external-forks-with-pullrequesttarget-hci).