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
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
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
	songDir  string
	lyricDir string
	silent   bool
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

	syncCmd.PersistentFlags().StringVar(&songDir, "song-dir", "./songs", "歌曲目录")
	syncCmd.PersistentFlags().StringVar(&lyricDir, "lyric-dir", "./lyrics", "歌词目录")
	syncCmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "是否静默模式")
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
			downloadDiss(*cli, d)
		}
	}
}

func downloadDiss(cli qqmusic.Client, d qqmusic.Diss) (songCount int, lyricCount int) {
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

	for _, v := range l.Cdlist {
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

			if res.SongErr == nil && res.LyricErr == nil && silent {
				continue
			}

			// Stdout Print
			fmt.Print(songname, " [ ")
			if res.SongErr != nil {
				fmt.Printf("Song: %s, Error: %s", log.SError(res.Song), log.SError(res.SongErr.Error()))
			} else {
				fmt.Printf("Song: %s", log.SOK(res.Song))
			}

			fmt.Print(", ")

			if res.LyricErr != nil {
				fmt.Printf("Lyric: %s, Error: %s", log.SError(res.Lyric), log.SError(res.LyricErr.Error()))
			} else {
				fmt.Printf("Lyric: %s", log.SOK(res.Lyric))
			}
			fmt.Println(" ]")

			if res.Song != "Already Exists" || res.Lyric != "Already Exists" {
				time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)
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
