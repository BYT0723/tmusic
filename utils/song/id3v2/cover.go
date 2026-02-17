package id3v2

import (
	"github.com/bogem/id3v2"
)

type CoverHandler struct{}

func (CoverHandler) CoverExist(fpath string) (exist bool) {
	// 打开 MP3 文件的 ID3v2 标签
	t, err := id3v2.Open(fpath, id3v2.Options{Parse: true})
	if err != nil {
		return
	}
	defer t.Close()

	// 获取所有的 APIC 帧（封面图片）
	pics := t.GetFrames(t.CommonID("Attached picture"))

	return len(pics) != 0
}

func (CoverHandler) EmbedCover(fpath string, cover []byte) (err error) {
	// 打开 MP3 文件的 ID3v2 标签
	t, err := id3v2.Open(fpath, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer t.Close()

	// 获取所有的 APIC 帧（封面图片）
	pics := t.GetFrames(t.CommonID("Attached picture"))
	if len(pics) == 0 {
		// 添加封面到 APIC 帧
		t.AddAttachedPicture(id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    "image/jpeg", // 或 "image/png"
			PictureType: id3v2.PTFrontCover,
			Description: "Front cover",
			Picture:     cover,
		})
	}
	return t.Save()
}
