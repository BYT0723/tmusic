/*
Copyright © 2025 BYT0723 twang9739@gmail.com
*/
package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BYT0723/apix/qqmusic"
	"github.com/BYT0723/tmusic/utils"
	"github.com/BYT0723/tmusic/utils/log"
	"github.com/bogem/id3v2"
	"github.com/dhowden/tag"
	"github.com/go-flac/flacpicture/v2"
	"github.com/go-flac/go-flac/v2"
	"github.com/spf13/cobra"
)

// qqmusicCmd represents the qqmusic command
var (
	qqmusicCmd = &cobra.Command{
		Use:   "qqmusic",
		Short: "toolset for qqmusic",
		Long:  `toolset for qqmusic`,
		Run:   func(cmd *cobra.Command, args []string) {},
	}
	qqmusicSyncCmd = &cobra.Command{
		Use:   "sync",
		Short: "sync user song list from qqmusic",
		Long:  `sync user song list from qqmusic`,
		Run: func(cmd *cobra.Command, args []string) {
			cb, err := os.ReadFile(cookiePath)
			if err != nil {
				log.Errorf("[Error] Cookie获取失败，无法读取%s: %v\n", cookiePath, err)
				os.Exit(1)
			}
			cli, err := qqmusic.NewClient(string(bytes.TrimSpace(cb)))
			cobra.CheckErr(err)

			syncUserSongList(cli, songDir, lyricDir)
		},
	}
)

var (
	ErrSongAlreayExists      = errors.New("song already exists")
	ErrLryricAlreayExists    = errors.New("lyric already exists")
	ErrAblumArtAlreadyExists = errors.New("album art already exists")
)

var (
	cookiePath                                     string
	songDir, lyricDir, mpdPlaylistDir              string
	silent, genMpdPlaylist, noEmbedArt, lyricTrans bool
	defaultSongType                                = qqmusic.SongTypeFLAC
	songTypes                                      = []qqmusic.SongType{
		qqmusic.SongTypeFLAC,
		qqmusic.SongTypeAPE,
		qqmusic.SongType320,
		qqmusic.SongType128,
		qqmusic.SongTypeM4A,
	}
	reTi = regexp.MustCompile(`(?m)^\[ti:.*\]`)
	reAr = regexp.MustCompile(`(?m)^\[ar:.*\]`)
	reAl = regexp.MustCompile(`(?m)^\[al:.*\]`)
)

func init() {
	rootCmd.AddCommand(qqmusicCmd)
	qqmusicCmd.AddCommand(qqmusicSyncCmd)

	home, _ := os.UserHomeDir()

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// syncCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// syncCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// qqmusic flags
	qqmusicCmd.PersistentFlags().StringVarP(&cookiePath, "cookie", "c", "./qqmusic-cookie.txt", "cookie filepath")

	// qqmusic sync flags
	qqmusicSyncCmd.PersistentFlags().StringVar(&songDir, "song", filepath.Join(home, "Music"), "歌曲目录")
	qqmusicSyncCmd.PersistentFlags().StringVar(&lyricDir, "lyric", filepath.Join(home, "Music", ".lyrics"), "歌词目录")
	qqmusicSyncCmd.PersistentFlags().BoolVar(&lyricTrans, "lyric-trans", false, "是否下载翻译歌词")
	qqmusicSyncCmd.PersistentFlags().StringVar(&mpdPlaylistDir, "mpd-playlist", filepath.Join(home, ".mpd", "playlists"), "MPD Playlist Directory")
	qqmusicSyncCmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "是否静默模式")
	qqmusicSyncCmd.PersistentFlags().BoolVarP(&genMpdPlaylist, "gen-mpd-playlist", "g", false, "是否生成MPD播放列表")
	qqmusicSyncCmd.PersistentFlags().BoolVar(&noEmbedArt, "no-art", false, "不嵌入专辑封面")
}

