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

type SongContext struct {
	DissName string
	Song     qqmusic.Song

	BaseName string
	Artist   string

	SongPath       string
	LyricPath      string
	LyricTransPath string

	SongType qqmusic.SongType
}

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

			songOK, songErr := handleSong(cli, ctx)
			lyricOK, lyricErr := handleLyric(cli, ctx)

			if songOK || errors.Is(songErr, ErrSongAlreayExists) {
				appendM3U(buf, cd.Dissname, ctx)
				songCount++
			}
			if lyricOK || errors.Is(lyricErr, ErrLryricAlreayExists) {
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

func handleSong(cli *qqmusic.Client, ctx *SongContext) (bool, error) {
	// 是否已存在
	for _, t := range songTypes {
		if _, err := os.Stat(ctx.SongPath + t.Suffix()); err == nil {
			ctx.SongType = t
			ctx.SongPath += t.Suffix()
			return false, ErrSongAlreayExists
		}
	}

	addr, rt, err := cli.GetSongUrl(
		ctx.Song.Songmid,
		ctx.Song.StrMediaMid,
		defaultSongType,
	)
	if err != nil {
		return false, err
	}

	ctx.SongType = rt
	ctx.SongPath += rt.Suffix()

	if err := utils.Download(addr, ctx.SongPath); err != nil {
		_ = os.Remove(ctx.SongPath)
		return false, err
	}

	if !noEmbedArt {
		_ = embedAlbumArt(cli, ctx.Song.Albummid, ctx.SongPath, false)
	}

	return true, nil
}

func handleLyric(cli *qqmusic.Client, ctx *SongContext) (bool, error) {
	if _, err := os.Stat(ctx.LyricPath); err == nil {
		return false, ErrLryricAlreayExists
	}

	lyric, trans, err := cli.GetSongLyric(ctx.Song.Songmid)
	if err != nil {
		return false, err
	}

	tag, err := id3v2.Open(ctx.SongPath, id3v2.Options{Parse: true})
	if err == nil {
		defer tag.Close()
	}

	lyric = normalizeLyric(lyric, ctx.Song.Songname, ctx.Artist, ctx.Song.Albumname)
	if err := os.WriteFile(ctx.LyricPath, lyric, os.ModePerm); err != nil {
		return false, err
	}

	if lyricTrans && len(trans) > 0 && tag != nil {
		trans = normalizeLyric(trans, tag.Title(), tag.Artist(), tag.Album())
		_ = os.WriteFile(ctx.LyricTransPath, trans, os.ModePerm)
	}

	return true, nil
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
	if silent {
		return
	}

	songExist := songErr != nil && errors.Is(songErr, ErrSongAlreayExists)
	lyricExist := lyricErr != nil && errors.Is(lyricErr, ErrLryricAlreayExists)
	if songErr != nil || lyricErr != nil {
		switch {
		case songExist && lyricExist:
		case !songExist:
			log.Errorf("%s ===> [ %s ]\n", ctx.BaseName, songErr)
		case !lyricExist:
			log.Errorf("%s ===> [ %s, %s ]\n", ctx.BaseName, songErr, lyricErr)
		default:
			log.Infof("%s%s ===> [ %s, %s ]\n", ctx.BaseName, ctx.SongType.Suffix(), songErr, lyricErr)
		}
	} else {
		log.Infof("%s%s ===> OK\n", ctx.BaseName, ctx.SongType.Suffix())
	}
}

func sleepRandom() {
	time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
}

func embedAlbumArt(cli *qqmusic.Client, ablumid, songpath string, override bool) error {
	// 打开 MP3 文件的 ID3v2 标签
	tag, err := id3v2.Open(songpath, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	// 获取所有的 APIC 帧（封面图片）
	pics := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(pics) != 0 && !override {
		return ErrAblumArtAlreadyExists
	}

	artURL := fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", ablumid)

	resp, err := http.Get(artURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code error: %d", resp.StatusCode)
	}

	cover, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(cover) == 0 {
		return errors.New("empty cover")
	}

	// 添加封面到 APIC 帧
	tag.AddAttachedPicture(id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    "image/jpeg", // 或 "image/png"
		PictureType: id3v2.PTFrontCover,
		Description: "Cover",
		Picture:     cover,
	})

	return tag.Save()
}
