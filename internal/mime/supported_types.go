package mime

// cSpell:ignore stdmime
import stdmime "mime"

func init() {
	// Add all supported file types that nixplay supports into the go mime type
	// catalog to ensure that we can identify these mime types based on
	// extension.
	//
	// see https://web.archive.org/web/20230328184513/https://support.nixplay.com/hc/en-us/articles/900002393886-What-photo-and-video-formats-does-Nixplay-support-

	stdmime.AddExtensionType(".jpg", "image/jpeg")
	stdmime.AddExtensionType(".jpeg", "image/jpeg")
	stdmime.AddExtensionType(".png", "image/png")
	stdmime.AddExtensionType(".tif", "image/tiff")
	stdmime.AddExtensionType(".tiff", "image/tiff")
	stdmime.AddExtensionType(".heic", "image/heic")
	stdmime.AddExtensionType(".heif", "image/heif")
	stdmime.AddExtensionType(".mp4", "video/mp4")
}