func syncUserSongList(cli *qqmusic.Client, songDir string, lyricDir string) {
	if err := os.MkdirAll(songDir, os.ModePerm); err != nil {
		log.Errorf("[Error] 无法创建目录 %s : %v\n", songDir, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(lyricDir, os.ModePerm); err != nil {
		log.Errorf("[Error] 无法目录 %s : %v\n", lyricDir, err)
		os.Exit(1)
	}

	sl, err := cli.GetUserSongList()
	if err != nil {
		log.Errorf("[Error] 获取用户歌单失败: %v\n", err)
		return
	}

	for _, d := range sl.DissList {
		if d.DirShow != 0 {
			syncDiss(cli, d)
		}
	}
}

type (
	SongContext struct {
		DissName string
		Song     qqmusic.Song

		BaseName string
		Artist   string

		SongPath       string
		SongTag        songTag
		LyricPath      string
		LyricTransPath string

		SongType qqmusic.SongType
	}
	songTag struct {
		Title  string
		Artist string
		Album  string
	}
)

func syncDiss(cli *qqmusic.Client, d qqmusic.Diss) (songCount int, lyricCount int) {
	l, err := cli.GetSongList(d.Tid)
	if err != nil || len(l.Cdlist) == 0 {
		log.Errorf("%s 歌单获取失败: %v", d.DissName, err)
		return
	}

	sdir := filepath.Join(songDir, d.DissName)
	if err := os.MkdirAll(sdir, os.ModePerm); err != nil {
		log.Errorf("歌单 %s 创建失败: %v", d.DissName, err)
		return
	}

	m3u := make(map[string]*bytes.Buffer)

	for _, cd := range l.Cdlist {
		buf := bytes.NewBuffer(make([]byte, 0, 1024))

		for _, song := range cd.Songlist {
			ctx := buildSongContext(sdir, cd.Dissname, song)

			songErr := handleSong(cli, ctx)
			lyricErr := handleLyric(cli, ctx)
			songOK := songErr == nil || errors.Is(songErr, ErrSongAlreayExists)
			lyricOK := lyricErr == nil || errors.Is(lyricErr, ErrLryricAlreayExists)

			if songOK {
				appendM3U(buf, cd.Dissname, ctx)
				songCount++
			}
			if lyricOK {
				lyricCount++
			}

			logSongResult(ctx, songErr, lyricErr)

			if songOK || lyricOK {
				sleepRandom()
			}
		}

		m3u[cd.Dissname] = buf
	}

	if genMpdPlaylist {
		writeM3UFiles(m3u)
	}

	return
}

func buildSongContext(baseDir, dissName string, s qqmusic.Song) *SongContext {
	artists := make([]string, 0, len(s.Singer))
	for _, a := range s.Singer {
		artists = append(artists, a.Name)
	}
	artist := strings.Join(artists, ",")

	base := fmt.Sprintf("%s - %s", s.Songname, artist)
	base = strings.NewReplacer("/", " ", "\\", " ").Replace(base)

	return &SongContext{
		DissName: dissName,
		Song:     s,
		BaseName: base,
		Artist:   artist,

		SongPath:       filepath.Join(baseDir, base),
		LyricPath:      filepath.Join(lyricDir, base+".lrc"),
		LyricTransPath: filepath.Join(lyricDir, base+"_trans.lrc"),
	}
}

func handleSong(cli *qqmusic.Client, ctx *SongContext) (err error) {
	// 是否已存在
	needDownload := true
	for _, t := range songTypes {
		if _, err := os.Stat(ctx.SongPath + t.Suffix()); err == nil {
			ctx.SongType = t
			ctx.SongPath += t.Suffix()
			needDownload = false
			break
		}
	}

	if needDownload {
		addr, rt, err := cli.GetSongUrl(
			ctx.Song.Songmid,
			ctx.Song.StrMediaMid,
			defaultSongType,
		)
		if err != nil {
			return err
		}

		ctx.SongType = rt
		ctx.SongPath += rt.Suffix()

		if err := utils.Download(addr, ctx.SongPath); err != nil {
			_ = os.Remove(ctx.SongPath)
			return err
		}
	}

	{
		f, err := os.Open(ctx.SongPath)
		if err == nil {
			defer f.Close()
			m, err := tag.ReadFrom(f)
			if err == nil {
				ctx.SongTag.Title = m.Title()
				ctx.SongTag.Artist = m.Artist()
				ctx.SongTag.Album = m.Album()
			}
		}
	}

	_ = songTagUpdate(ctx)

	if !needDownload {
		err = ErrSongAlreayExists
	}
	return
}

func songTagUpdate(ctx *SongContext) error {
	switch ctx.SongType {
	case qqmusic.SongTypeFLAC:
		f, err := flac.ParseFile(ctx.SongPath)
		if err == nil {
			defer f.Close()

			if !noEmbedArt {
				coverExist := false
				for _, mdb := range f.Meta {
					if mdb.Type == flac.Picture {
						coverExist = true
						break
					}
				}

				if !coverExist {
					cover, err := downloadPicture(ctx)
					if err == nil {
						mbp, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", cover, "image/jpeg")
						if err == nil {
							mdb := mbp.Marshal()
							f.Meta = append(f.Meta, &mdb)
							f.Save(ctx.SongPath)
						}
					}
				}
			}
		}
	default:
		// 打开 MP3 文件的 ID3v2 标签
		t, err := id3v2.Open(ctx.SongPath, id3v2.Options{Parse: true})
		if err != nil {
			log.Errorf("open song tag err: %v\n", err)
			return err
		}
		defer t.Close()

		if !noEmbedArt {
			// 获取所有的 APIC 帧（封面图片）
			pics := t.GetFrames(t.CommonID("Attached picture"))
			if len(pics) == 0 {
				cover, err := downloadPicture(ctx)
				if err == nil {
					// 添加封面到 APIC 帧
					t.AddAttachedPicture(id3v2.PictureFrame{
						Encoding:    id3v2.EncodingUTF8,
						MimeType:    "image/jpeg", // 或 "image/png"
						PictureType: id3v2.PTFrontCover,
						Description: "Front cover",
						Picture:     cover,
					})
				}
			}
		}
		return t.Save()
	}
	return nil
}

func downloadPicture(ctx *SongContext) (cover []byte, err error) {
	artURL := fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", ctx.Song.Albummid)

	resp, err := http.Get(artURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = errors.New("status code: " + resp.Status)
		return
	}

	cover, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if len(cover) == 0 {
		err = errors.New("empty cover")
	}
	return
}

func handleLyric(cli *qqmusic.Client, ctx *SongContext) (err error) {
	needDownload := true
	if _, err := os.Stat(ctx.LyricPath); err == nil {
		needDownload = false
	}

	var lyric, trans []byte
	if needDownload {
		lyric, trans, err = cli.GetSongLyric(ctx.Song.Songmid)
		if err != nil {
			return
		}
	} else {
		lyric, err = os.ReadFile(ctx.LyricPath)
		if err != nil {
			return
		}
		trans, err = os.ReadFile(ctx.LyricTransPath)
		if err != nil && errors.Is(err, ErrLryricAlreayExists) {
			return
		}
	}

	if err = os.WriteFile(ctx.LyricPath, normalizeLyric(lyric, ctx.SongTag.Title, ctx.SongTag.Artist, ctx.SongTag.Album), os.ModePerm); err != nil {
		return
	}

	if lyricTrans && len(trans) > 0 {
		_ = os.WriteFile(ctx.LyricTransPath, normalizeLyric(lyric, ctx.SongTag.Title, ctx.SongTag.Artist, ctx.SongTag.Album), os.ModePerm)
	}
	if !needDownload {
		err = ErrLryricAlreayExists
	}
	return
}

func appendM3U(buf *bytes.Buffer, dissName string, ctx *SongContext) {
	buf.WriteString(filepath.Join(dissName, ctx.BaseName+ctx.SongType.Suffix()))
	buf.WriteByte('\n')
}

func writeM3UFiles(m3u map[string]*bytes.Buffer) {
	_ = os.MkdirAll(mpdPlaylistDir, os.ModePerm)
	for name, buf := range m3u {
		_ = os.WriteFile(
			filepath.Join(mpdPlaylistDir, name+".m3u"),
			buf.Bytes(),
			os.ModePerm,
		)
	}
}

func normalizeLyric(bs []byte, title, artist, album string) []byte {
	bs = bytes.ReplaceAll(bs, []byte("&apos;"), []byte("'"))
	bs = bytes.ReplaceAll(bs, []byte("&quot;"), []byte("\""))
	bs = bytes.ReplaceAll(bs, []byte("&nbsp;"), []byte(" "))

	bs = reTi.ReplaceAll(bs, []byte("[ti:"+title+"]"))
	bs = reAr.ReplaceAll(bs, []byte("[ar:"+artist+"]"))
	bs = reAl.ReplaceAll(bs, []byte("[al:"+album+"]"))
	return bs
}

func logSongResult(ctx *SongContext, songErr, lyricErr error) {
	if silent || (errors.Is(songErr, ErrSongAlreayExists) && errors.Is(lyricErr, ErrLryricAlreayExists)) {
		return
	}

	level, song, lyric := "info", "OK", "OK"
	if songErr != nil {
		if errors.Is(songErr, ErrSongAlreayExists) {
			song = "already exists"
		} else {
			level = "error"
			song = songErr.Error()
		}
	}
	if lyricErr != nil {
		if errors.Is(lyricErr, ErrLryricAlreayExists) {
			lyric = "already exists"
		} else {
			level = "error"
			lyric = lyricErr.Error()
		}
	}

	if level == "error" {
		log.Errorf("%s ===> [ %s, %s ]\n", ctx.BaseName, song, lyric)
	} else {
		log.Infof("%s%s ===> [ %s, %s ]\n", ctx.BaseName, ctx.SongType.Suffix(), song, lyric)
	}
}

func sleepRandom() {
	time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
}
