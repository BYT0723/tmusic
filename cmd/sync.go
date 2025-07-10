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
			fmt.Printf("[Error] 无法读取%s: %v\n", cookiePath, err)
			return
		}
		cli, err := qqmusic.NewClient(string(bytes.TrimSpace(cb)))
		cobra.CheckErr(err)

		syncUserSongList(cli, songDir, lyricDir)
	},
}

var (
	songDir  string
	lyricDir string
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
}

func syncUserSongList(cli *qqmusic.Client, songDir string, lyricDir string) {
	if err := os.MkdirAll(songDir, os.ModePerm); err != nil {
		fmt.Println("[Error] 无法创建歌曲目录:", err)
		return
	}
	if err := os.MkdirAll(lyricDir, os.ModePerm); err != nil {
		fmt.Println("[Error] 无法创建歌曲目录:", err)
		return
	}

	sl, err := cli.GetUserSongList()
	if err != nil {
		fmt.Println("[Error] 获取用户歌单失败:", err)
		return
	}

	for _, d := range sl.DissList {
		downloadDiss(*cli, d)
	}
}

func downloadDiss(cli qqmusic.Client, d qqmusic.Diss) {
	l, err := cli.GetSongList(d.Tid)
	if err != nil {
		fmt.Printf("歌单 %s 获取失败: %v\n", d.DissName, err)
		return
	}
	if len(l.Cdlist) == 0 {
		return
	}

	sdir := filepath.Join(songDir, d.DissName)
	if err := os.MkdirAll(sdir, os.ModePerm); err != nil {
		fmt.Printf("歌单 %s 创建失败: %v\n", d.DissName, err)
		return
	}

	for _, v := range l.Cdlist {
		for _, s := range v.Songlist {
			var (
				nameBuilder                         strings.Builder
				n                                   = len(s.Singer)
				st                                  = qqmusic.SongType320
				dsErr, dlErr                        error
				songStatus, lyricStatus             string = "OK", "OK"
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

			songPath = filepath.Join(sdir, nameBuilder.String()+st.Suffix())
			lyricPath = filepath.Join(lyricDir, nameBuilder.String()+".lrc")
			lyricTransPath = filepath.Join(lyricDir, nameBuilder.String()+"_trans.lrc")

			if _, err := os.Stat(songPath); os.IsNotExist(err) {
				addr, err := cli.GetSongUrl(s.Songmid, s.StrMediaMid, st)
				if err != nil {
					fmt.Printf("%s 获取下载连接失败: %v\n", nameBuilder.String(), err)
					continue
				}

				if dsErr = download(addr, songPath); dsErr != nil {
					songStatus = "Download Failed"
				}
			} else {
				songStatus = "Already Exists"
			}

			if _, err := os.Stat(lyricPath); os.IsNotExist(err) {
				lyric, trans, dlErr := cli.GetSongLyric(s.Songmid)
				if dlErr != nil {
					lyricStatus = "Download Failed"
				} else {
					if dlErr = os.WriteFile(lyricPath, lyric, os.ModePerm); dlErr != nil {
						lyricStatus = "Write Failed"
					}
					if len(trans) > 0 {
						os.WriteFile(lyricTransPath, trans, os.ModePerm)
					}
				}
			} else {
				lyricStatus = "Already Exists"
			}

			// Stdout Print
			if dsErr != nil {
				fmt.Println(nameBuilder.String(), "\tSong: ", songStatus, "\tError:", dsErr)
			} else {
				fmt.Println(nameBuilder.String(), "\tSong: ", songStatus)
			}

			if dlErr != nil {
				fmt.Println(nameBuilder.String(), "\tLyric: ", lyricStatus, "\tError:", dlErr)
			} else {
				fmt.Println(nameBuilder.String(), "\tLyric: ", lyricStatus)
			}

			if songStatus != "Already Exists" || lyricStatus != "Already Exists" {
				time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)
			}
		}
	}
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
