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

var ErrAblumArtAlreadyExists = fmt.Errorf("album art already exists")

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

func syncDiss(cli *qqmusic.Client, d qqmusic.Diss) (songCount int, lyricCount int) {
	l, err := cli.GetSongList(d.Tid)
	if err != nil {
		log.Errorf("%s 歌单获取失败: %v\n", d.DissName, err)
		return songCount, lyricCount
	}
	if len(l.Cdlist) == 0 {
		return songCount, lyricCount
	}

	sdir := filepath.Join(songDir, d.DissName)
	if err := os.MkdirAll(sdir, os.ModePerm); err != nil {
		log.Errorf("歌单 %s 创建失败: %v\n", d.DissName, err)
		return songCount, lyricCount
	}

	type result struct {
		Song     string `json:"song,omitempty"`
		Lyric    string `json:"lyric,omitempty"`
		SongErr  error  `json:"song_err,omitempty"`
		LyricErr error  `json:"lyric_err,omitempty"`
	}

	m3u := map[string]*bytes.Buffer{}

	for _, v := range l.Cdlist {
		m3u[v.Dissname] = bytes.NewBuffer(make([]byte, 0, 1024))
		for _, s := range v.Songlist {
			var (
				nameBuilder strings.Builder

				songname       string
				songExists     bool
				n              = len(s.Singer)
				targetSongType = defaultSongType
				dstSongType    qqmusic.SongType
				res            = result{
					Song:  "OK",
					Lyric: "OK",
				}
				songPath, lyricPath, lyricTransPath string
			)

			nameBuilder.WriteString(s.Songname)
			nameBuilder.WriteString(" - ")

			for i, s := range s.Singer {
				nameBuilder.WriteString(s.Name)
				if i < n-1 {
					nameBuilder.WriteString(",")
				}
			}

			songname = strings.ReplaceAll(nameBuilder.String(), "/", " ")
			songname = strings.ReplaceAll(songname, "\\", " ")

			songPath = filepath.Join(sdir, songname)
			lyricPath = filepath.Join(lyricDir, songname+".lrc")
			lyricTransPath = filepath.Join(lyricDir, songname+"_trans.lrc")

			for _, t := range songTypes {
				if _, err := os.Stat(songPath + t.Suffix()); err == nil {
					songExists = true
					dstSongType = t
					songPath += dstSongType.Suffix()
					break
				}
			}

			// 下载歌曲
			if !songExists {
				addr, rt, err := cli.GetSongUrl(s.Songmid, s.StrMediaMid, targetSongType)
				if err != nil {
					log.Errorf("%s 获取下载连接失败: %v\n", songname, err)
					continue
				}
				dstSongType = rt
				songPath += dstSongType.Suffix()

				if res.SongErr = utils.Download(addr, songPath); res.SongErr != nil {
					res.Song = "Download Failed"
					os.Remove(songPath)
				} else {
					songCount++
				}
			} else {
				res.Song = "Already Exists"
			}

			if !noEmbedArt {
				if err := embedAlbumArt(cli, s.Albummid, songPath, false); err != nil {
					if !errors.Is(err, ErrAblumArtAlreadyExists) {
						log.Warnf("%s 封面嵌入失败: %v\n", v.Dissname+"/"+songname+dstSongType.Suffix(), err)
					}
				}
			}

			// 下载歌词
			if _, err := os.Stat(lyricPath); os.IsNotExist(err) {
				lyric, trans, err := cli.GetSongLyric(s.Songmid)
				if err != nil {
					res.Lyric = "Download Failed"
					res.LyricErr = err
				} else {
					// 打开 MP3 文件
					tag, err := id3v2.Open(songPath, id3v2.Options{Parse: true})
					if err != nil {
						log.Errorf("%s 打开 MP3 文件失败: %v", songPath, err)
					} else {
						lyric = bytes.ReplaceAll(lyric, []byte("&apos;"), []byte("'"))
						lyric = bytes.ReplaceAll(lyric, []byte("&quot;"), []byte("\""))
						lyric = bytes.ReplaceAll(lyric, []byte("&nbsp;"), []byte(" "))

						// PERF: 发现qqmusic存在歌词没有 titiel/artist/album tag导致rmpc无法匹配
						lyric = reTi.ReplaceAll(lyric, []byte("[ti:"+tag.Title()+"]"))
						lyric = reAr.ReplaceAll(lyric, []byte("[ar:"+tag.Artist()+"]"))
						lyric = reAl.ReplaceAll(lyric, []byte("[al:"+tag.Album()+"]"))

						if err := os.WriteFile(lyricPath, lyric, os.ModePerm); err != nil {
							res.Lyric = "Write Failed"
							res.LyricErr = err
						} else {
							lyricCount++
						}

						if len(trans) > 0 && lyricTrans {
							trans = bytes.ReplaceAll(trans, []byte("&apos;"), []byte("'"))
							trans = bytes.ReplaceAll(trans, []byte("&quot;"), []byte("\""))
							trans = bytes.ReplaceAll(trans, []byte("&nbsp;"), []byte(" "))

							// PERF: 发现qqmusic存在歌词没有 titiel/artist/album tag导致rmpc无法匹配
							trans = reTi.ReplaceAll(trans, []byte("[ti:"+tag.Title()+"]"))
							trans = reAr.ReplaceAll(trans, []byte("[ar:"+tag.Artist()+"]"))
							trans = reAl.ReplaceAll(trans, []byte("[al:"+tag.Album()+"]"))

							os.WriteFile(lyricTransPath, trans, os.ModePerm)
						}
					}

				}
			} else {
				res.Lyric = "Already Exists"
			}

			var (
				errFlag    = res.SongErr != nil || res.LyricErr != nil
				str1, str2 string
			)

			// Stdout Print
			if res.SongErr != nil {
				str1 = fmt.Sprintf("Song: %s, Error: %v", res.Song, res.SongErr)
			} else {
				m3u[v.Dissname].WriteString(filepath.Join(v.Dissname, songname+dstSongType.Suffix()))
				m3u[v.Dissname].WriteByte('\n')
				str1 = fmt.Sprintf("Song: %s", res.Song)
			}

			if res.LyricErr != nil {
				str2 = fmt.Sprintf("Lyric: %s, Error: %v", res.Lyric, res.LyricErr)
			} else {
				str2 = fmt.Sprintf("Lyric: %s", res.Lyric)
			}

			if !silent {
				if errFlag {
					log.Errorf("%s%s ===> [ %s, %s ]\n", songname, dstSongType.Suffix(), str1, str2)
				} else if res.Song != "Already Exists" || res.Lyric != "Already Exists" {
					log.Infof("%s%s ===> [ %s, %s ]\n", songname, dstSongType.Suffix(), str1, str2)
				}
			}

			if res.Song != "Already Exists" || res.Lyric != "Already Exists" {
				time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
			}
		}
	}
	if genMpdPlaylist {
		if err := os.MkdirAll(mpdPlaylistDir, os.ModePerm); err != nil {
			fmt.Printf("mpdPlaylistDir: %v\n", mpdPlaylistDir)
			log.Errorf("无法创建目录 %s : %v\n", mpdPlaylistDir, err)
			os.Exit(1)
		}
		for name, bs := range m3u {
			if err := os.WriteFile(filepath.Join(mpdPlaylistDir, fmt.Sprintf("%s.m3u", name)), bs.Bytes(), os.ModePerm); err != nil {
				log.Errorf("%s.m3u 写入失败: %v\n", name, err)
			}
		}
	}
	return songCount, lyricCount
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
