package song

type TagHandler interface {
	CoverExist(fpath string) (exist bool)
	EmbedCover(fpath string, cover []byte) error
}
