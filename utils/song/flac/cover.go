package flac

import (
	"github.com/go-flac/flacpicture/v2"
	"github.com/go-flac/go-flac/v2"
)

type CoverHandler struct{}

func (CoverHandler) CoverExist(fpath string) (exist bool) {
	f, err := flac.ParseFile(fpath)
	if err != nil {
		return
	}
	defer f.Close()

	for _, mdb := range f.Meta {
		if mdb.Type == flac.Picture {
			exist = true
			break
		}
	}
	return
}

func (CoverHandler) EmbedCover(fpath string, cover []byte) (err error) {
	f, err := flac.ParseFile(fpath)
	if err != nil {
		return
	}
	defer f.Close()

	mbp, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", cover, "image/jpeg")
	if err != nil {
		return
	}

	mdb := mbp.Marshal()
	f.Meta = append(f.Meta, &mdb)

	return f.Save(fpath)
}
