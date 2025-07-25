/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BYT0723/apix/qqmusic"
	"github.com/BYT0723/tmusic/utils/log"
	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
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

var (
	songDir, lyricDir, mpdPlaylistDir string
	silent, genMpdPlaylist            bool
)

func init() {
	qqmusicCmd.AddCommand(syncCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// syncCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// syncCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	home, _ := os.UserHomeDir()

	syncCmd.PersistentFlags().StringVar(&songDir, "song", filepath.Join(home, "Music"), "歌曲目录")
	syncCmd.PersistentFlags().StringVar(&lyricDir, "lyric", filepath.Join(home, "Music", ".lyrics"), "歌词目录")
	syncCmd.PersistentFlags().StringVar(&mpdPlaylistDir, "mpd-playlist", filepath.Join(home, ".mpd", "playlists"), "MPD Playlist Directory")
	syncCmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "是否静默模式")
	syncCmd.PersistentFlags().BoolVarP(&genMpdPlaylist, "gen-mpd-playlist", "g", false, "是否生成MPD播放列表")
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
			syncDiss(*cli, d)
		}
	}
}

func syncDiss(cli qqmusic.Client, d qqmusic.Diss) (songCount int, lyricCount int) {
	l, err := cli.GetSongList(d.Tid)
	if err != nil {
		log.Errorf("%s 歌单获取失败: %v\n", d.DissName, err)
		return
	}
	if len(l.Cdlist) == 0 {
		return
	}

	sdir := filepath.Join(songDir, d.DissName)
	if err := os.MkdirAll(sdir, os.ModePerm); err != nil {
		log.Errorf("歌单 %s 创建失败: %v\n", d.DissName, err)
		return
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

				songname string
				n        = len(s.Singer)
				st       = qqmusic.SongType320
				res      = result{
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

			songPath = filepath.Join(sdir, songname+st.Suffix())
			lyricPath = filepath.Join(lyricDir, songname+".lrc")
			lyricTransPath = filepath.Join(lyricDir, songname+"_trans.lrc")

			if _, err := os.Stat(songPath); os.IsNotExist(err) {
				addr, err := cli.GetSongUrl(s.Songmid, s.StrMediaMid, st)
				if err != nil {
					log.Errorf("%s 获取下载连接失败: %v\n", songname, err)
					continue
				}

				if res.SongErr = download(addr, songPath); res.SongErr != nil {
					res.Song = "Download Failed"
					os.Remove(songPath)
				} else {
					songCount++
				}
			} else {
				res.Song = "Already Exists"
			}

			if _, err := os.Stat(lyricPath); os.IsNotExist(err) {
				lyric, trans, err := cli.GetSongLyric(s.Songmid)
				if err != nil {
					res.Lyric = "Download Failed"
					res.LyricErr = err
				} else {
					lyric = bytes.ReplaceAll(lyric, []byte("&apos;"), []byte("'"))
					lyric = bytes.ReplaceAll(lyric, []byte("&quot;"), []byte("\""))
					lyric = bytes.ReplaceAll(lyric, []byte("&nbsp;"), []byte(" "))
					if err := os.WriteFile(lyricPath, lyric, os.ModePerm); err != nil {
						res.Lyric = "Write Failed"
						res.LyricErr = err
					} else {
						lyricCount++
					}
					if len(trans) > 0 {
						trans = bytes.ReplaceAll(trans, []byte("&apos;"), []byte("'"))
						trans = bytes.ReplaceAll(trans, []byte("&quot;"), []byte("\""))
						trans = bytes.ReplaceAll(trans, []byte("&nbsp;"), []byte(" "))
						os.WriteFile(lyricTransPath, trans, os.ModePerm)
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
				m3u[v.Dissname].WriteString(filepath.Join(v.Dissname, songname+st.Suffix()))
				m3u[v.Dissname].WriteByte('\n')
				str1 = fmt.Sprintf("Song: %s", res.Song)
			}

			if res.LyricErr != nil {
				str2 = fmt.Sprintf("Lyric: %s, Error: %v", res.Lyric, res.LyricErr)
			} else {
				str2 = fmt.Sprintf("Lyric: %s", res.Lyric)
			}

			if errFlag {
				log.Errorf("%s ===> [ %s, %s ]\n", songname, str1, str2)
			} else if !silent {
				log.OKf("%s ===> [ %s, %s ]\n", songname, str1, str2)
			}

			if res.Song != "Already Exists" || res.Lyric != "Already Exists" {
				time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)
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
	return
}

func download(url string, filepath string) (err error) {
	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return
	}
	defer f.Close()

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status code error: %d", resp.StatusCode)
		return
	}
	_, err = io.Copy(f, resp.Body)
	return
}
