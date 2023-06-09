package nixplay

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/anitschke/go-nixplay/httpx"
	"github.com/anitschke/go-nixplay/internal/errorx"
	"github.com/anitschke/go-nixplay/types"
)

var errDuplicateImage = errors.New("failed to upload image as duplicate image with the same content already exists in this album")

type uploadContainerID struct {
	idName string
	id     string
}

type uploadedPhoto struct {
	name    string
	md5Hash types.MD5Hash
	size    int64
}

func addPhoto(ctx context.Context, client httpx.Client, containerID uploadContainerID, name string, r io.Reader, opts AddPhotoOptions) (retData uploadedPhoto, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	photoData, r, err := getUploadPhotoData(name, r, opts)
	if err != nil {
		return uploadedPhoto{}, err
	}

	uploadToken, err := getUploadToken(ctx, client, containerID)
	if err != nil {
		return uploadedPhoto{}, err
	}

	uploadNixplayResponse, err := uploadNixplay(ctx, client, containerID, photoData, uploadToken)
	if err != nil {
		return uploadedPhoto{}, err
	}

	hasher := md5.New()
	readAndHash := io.TeeReader(r, hasher)

	if err := uploadS3(ctx, client, uploadNixplayResponse, name, readAndHash); err != nil {
		return uploadedPhoto{}, err
	}

	md5Hash := *(*types.MD5Hash)(hasher.Sum(nil))

	if len(uploadNixplayResponse.UserUploadIDs) != 1 {
		return uploadedPhoto{}, errors.New("unable to wait for photo to be uploaded")
	}
	monitorId := uploadNixplayResponse.UserUploadIDs[0]

	// We still need to return uploadedPhoto even if monitorUpload errors out because
	// sometimes monitorUpload returns an error but we can still recover from when uploading
	// to a playlist. See comments in container.AddPhoto for details
	err = monitorUpload(ctx, client, monitorId)

	return uploadedPhoto{
		name:    name,
		md5Hash: md5Hash,
		size:    int64(photoData.FileSize),
	}, err
}

type uploadPhotoData struct {
	AddPhotoOptions
	Name string
}

func getUploadPhotoData(name string, r io.Reader, opts AddPhotoOptions) (retData uploadPhotoData, retR io.Reader, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	data := uploadPhotoData{
		AddPhotoOptions: opts,
		Name:            name,
	}

	if data.MIMEType == "" {
		ext := filepath.Ext(name)
		if ext == "" {
			return uploadPhotoData{}, nil, fmt.Errorf("could not determine file extension for file %q", name)
		}
		data.MIMEType = mime.TypeByExtension(ext)
		if data.MIMEType == "" {
			return uploadPhotoData{}, nil, fmt.Errorf("could not determine mime type for file %q", name)
		}
	}

	// If we don't know the file size we will try a few different APIs to try to
	// determine the size of the photo efficiently. If that doesn't work we will
	// resort to reading into a buffer which requires us to buffer the entire
	// file into memory, not ideal.
	if data.FileSize == 0 {
		switch photo := r.(type) {
		case *os.File:
			fileInfo, err := photo.Stat()
			if err != nil {
				return uploadPhotoData{}, nil, err
			}
			data.FileSize = fileInfo.Size()
		case *bytes.Buffer:
			data.FileSize = int64(photo.Len())
		case *bytes.Reader:
			data.FileSize = photo.Size()
		case io.Seeker:
			var err error
			data.FileSize, err = photo.Seek(0, io.SeekEnd)
			if err != nil {
				return uploadPhotoData{}, nil, err
			}
			// seek back to the start of file so that it can be read again properly
			if _, err := photo.Seek(0, io.SeekStart); err != nil {
				return uploadPhotoData{}, nil, err
			}
		default:
			var err error
			buf := new(bytes.Buffer)
			data.FileSize, err = buf.ReadFrom(r)
			if err != nil {
				return uploadPhotoData{}, nil, err
			}
			r = buf

		}
	}

	return data, r, nil
}

func getUploadToken(ctx context.Context, client httpx.Client, containerID uploadContainerID) (returnedToken string, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	form := url.Values{
		containerID.idName: {containerID.id},
		"total":            {"1"},
	}

	req, err := httpx.NewPostFormRequest(ctx, "https://api.nixplay.com/v3/upload/receivers/", form)
	if err != nil {
		return "", err
	}

	var response uploadTokenResponse
	if err := httpx.DoUnmarshalJSONResponse(client, req, &response); err != nil {
		return "", err
	}

	return response.Token, nil
}

func uploadNixplay(ctx context.Context, client httpx.Client, containerID uploadContainerID, photo uploadPhotoData, token string) (returnedResponse uploadNixplayResponse, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	form := url.Values{
		containerID.idName: {containerID.id},
		"uploadToken":      {token},
		"fileName":         {photo.Name},
		"fileType":         {photo.MIMEType},
		"fileSize":         {strconv.FormatInt(photo.FileSize, 10)},
	}

	req, err := httpx.NewPostFormRequest(ctx, "https://api.nixplay.com/v3/photo/upload/", form)
	if err != nil {
		return uploadNixplayResponse{}, err
	}

	var response uploadNixplayResponseContainer
	if err := httpx.DoUnmarshalJSONResponse(client, req, &response); err != nil {
		return uploadNixplayResponse{}, err
	}

	return response.Data, nil
}

func uploadS3(ctx context.Context, client httpx.Client, u uploadNixplayResponse, filename string, r io.Reader) (err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	reqBody := &bytes.Buffer{}
	writer := multipart.NewWriter(reqBody)

	formValues := map[string]string{
		"key":                        u.Key,
		"acl":                        u.ACL,
		"content-type":               u.FileType,
		"x-amz-meta-batch-upload-id": u.BatchUploadID,
		"success_action_status":      "201",
		"AWSAccessKeyId":             u.AWSAccessKeyID,
		"Policy":                     u.Policy,
		"Signature":                  u.Signature,
	}
	for k, v := range formValues {
		w, err := writer.CreateFormField(k)
		if err != nil {
			return err
		}
		io.WriteString(w, v)
	}

	w, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, r)
	if err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.S3UploadURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("content-type", fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set("origin", "https://app.nixplay.com")
	req.Header.Set("referer", "https://app.nixplay.com")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return fmt.Errorf("error uploading: %s", resp.Status)
	}
	return nil
}

func monitorUpload(ctx context.Context, client httpx.Client, monitorID string) (err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	url := fmt.Sprintf("https://upload-monitor.nixplay.com/status?id=%s", monitorID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	// Special logic to detect duplicate uploads. See comments in
	// container.AddPhoto.
	if resp.StatusCode == 400 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if string(body) == "Error: image-exists" {
			return errDuplicateImage
		}
		return fmt.Errorf("http status: %s: body: %s", resp.Status, body)
	}

	return httpx.StatusError(resp)
}
